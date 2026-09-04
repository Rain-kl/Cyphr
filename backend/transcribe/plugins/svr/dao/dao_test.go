// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao_test

import (
	"Wavelet/pkg/idgen"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	_ = idgen.Init(1)

	dbPath := filepath.Join(t.TempDir(), "test_transcribe.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Locate and apply sqlite migration
	migrationPath := filepath.Join("..", "migrations", "sqlite", "00001_init_transcribe.sql")
	content, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "migration file must exist at %s", migrationPath)

	applyMigration(t, db, string(content))

	// v2 targets the admin-owned w_system_configs table (absent in unit tests); apply v3 & v4 model registration only.
	qwenMigrationPath := filepath.Join("..", "migrations", "sqlite", "00003_register_qwen3_asr.sql")
	qwenContent, err := os.ReadFile(qwenMigrationPath)
	require.NoError(t, err, "migration file must exist at %s", qwenMigrationPath)
	applyMigration(t, db, string(qwenContent))

	qwen17MigrationPath := filepath.Join("..", "migrations", "sqlite", "00004_register_qwen3_asr_1_7b.sql")
	qwen17Content, err := os.ReadFile(qwen17MigrationPath)
	require.NoError(t, err, "migration file must exist at %s", qwen17MigrationPath)
	applyMigration(t, db, string(qwen17Content))

	return db, dbPath
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
		require.NoError(t, err, "failed to execute migration statement: %s", trimmed)
	}
}

func TestMigrationSchemaAndSeed(t *testing.T) {
	db, _ := setupTestDB(t)

	// Verify all 4 tables exist in sqlite_master
	for _, tbl := range []string{"t_models", "t_nodes", "t_jobs", "t_job_logs"} {
		var count int64
		err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tbl).Scan(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "table %s should exist", tbl)
	}

	// Verify seed model mock-whisper-base
	var seedModel entity.ModelEntity
	err := db.Where("name = ?", "mock-whisper-base").First(&seedModel).Error
	require.NoError(t, err)
	assert.Equal(t, uint64(1), seedModel.ID)
	assert.Equal(t, "asr", seedModel.TaskType)
	assert.True(t, seedModel.IsActive)
}

func TestModelDAO(t *testing.T) {
	db, _ := setupTestDB(t)
	modelDAO := dao.NewModelDAO(db)
	ctx := context.Background()

	// 1. GetByName: existing seed
	m, err := modelDAO.GetByName(ctx, consts.DefaultModelName)
	require.NoError(t, err)
	assert.Equal(t, consts.DefaultModelName, m.Name)
	assert.True(t, m.IsActive)

	// 2. GetByName: non-existing
	_, err = modelDAO.GetByName(ctx, "non-existent-model")
	assert.ErrorIs(t, err, consts.ErrRecordNotFound)

	// 3. Create: new active model
	newModel := entity.ModelEntity{
		Name:        "whisper-large-v3",
		TaskType:    consts.TaskTypeASR,
		Description: "Whisper Large V3 production model",
		IsActive:    true,
	}
	require.NoError(t, modelDAO.Create(ctx, &newModel))
	assert.NotZero(t, newModel.ID)

	// Create inactive model
	inactiveModel := entity.ModelEntity{
		Name:        "whisper-tiny-deprecated",
		TaskType:    consts.TaskTypeASR,
		Description: "Deprecated model",
		IsActive:    false,
	}
	require.NoError(t, modelDAO.Create(ctx, &inactiveModel))

	// 4. ListActive: without keyword
	activeList, err := modelDAO.ListActive(ctx)
	require.NoError(t, err)
	assert.Len(t, activeList, 4) // mock-whisper-base, qwen3-asr-0.6b, qwen3-asr-1.7b, whisper-large-v3

	// 5. ListActive: with keyword
	keywordList, err := modelDAO.ListActive(ctx, "large")
	require.NoError(t, err)
	assert.Len(t, keywordList, 1)
	assert.Equal(t, "whisper-large-v3", keywordList[0].Name)

	// 6. ListActive: keyword with special SQL LIKE chars
	specialList, err := modelDAO.ListActive(ctx, "%_")
	require.NoError(t, err)
	assert.Empty(t, specialList)
}

func TestNodeDAO(t *testing.T) {
	db, _ := setupTestDB(t)
	nodeDAO := dao.NewNodeDAO(db)
	ctx := context.Background()

	// 1. Create node
	n1 := entity.NodeEntity{
		Name:        "worker-node-1",
		TokenHash:   "hash_alpha_12345",
		TokenPrefix: "agt_alpha",
		IsActive:    true,
	}
	require.NoError(t, nodeDAO.Create(ctx, &n1))
	assert.NotZero(t, n1.ID)

	n2 := entity.NodeEntity{
		Name:        "worker-node-2",
		TokenHash:   "hash_beta_67890",
		TokenPrefix: "agt_beta",
		IsActive:    true,
	}
	require.NoError(t, nodeDAO.Create(ctx, &n2))
	assert.NotZero(t, n2.ID)

	// 2. GetByTokenHash
	found, err := nodeDAO.GetByTokenHash(ctx, "hash_alpha_12345")
	require.NoError(t, err)
	assert.Equal(t, n1.ID, found.ID)
	assert.Equal(t, "worker-node-1", found.Name)

	_, err = nodeDAO.GetByTokenHash(ctx, "hash_unknown")
	assert.ErrorIs(t, err, consts.ErrRecordNotFound)

	// 3. GetByID
	foundByID, err := nodeDAO.GetByID(ctx, n2.ID)
	require.NoError(t, err)
	assert.Equal(t, "worker-node-2", foundByID.Name)

	_, err = nodeDAO.GetByID(ctx, 999999)
	assert.ErrorIs(t, err, consts.ErrRecordNotFound)

	// 4. ListAll
	allNodes, err := nodeDAO.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, allNodes, 2)

	// ListAll with keyword
	filteredNodes, err := nodeDAO.ListAll(ctx, "node-1")
	require.NoError(t, err)
	assert.Len(t, filteredNodes, 1)
	assert.Equal(t, "worker-node-1", filteredNodes[0].Name)

	// 5. UpdateLastSeen
	require.NoError(t, nodeDAO.UpdateLastSeen(ctx, n1.ID, "10.0.0.42"))
	updated, err := nodeDAO.GetByID(ctx, n1.ID)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.42", updated.LastIP)
	assert.NotNil(t, updated.LastSeenAt)
	assert.WithinDuration(t, time.Now(), *updated.LastSeenAt, 2*time.Second)

	// UpdateLastSeen on non-existent node
	err = nodeDAO.UpdateLastSeen(ctx, 888888, "10.0.0.1")
	assert.ErrorIs(t, err, consts.ErrRecordNotFound)
}

func TestJobDAO(t *testing.T) {
	db, _ := setupTestDB(t)
	jobDAO := dao.NewJobDAO(db)
	ctx := context.Background()

	// 1. Create Job
	job1 := entity.JobEntity{
		UserID:           1001,
		ModelName:        consts.DefaultModelName,
		AudioStoragePath: "/storage/audios/audio1.mp3",
		OriginalFileName: "interview_recording.mp3",
	}
	require.NoError(t, jobDAO.Create(ctx, &job1))
	assert.NotZero(t, job1.ID)
	assert.Equal(t, consts.StatusPending, job1.Status)
	assert.Equal(t, consts.TaskTypeASR, job1.TaskType)

	job2 := entity.JobEntity{
		UserID:           1001,
		ModelName:        consts.DefaultModelName,
		AudioStoragePath: "/storage/audios/audio2.mp3",
		OriginalFileName: "meeting_recording.mp3",
	}
	require.NoError(t, jobDAO.Create(ctx, &job2))

	job3 := entity.JobEntity{
		UserID:           1002,
		ModelName:        consts.DefaultModelName,
		AudioStoragePath: "/storage/audios/audio3.mp3",
		OriginalFileName: "lecture.mp3",
	}
	require.NoError(t, jobDAO.Create(ctx, &job3))

	// 2. GetByID
	fetched, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, "interview_recording.mp3", fetched.OriginalFileName)

	_, err = jobDAO.GetByID(ctx, 777777)
	assert.ErrorIs(t, err, consts.ErrRecordNotFound)

	// 3. ListByUserID: pagination and status filter
	jobsUser1, total, err := jobDAO.ListByUserID(ctx, 1001, 1, 10, "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, jobsUser1, 2)

	// ListByUserID with status filter
	jobsPending, totalPending, err := jobDAO.ListByUserID(ctx, 1001, 1, 10, consts.StatusPending)
	require.NoError(t, err)
	assert.Equal(t, int64(2), totalPending)
	assert.Len(t, jobsPending, 2)

	// ListByUserID with keyword filter
	jobsKeyword, totalKeyword, err := jobDAO.ListByUserID(ctx, 1001, 1, 10, "", "meeting")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalKeyword)
	assert.Len(t, jobsKeyword, 1)
	assert.Equal(t, "meeting_recording.mp3", jobsKeyword[0].OriginalFileName)

	// 4. UpdateStatus
	require.NoError(t, jobDAO.UpdateStatus(ctx, job1.ID, consts.StatusRunning))
	runningJob, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusRunning, runningJob.Status)
	assert.NotNil(t, runningJob.StartedAt)

	// 5. AppendLogs & GetLogsByJobID
	logsBatch1 := []entity.JobLogEntity{
		{Progress: 20, Message: "Audio decoded"},
		{Progress: 50, Message: "Transcribing chunk 1"},
	}
	require.NoError(t, jobDAO.AppendLogs(ctx, job1.ID, logsBatch1))

	// Check job progress updated to 50
	progressJob, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, 50, progressJob.Progress)

	// Append second batch of logs
	logsBatch2 := []entity.JobLogEntity{
		{Progress: 80, Message: "Transcribing chunk 2"},
	}
	require.NoError(t, jobDAO.AppendLogs(ctx, job1.ID, logsBatch2))

	// Verify job progress updated to 80
	progressJob80, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, 80, progressJob80.Progress)

	// Append out-of-order log with lower progress, verify progress does NOT regress
	logsBatchLower := []entity.JobLogEntity{
		{Progress: 30, Message: "Delayed log from earlier step"},
	}
	require.NoError(t, jobDAO.AppendLogs(ctx, job1.ID, logsBatchLower))

	progressJobStill80, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, 80, progressJobStill80.Progress)

	allLogs, err := jobDAO.GetLogsByJobID(ctx, job1.ID)
	require.NoError(t, err)
	require.Len(t, allLogs, 4)
	assert.Equal(t, 1, allLogs[0].Seq)
	assert.Equal(t, 2, allLogs[1].Seq)
	assert.Equal(t, 3, allLogs[2].Seq)
	assert.Equal(t, 4, allLogs[3].Seq)
	assert.Equal(t, "Audio decoded", allLogs[0].Message)
	assert.Equal(t, "Transcribing chunk 2", allLogs[2].Message)
	assert.Equal(t, "Delayed log from earlier step", allLogs[3].Message)

	// 6. UpdateCompletion
	resultText := "Hello, this is a test transcription result."
	resultJSON := `{"task":"transcribe","text":"Hello, this is a test transcription result."}`
	require.NoError(t, jobDAO.UpdateCompletion(ctx, job1.ID, consts.StatusCompleted, 4.5, resultText, resultJSON, ""))

	completedJob, err := jobDAO.GetByID(ctx, job1.ID)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusCompleted, completedJob.Status)
	assert.Equal(t, 100, completedJob.Progress)
	assert.Equal(t, 4.5, completedJob.DurationSeconds)
	assert.Equal(t, resultText, completedJob.ResultText)
	assert.Equal(t, resultJSON, completedJob.ResultJSON)
	assert.NotNil(t, completedJob.CompletedAt)

	// 7. ListPendingJobs
	pendingJobs, err := jobDAO.ListPendingJobs(ctx)
	require.NoError(t, err)
	// job1 is completed, job2 and job3 should be pending
	assert.Len(t, pendingJobs, 2)

	// 8. UpdateNodeID
	require.NoError(t, jobDAO.UpdateNodeID(ctx, job2.ID, 5001, consts.StatusRunning))
	dispatchedJob, err := jobDAO.GetByID(ctx, job2.ID)
	require.NoError(t, err)
	require.NotNil(t, dispatchedJob.NodeID)
	assert.Equal(t, uint64(5001), *dispatchedJob.NodeID)
	assert.Equal(t, consts.StatusRunning, dispatchedJob.Status)

	// 9. ResetRunningJobsToPending
	require.NoError(t, jobDAO.ResetRunningJobsToPending(ctx, 5001))
	resetJob, err := jobDAO.GetByID(ctx, job2.ID)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusPending, resetJob.Status)
	assert.Nil(t, resetJob.NodeID)
	assert.Equal(t, 0, resetJob.Progress)
}
