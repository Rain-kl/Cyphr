// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
	"Wavelet/transcribe/plugins/cli/media"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	return executeCommandWithInput(root, "", args...)
}

func executeCommandWithInput(root *cobra.Command, input string, args ...string) (output string, err error) {
	inBuf := bytes.NewBufferString(input)
	outBuf := new(bytes.Buffer)
	root.SetIn(inBuf)
	root.SetOut(outBuf)
	root.SetErr(outBuf)
	root.SetArgs(args)

	err = root.Execute()
	return outBuf.String(), err
}

func TestConfigLoadAndSave(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")

	t.Run("default config when file missing", func(t *testing.T) {
		cfg, err := config.Load(cfgPath)
		require.NoError(t, err)
		assert.Equal(t, config.DefaultControllerURL, cfg.ControllerURL)
		assert.Equal(t, config.DefaultModel, cfg.DefaultModel)
		assert.Empty(t, cfg.AccessToken)
	})

	t.Run("save and load roundtrip", func(t *testing.T) {
		initial := &config.Config{
			ControllerURL: "http://example.com:9000/",
			AccessToken:   "secret-token-123",
			DefaultModel:  "whisper-large-v3",
		}
		err := config.Save(initial, cfgPath)
		require.NoError(t, err)

		loaded, err := config.Load(cfgPath)
		require.NoError(t, err)
		// Normalized trailing slash
		assert.Equal(t, "http://example.com:9000", loaded.ControllerURL)
		assert.Equal(t, "secret-token-123", loaded.AccessToken)
		assert.Equal(t, "whisper-large-v3", loaded.DefaultModel)
	})

	t.Run("environment variable overrides", func(t *testing.T) {
		t.Setenv(config.EnvCyphrURL, "http://env-controller:8080")
		t.Setenv(config.EnvCyphrToken, "env-token-xyz")
		t.Setenv(config.EnvCyphrModel, "custom-env-model")

		cfg, err := config.Load(cfgPath)
		require.NoError(t, err)
		assert.Equal(t, "http://env-controller:8080", cfg.ControllerURL)
		assert.Equal(t, "env-token-xyz", cfg.AccessToken)
		assert.Equal(t, "custom-env-model", cfg.DefaultModel)
	})
}

func TestClientOperations(t *testing.T) {
	mux := http.NewServeMux()

	// GET /api/v1/models
	mux.HandleFunc("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-valid-token" && authHeader != "" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error_msg": "invalid token"})
			return
		}
		data := []client.ModelInfo{
			{ID: 1, Name: "mock-whisper-base", TaskType: "asr", IsActive: true},
			{ID: 2, Name: "whisper-large-v3", TaskType: "asr", IsActive: true},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data":      data,
		})
	})

	// GET /api/v1/user-info
	mux.HandleFunc("/api/v1/user-info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data := client.UserProfile{
			ID:       1001,
			Username: "testuser",
			Nickname: "Tester",
			Email:    "test@example.com",
			IsAdmin:  true,
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data":      data,
		})
	})

	// POST /api/v1/audio/transcriptions
	mux.HandleFunc("/api/v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-Async") != "true" {
			http.Error(w, "missing X-Async header", http.StatusBadRequest)
			return
		}
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		model := r.FormValue("model")
		if model == "" {
			http.Error(w, "missing model", http.StatusBadRequest)
			return
		}
		_, header, err := r.FormFile("file")
		if err != nil || header == nil {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": map[string]any{
				"job_id": 10001,
				"status": "pending",
			},
		})
	})

	// GET /api/v1/jobs
	mux.HandleFunc("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs" {
			// Sub-path like /api/v1/jobs/10001 or /api/v1/jobs/10001/stream handled below
			return
		}
		now := time.Now()
		respData := client.JobListResponse{
			Items: []client.JobInfo{
				{
					ID:               10001,
					Model:            "mock-whisper-base",
					Status:           "completed",
					Progress:         100,
					Duration:         3.45,
					OriginalFileName: "test.wav",
					CreatedAt:        now,
				},
			},
			Total:    1,
			Page:     1,
			PageSize: 20,
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data":      respData,
		})
	})

	// GET /api/v1/jobs/10001/stream
	mux.HandleFunc("/api/v1/jobs/10001/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		// Emit log event
		logData, _ := json.Marshal(client.LogEvent{Seq: 1, Progress: 50, Message: "Transcribing chunk 1"})
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", logData)
		flusher.Flush()

		// Emit finish event
		finishData, _ := json.Marshal(client.FinishEvent{
			Status:     "completed",
			Duration:   3.45,
			ResultText: "Hello world, this is a successful transcription.",
		})
		_, _ = fmt.Fprintf(w, "event: finish\ndata: %s\n\n", finishData)
		flusher.Flush()
	})

	// GET /api/v1/jobs/10001
	mux.HandleFunc("/api/v1/jobs/10001", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": client.JobInfo{
				ID:         10001,
				Model:      "mock-whisper-base",
				Status:     "completed",
				Progress:   100,
				Duration:   3.45,
				ResultText: "Hello world, this is a successful transcription.",
				CreatedAt:  time.Now(),
			},
		})
	})

	// GET /api/v1/jobs/10002/stream (large payload > 64KB)
	mux.HandleFunc("/api/v1/jobs/10002/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		largeText := strings.Repeat("A very long transcription segment with details. ", 2000)
		finishData, _ := json.Marshal(client.FinishEvent{
			Status:     "completed",
			Duration:   120.0,
			ResultText: largeText,
		})
		_, _ = fmt.Fprintf(w, "event: finish\ndata: %s\n\n", finishData)
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cli := client.New(server.URL, "test-valid-token", client.WithHTTPClient(server.Client()))

	t.Run("TestConnection & ListModels", func(t *testing.T) {
		err := cli.TestConnection(context.Background())
		require.NoError(t, err)

		models, err := cli.ListModels(context.Background())
		require.NoError(t, err)
		assert.Len(t, models, 2)
		assert.Equal(t, "mock-whisper-base", models[0].Name)
	})

	t.Run("GetProfile", func(t *testing.T) {
		profile, err := cli.GetProfile(context.Background())
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, uint64(1001), profile.ID)
		assert.Equal(t, "testuser", profile.Username)
		assert.True(t, profile.IsAdmin)
	})

	t.Run("SubmitTranscription", func(t *testing.T) {
		tempDir := t.TempDir()
		dummyAudio := filepath.Join(tempDir, "test.wav")
		require.NoError(t, os.WriteFile(dummyAudio, []byte("fake wav audio data"), 0o600))

		resp, err := cli.SubmitTranscription(context.Background(), client.TranscriptionRequest{
			FilePath: dummyAudio,
			Model:    "mock-whisper-base",
		})
		require.NoError(t, err)
		assert.Equal(t, uint64(10001), resp.JobID)
		assert.Equal(t, "pending", resp.Status)
	})

	t.Run("ListJobs & GetJob", func(t *testing.T) {
		list, err := cli.ListJobs(context.Background(), 1, 10, "")
		require.NoError(t, err)
		assert.Len(t, list.Items, 1)
		assert.Equal(t, uint64(10001), list.Items[0].ID)

		job, err := cli.GetJob(context.Background(), 10001)
		require.NoError(t, err)
		assert.Equal(t, uint64(10001), job.ID)
		assert.Equal(t, "completed", job.Status)
	})

	t.Run("StreamJobLogs", func(t *testing.T) {
		var receivedLogs []client.LogEvent
		var receivedFinish *client.FinishEvent

		err := cli.StreamJobLogs(
			context.Background(),
			10001,
			func(log client.LogEvent) {
				receivedLogs = append(receivedLogs, log)
			},
			func(finish client.FinishEvent) {
				receivedFinish = &finish
			},
		)
		require.NoError(t, err)
		assert.Len(t, receivedLogs, 1)
		assert.Equal(t, 50, receivedLogs[0].Progress)
		assert.Equal(t, "Transcribing chunk 1", receivedLogs[0].Message)
		require.NotNil(t, receivedFinish)
		assert.Equal(t, "completed", receivedFinish.Status)
		assert.Contains(t, receivedFinish.ResultText, "Hello world")
	})

	t.Run("StreamJobLogs with large payload >64KB", func(t *testing.T) {
		var receivedFinish *client.FinishEvent
		err := cli.StreamJobLogs(
			context.Background(),
			10002,
			nil,
			func(finish client.FinishEvent) {
				receivedFinish = &finish
			},
		)
		require.NoError(t, err)
		require.NotNil(t, receivedFinish)
		assert.Greater(t, len(receivedFinish.ResultText), 65536)
	})
}

func TestCobraCommands(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": []client.ModelInfo{
				{ID: 1, Name: "mock-whisper-base", IsActive: true},
			},
		})
	})

	mux.HandleFunc("/api/v1/user-info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": client.UserProfile{
				ID:       1001,
				Username: "testuser",
				Nickname: "Tester",
				Email:    "test@example.com",
				IsAdmin:  true,
			},
		})
	})

	mux.HandleFunc("/api/v1/audio/transcriptions", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": map[string]any{
				"job_id": 20002,
				"status": "pending",
			},
		})
	})

	mux.HandleFunc("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": client.JobListResponse{
				Items: []client.JobInfo{
					{
						ID:               20002,
						Model:            "mock-whisper-base",
						Status:           "completed",
						Progress:         100,
						Duration:         4.20,
						OriginalFileName: "voice.mp3",
						CreatedAt:        time.Now(),
					},
				},
				Total:    1,
				Page:     1,
				PageSize: 20,
			},
		})
	})

	mux.HandleFunc("/api/v1/jobs/20002", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": client.JobInfo{
				ID:               20002,
				Model:            "mock-whisper-base",
				Status:           "completed",
				Progress:         100,
				Duration:         4.20,
				OriginalFileName: "voice.mp3",
				ResultText:       "Speech recognition completed successfully.",
				CreatedAt:        time.Now(),
			},
		})
	})

	mux.HandleFunc("/api/v1/jobs/20003", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error_msg": "",
			"data": client.JobInfo{
				ID:        20003,
				Model:     "mock-whisper-base",
				Status:    "running",
				Progress:  45,
				CreatedAt: time.Now(),
			},
		})
	})

	mux.HandleFunc("/api/v1/jobs/20002/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		logData, _ := json.Marshal(client.LogEvent{Seq: 1, Progress: 100, Message: "Finished decoding"})
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", logData)
		flusher.Flush()

		finishData, _ := json.Marshal(client.FinishEvent{
			Status:     "completed",
			Duration:   4.20,
			ResultText: "Speech recognition completed successfully.",
		})
		_, _ = fmt.Fprintf(w, "event: finish\ndata: %s\n\n", finishData)
		flusher.Flush()
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "cli_config.yaml")

	t.Run("login command", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "login", "--config", cfgFile, "--url", server.URL, "--token", "tok-123")
		require.NoError(t, err)
		assert.Contains(t, out, "Successfully logged in")

		// Verify file written
		loaded, err := config.Load(cfgFile)
		require.NoError(t, err)
		assert.Equal(t, server.URL, loaded.ControllerURL)
		assert.Equal(t, "tok-123", loaded.AccessToken)
	})

	t.Run("interactive login prompt", func(t *testing.T) {
		interactiveCfg := filepath.Join(tempDir, "interactive_cli_config.yaml")
		root := NewRootCmd()
		input := fmt.Sprintf("%s\ninteractive-token-456\n", server.URL)
		out, err := executeCommandWithInput(root, input, "login", "--config", interactiveCfg)
		require.NoError(t, err)
		assert.Contains(t, out, "Controller URL [")
		assert.Contains(t, out, "Access Token:")
		assert.Contains(t, out, "Successfully logged in")

		loaded, err := config.Load(interactiveCfg)
		require.NoError(t, err)
		assert.Equal(t, server.URL, loaded.ControllerURL)
		assert.Equal(t, "interactive-token-456", loaded.AccessToken)
	})

	t.Run("jobs ls command", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "jobs", "ls", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "JOB ID")
		assert.Contains(t, out, "20002")
		assert.Contains(t, out, "mock-whisper-base")
		assert.Contains(t, out, "completed")
	})

	t.Run("jobs log command", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "jobs", "log", "20002", "--config", cfgFile, "-f")
		require.NoError(t, err)
		assert.Contains(t, out, "Finished decoding")
		assert.Contains(t, out, "Speech recognition completed successfully.")
	})

	t.Run("models command", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "models", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "MODEL")
		assert.Contains(t, out, "mock-whisper-base")
		assert.Contains(t, out, "active")
	})

	t.Run("asr command with audio file", func(t *testing.T) {
		t.Chdir(t.TempDir()) // 结果文件落盘到隔离目录, 不污染包目录
		dummyAudio := filepath.Join(tempDir, "sample.mp3")
		require.NoError(t, os.WriteFile(dummyAudio, []byte("fake mp3"), 0o600))

		root := NewRootCmd()
		out, err := executeCommand(root, "asr", dummyAudio, "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Job submitted successfully: ID #20002")
		assert.Contains(t, out, "Finished decoding")
		assert.Contains(t, out, "Speech recognition completed successfully.")
		data, err := os.ReadFile("sample.txt")
		require.NoError(t, err)
		assert.Equal(t, "Speech recognition completed successfully.", string(data))
		srtData, err := os.ReadFile("sample.srt")
		require.NoError(t, err)
		assert.Contains(t, string(srtData), "-->")
		assert.Contains(t, string(srtData), "Speech recognition completed successfully.")
	})

	t.Run("asr command with output directory flag", func(t *testing.T) {
		outDir := filepath.Join(t.TempDir(), "sub", "custom_out")
		dummyAudio := filepath.Join(tempDir, "sample_dir.mp3")
		require.NoError(t, os.WriteFile(dummyAudio, []byte("fake mp3"), 0o600))

		root := NewRootCmd()
		out, err := executeCommand(root, "asr", dummyAudio, "-o", outDir, "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Job submitted successfully: ID #20002")
		assert.FileExists(t, filepath.Join(outDir, "sample_dir.txt"))
		assert.FileExists(t, filepath.Join(outDir, "sample_dir.srt"))
	})

	t.Run("asr command with detach flag", func(t *testing.T) {
		dummyAudio := filepath.Join(tempDir, "sample_detach.mp3")
		require.NoError(t, os.WriteFile(dummyAudio, []byte("fake mp3"), 0o600))

		root := NewRootCmd()
		out, err := executeCommand(root, "asr", dummyAudio, "-d", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Job submitted successfully: ID #20002")
		assert.Contains(t, out, "Job is running in background")
		assert.Contains(t, out, "cyphr jobs get 20002")
	})

	t.Run("jobs get command for completed job", func(t *testing.T) {
		t.Chdir(t.TempDir())
		root := NewRootCmd()
		out, err := executeCommand(root, "jobs", "get", "20002", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Downloading results for job #20002")
		assert.FileExists(t, "voice.txt")
		assert.FileExists(t, "voice.srt")
		data, err := os.ReadFile("voice.txt")
		require.NoError(t, err)
		assert.Equal(t, "Speech recognition completed successfully.", string(data))
	})

	t.Run("jobs get command with custom directory", func(t *testing.T) {
		outDir := filepath.Join(t.TempDir(), "downloaded")
		root := NewRootCmd()
		out, err := executeCommand(root, "jobs", "get", "20002", "-d", outDir, "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Downloading results for job #20002")
		assert.FileExists(t, filepath.Join(outDir, "voice.txt"))
		assert.FileExists(t, filepath.Join(outDir, "voice.srt"))
	})

	t.Run("jobs get command for uncompleted job", func(t *testing.T) {
		root := NewRootCmd()
		_, err := executeCommand(root, "jobs", "get", "20003", "--config", cfgFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not completed yet")
	})

	t.Run("asr command with unsupported file", func(t *testing.T) {
		dummyDoc := filepath.Join(tempDir, "sample.txt")
		require.NoError(t, os.WriteFile(dummyDoc, []byte("text file"), 0o600))

		root := NewRootCmd()
		_, err := executeCommand(root, "asr", dummyDoc, "--config", cfgFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported media file format")
	})

	t.Run("asr command with video file", func(t *testing.T) {
		t.Chdir(t.TempDir()) // 结果文件落盘到隔离目录, 不污染包目录
		dummyVideo := filepath.Join(tempDir, "clip.mp4")
		require.NoError(t, os.WriteFile(dummyVideo, []byte("fake mp4 content"), 0o600))

		cleanup := media.SetFFmpegRunnerForTest(
			func(string) (string, error) { return "/mock/ffmpeg", nil },
			func(ctx context.Context, name string, args ...string) ([]byte, error) {
				outPath := args[len(args)-1]
				_ = os.WriteFile(outPath, []byte("mock extracted mp3 audio"), 0o600)
				return []byte("ok"), nil
			},
		)
		defer cleanup()

		root := NewRootCmd()
		out, err := executeCommand(root, "asr", dummyVideo, "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "Video file detected")
		assert.Contains(t, out, "Job submitted successfully: ID #20002")
		assert.Contains(t, out, "Speech recognition completed successfully.")
		data, err := os.ReadFile("clip.txt")
		require.NoError(t, err)
		assert.Equal(t, "Speech recognition completed successfully.", string(data))
		srtData, err := os.ReadFile("clip.srt")
		require.NoError(t, err)
		assert.Contains(t, string(srtData), "-->")
	})

	t.Run("profile command", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "profile", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, "User ID:")
		assert.Contains(t, out, "1001")
		assert.Contains(t, out, "Username:")
		assert.Contains(t, out, "testuser")
		assert.Contains(t, out, "Nickname:")
		assert.Contains(t, out, "Tester")
		assert.Contains(t, out, "Role:")
		assert.Contains(t, out, "Administrator")
	})

	t.Run("profile command json output", func(t *testing.T) {
		root := NewRootCmd()
		out, err := executeCommand(root, "profile", "--json", "--config", cfgFile)
		require.NoError(t, err)
		assert.Contains(t, out, `"username": "testuser"`)
		assert.Contains(t, out, `"is_admin": true`)
	})
}
