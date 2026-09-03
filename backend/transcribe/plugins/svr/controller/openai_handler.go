// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service"
	"errors"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const defaultSyncTimeout = 300 * time.Second

// OpenAIHandler handles OpenAI-compatible audio transcription endpoints.
type OpenAIHandler struct {
	jobService     service.JobService
	storageService contracts.StorageService
	uploadService  contracts.UploadService
	logBroker      service.LogBroker
	authService    contracts.AuthService
	syncTimeout    time.Duration
}

// OpenAIOption configures optional parameters for OpenAIHandler.
type OpenAIOption func(*OpenAIHandler)

// WithOpenAIStorageService sets the platform storage service for file uploads.
func WithOpenAIStorageService(s contracts.StorageService) OpenAIOption {
	return func(h *OpenAIHandler) {
		h.storageService = s
	}
}

// WithAuthService sets the platform auth service for extracting caller identity.
func WithAuthService(a contracts.AuthService) OpenAIOption {
	return func(h *OpenAIHandler) {
		h.authService = a
	}
}

// WithOpenAISyncTimeout sets the maximum duration to wait for synchronous transcription completion.
func WithOpenAISyncTimeout(d time.Duration) OpenAIOption {
	return func(h *OpenAIHandler) {
		h.syncTimeout = d
	}
}

// NewOpenAIHandler creates a new OpenAIHandler instance.
func NewOpenAIHandler(jobSvc service.JobService, logBroker service.LogBroker, opts ...OpenAIOption) *OpenAIHandler {
	h := &OpenAIHandler{
		jobService:  jobSvc,
		logBroker:   logBroker,
		syncTimeout: defaultSyncTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	return h
}

// resolveAudioSource maps the request to a stored audio path: either reusing an
// existing upload by content hash (no file part), or persisting the freshly
// uploaded file. It writes the error response and returns ok=false on failure.
func (h *OpenAIHandler) resolveAudioSource(c *gin.Context, fileHeader *multipart.FileHeader, fileHash, originalFileName string) (string, string, bool) {
	if fileHeader != nil {
		savedPath, saveErr := h.saveAudioFile(c, fileHeader)
		if saveErr != nil {
			logger.ErrorF(c.Request.Context(), "[OpenAIHandler] failed to save audio file: %v", saveErr)
			response.AbortInternal(c, consts.ErrFileUploadFailed)
			return "", "", false
		}
		if originalFileName == "" {
			originalFileName = fileHeader.Filename
		}
		return savedPath, originalFileName, true
	}

	var fileSize int64
	if n, convErr := strconv.ParseInt(strings.TrimSpace(c.PostForm("file_size")), 10, 64); convErr == nil && n > 0 {
		fileSize = n
	}
	if h.uploadService == nil {
		response.AbortInternal(c, consts.ErrInternal)
		return "", "", false
	}
	existing, findErr := h.uploadService.FindByHash(c.Request.Context(), fileHash, fileSize)
	if findErr != nil || existing == nil {
		response.AbortNotFound(c, consts.ErrNotFound)
		return "", "", false
	}
	if originalFileName == "" {
		originalFileName = existing.FileName
	}
	return existing.FilePath, originalFileName, true
}

// HandleTranscription handles POST /api/v1/audio/transcriptions.
func (h *OpenAIHandler) HandleTranscription(c *gin.Context) {
	fileHeader, _ := c.FormFile("file")
	fileHash := strings.TrimSpace(c.PostForm("file_hash"))
	if fileHeader == nil && fileHash == "" {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	modelName := strings.TrimSpace(c.PostForm("model"))
	if modelName == "" {
		modelName = consts.DefaultModelName
	}

	language := strings.TrimSpace(c.PostForm("language"))
	responseFormat := strings.TrimSpace(c.DefaultPostForm("response_format", consts.ResponseFormatJSON))
	if responseFormat == "" {
		responseFormat = consts.ResponseFormatJSON
	}

	prompt := c.PostForm("prompt")

	var temperature float64
	if tempStr := strings.TrimSpace(c.PostForm("temperature")); tempStr != "" {
		if t, err := strconv.ParseFloat(tempStr, 64); err == nil {
			temperature = t
		}
	}

	// Resolve audio bytes: reuse an existing upload by content hash (no file part),
	// or persist the freshly uploaded file.
	originalFileName := strings.TrimSpace(c.PostForm("original_file_name"))
	storagePath, originalFileName, ok := h.resolveAudioSource(c, fileHeader, fileHash, originalFileName)
	if !ok {
		return
	}

	userID := GetCurrentUserID(c, h.authService)

	jobReq := &do.CreateJobRequest{
		UserID:           userID,
		Model:            modelName,
		TaskType:         consts.TaskTypeASR,
		AudioStoragePath: storagePath,
		OriginalFileName: originalFileName,
		Language:         language,
		Prompt:           prompt,
		ResponseFormat:   responseFormat,
		Temperature:      temperature,
	}

	job, err := h.jobService.CreateJob(c.Request.Context(), jobReq)
	if err != nil {
		if err.Error() == consts.ErrModelUnavailable {
			response.AbortBadRequest(c, consts.ErrModelUnavailable)
			return
		}
		logger.ErrorF(c.Request.Context(), "[OpenAIHandler] failed to create job: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	// Check if asynchronous job mode is requested
	if strings.EqualFold(c.GetHeader("X-Async"), "true") {
		c.JSON(http.StatusOK, response.OK(gin.H{
			"job_id": strconv.FormatUint(job.ID, 10),
			"status": consts.StatusPending,
		}))
		return
	}

	// Synchronous blocking mode: wait for completion via finish subscriber
	h.waitForJobCompletion(c, job.ID, language, responseFormat)
}

func (h *OpenAIHandler) saveAudioFile(c *gin.Context, fileHeader *multipart.FileHeader) (string, error) {
	if h.uploadService == nil {
		return "", errors.New("upload service not available")
	}

	f, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	ext := strings.TrimPrefix(filepath.Ext(fileHeader.Filename), ".")
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	userID := GetCurrentUserID(c, h.authService)

	result, err := h.uploadService.Ingest(c.Request.Context(), contracts.UploadIngestRequest{
		UserID:             userID,
		Type:               "audio",
		FileName:           fileHeader.Filename,
		MimeType:           mimeType,
		Extension:          ext,
		Reader:             f,
		Size:               fileHeader.Size,
		SkipExtensionCheck: true,
	})
	if err != nil {
		return "", err
	}
	return result.FilePath, nil
}

func abortOpenAIError(c *gin.Context, statusCode int, errMsg, errType string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": errMsg,
			"type":    errType,
		},
	})
}

// waitForJobCompletion blocks until the job finishes or timeout occurs.
func (h *OpenAIHandler) waitForJobCompletion(c *gin.Context, jobID uint64, language, responseFormat string) {
	var finishCh <-chan do.FinishMessage
	var cancelFinish func()
	if h.logBroker != nil {
		finishCh, cancelFinish = h.logBroker.SubscribeFinish(jobID)
		defer cancelFinish()
	}

	// Check if job is already finished (e.g. mock completed immediately)
	if detail, err := h.jobService.GetJobDetail(c.Request.Context(), jobID); err == nil {
		if detail.Status == consts.StatusCompleted {
			h.renderOpenAIResponse(c, detail, responseFormat, language)
			return
		}
		if detail.Status == consts.StatusFailed {
			errMsg := detail.ErrorMsg
			if errMsg == "" {
				errMsg = "transcription failed"
			}
			abortOpenAIError(c, http.StatusInternalServerError, errMsg, "server_error")
			return
		}
	}

	if finishCh == nil {
		c.JSON(http.StatusOK, response.OK(gin.H{
			"job_id": jobID,
			"status": consts.StatusPending,
		}))
		return
	}

	timer := time.NewTimer(h.syncTimeout)
	defer timer.Stop()

	select {
	case <-c.Request.Context().Done():
		return
	case finishMsg, ok := <-finishCh:
		if !ok || finishMsg.Status == consts.StatusFailed {
			errMsg := finishMsg.ErrorMsg
			if errMsg == "" {
				errMsg = "transcription failed"
			}
			abortOpenAIError(c, http.StatusInternalServerError, errMsg, "server_error")
			return
		}

		detail, err := h.jobService.GetJobDetail(c.Request.Context(), jobID)
		if err != nil {
			detail = &do.JobDTO{
				ID:         jobID,
				Status:     finishMsg.Status,
				Duration:   finishMsg.Duration,
				ResultText: finishMsg.ResultText,
			}
		}
		h.renderOpenAIResponse(c, detail, responseFormat, language)

	case <-timer.C:
		abortOpenAIError(c, http.StatusGatewayTimeout, "transcription timed out", "timeout_error")
	}
}

func (h *OpenAIHandler) renderOpenAIResponse(c *gin.Context, job *do.JobDTO, format, language string) {
	switch format {
	case consts.ResponseFormatVerboseJSON:
		if job.OpenAIResponse != nil {
			c.JSON(http.StatusOK, job.OpenAIResponse)
			return
		}
		c.JSON(http.StatusOK, do.OpenAIVerboseTranscriptionResponse{
			Task:     "transcribe",
			Language: language,
			Duration: job.Duration,
			Text:     job.ResultText,
		})
	case consts.ResponseFormatText, consts.ResponseFormatSRT, consts.ResponseFormatVTT:
		c.String(http.StatusOK, job.ResultText)
	default:
		// Default format: "json"
		c.JSON(http.StatusOK, do.OpenAITranscriptionResponse{
			Text: job.ResultText,
		})
	}
}
