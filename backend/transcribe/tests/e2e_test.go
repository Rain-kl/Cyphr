// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package tests_test

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/controller"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = idgen.Init(1)
}

// ─── Mock AuthService ─────────────────────────────────────────────────────────

type mockAuthService struct {
	userID uint64
}

func newMockAuthService(userID uint64) *mockAuthService {
	return &mockAuthService{userID: userID}
}

func (m *mockAuthService) RequireAuthMiddleware() any {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Set(contracts.AuthUserIDKey, m.userID)
		c.Next()
	})
}

func (m *mockAuthService) RequireAdminMiddleware() any {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Set(contracts.AuthUserIDKey, m.userID)
		c.Next()
	})
}

func (m *mockAuthService) DisallowTokenAuthMiddleware() any {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Next()
	})
}

func (m *mockAuthService) GetCurrentUser(_ context.Context) (*contracts.UserDTO, error) {
	return &contracts.UserDTO{ID: m.userID, Username: "e2e-user", IsActive: true}, nil
}

func (m *mockAuthService) GetCurrentUserID(_ context.Context) (uint64, error) {
	return m.userID, nil
}

func (m *mockAuthService) VerifyToken(_ context.Context, _ string) (*contracts.UserDTO, error) {
	return &contracts.UserDTO{ID: m.userID, Username: "e2e-user", IsActive: true}, nil
}

func (m *mockAuthService) CreateSession(_ context.Context, _ uint64, _ map[string]any) (string, error) {
	return "test_session_token", nil
}

func (m *mockAuthService) RevokeToken(_ context.Context, _ string) error        { return nil }
func (m *mockAuthService) RevokeUserSessions(_ context.Context, _ uint64) error { return nil }
func (m *mockAuthService) InvalidateCachedUser(_ context.Context, _ uint64)     {}
func (m *mockAuthService) InvalidateCachedToken(_ context.Context, _ string)    {}
func (m *mockAuthService) ListAuthSources(_ context.Context) ([]contracts.AuthSourceViewDTO, error) {
	return nil, nil
}
func (m *mockAuthService) CreateAuthSource(_ context.Context, _ contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}
func (m *mockAuthService) UpdateAuthSource(_ context.Context, _ uint64, _ contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}
func (m *mockAuthService) DeleteAuthSource(_ context.Context, _ uint64) error { return nil }
func (m *mockAuthService) ToggleAuthSource(_ context.Context, _ uint64) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}

// ─── Test Environment Setup ───────────────────────────────────────────────────

type e2eEnv struct {
	db        *gorm.DB
	server    *httptest.Server
	jobDAO    dao.JobDAO
	nodeDAO   dao.NodeDAO
	modelDAO  dao.ModelDAO
	hub       hub.AgentHub
	broker    service.LogBroker
	sched     scheduler.Scheduler
	nodeSvc   service.NodeService
	jobSvc    service.JobService
	ctrl      *controller.Controller
	cliClient *client.Client
	authSvc   *mockAuthService
	mediaDir  string
}

func setupE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	// 1. Create SQLite DB and run Goose migrations
	dbPath := filepath.Join(t.TempDir(), "transcribe_e2e.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	applyGooseMigrations(t, db)

	// 2. Initialize DAOs, Services, Hub, and Scheduler
	jobDAO := dao.NewJobDAO(db)
	nodeDAO := dao.NewNodeDAO(db)
	modelDAO := dao.NewModelDAO(db)

	broker := service.NewLogBroker()
	agentHub := hub.NewAgentHub(jobDAO)
	sched := scheduler.NewScheduler(jobDAO, agentHub)
	nodeSvc := service.NewNodeService(nodeDAO, agentHub)
	jobSvc := service.NewJobService(jobDAO, modelDAO, sched, broker, agentHub)

	authSvc := newMockAuthService(1001)
	mediaDir := filepath.Join(t.TempDir(), "transcribe_media")

	ctrl := controller.New(
		jobSvc,
		nodeSvc,
		modelDAO,
		agentHub,
		broker,
		controller.WithSyncTimeout(5*time.Second),
		controller.WithScheduler(sched),
	)
	ctrl.SetAuthService(authSvc)
	// Reconfigure localDir on OpenAIHandler
	ctrl.OpenAI = controller.NewOpenAIHandler(
		jobSvc,
		broker,
		controller.WithLocalDir(mediaDir),
		controller.WithAuthService(authSvc),
		controller.WithOpenAISyncTimeout(5*time.Second),
	)

	// 3. Register routes via router extension
	routerExt := extpoints.NewRouterRegistry()
	ctrl.RegisterRoutes(routerExt)

	engine := gin.New()
	engine.Use(gin.Recovery(), response.ErrorHandlerMiddleware())

	for _, rd := range routerExt.Routes() {
		allHandlers := make([]gin.HandlerFunc, 0, len(rd.Middlewares)+len(rd.Handlers))
		for _, mw := range rd.Middlewares {
			allHandlers = append(allHandlers, toTestGinHandler(mw))
		}
		for _, h := range rd.Handlers {
			allHandlers = append(allHandlers, toTestGinHandler(h))
		}
		engine.Handle(rd.Method, rd.Path, allHandlers...)
	}

	server := httptest.NewServer(engine)
	cliClient := client.New(server.URL, "mock-e2e-user-token")

	return &e2eEnv{
		db:        db,
		server:    server,
		jobDAO:    jobDAO,
		nodeDAO:   nodeDAO,
		modelDAO:  modelDAO,
		hub:       agentHub,
		broker:    broker,
		sched:     sched,
		nodeSvc:   nodeSvc,
		jobSvc:    jobSvc,
		ctrl:      ctrl,
		cliClient: cliClient,
		authSvc:   authSvc,
		mediaDir:  mediaDir,
	}
}

func applyGooseMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)

	migrationDir := filepath.Join("..", "plugins", "svr", "migrations", "sqlite")
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, os.DirFS(migrationDir))
	require.NoError(t, err, "goose.NewProvider should succeed")

	results, err := provider.Up(context.Background())
	require.NoError(t, err, "goose provider Up should apply migrations")
	require.NotEmpty(t, results, "expected migration results from goose Up")
}

func toTestGinHandler(h any) gin.HandlerFunc {
	switch fn := h.(type) {
	case gin.HandlerFunc:
		return fn
	case func(*gin.Context):
		return gin.HandlerFunc(fn)
	default:
		panic(fmt.Sprintf("unsupported handler type: %T", h))
	}
}

func createMultipartAudio(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)

	err = writer.WriteField("model", consts.DefaultModelName)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

// ─── End-to-End Tests ─────────────────────────────────────────────────────────

// TestE2E_FullAsyncPipeline tests the full end-to-end transcription flow:
// 1. Controller boots with SQLite and Goose migrations.
// 2. Controller creates node via HTTP and yields node token.
// 3. WebSocket client connects using node token and sends heartbeat advertising loaded model.
// 4. CLI client submits transcription request with audio data (X-Async: true).
// 5. Agent receives dispatch_job command over WebSocket.
// 6. Agent downloads audio file from /api/v1/agent/jobs/:id/media.
// 7. Agent sends batch progress logs to /api/v1/agent/jobs/:id/logs.
// 8. Agent sends completion payload to /api/v1/agent/jobs/:id/complete.
// 9. SSE stream client receives real-time progress logs and terminal finish event.
// 10. CLI client queries GET /api/v1/jobs and GET /api/v1/jobs/:id verifying final state.
func TestE2E_FullAsyncPipeline(t *testing.T) {
	env := setupE2EEnv(t)
	defer env.server.Close()
	ctx := context.Background()

	// ─── Step 1: Create Node via HTTP ───────────────────────────────────────────
	createNodePayload := `{"name": "e2e-worker-node-1"}`
	createNodeReq, err := http.NewRequest(
		http.MethodPost,
		env.server.URL+"/api/v1/controller/nodes",
		strings.NewReader(createNodePayload),
	)
	require.NoError(t, err)
	createNodeReq.Header.Set("Content-Type", "application/json")
	createNodeReq.Header.Set("Authorization", "Bearer mock-e2e-user-token")

	createNodeResp, err := env.server.Client().Do(createNodeReq)
	require.NoError(t, err)
	defer func() { _ = createNodeResp.Body.Close() }()
	require.Equal(t, http.StatusOK, createNodeResp.StatusCode)

	var nodeCreated response.Response[do.NodeCreatedDTO]
	err = json.NewDecoder(createNodeResp.Body).Decode(&nodeCreated)
	require.NoError(t, err)
	nodeID := nodeCreated.Data.ID
	agentToken := nodeCreated.Data.AgentToken
	require.NotZero(t, nodeID)
	require.NotEmpty(t, agentToken)
	require.True(t, strings.HasPrefix(agentToken, consts.AgentTokenPrefix))

	// ─── Step 2: Connect WebSocket Agent ────────────────────────────────────────
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/api/v1/agent/ws?token=" + agentToken
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	wsConn, wsHTTPResp, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()
	if wsHTTPResp != nil && wsHTTPResp.Body != nil {
		_ = wsHTTPResp.Body.Close()
	}

	// Send heartbeat advertising loaded model
	hbMsg := do.WSMessage{
		Type: "heartbeat",
		Payload: map[string]any{
			"loaded_models": []string{consts.DefaultModelName},
			"running_jobs":  0,
			"system": map[string]any{
				"cpu_percent": 15.0,
				"ram_percent": 30.0,
			},
		},
	}
	err = wsConn.WriteJSON(hbMsg)
	require.NoError(t, err)

	// Verify session registered in AgentHub
	require.Eventually(t, func() bool {
		sess, ok := env.hub.GetSession(nodeID)
		return ok && len(sess.GetLoadedModels()) > 0 && sess.GetLoadedModels()[0] == consts.DefaultModelName
	}, 2*time.Second, 20*time.Millisecond, "agent session must be registered and active with loaded model")

	// ─── Step 3: CLI Submits Transcription (X-Async: true) ──────────────────────
	testAudioData := []byte("RIFF mock wav binary sound data for e2e test pipeline 1234567890")
	audioFilePath := filepath.Join(t.TempDir(), "sample_audio.wav")
	require.NoError(t, os.WriteFile(audioFilePath, testAudioData, 0o600))

	submitResp, err := env.cliClient.SubmitTranscription(ctx, client.TranscriptionRequest{
		FilePath:         audioFilePath,
		OriginalFileName: "sample_audio.wav",
		Model:            consts.DefaultModelName,
		Language:         "en",
		Prompt:           "Transcribe speech accurately",
	})
	require.NoError(t, err)
	require.NotNil(t, submitResp)
	require.NotZero(t, submitResp.JobID)
	require.Equal(t, consts.StatusPending, submitResp.Status)
	jobID := submitResp.JobID

	// ─── Step 4: Concurrently Start SSE Stream Listener ─────────────────────────
	var (
		logMu      sync.Mutex
		streamLogs []client.LogEvent
		streamEnd  client.FinishEvent
		streamErr  error
		streamDone = make(chan struct{})
	)

	go func() {
		defer close(streamDone)
		streamErr = env.cliClient.StreamJobLogs(ctx, jobID, func(le client.LogEvent) {
			logMu.Lock()
			streamLogs = append(streamLogs, le)
			logMu.Unlock()
		}, func(fe client.FinishEvent) {
			streamEnd = fe
		})
	}()

	// ─── Step 5: Agent Receives dispatch_job over WebSocket ─────────────────────
	_ = wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	msgType, msgBytes, err := wsConn.ReadMessage()
	require.NoError(t, err, "agent should receive dispatch command over WebSocket")
	assert.Equal(t, websocket.TextMessage, msgType)

	type wsDispatchMsg struct {
		Type    string                `json:"type"`
		Action  string                `json:"action,omitempty"`
		Seq     int64                 `json:"seq,omitempty"`
		Payload do.DispatchJobPayload `json:"payload,omitempty"`
	}

	var wsCmd wsDispatchMsg
	err = json.Unmarshal(msgBytes, &wsCmd)
	require.NoError(t, err)
	assert.Equal(t, "command", wsCmd.Type)
	assert.Equal(t, "dispatch_job", wsCmd.Action)
	assert.Equal(t, jobID, wsCmd.Payload.JobID)
	assert.Equal(t, consts.DefaultModelName, wsCmd.Payload.ModelName)
	assert.Equal(t, fmt.Sprintf("/api/v1/agent/jobs/%d/media", jobID), wsCmd.Payload.MediaPath)
	dispatchPayload := wsCmd.Payload

	// ─── Step 6: Agent Downloads Audio Media ────────────────────────────────────
	mediaURL := env.server.URL + dispatchPayload.MediaPath
	mediaReq, err := http.NewRequest(http.MethodGet, mediaURL, nil)
	require.NoError(t, err)
	mediaReq.Header.Set("Authorization", "Bearer "+agentToken)

	mediaResp, err := env.server.Client().Do(mediaReq)
	require.NoError(t, err)
	defer func() { _ = mediaResp.Body.Close() }()
	require.Equal(t, http.StatusOK, mediaResp.StatusCode)

	downloadedAudio, err := io.ReadAll(mediaResp.Body)
	require.NoError(t, err)
	assert.Equal(t, testAudioData, downloadedAudio, "downloaded audio content must exactly match uploaded file")

	// ─── Step 7: Agent Sends Progress Logs ──────────────────────────────────────
	batchLogsPayload := `{
		"progress": 50,
		"logs": [
			{"seq": 1, "progress": 25, "message": "Pre-processing audio waveforms"},
			{"seq": 2, "progress": 50, "message": "Transcribing speech segment 00:00 - 00:04"}
		]
	}`
	logsReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/api/v1/agent/jobs/%d/logs", env.server.URL, jobID),
		strings.NewReader(batchLogsPayload),
	)
	require.NoError(t, err)
	logsReq.Header.Set("Content-Type", "application/json")
	logsReq.Header.Set("Authorization", "Bearer "+agentToken)

	logsResp, err := env.server.Client().Do(logsReq)
	require.NoError(t, err)
	defer func() { _ = logsResp.Body.Close() }()
	require.Equal(t, http.StatusOK, logsResp.StatusCode)

	// ─── Step 8: Agent Sends Job Completion ─────────────────────────────────────
	expectedResultText := "Hello world, this is a fully verified end-to-end transcription pipeline."
	completePayload := fmt.Sprintf(`{
		"status": "completed",
		"duration_seconds": 4.15,
		"result_text": %q,
		"openai_response": {
			"task": "transcribe",
			"language": "en",
			"duration": 4.15,
			"text": %q,
			"segments": [
				{"id": 0, "start": 0.0, "end": 4.15, "text": %q}
			]
		}
	}`, expectedResultText, expectedResultText, expectedResultText)

	compReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/api/v1/agent/jobs/%d/complete", env.server.URL, jobID),
		strings.NewReader(completePayload),
	)
	require.NoError(t, err)
	compReq.Header.Set("Content-Type", "application/json")
	compReq.Header.Set("Authorization", "Bearer "+agentToken)

	compResp, err := env.server.Client().Do(compReq)
	require.NoError(t, err)
	defer func() { _ = compResp.Body.Close() }()
	require.Equal(t, http.StatusOK, compResp.StatusCode)

	// ─── Step 9: Verify SSE Stream Received Logs & Terminal Finish ──────────────
	select {
	case <-streamDone:
		require.NoError(t, streamErr, "SSE streaming should exit without error upon finish event")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SSE stream to complete")
	}

	logMu.Lock()
	require.Len(t, streamLogs, 2, "SSE stream must receive both dispatched log events")
	assert.Equal(t, 1, streamLogs[0].Seq)
	assert.Equal(t, "Pre-processing audio waveforms", streamLogs[0].Message)
	assert.Equal(t, 2, streamLogs[1].Seq)
	assert.Equal(t, "Transcribing speech segment 00:00 - 00:04", streamLogs[1].Message)
	logMu.Unlock()

	assert.Equal(t, consts.StatusCompleted, streamEnd.Status)
	assert.Equal(t, expectedResultText, streamEnd.ResultText)
	assert.InDelta(t, 4.15, streamEnd.Duration, 0.01)

	// ─── Step 10: Verify Jobs Listing & Detail State via CLI Client ─────────────
	// 10.1 List Jobs
	jobList, err := env.cliClient.ListJobs(ctx, 1, 10, "")
	require.NoError(t, err)
	require.NotNil(t, jobList)
	assert.GreaterOrEqual(t, jobList.Total, int64(1))

	var foundJob *client.JobInfo
	for i := range jobList.Items {
		if jobList.Items[i].ID == jobID {
			foundJob = &jobList.Items[i]
			break
		}
	}
	require.NotNil(t, foundJob, "submitted job must be returned in user job list")
	assert.Equal(t, consts.StatusCompleted, foundJob.Status)
	assert.Equal(t, "sample_audio.wav", foundJob.OriginalFileName)
	assert.Equal(t, expectedResultText, foundJob.ResultText)

	// 10.2 Job Detail
	jobDetail, err := env.cliClient.GetJob(ctx, jobID)
	require.NoError(t, err)
	require.NotNil(t, jobDetail)
	assert.Equal(t, jobID, jobDetail.ID)
	assert.Equal(t, consts.StatusCompleted, jobDetail.Status)
	assert.Equal(t, expectedResultText, jobDetail.ResultText)
	assert.InDelta(t, 4.15, jobDetail.Duration, 0.01)
	require.NotNil(t, jobDetail.NodeID)
	assert.Equal(t, nodeID, *jobDetail.NodeID)
	assert.NotNil(t, jobDetail.CompletedAt)
}

// TestE2E_SynchronousTranscriptionPipeline tests the synchronous blocking mode
// of /v1/audio/transcriptions where the caller blocks waiting for background agent completion.
func TestE2E_SynchronousTranscriptionPipeline(t *testing.T) {
	env := setupE2EEnv(t)
	defer env.server.Close()

	// 1. Setup active worker node
	node, agentToken, err := env.nodeSvc.CreateNode(context.Background(), "sync-e2e-node")
	require.NoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/api/v1/agent/ws?token=" + agentToken
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	wsConn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()

	_ = wsConn.WriteJSON(do.WSMessage{
		Type: "heartbeat",
		Payload: map[string]any{
			"loaded_models": []string{consts.DefaultModelName},
			"running_jobs":  0,
		},
	})

	require.Eventually(t, func() bool {
		sess, ok := env.hub.GetSession(node.ID)
		return ok && len(sess.GetLoadedModels()) > 0
	}, 2*time.Second, 20*time.Millisecond)

	// 2. Background simulated agent to fulfill dispatched jobs
	expectedSyncText := "Synchronous transcription result content."
	stopAgent := make(chan struct{})
	defer close(stopAgent)

	go func() {
		for {
			_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, rawBytes, err := wsConn.ReadMessage()
			if err != nil {
				select {
				case <-stopAgent:
					return
				default:
					continue
				}
			}

			type wsDispatchMsg struct {
				Type    string                `json:"type"`
				Action  string                `json:"action,omitempty"`
				Seq     int64                 `json:"seq,omitempty"`
				Payload do.DispatchJobPayload `json:"payload,omitempty"`
			}

			var msg wsDispatchMsg
			if err := json.Unmarshal(rawBytes, &msg); err != nil {
				continue
			}

			if msg.Type == "command" && msg.Action == "dispatch_job" {
				dp := msg.Payload

				// Immediately complete job
				compPayload := fmt.Sprintf(`{
					"status": "completed",
					"duration_seconds": 1.88,
					"result_text": %q,
					"openai_response": {
						"text": %q
					}
				}`, expectedSyncText, expectedSyncText)

				compReq, _ := http.NewRequest(
					http.MethodPost,
					fmt.Sprintf("%s/api/v1/agent/jobs/%d/complete", env.server.URL, dp.JobID),
					strings.NewReader(compPayload),
				)
				compReq.Header.Set("Content-Type", "application/json")
				compReq.Header.Set("Authorization", "Bearer "+agentToken)
				compResp, cErr := env.server.Client().Do(compReq)
				if cErr == nil {
					_ = compResp.Body.Close()
				}
				return
			}
		}
	}()

	// 3. Synchronous POST /v1/audio/transcriptions (no X-Async header)
	bodyBuf, contentType := createMultipartAudio(t, "sync_test.wav", []byte("audio payload"))

	syncReq, err := http.NewRequest(http.MethodPost, env.server.URL+"/v1/audio/transcriptions", bodyBuf)
	require.NoError(t, err)
	syncReq.Header.Set("Content-Type", contentType)
	syncReq.Header.Set("Authorization", "Bearer mock-e2e-user-token")

	syncResp, err := env.server.Client().Do(syncReq)
	require.NoError(t, err)
	defer func() { _ = syncResp.Body.Close() }()

	require.Equal(t, http.StatusOK, syncResp.StatusCode)
	respBytes, err := io.ReadAll(syncResp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(respBytes), expectedSyncText)
}

// TestE2E_JobStreamingHistorical tests that connecting to an already-completed job's SSE stream
// immediately replays all historical logs and emits the finish event.
func TestE2E_JobStreamingHistorical(t *testing.T) {
	env := setupE2EEnv(t)
	defer env.server.Close()
	ctx := context.Background()

	// 1. Create completed job with logs in DB
	job, err := env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           1001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: filepath.Join(t.TempDir(), "fake.wav"),
		OriginalFileName: "historical.wav",
	})
	require.NoError(t, err)

	err = env.jobSvc.AppendLogs(ctx, job.ID, &do.AgentLogBatchRequest{
		Progress: 60,
		Logs: []do.AgentLogBatchItem{
			{Message: "Chunk 1 processed"},
			{Message: "Chunk 2 processed"},
		},
	})
	require.NoError(t, err)

	err = env.jobSvc.CompleteJob(ctx, job.ID, &do.AgentCompleteRequest{
		Status:          consts.StatusCompleted,
		DurationSeconds: 2.1,
		ResultText:      "Replayed historical transcription",
	})
	require.NoError(t, err)

	// 2. Stream logs via CLI client
	var (
		logs   []client.LogEvent
		finish client.FinishEvent
	)
	err = env.cliClient.StreamJobLogs(ctx, job.ID, func(le client.LogEvent) {
		logs = append(logs, le)
	}, func(fe client.FinishEvent) {
		finish = fe
	})
	require.NoError(t, err)

	require.Len(t, logs, 2)
	assert.Equal(t, 1, logs[0].Seq)
	assert.Equal(t, "Chunk 1 processed", logs[0].Message)
	assert.Equal(t, 2, logs[1].Seq)
	assert.Equal(t, "Chunk 2 processed", logs[1].Message)
	assert.Equal(t, consts.StatusCompleted, finish.Status)
	assert.Equal(t, "Replayed historical transcription", finish.ResultText)
}

// TestE2E_NodeManagementAndCommand tests listing nodes and issuing load/unload model commands
// over the WebSocket session.
func TestE2E_NodeManagementAndCommand(t *testing.T) {
	env := setupE2EEnv(t)
	defer env.server.Close()
	ctx := context.Background()

	// 1. Create node
	node, token, err := env.nodeSvc.CreateNode(ctx, "cmd-node-1")
	require.NoError(t, err)

	// 2. Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/api/v1/agent/ws?token=" + token
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	wsConn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()

	_ = wsConn.WriteJSON(do.WSMessage{
		Type: "heartbeat",
		Payload: map[string]any{
			"loaded_models": []string{consts.DefaultModelName},
			"running_jobs":  0,
		},
	})

	require.Eventually(t, func() bool {
		sess, ok := env.hub.GetSession(node.ID)
		return ok && len(sess.GetLoadedModels()) > 0
	}, 2*time.Second, 20*time.Millisecond)

	// 3. Issue load-model command via HTTP
	loadPayload := `{"model_name": "whisper-large-v3"}`
	loadReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/api/v1/controller/nodes/%d/load-model", env.server.URL, node.ID),
		strings.NewReader(loadPayload),
	)
	require.NoError(t, err)
	loadReq.Header.Set("Content-Type", "application/json")
	loadReq.Header.Set("Authorization", "Bearer mock-e2e-user-token")

	loadResp, err := env.server.Client().Do(loadReq)
	require.NoError(t, err)
	defer func() { _ = loadResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, loadResp.StatusCode)

	// Agent receives load_model command
	_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, loadCmdBytes, err := wsConn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(loadCmdBytes), "load_model")
	assert.Contains(t, string(loadCmdBytes), "whisper-large-v3")

	// 4. Issue unload-model command via HTTP
	unloadPayload := `{"model_name": "mock-whisper-base"}`
	unloadReq, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/api/v1/controller/nodes/%d/unload-model", env.server.URL, node.ID),
		strings.NewReader(unloadPayload),
	)
	require.NoError(t, err)
	unloadReq.Header.Set("Content-Type", "application/json")
	unloadReq.Header.Set("Authorization", "Bearer mock-e2e-user-token")

	unloadResp, err := env.server.Client().Do(unloadReq)
	require.NoError(t, err)
	defer func() { _ = unloadResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, unloadResp.StatusCode)

	// Agent receives unload_model command
	_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, unloadCmdBytes, err := wsConn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(unloadCmdBytes), "unload_model")
	assert.Contains(t, string(unloadCmdBytes), "mock-whisper-base")
}

// TestE2E_ModelListing verifies the public models endpoint lists active seeded models.
func TestE2E_ModelListing(t *testing.T) {
	env := setupE2EEnv(t)
	defer env.server.Close()
	ctx := context.Background()

	models, err := env.cliClient.ListModels(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, models)

	var foundDefault bool
	for _, m := range models {
		if m.Name == consts.DefaultModelName {
			foundDefault = true
			assert.True(t, m.IsActive)
			assert.Equal(t, consts.TaskTypeASR, m.TaskType)
		}
	}
	assert.True(t, foundDefault, "mock-whisper-base seeded model must be present in models list")
}
