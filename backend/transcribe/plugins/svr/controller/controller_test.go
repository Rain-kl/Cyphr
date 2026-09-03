// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/controller"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type testEnv struct {
	db        *gorm.DB
	jobDAO    dao.JobDAO
	nodeDAO   dao.NodeDAO
	modelDAO  dao.ModelDAO
	hub       hub.AgentHub
	broker    service.LogBroker
	sched     scheduler.Scheduler
	nodeSvc   service.NodeService
	jobSvc    service.JobService
	ctrl      *controller.Controller
	engine    *gin.Engine
	routerExt extpoints.RouterExtension
}

func setupTestEnv(t *testing.T, customUserAuthMW ...gin.HandlerFunc) *testEnv {
	t.Helper()
	_ = idgen.Init(1)

	dbPath := filepath.Join(t.TempDir(), "test_controller.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	migrationPath := filepath.Join("..", "migrations", "sqlite", "00001_init_transcribe.sql")
	content, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "migration file must exist at %s", migrationPath)
	applyMigration(t, db, string(content))

	jobDAO := dao.NewJobDAO(db)
	nodeDAO := dao.NewNodeDAO(db)
	modelDAO := dao.NewModelDAO(db)

	broker := service.NewLogBroker()
	agentHub := hub.NewAgentHub(jobDAO)
	sched := scheduler.NewScheduler(jobDAO, agentHub)
	nodeSvc := service.NewNodeService(nodeDAO, agentHub)
	jobSvc := service.NewJobService(jobDAO, modelDAO, sched, broker, agentHub)

	ctrl := controller.New(
		jobSvc,
		nodeSvc,
		modelDAO,
		agentHub,
		broker,
		controller.WithSyncTimeout(3*time.Second),
		controller.WithScheduler(sched),
	)

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

	return &testEnv{
		db:        db,
		jobDAO:    jobDAO,
		nodeDAO:   nodeDAO,
		modelDAO:  modelDAO,
		hub:       agentHub,
		broker:    broker,
		sched:     sched,
		nodeSvc:   nodeSvc,
		jobSvc:    jobSvc,
		ctrl:      ctrl,
		engine:    engine,
		routerExt: routerExt,
	}
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

	for _, stmt := range strings.Split(upSection, ";") {
		trimmed := strings.TrimSpace(stmt)
		if trimmed != "" {
			err := db.Exec(trimmed).Error
			require.NoError(t, err, "failed executing statement: %s", trimmed)
		}
	}
}

// ─── Middleware Tests ─────────────────────────────────────────────────────────

func TestRequireAgentTokenMiddleware(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, token, err := env.nodeSvc.CreateNode(ctx, "worker-node-1")
	require.NoError(t, err)

	// Create an inactive node
	inactiveNode, inactiveToken, err := env.nodeSvc.CreateNode(ctx, "inactive-node")
	require.NoError(t, err)
	require.NoError(t, env.db.Model(&entity.NodeEntity{}).Where("id = ?", inactiveNode.ID).Update("is_active", false).Error)

	r := gin.New()
	r.Use(response.ErrorHandlerMiddleware())
	r.GET("/protected", controller.RequireAgentToken(env.nodeSvc), func(c *gin.Context) {
		extractedNode, ok := controller.GetNodeFromContext(c)
		nodeID, hasID := controller.GetNodeIDFromContext(c)
		if !ok || !hasID {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "node not in context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":   nodeID,
			"name": extractedNode.Name,
		})
	})

	t.Run("missing token -> 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token -> 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected?token=agt_invalid12345678", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("inactive node token -> 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected?token="+inactiveToken, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("valid token via query -> 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected?token="+token, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), node.Name)
	})

	t.Run("valid token via Bearer header -> 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), node.Name)
	})

	t.Run("dynamic resolution via getter -> 200 after binding", func(t *testing.T) {
		var dynamicSvc service.NodeService
		getter := func() service.NodeService { return dynamicSvc }

		dynR := gin.New()
		dynR.Use(response.ErrorHandlerMiddleware())
		dynR.GET("/dyn-protected", controller.RequireAgentToken(getter), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// Before service is set -> 500 errInternal
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/dyn-protected?token="+token, nil)
		dynR.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// After service is bound -> 200 OK
		dynamicSvc = env.nodeSvc
		w = httptest.NewRecorder()
		req, _ = http.NewRequest(http.MethodGet, "/dyn-protected?token="+token, nil)
		dynR.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// ─── OpenAI Transcription Handler Tests ───────────────────────────────────────

func TestOpenAIHandler_AsyncAndSync(t *testing.T) {
	env := setupTestEnv(t)

	t.Run("missing file -> 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("model", consts.DefaultModelName)
		_ = writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		env.engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid model -> 400 model unavailable", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.mp3")
		_, _ = part.Write([]byte("fake audio content"))
		_ = writer.WriteField("model", "nonexistent-model-xyz")
		_ = writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		env.engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("async mode returns job_id and pending status", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "speech.wav")
		_, _ = part.Write([]byte("riff wave dummy audio"))
		_ = writer.WriteField("model", consts.DefaultModelName)
		_ = writer.WriteField("language", "zh")
		_ = writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Async", "true")
		env.engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[gin.H]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "pending", resp.Data["status"])
		assert.NotNil(t, resp.Data["job_id"])
	})

	t.Run("sync mode blocks and returns openai json on completion", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "speech_sync.wav")
		_, _ = part.Write([]byte("dummy audio bytes for sync"))
		_ = writer.WriteField("model", consts.DefaultModelName)
		_ = writer.WriteField("response_format", "json")
		_ = writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Goroutine simulating worker completion
		go func() {
			for i := 0; i < 50; i++ {
				time.Sleep(10 * time.Millisecond)
				jobs, _ := env.jobDAO.ListPendingJobs(context.Background())
				for _, j := range jobs {
					if j.OriginalFileName == "speech_sync.wav" {
						_ = env.jobSvc.CompleteJob(context.Background(), j.ID, &do.AgentCompleteRequest{
							Status:          consts.StatusCompleted,
							DurationSeconds: 2.5,
							ResultText:      "Hello from simulated transcription",
						})
						return
					}
				}
			}
		}()

		env.engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var openAIResp do.OpenAITranscriptionResponse
		err := json.Unmarshal(w.Body.Bytes(), &openAIResp)
		require.NoError(t, err)
		assert.Equal(t, "Hello from simulated transcription", openAIResp.Text)
	})

	t.Run("sync mode with verbose_json response format", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "speech_verbose.wav")
		_, _ = part.Write([]byte("dummy audio bytes for verbose"))
		_ = writer.WriteField("model", consts.DefaultModelName)
		_ = writer.WriteField("response_format", "verbose_json")
		_ = writer.Close()

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/audio/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		go func() {
			for i := 0; i < 50; i++ {
				time.Sleep(10 * time.Millisecond)
				jobs, _ := env.jobDAO.ListPendingJobs(context.Background())
				for _, j := range jobs {
					if j.OriginalFileName == "speech_verbose.wav" {
						_ = env.jobSvc.CompleteJob(context.Background(), j.ID, &do.AgentCompleteRequest{
							Status:          consts.StatusCompleted,
							DurationSeconds: 3.2,
							ResultText:      "Verbose speech transcription",
							OpenAIResponse: map[string]any{
								"task":     "transcribe",
								"language": "en",
								"duration": 3.2,
								"text":     "Verbose speech transcription",
								"segments": []map[string]any{
									{"id": 0, "start": 0.0, "end": 3.2, "text": "Verbose speech transcription"},
								},
							},
						})
						return
					}
				}
			}
		}()

		env.engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Verbose speech transcription")
		assert.Contains(t, w.Body.String(), "segments")
	})
}

// ─── Model Handler Tests ──────────────────────────────────────────────────────

func TestModelHandler(t *testing.T) {
	env := setupTestEnv(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/models", nil)
	env.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp response.Response[[]do.ModelDTO]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Data)
	assert.Equal(t, consts.DefaultModelName, resp.Data[0].Name)

	t.Run("GET /api/v1/controller/models lists all models", func(t *testing.T) {
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodGet, "/api/v1/controller/models", nil)
		engine := gin.New()
		engine.GET("/api/v1/controller/models", env.ctrl.Model.ListAllModels)
		engine.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)
		var resp2 response.Response[[]do.ModelDTO]
		err2 := json.Unmarshal(w2.Body.Bytes(), &resp2)
		require.NoError(t, err2)
		require.NotEmpty(t, resp2.Data)
	})

	t.Run("PUT /api/v1/controller/models/:id/status toggles model active state", func(t *testing.T) {
		modelID := resp.Data[0].ID
		engine := gin.New()
		engine.PUT("/api/v1/controller/models/:id/status", env.ctrl.Model.ToggleModelStatus)

		// Disable model
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/controller/models/%d/status", modelID), strings.NewReader(`{"is_active": false}`))
		req3.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusOK, w3.Code)

		// Check status in DB
		m, err := env.modelDAO.GetByName(context.Background(), resp.Data[0].Name)
		require.NoError(t, err)
		assert.False(t, m.IsActive)

		// Re-enable model
		w4 := httptest.NewRecorder()
		req4, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/controller/models/%d/status", modelID), strings.NewReader(`{"is_active": true}`))
		req4.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w4, req4)
		assert.Equal(t, http.StatusOK, w4.Code)
	})
}

// ─── Job Handler & SSE Streaming Tests ────────────────────────────────────────

func TestJobHandler_ListAndDetail(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// Seed jobs
	job1, err := env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           1001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: "/tmp/fake1.mp3",
		OriginalFileName: "file1.mp3",
	})
	require.NoError(t, err)

	_, err = env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           1001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: "/tmp/fake2.mp3",
		OriginalFileName: "file2.mp3",
	})
	require.NoError(t, err)

	t.Run("GET /api/v1/jobs lists user jobs", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs?page=1&page_size=10", nil)

		// Mock user context
		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.Use(func(c *gin.Context) {
			c.Set(contracts.AuthUserIDKey, uint64(1001))
			c.Next()
		})
		engine.GET("/api/v1/jobs", env.ctrl.Job.ListJobs)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[do.JobListDTO]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, int64(2), resp.Data.Total)
		assert.Len(t, resp.Data.Items, 2)
	})

	t.Run("GET /api/v1/jobs/:id returns job detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%d", job1.ID), nil)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.GET("/api/v1/jobs/:id", env.ctrl.Job.GetJob)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[do.JobDTO]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, job1.ID, resp.Data.ID)
		assert.Equal(t, "file1.mp3", resp.Data.OriginalFileName)
	})

	t.Run("GET /api/v1/jobs/:id not found -> 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/jobs/999999999", nil)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.GET("/api/v1/jobs/:id", env.ctrl.Job.GetJob)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GET /api/v1/jobs/:id forbidden for different user -> 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%d", job1.ID), nil)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.Use(func(c *gin.Context) {
			c.Set(contracts.AuthUserIDKey, uint64(9999)) // Different non-admin user
			c.Next()
		})
		engine.GET("/api/v1/jobs/:id", env.ctrl.Job.GetJob)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), consts.ErrForbidden)
	})

	t.Run("GET /api/v1/controller/jobs lists jobs across all users for admin", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/controller/jobs?page=1&page_size=10", nil)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.GET("/api/v1/controller/jobs", env.ctrl.Job.ListAllJobs)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[do.JobListDTO]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Data.Total >= 2)
	})
}

func TestJobHandler_SSEStream(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	job, err := env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           2001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: "/tmp/stream_test.mp3",
		OriginalFileName: "stream_test.mp3",
	})
	require.NoError(t, err)

	// Append initial log
	_ = env.jobSvc.AppendLogs(ctx, job.ID, &do.AgentLogBatchRequest{
		Progress: 10,
		Logs: []do.AgentLogBatchItem{
			{Message: "Audio loaded"},
		},
	})

	t.Run("stream receives historical logs and live finish event", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, _ := gin.CreateTestContext(w)
			c.Request = r
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", job.ID)}}
			env.ctrl.Job.StreamJob(c)
		}))
		defer server.Close()

		// Publish completion after short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = env.jobSvc.CompleteJob(ctx, job.ID, &do.AgentCompleteRequest{
				Status:          consts.StatusCompleted,
				DurationSeconds: 1.5,
				ResultText:      "Streaming complete text",
			})
		}()

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(server.URL)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

		reader := bufio.NewReader(resp.Body)
		var bodyBuilder strings.Builder
		for {
			line, rErr := reader.ReadString('\n')
			bodyBuilder.WriteString(line)
			if strings.Contains(line, "Streaming complete text") || rErr != nil {
				break
			}
		}

		fullOutput := bodyBuilder.String()
		assert.Contains(t, fullOutput, "Audio loaded")
		assert.Contains(t, fullOutput, "Streaming complete text")
		assert.Contains(t, fullOutput, "event: finish")
	})

	t.Run("stream already completed job flushes history and emits finish immediately", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%d/stream", job.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", job.ID)}}

		env.ctrl.Job.StreamJob(c)

		output := w.Body.String()
		assert.Contains(t, output, "Audio loaded")
		assert.Contains(t, output, "event: finish")
		assert.Contains(t, output, "completed")
	})
}

// ─── Controller Node Handler Tests ────────────────────────────────────────────

func TestControllerNodeHandler(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	t.Run("create node returns token and prefix", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := `{"name": "gpu-node-asia-1"}`
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/controller/nodes", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		engine := gin.New()
		engine.POST("/api/v1/controller/nodes", env.ctrl.Node.CreateNode)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[do.NodeCreatedDTO]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "gpu-node-asia-1", resp.Data.Name)
		assert.True(t, strings.HasPrefix(resp.Data.AgentToken, consts.AgentTokenPrefix))
		assert.Equal(t, resp.Data.AgentToken[:12], resp.Data.TokenPrefix)
	})

	t.Run("list nodes", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/controller/nodes", nil)

		engine := gin.New()
		engine.GET("/api/v1/controller/nodes", env.ctrl.Node.ListNodes)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp response.Response[[]do.NodeDTO]
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Data)
	})

	t.Run("load/unload model commands", func(t *testing.T) {
		node, _, err := env.nodeSvc.CreateNode(ctx, "load-unload-node")
		require.NoError(t, err)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.POST("/api/v1/controller/nodes/:id/load-model", env.ctrl.Node.LoadModel)
		engine.POST("/api/v1/controller/nodes/:id/unload-model", env.ctrl.Node.UnloadModel)

		// 1. Offline node returns 400
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/controller/nodes/%d/load-model", node.ID), strings.NewReader(`{"model_name": "mock-whisper-base"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// 2. Online node session receives load command
		mockConn := newTestWSConn()
		sess := hub.NewAgentSession(node.ID, node.Name, "127.0.0.1", mockConn)
		env.hub.RegisterSession(sess)
		defer env.hub.UnregisterSession(node.ID)

		w = httptest.NewRecorder()
		req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/controller/nodes/%d/load-model", node.ID), strings.NewReader(`{"model_name": "mock-whisper-base"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		require.Len(t, mockConn.getSentMessages(), 1)

		// 3. Online node session receives unload command
		w = httptest.NewRecorder()
		req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/controller/nodes/%d/unload-model", node.ID), strings.NewReader(`{"model_name": "mock-whisper-base"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		require.Len(t, mockConn.getSentMessages(), 2)
	})

	t.Run("get node details and delete node", func(t *testing.T) {
		node, _, err := env.nodeSvc.CreateNode(ctx, "temp-delete-node")
		require.NoError(t, err)

		engine := gin.New()
		engine.Use(response.ErrorHandlerMiddleware())
		engine.GET("/api/v1/controller/nodes/:id", env.ctrl.Node.GetNode)
		engine.DELETE("/api/v1/controller/nodes/:id", env.ctrl.Node.DeleteNode)

		// 1. GET node details -> 200
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/controller/nodes/%d", node.ID), nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var getResp response.Response[do.NodeDTO]
		err = json.Unmarshal(w.Body.Bytes(), &getResp)
		require.NoError(t, err)
		assert.Equal(t, "temp-delete-node", getResp.Data.Name)

		// 2. DELETE node -> 200
		w = httptest.NewRecorder()
		req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/controller/nodes/%d", node.ID), nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// 3. GET deleted node -> 404
		w = httptest.NewRecorder()
		req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/controller/nodes/%d", node.ID), nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// ─── Agent Handler HTTP & WebSocket Tests ────────────────────────────────────

func TestAgentHandler_HTTP(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// Create temp audio file for media download test
	tmpFile, err := os.CreateTemp("", "test_audio_*.mp3")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_, _ = tmpFile.WriteString("fake audio binary data content")
	_ = tmpFile.Close()

	job, err := env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           3001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: tmpFile.Name(),
		OriginalFileName: "sample.mp3",
	})
	require.NoError(t, err)

	engine := gin.New()
	engine.Use(response.ErrorHandlerMiddleware())
	engine.GET("/api/v1/agent/jobs/:id/media", env.ctrl.Agent.DownloadMedia)
	engine.POST("/api/v1/agent/jobs/:id/logs", env.ctrl.Agent.AppendLogs)
	engine.POST("/api/v1/agent/jobs/:id/complete", env.ctrl.Agent.CompleteJob)

	t.Run("download media serves file content", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/agent/jobs/%d/media", job.ID), nil)
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "fake audio binary data content", w.Body.String())
	})

	t.Run("append logs records batch logs into DB and broker", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := `{
			"progress": 50,
			"logs": [
				{"message": "Transcribing chunk 1"},
				{"message": "Transcribing chunk 2"}
			]
		}`
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/agent/jobs/%d/logs", job.ID), strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		logs, err := env.jobSvc.GetJobLogs(ctx, job.ID)
		require.NoError(t, err)
		assert.Len(t, logs, 2)
	})

	t.Run("complete job records final completion", func(t *testing.T) {
		w := httptest.NewRecorder()
		payload := `{
			"status": "completed",
			"duration_seconds": 4.5,
			"result_text": "Completed transcription output",
			"openai_response": {
				"text": "Completed transcription output"
			}
		}`
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/agent/jobs/%d/complete", job.ID), strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		detail, err := env.jobSvc.GetJobDetail(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusCompleted, detail.Status)
		assert.Equal(t, "Completed transcription output", detail.ResultText)
	})
}

func TestAgentHandler_WebSocket(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	node, token, err := env.nodeSvc.CreateNode(ctx, "ws-test-node")
	require.NoError(t, err)

	// Spin up real HTTP test server for WebSocket upgrade
	server := httptest.NewServer(env.engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/agent/ws?token=" + token

	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	wsConn, httpResp, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()
	defer func() {
		if httpResp != nil && httpResp.Body != nil {
			_ = httpResp.Body.Close()
		}
	}()

	// Verify session registered in hub
	sess, ok := env.hub.GetSession(node.ID)
	require.True(t, ok, "session must be active in hub")
	assert.Equal(t, node.ID, sess.NodeID)

	// Send heartbeat message
	hbMsg := do.WSMessage{
		Type: "heartbeat",
		Payload: map[string]any{
			"loaded_models": []string{"mock-whisper-base"},
			"running_jobs":  2,
			"system": map[string]any{
				"cpu_percent": 30.5,
				"ram_percent": 45.0,
			},
		},
	}
	err = wsConn.WriteJSON(hbMsg)
	require.NoError(t, err)

	// Allow reader loop to process
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, []string{"mock-whisper-base"}, sess.GetLoadedModels())
	assert.Equal(t, 2, sess.GetRunningJobs())
	stats := sess.GetSystemStats()
	require.NotNil(t, stats)
	assert.Equal(t, 30.5, stats.CPUPercent)

	// Check NodeService decorated view
	nodeDTO, err := env.nodeSvc.GetNode(ctx, node.ID)
	require.NoError(t, err)
	assert.True(t, nodeDTO.IsOnline)
	assert.Equal(t, 2, nodeDTO.RunningJobs)

	// Close connection and verify session unregisters
	_ = wsConn.Close()
	time.Sleep(50 * time.Millisecond)

	_, stillActive := env.hub.GetSession(node.ID)
	assert.False(t, stillActive, "session should be unregistered after connection close")
}

func TestAgentHandler_TriggerScheduleOnAgentConnect(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()

	// 1. Submit a job when no agent is online -> stays pending
	job, err := env.jobSvc.CreateJob(ctx, &do.CreateJobRequest{
		UserID:           1001,
		Model:            consts.DefaultModelName,
		AudioStoragePath: filepath.Join(t.TempDir(), "pending.wav"),
		OriginalFileName: "pending.wav",
	})
	require.NoError(t, err)
	assert.Equal(t, consts.StatusPending, job.Status)

	// 2. An agent registers and connects
	node, token, err := env.nodeSvc.CreateNode(ctx, "schedule-trigger-node")
	require.NoError(t, err)

	server := httptest.NewServer(env.engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/agent/ws?token=" + token
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	wsConn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = wsConn.Close() }()

	// 3. Agent reports loaded model via heartbeat
	hbMsg := do.WSMessage{
		Type: "heartbeat",
		Payload: map[string]any{
			"loaded_models": []string{consts.DefaultModelName},
			"running_jobs":  0,
		},
	}
	err = wsConn.WriteJSON(hbMsg)
	require.NoError(t, err)

	// 4. Verify agent receives dispatch_job command because triggerSchedule was called
	var dispatchedMsg do.WSMessage
	err = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	require.NoError(t, err)
	err = wsConn.ReadJSON(&dispatchedMsg)
	require.NoError(t, err)
	assert.Equal(t, "dispatch_job", dispatchedMsg.Action)

	// 5. Verify job status updated in DB
	detail, err := env.jobSvc.GetJobDetail(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusRunning, detail.Status)
	assert.Equal(t, &node.ID, detail.NodeID)

	_ = wsConn.Close()
	time.Sleep(50 * time.Millisecond)
}

// ─── Test WSConn Mock Helper ──────────────────────────────────────────────────

type testWSConn struct {
	sent []any
}

func newTestWSConn() *testWSConn {
	return &testWSConn{sent: make([]any, 0)}
}

func (t *testWSConn) WriteJSON(v any) error {
	t.sent = append(t.sent, v)
	return nil
}

func (t *testWSConn) Close() error {
	return nil
}

func (t *testWSConn) getSentMessages() []any {
	return t.sent
}
