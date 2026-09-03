// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package client provides an HTTP client for interacting with the Transcribe controller.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 60 * time.Second

	scannerInitBufferSize = 64 * 1024
	scannerMaxTokenSize   = 10 * 1024 * 1024

	// StatusPending represents a pending job status.
	StatusPending = "pending"
	// StatusRunning represents an actively running job status.
	StatusRunning = "running"
	// StatusCompleted represents a successfully completed job status.
	StatusCompleted = "completed"
	// StatusFailed represents a failed job status.
	StatusFailed = "failed"
)

// APIResponse is the standard envelope returned by controller endpoints.
type APIResponse struct {
	ErrorMsg string          `json:"error_msg"`
	Data     json.RawMessage `json:"data"`
}

// ModelInfo represents available transcription model metadata.
type ModelInfo struct {
	ID          uint64    `json:"id,string"`
	Name        string    `json:"name"`
	TaskType    string    `json:"task_type"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// JobInfo represents transcription job detail.
type JobInfo struct {
	ID               uint64     `json:"id,string"`
	UserID           uint64     `json:"user_id,string,omitempty"`
	NodeID           *uint64    `json:"node_id,string,omitempty"`
	Model            string     `json:"model"`
	TaskType         string     `json:"task_type,omitempty"`
	Status           string     `json:"status"`
	Progress         int        `json:"progress"`
	Duration         float64    `json:"duration"`
	OriginalFileName string     `json:"original_file_name"`
	ResultText       string     `json:"result_text,omitempty"`
	ErrorMsg         string     `json:"error_msg,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// JobListResponse represents paginated job listing data.
type JobListResponse struct {
	Items    []JobInfo `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// TranscriptionRequest specifies parameters for submitting a transcription job.
type TranscriptionRequest struct {
	FilePath         string
	OriginalFileName string
	Model            string
	Language         string
	Prompt           string
	ResponseFormat   string
	Temperature      *float64
}

// TranscriptionSubmitResponse contains job ID and status returned on submission.
type TranscriptionSubmitResponse struct {
	JobID  uint64 `json:"job_id,string"`
	Status string `json:"status"`
}

// UnmarshalJSON supports decoding JobID from either a JSON string or a number.
func (r *TranscriptionSubmitResponse) UnmarshalJSON(data []byte) error {
	type Alias TranscriptionSubmitResponse
	aux := struct {
		RawJobID any `json:"job_id"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.RawJobID.(type) {
	case string:
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}
		r.JobID = id
	case float64:
		if v >= 0 {
			r.JobID = uint64(v)
		}
	case int64:
		if v >= 0 {
			r.JobID = uint64(v)
		}
	}
	return nil
}

// LogEvent represents a single progress log received via SSE stream.
type LogEvent struct {
	Seq      int    `json:"seq"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

// FinishEvent represents the terminal event of a transcription job via SSE stream.
type FinishEvent struct {
	Status     string  `json:"status"`
	Duration   float64 `json:"duration"`
	ResultText string  `json:"result_text"`
	ErrorMsg   string  `json:"error_msg"`
}

// Client interacts with the Transcribe Controller server.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option configures Client instances.
type Option func(*Client)

// WithHTTPClient sets custom *http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// New creates a new Client instance.
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, target any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr APIResponse
		if jsonErr := json.Unmarshal(bodyBytes, &apiErr); jsonErr == nil && apiErr.ErrorMsg != "" {
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, apiErr.ErrorMsg)
		}
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	if target != nil {
		return decodeResponseData(bodyBytes, target)
	}

	return nil
}

func decodeResponseData(bodyBytes []byte, target any) error {
	var envelope APIResponse
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		// Direct unmarshal fallback
		if directErr := json.Unmarshal(bodyBytes, target); directErr != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}

	if envelope.ErrorMsg != "" {
		return fmt.Errorf("api error: %s", envelope.ErrorMsg)
	}

	if len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("failed to decode data field: %w", err)
		}
		return nil
	}

	// If envelope data is empty and there's no error_msg, attempt direct unmarshal into target
	if directErr := json.Unmarshal(bodyBytes, target); directErr != nil {
		return directErr
	}
	return nil
}

// TestConnection checks connectivity with the controller by requesting the models list.
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ListModels(ctx)
	return err
}

// ListModels queries available active models from the controller.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/models", nil)
	if err != nil {
		return nil, err
	}

	var models []ModelInfo
	if err := c.doJSON(req, &models); err != nil {
		return nil, err
	}
	return models, nil
}

// SubmitTranscription uploads an audio file and creates an asynchronous transcription job.
func (c *Client) SubmitTranscription(ctx context.Context, req TranscriptionRequest) (*TranscriptionSubmitResponse, error) {
	file, err := os.Open(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open media file %s: %w", req.FilePath, err)
	}
	defer func() { _ = file.Close() }()

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	fileName := req.OriginalFileName
	if fileName == "" {
		fileName = filepath.Base(req.FilePath)
	}

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart file part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file to multipart: %w", err)
	}

	if req.Model != "" {
		_ = writer.WriteField("model", req.Model)
	}
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = writer.WriteField("prompt", req.Prompt)
	}
	if req.ResponseFormat != "" {
		_ = writer.WriteField("response_format", req.ResponseFormat)
	}
	if req.Temperature != nil {
		_ = writer.WriteField("temperature", strconv.FormatFloat(*req.Temperature, 'f', -1, 64))
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/api/v1/audio/transcriptions", bodyBuf)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("X-Async", "true")

	var submitResp TranscriptionSubmitResponse
	if err := c.doJSON(httpReq, &submitResp); err != nil {
		return nil, err
	}
	return &submitResp, nil
}

// ListJobs retrieves paginated jobs list for the current user.
func (c *Client) ListJobs(ctx context.Context, page, pageSize int, status string) (*JobListResponse, error) {
	queryParams := url.Values{}
	if page > 0 {
		queryParams.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		queryParams.Set("page_size", strconv.Itoa(pageSize))
	}
	if status != "" {
		queryParams.Set("status", status)
	}

	path := "/api/v1/jobs"
	if enc := queryParams.Encode(); enc != "" {
		path += "?" + enc
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp JobListResponse
	if err := c.doJSON(req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetJob retrieves details of a specific job by ID.
func (c *Client) GetJob(ctx context.Context, jobID uint64) (*JobInfo, error) {
	path := fmt.Sprintf("/api/v1/jobs/%d", jobID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var job JobInfo
	if err := c.doJSON(req, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// StreamJobLogs connects to the job SSE endpoint, streaming real-time logs and the terminal finish event.
func (c *Client) StreamJobLogs(
	ctx context.Context,
	jobID uint64,
	onLog func(LogEvent),
	onFinish func(FinishEvent),
) error {
	path := fmt.Sprintf("/api/v1/jobs/%d/stream", jobID)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// SSE streaming must not inherit standard request timeout
	streamClient := c.httpClient
	if streamClient.Timeout > 0 {
		copyClient := *streamClient
		copyClient.Timeout = 0
		streamClient = &copyClient
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, scannerInitBufferSize)
	scanner.Buffer(buf, scannerMaxTokenSize)
	var currentEvent string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if dispatchSSE(currentEvent, dataLines, onLog, onFinish) {
				return nil
			}
			currentEvent = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("stream read error: %w", err)
	}

	return nil
}

func dispatchSSE(
	currentEvent string,
	dataLines []string,
	onLog func(LogEvent),
	onFinish func(FinishEvent),
) bool {
	if len(dataLines) == 0 {
		return false
	}
	dataStr := strings.Join(dataLines, "\n")
	switch currentEvent {
	case "log":
		var logEv LogEvent
		if err := json.Unmarshal([]byte(dataStr), &logEv); err == nil && onLog != nil {
			onLog(logEv)
		}
	case "finish":
		var finishEv FinishEvent
		if err := json.Unmarshal([]byte(dataStr), &finishEv); err == nil && onFinish != nil {
			onFinish(finishEv)
		}
		return true
	}
	return false
}
