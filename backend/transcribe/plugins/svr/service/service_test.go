// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"Wavelet/pkg/idgen"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// mockWSConn captures JSON messages written to a session.
type mockWSConn struct {
	mu       sync.Mutex
	messages []any
	closed   bool
}

func newMockWSConn() *mockWSConn {
	return &mockWSConn{messages: make([]any, 0)}
}

func (m *mockWSConn) WriteJSON(v any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("connection closed")
	}
	m.messages = append(m.messages, v)
	return nil
}

func (m *mockWSConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWSConn) getMessages() []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]any, len(m.messages))
	copy(res, m.messages)
	return res
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = idgen.Init(1)

	dbPath := filepath.Join(t.TempDir(), "test_service.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	migrationPath := filepath.Join("..", "migrations", "sqlite", "00001_init_transcribe.sql")
	content, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "migration file must exist at %s", migrationPath)

	applyMigration(t, db, string(content))
	return db
}

func applyMigration(t *testing.T, db *gorm.DB, sqlContent string) {
	t.Helper()
	upIdx := strings.Index(sqlContent, "-- +goose Up")
	downIdx := strings.Index(sqlContent, "-- +goose Down")
	upSection := sqlContent
	if upIdx != -1 {
		if downIdx != -1 {
			upSection = sqlContent[upIdx+len("-- +goose Up") : downIdx]
		} else {
			upSection = sqlContent[upIdx+len("-- +goose Up"):]
		}
	}

	upSection = strings.ReplaceAll(upSection, "-- +goose StatementBegin", "")
	upSection = strings.ReplaceAll(upSection, "-- +goose StatementEnd", "")

	stmts := strings.Split(upSection, ";")
	for _, stmt := range stmts {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		err := db.Exec(trimmed).Error
		require.NoError(t, err, "failed executing statement: %s", trimmed)
	}
}

// 1. LogBroker tests
func TestLogBroker(t *testing.T) {
	broker := service.NewLogBroker(10)

	t.Run("subscribe and receive logs", func(t *testing.T) {
		jobID := uint64(101)
		ch, cancel := broker.Subscribe(jobID)
		defer cancel()

		broker.Publish(jobID, do.LogMessage{Seq: 1, Progress: 20, Message: "loading model"})

		select {
		case msg := <-ch:
			assert.Equal(t, 1, msg.Seq)
			assert.Equal(t, 20, msg.Progress)
			assert.Equal(t, "loading model", msg.Message)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for log message")
		}
	})

	t.Run("slow subscriber non-blocking drop", func(t *testing.T) {
		tinyBroker := service.NewLogBroker(2)
		jobID := uint64(102)
		ch, cancel := tinyBroker.Subscribe(jobID)
		defer cancel()

		// Publish 5 messages to a buffer of size 2
		for i := 1; i <= 5; i++ {
			tinyBroker.Publish(jobID, do.LogMessage{Seq: i, Message: "msg"})
		}

		// Read the 2 buffered messages
		msg1 := <-ch
		assert.Equal(t, 1, msg1.Seq)
		msg2 := <-ch
		assert.Equal(t, 2, msg2.Seq)

		// Channel should now be empty (subsequent messages dropped)
		select {
		case m := <-ch:
			t.Fatalf("unexpected message in buffer: %v", m)
		default:
			// Buffer properly dropped overflow without blocking
		}
	})

	t.Run("cancellation stops delivery and is idempotent", func(t *testing.T) {
		jobID := uint64(103)
		ch, cancel := broker.Subscribe(jobID)

		cancel()
		cancel() // Idempotent check

		broker.Publish(jobID, do.LogMessage{Seq: 1, Message: "after cancel"})
		select {
		case m := <-ch:
			t.Fatalf("received message after cancellation: %v", m)
		default:
			// Expected
		}
	})

	t.Run("publish and subscribe finish", func(t *testing.T) {
		jobID := uint64(104)
		finishCh, cancel := broker.SubscribeFinish(jobID)
		defer cancel()

		broker.PublishFinish(jobID, do.FinishMessage{
			Status:     consts.StatusCompleted,
			Duration:   5.4,
			ResultText: "Transcription output",
		})

		select {
		case finish := <-finishCh:
			assert.Equal(t, consts.StatusCompleted, finish.Status)
			assert.Equal(t, 5.4, finish.Duration)
			assert.Equal(t, "Transcription output", finish.ResultText)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for finish message")
		}

		broker.CloseJob(jobID)
	})
}

// 2. Hub Session & AgentHub tests
func TestAgentSessionAndHub(t *testing.T) {
	db := setupTestDB(t)
	jobDAO := dao.NewJobDAO(db)
	nodeDAO := dao.NewNodeDAO(db)
	agentHub := hub.NewAgentHub(jobDAO)
	ctx := context.Background()

	t.Run("session state mutations", func(t *testing.T) {
		mockConn := newMockWSConn()
		sess := hub.NewAgentSession(1, "node-1", "127.0.0.1", mockConn)

		assert.Equal(t, uint64(1), sess.NodeID)
		assert.Equal(t, "node-1", sess.NodeName)
		assert.False(t, sess.HasModel("mock-whisper-base"))
		assert.Equal(t, 0, sess.GetRunningJobs())

		sess.UpdateHeartbeat([]string{"mock-whisper-base"}, 2, &do.SystemStatsDTO{CPUPercent: 25.5})
		assert.True(t, sess.HasModel("mock-whisper-base"))
		assert.Equal(t, 2, sess.GetRunningJobs())
		assert.Equal(t, 25.5, sess.GetSystemStats().CPUPercent)

		sess.IncrementRunningJobs()
		assert.Equal(t, 3, sess.GetRunningJobs())
		sess.DecrementRunningJobs()
		assert.Equal(t, 2, sess.GetRunningJobs())

		err := sess.WriteJSON(map[string]string{"foo": "bar"})
		require.NoError(t, err)
		assert.Len(t, mockConn.getMessages(), 1)
	})

	t.Run("agent hub session registration and broadcasting", func(t *testing.T) {
		mockConn1 := newMockWSConn()
		sess1 := hub.NewAgentSession(10, "node-10", "127.0.0.1", mockConn1)
		agentHub.RegisterSession(sess1)

		retrieved, ok := agentHub.GetSession(10)
		require.True(t, ok)
		assert.Equal(t, "node-10", retrieved.NodeName)

		active := agentHub.ListActiveSessions()
		assert.Len(t, active, 1)

		// Broadcast to online node
		err := agentHub.BroadcastToNode(10, map[string]string{"test": "hello"})
		require.NoError(t, err)
		assert.Len(t, mockConn1.getMessages(), 1)

		// Broadcast to offline node
		err = agentHub.BroadcastToNode(999, map[string]string{"test": "offline"})
		require.Error(t, err)
		assert.Equal(t, consts.ErrNodeOffline, err.Error())

		// Re-register node 10 closes previous connection cleanly
		mockConn2 := newMockWSConn()
		sess2 := hub.NewAgentSession(10, "node-10-new", "127.0.0.2", mockConn2)
		agentHub.RegisterSession(sess2)
		assert.True(t, mockConn1.closed)

		// Unregister node
		agentHub.UnregisterSession(10)
		assert.True(t, mockConn2.closed)
		_, ok = agentHub.GetSession(10)
		assert.False(t, ok)
	})

	t.Run("watchdog detects timeout and resets running jobs", func(t *testing.T) {
		// Create node entity in DB
		node := &entity.NodeEntity{
			Name:        "watchdog-test-node",
			TokenHash:   "test_hash_watchdog",
			TokenPrefix: "agt_watchdog",
			IsActive:    true,
		}
		require.NoError(t, nodeDAO.Create(ctx, node))

		// Create a job running on this node
		job := &entity.JobEntity{
			UserID:           1,
			ModelName:        "mock-whisper-base",
			TaskType:         consts.TaskTypeASR,
			AudioStoragePath: "/tmp/watchdog.mp3",
			OriginalFileName: "watchdog.mp3",
			Status:           consts.StatusRunning,
			NodeID:           &node.ID,
			Progress:         45,
		}
		require.NoError(t, jobDAO.Create(ctx, job))

		mockConn := newMockWSConn()
		sess := hub.NewAgentSession(node.ID, node.Name, "127.0.0.1", mockConn)
		// Set heartbeat in the past to trigger timeout
		sess.SetLastHeartbeat(time.Now().Add(-30 * time.Second))
		agentHub.RegisterSession(sess)

		disconnectedNode := make(chan uint64, 1)
		agentHub.OnNodeDisconnected(func(id uint64) {
			disconnectedNode <- id
		})

		// Start watchdog with fast check interval (10ms) and 100ms timeout
		watchdogCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		agentHub.StartWatchdog(watchdogCtx, 10*time.Millisecond, 100*time.Millisecond)
		defer agentHub.Stop()

		select {
		case id := <-disconnectedNode:
			assert.Equal(t, node.ID, id)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for watchdog disconnect")
		}

		// Verify session was unregistered
		_, ok := agentHub.GetSession(node.ID)
		assert.False(t, ok)

		// Verify running job was reset back to pending with 0 progress
		updatedJob, err := jobDAO.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusPending, updatedJob.Status)
		assert.Nil(t, updatedJob.NodeID)
		assert.Equal(t, 0, updatedJob.Progress)
	})
}

func TestScheduler(t *testing.T) {
	ctx := context.Background()

	t.Run("dispatches to node with loaded model and lowest jobs", func(t *testing.T) {
		db := setupTestDB(t)
		jobDAO := dao.NewJobDAO(db)
		agentHub := hub.NewAgentHub(jobDAO)
		sched := scheduler.NewScheduler(jobDAO, agentHub)

		// Node 1: loaded model, 2 running jobs
		conn1 := newMockWSConn()
		sess1 := hub.NewAgentSession(101, "node-101", "10.0.0.1", conn1)
		sess1.UpdateHeartbeat([]string{"mock-whisper-base"}, 2, &do.SystemStatsDTO{CPUPercent: 40})
		agentHub.RegisterSession(sess1)

		// Node 2: loaded model, 0 running jobs (preferred!)
		conn2 := newMockWSConn()
		sess2 := hub.NewAgentSession(102, "node-102", "10.0.0.2", conn2)
		sess2.UpdateHeartbeat([]string{"mock-whisper-base"}, 0, &do.SystemStatsDTO{CPUPercent: 20})
		agentHub.RegisterSession(sess2)

		// Create pending job
		job := &entity.JobEntity{
			UserID:           1,
			ModelName:        "mock-whisper-base",
			TaskType:         consts.TaskTypeASR,
			AudioStoragePath: "/storage/audio1.mp3",
			OriginalFileName: "audio1.mp3",
			Status:           consts.StatusPending,
		}
		require.NoError(t, jobDAO.Create(ctx, job))

		err := sched.SchedulePendingJobs(ctx)
		require.NoError(t, err)

		// Verify dispatched to node 102
		updatedJob, err := jobDAO.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusRunning, updatedJob.Status)
		require.NotNil(t, updatedJob.NodeID)
		assert.Equal(t, uint64(102), *updatedJob.NodeID)

		// Node 102 received dispatch_job WS command
		msgs := conn2.getMessages()
		require.Len(t, msgs, 1)
		wsMsg, ok := msgs[0].(do.WSMessage)
		require.True(t, ok)
		assert.Equal(t, "command", wsMsg.Type)
		assert.Equal(t, "dispatch_job", wsMsg.Action)
		payload, ok := wsMsg.Payload.(do.DispatchJobPayload)
		require.True(t, ok)
		assert.Equal(t, job.ID, payload.JobID)
		assert.Equal(t, "mock-whisper-base", payload.ModelName)

		// Node 101 received no messages
		assert.Empty(t, conn1.getMessages())
	})

	t.Run("sends load_model command when model not yet loaded on idle node", func(t *testing.T) {
		db := setupTestDB(t)
		jobDAO := dao.NewJobDAO(db)
		agentHub := hub.NewAgentHub(jobDAO)
		sched := scheduler.NewScheduler(jobDAO, agentHub)

		// Node 3: online, but has no models loaded
		conn3 := newMockWSConn()
		sess3 := hub.NewAgentSession(103, "node-103", "10.0.0.3", conn3)
		sess3.UpdateHeartbeat([]string{}, 0, &do.SystemStatsDTO{CPUPercent: 10})
		agentHub.RegisterSession(sess3)

		// Create pending job for a different model
		job := &entity.JobEntity{
			UserID:           1,
			ModelName:        "whisper-large-v3",
			TaskType:         consts.TaskTypeASR,
			AudioStoragePath: "/storage/large.mp3",
			OriginalFileName: "large.mp3",
			Status:           consts.StatusPending,
		}
		require.NoError(t, jobDAO.Create(ctx, job))

		err := sched.SchedulePendingJobs(ctx)
		require.NoError(t, err)

		// Job remains pending
		updatedJob, err := jobDAO.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusPending, updatedJob.Status)

		// Node 3 received load_model command
		msgs := conn3.getMessages()
		require.Len(t, msgs, 1)
		wsMsg, ok := msgs[0].(do.WSMessage)
		require.True(t, ok)
		assert.Equal(t, "command", wsMsg.Type)
		assert.Equal(t, "load_model", wsMsg.Action)
		payload, ok := wsMsg.Payload.(do.LoadModelPayload)
		require.True(t, ok)
		assert.Equal(t, "whisper-large-v3", payload.ModelName)
	})

	t.Run("jobs remain pending when no nodes online", func(t *testing.T) {
		db := setupTestDB(t)
		jobDAO := dao.NewJobDAO(db)
		agentHub := hub.NewAgentHub(jobDAO)
		sched := scheduler.NewScheduler(jobDAO, agentHub)

		job := &entity.JobEntity{
			UserID:           1,
			ModelName:        "mock-whisper-base",
			TaskType:         consts.TaskTypeASR,
			AudioStoragePath: "/storage/no_node.mp3",
			OriginalFileName: "no_node.mp3",
			Status:           consts.StatusPending,
		}
		require.NoError(t, jobDAO.Create(ctx, job))

		err := sched.SchedulePendingJobs(ctx)
		require.NoError(t, err)

		updatedJob, err := jobDAO.GetByID(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusPending, updatedJob.Status)
	})
}

// 4. NodeService tests
func TestNodeService(t *testing.T) {
	db := setupTestDB(t)
	nodeDAO := dao.NewNodeDAO(db)
	jobDAO := dao.NewJobDAO(db)
	agentHub := hub.NewAgentHub(jobDAO)
	nodeSvc := service.NewNodeService(nodeDAO, agentHub)
	ctx := context.Background()

	t.Run("create node and verify token", func(t *testing.T) {
		nodeDTO, rawToken, err := nodeSvc.CreateNode(ctx, "gpu-worker-1")
		require.NoError(t, err)
		assert.NotEmpty(t, nodeDTO.ID)
		assert.Equal(t, "gpu-worker-1", nodeDTO.Name)
		assert.True(t, nodeDTO.IsActive)
		assert.True(t, strings.HasPrefix(rawToken, consts.AgentTokenPrefix))
		assert.Equal(t, 36, len(rawToken))
		assert.Equal(t, rawToken[:12], nodeDTO.TokenPrefix)

		// Verify with valid raw token
		verified, err := nodeSvc.VerifyNodeToken(ctx, rawToken)
		require.NoError(t, err)
		assert.Equal(t, nodeDTO.ID, verified.ID)
		assert.Equal(t, "gpu-worker-1", verified.Name)

		// Verify with invalid token fails
		_, err = nodeSvc.VerifyNodeToken(ctx, "agt_invalidtoken1234567890abcdef")
		require.Error(t, err)
		assert.Equal(t, consts.ErrInvalidToken, err.Error())

		// Verify with empty token fails
		_, err = nodeSvc.VerifyNodeToken(ctx, "")
		require.Error(t, err)
		assert.Equal(t, consts.ErrInvalidToken, err.Error())
	})

	t.Run("list nodes decorated with live hub session info", func(t *testing.T) {
		nodeDTO, _, err := nodeSvc.CreateNode(ctx, "gpu-worker-live")
		require.NoError(t, err)

		// Register live session in AgentHub
		sess := hub.NewAgentSession(nodeDTO.ID, nodeDTO.Name, "192.168.1.100", newMockWSConn())
		sess.UpdateHeartbeat([]string{"mock-whisper-base"}, 1, &do.SystemStatsDTO{CPUPercent: 32.0})
		agentHub.RegisterSession(sess)

		nodes, err := nodeSvc.ListNodes(ctx)
		require.NoError(t, err)

		var found *do.NodeDTO
		for i := range nodes {
			if nodes[i].ID == nodeDTO.ID {
				found = &nodes[i]
				break
			}
		}
		require.NotNil(t, found)
		assert.True(t, found.IsOnline)
		assert.Equal(t, 1, found.RunningJobs)
		assert.Contains(t, found.LoadedModels, "mock-whisper-base")
		assert.Equal(t, 32.0, found.System.CPUPercent)
	})

	t.Run("update last seen", func(t *testing.T) {
		nodeDTO, _, err := nodeSvc.CreateNode(ctx, "gpu-worker-seen")
		require.NoError(t, err)

		err = nodeSvc.UpdateLastSeen(ctx, nodeDTO.ID, "10.0.0.99")
		require.NoError(t, err)

		retrieved, err := nodeSvc.GetNode(ctx, nodeDTO.ID)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.99", retrieved.LastIP)
		assert.NotNil(t, retrieved.LastSeenAt)
	})
}

// 5. JobService tests
func TestJobService(t *testing.T) {
	db := setupTestDB(t)
	jobDAO := dao.NewJobDAO(db)
	modelDAO := dao.NewModelDAO(db)
	agentHub := hub.NewAgentHub(jobDAO)
	sched := scheduler.NewScheduler(jobDAO, agentHub)
	broker := service.NewLogBroker()
	jobSvc := service.NewJobService(jobDAO, modelDAO, sched, broker, agentHub)
	ctx := context.Background()

	t.Run("create and get job detail", func(t *testing.T) {
		req := &do.CreateJobRequest{
			UserID:           1001,
			Model:            "mock-whisper-base",
			TaskType:         consts.TaskTypeASR,
			AudioStoragePath: "/storage/test_speech.mp3",
			OriginalFileName: "speech.mp3",
		}
		created, err := jobSvc.CreateJob(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, created.ID)
		assert.Equal(t, consts.StatusPending, created.Status)
		assert.Equal(t, "mock-whisper-base", created.Model)
		assert.Equal(t, "speech.mp3", created.OriginalFileName)

		detail, err := jobSvc.GetJobDetail(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, detail.ID)
		assert.Equal(t, uint64(1001), detail.UserID)

		// Non-existent job
		_, err = jobSvc.GetJobDetail(ctx, 99999999)
		require.Error(t, err)
		assert.ErrorIs(t, err, consts.ErrJobNotFound)

		// Inactive model rejection
		inactiveModel := &entity.ModelEntity{
			Name:     "inactive-test-model",
			TaskType: consts.TaskTypeASR,
			IsActive: false,
		}
		require.NoError(t, modelDAO.Create(ctx, inactiveModel))
		_, err = jobSvc.CreateJob(ctx, &do.CreateJobRequest{
			UserID:           1001,
			Model:            "inactive-test-model",
			AudioStoragePath: "/storage/inactive.mp3",
			OriginalFileName: "inactive.mp3",
		})
		require.Error(t, err)
		assert.Equal(t, consts.ErrModelUnavailable, err.Error())
	})

	t.Run("list jobs with pagination", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			_, err := jobSvc.CreateJob(ctx, &do.CreateJobRequest{
				UserID:           2002,
				Model:            "mock-whisper-base",
				AudioStoragePath: "/storage/batch.mp3",
				OriginalFileName: "batch.mp3",
			})
			require.NoError(t, err)
		}

		list, err := jobSvc.ListJobs(ctx, 2002, 1, 3)
		require.NoError(t, err)
		assert.Equal(t, int64(5), list.Total)
		assert.Len(t, list.Items, 3)
		assert.Equal(t, 1, list.Page)
		assert.Equal(t, 3, list.PageSize)
	})

	t.Run("append logs and broadcast to broker", func(t *testing.T) {
		job, err := jobSvc.CreateJob(ctx, &do.CreateJobRequest{
			UserID:           3003,
			Model:            "mock-whisper-base",
			AudioStoragePath: "/storage/logs.mp3",
			OriginalFileName: "logs.mp3",
		})
		require.NoError(t, err)

		logCh, cancel := broker.Subscribe(job.ID)
		defer cancel()

		err = jobSvc.AppendLogs(ctx, job.ID, &do.AgentLogBatchRequest{
			Progress: 40,
			Logs: []do.AgentLogBatchItem{
				{Message: "Decoding segment 1"},
				{Message: "Decoding segment 2"},
			},
		})
		require.NoError(t, err)

		// Verify SSE broker received the messages
		select {
		case msg := <-logCh:
			assert.Equal(t, 40, msg.Progress)
			assert.Equal(t, "Decoding segment 1", msg.Message)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for log 1")
		}

		select {
		case msg := <-logCh:
			assert.Equal(t, 40, msg.Progress)
			assert.Equal(t, "Decoding segment 2", msg.Message)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for log 2")
		}

		// Verify job progress updated in DB
		detail, err := jobSvc.GetJobDetail(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, 40, detail.Progress)
	})

	t.Run("complete job with openai response and finish broadcast", func(t *testing.T) {
		job, err := jobSvc.CreateJob(ctx, &do.CreateJobRequest{
			UserID:           4004,
			Model:            "mock-whisper-base",
			AudioStoragePath: "/storage/done.mp3",
			OriginalFileName: "done.mp3",
		})
		require.NoError(t, err)

		// Assign running job to session
		sess := hub.NewAgentSession(404, "node-404", "127.0.0.1", newMockWSConn())
		sess.IncrementRunningJobs()
		agentHub.RegisterSession(sess)
		require.NoError(t, jobDAO.UpdateNodeID(ctx, job.ID, 404, consts.StatusRunning))

		finishCh, cancel := broker.SubscribeFinish(job.ID)
		defer cancel()

		err = jobSvc.CompleteJob(ctx, job.ID, &do.AgentCompleteRequest{
			Status:          consts.StatusCompleted,
			DurationSeconds: 12.3,
			ResultText:      "Test audio recognized text",
			OpenAIResponse: map[string]any{
				"text": "Test audio recognized text",
			},
		})
		require.NoError(t, err)

		// Verify finish broadcast
		select {
		case finish := <-finishCh:
			assert.Equal(t, consts.StatusCompleted, finish.Status)
			assert.Equal(t, 12.3, finish.Duration)
			assert.Equal(t, "Test audio recognized text", finish.ResultText)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for finish broadcast")
		}

		detail, err := jobSvc.GetJobDetail(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusCompleted, detail.Status)
		assert.Equal(t, 100, detail.Progress)
		assert.Equal(t, 12.3, detail.Duration)
		assert.NotNil(t, detail.CompletedAt)
		assert.NotNil(t, detail.OpenAIResponse)

		// Verify session running jobs decremented
		assert.Equal(t, 0, sess.GetRunningJobs())

		// Verify idempotency: completing an already completed job safely returns nil without double-decrementing
		err = jobSvc.CompleteJob(ctx, job.ID, &do.AgentCompleteRequest{
			Status:          consts.StatusCompleted,
			DurationSeconds: 99.9,
			ResultText:      "duplicate completion",
		})
		require.NoError(t, err)
		assert.Equal(t, 0, sess.GetRunningJobs())
	})

	t.Run("cancel active job", func(t *testing.T) {
		job, err := jobSvc.CreateJob(ctx, &do.CreateJobRequest{
			UserID:           5005,
			Model:            "mock-whisper-base",
			AudioStoragePath: "/storage/cancel.mp3",
			OriginalFileName: "cancel.mp3",
		})
		require.NoError(t, err)

		// Assign running job to session
		sess := hub.NewAgentSession(505, "node-505", "127.0.0.1", newMockWSConn())
		sess.IncrementRunningJobs()
		agentHub.RegisterSession(sess)
		require.NoError(t, jobDAO.UpdateNodeID(ctx, job.ID, 505, consts.StatusRunning))

		finishCh, cancel := broker.SubscribeFinish(job.ID)
		defer cancel()

		err = jobSvc.CancelJob(ctx, job.ID)
		require.NoError(t, err)

		select {
		case finish := <-finishCh:
			assert.Equal(t, consts.StatusFailed, finish.Status)
			assert.Equal(t, "cancelled by user", finish.ErrorMsg)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for cancel finish event")
		}

		detail, err := jobSvc.GetJobDetail(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusFailed, detail.Status)

		// Verify session running jobs decremented
		assert.Equal(t, 0, sess.GetRunningJobs())

		// Cancelling an already failed job errors with ErrInvalidStatus
		err = jobSvc.CancelJob(ctx, job.ID)
		require.Error(t, err)
		assert.Equal(t, consts.ErrInvalidStatus, err.Error())

		// Completing an already cancelled (failed) job safely returns nil idempotently
		err = jobSvc.CompleteJob(ctx, job.ID, &do.AgentCompleteRequest{
			Status:          consts.StatusCompleted,
			DurationSeconds: 10.0,
			ResultText:      "late result",
		})
		require.NoError(t, err)

		detail, err = jobSvc.GetJobDetail(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusFailed, detail.Status)
	})
}
