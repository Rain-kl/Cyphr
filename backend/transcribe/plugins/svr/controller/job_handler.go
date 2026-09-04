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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// JobHandler handles user job queries and SSE streaming.
type JobHandler struct {
	jobService  service.JobService
	logBroker   service.LogBroker
	authService contracts.AuthService
}

// NewJobHandler creates a new JobHandler instance.
func NewJobHandler(jobSvc service.JobService, logBroker service.LogBroker, authSvc ...contracts.AuthService) *JobHandler {
	h := &JobHandler{
		jobService: jobSvc,
		logBroker:  logBroker,
	}
	if len(authSvc) > 0 {
		h.authService = authSvc[0]
	}
	return h
}

// ListJobs handles GET /api/v1/jobs, returning paginated jobs for current user.
func (h *JobHandler) ListJobs(c *gin.Context) {
	userID := GetCurrentUserID(c, h.authService)

	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	pageSize := 20
	if ps, err := strconv.Atoi(c.Query("page_size")); err == nil && ps > 0 {
		pageSize = ps
	}

	status := c.Query("status")

	jobs, err := h.jobService.ListJobs(c.Request.Context(), userID, page, pageSize, status)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[JobHandler] list jobs failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(jobs))
}

// ListAllJobs handles GET /api/v1/controller/jobs, returning paginated jobs across all users for admin.
func (h *JobHandler) ListAllJobs(c *gin.Context) {
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	pageSize := 20
	if ps, err := strconv.Atoi(c.Query("page_size")); err == nil && ps > 0 {
		pageSize = ps
	}

	status := c.Query("status")
	keyword := c.Query("keyword")

	var nodeID uint64
	if nid, err := strconv.ParseUint(c.Query("node_id"), 10, 64); err == nil {
		nodeID = nid
	}

	var userID uint64
	if uid, err := strconv.ParseUint(c.Query("user_id"), 10, 64); err == nil {
		userID = uid
	}

	jobs, err := h.jobService.ListAllJobs(c.Request.Context(), page, pageSize, status, nodeID, userID, keyword)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[JobHandler] list all jobs failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(jobs))
}

func isJobAccessible(c *gin.Context, authSvc contracts.AuthService, jobUserID, currentUID uint64) bool {
	if jobUserID == 0 {
		return true
	}
	if currentUID > 0 && currentUID == jobUserID {
		return true
	}
	if authSvc != nil {
		if u, err := authSvc.GetCurrentUser(c.Request.Context()); err == nil && u != nil && u.IsAdmin {
			return true
		}
	}
	if val, ok := c.Get(contracts.AuthTokenAdminKey); ok {
		if isAdmin, ok := val.(bool); ok && isAdmin {
			return true
		}
	}
	if val, ok := c.Get("is_admin"); ok {
		if isAdmin, ok := val.(bool); ok && isAdmin {
			return true
		}
	}
	return currentUID == 0 && authSvc == nil
}

// GetJob handles GET /api/v1/jobs/:id, returning detailed job information.
func (h *JobHandler) GetJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	job, err := h.jobService.GetJobDetail(c.Request.Context(), id)
	if err != nil {
		response.AbortNotFound(c, consts.ErrJobNotFound.Error())
		return
	}

	currentUID := GetCurrentUserID(c, h.authService)
	if !isJobAccessible(c, h.authService, job.UserID, currentUID) {
		response.AbortForbidden(c, consts.ErrForbidden)
		return
	}

	c.JSON(http.StatusOK, response.OK(job))
}

// StreamJob handles GET /api/v1/jobs/:id/stream, streaming SSE log events and completion.
func (h *JobHandler) StreamJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	job, err := h.jobService.GetJobDetail(c.Request.Context(), id)
	if err != nil {
		response.AbortNotFound(c, consts.ErrJobNotFound.Error())
		return
	}

	currentUID := GetCurrentUserID(c, h.authService)
	if !isJobAccessible(c, h.authService, job.UserID, currentUID) {
		response.AbortForbidden(c, consts.ErrForbidden)
		return
	}

	// Prepare SSE response headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.AbortInternal(c, consts.ErrStreamingUnsupported)
		return
	}

	// 1. Flush historical logs from DB
	if logs, logErr := h.jobService.GetJobLogs(c.Request.Context(), id); logErr == nil && len(logs) > 0 {
		for _, logMsg := range logs {
			data, _ := json.Marshal(logMsg)
			_, _ = fmt.Fprintf(c.Writer, "event: log\ndata: %s\n\n", data)
		}
		flusher.Flush()
	}

	// 2. If job is already finished, write finish event and terminate
	if job.Status == consts.StatusCompleted || job.Status == consts.StatusFailed {
		finishMsg := do.FinishMessage{
			Status:     job.Status,
			Duration:   job.Duration,
			ResultText: job.ResultText,
			ErrorMsg:   job.ErrorMsg,
		}
		data, _ := json.Marshal(finishMsg)
		_, _ = fmt.Fprintf(c.Writer, "event: finish\ndata: %s\n\n", data)
		flusher.Flush()
		return
	}

	// 3. For ongoing jobs, subscribe to LogBroker for real-time events
	if h.logBroker == nil {
		return
	}

	logCh, cancelLogs := h.logBroker.Subscribe(id)
	defer cancelLogs()

	finishCh, cancelFinish := h.logBroker.SubscribeFinish(id)
	defer cancelFinish()

	// Double-check if job completed right after subscribing to avoid lost updates
	if latest, detailErr := h.jobService.GetJobDetail(c.Request.Context(), id); detailErr == nil {
		if latest.Status == consts.StatusCompleted || latest.Status == consts.StatusFailed {
			finishMsg := do.FinishMessage{
				Status:     latest.Status,
				Duration:   latest.Duration,
				ResultText: latest.ResultText,
				ErrorMsg:   latest.ErrorMsg,
			}
			data, _ := json.Marshal(finishMsg)
			_, _ = fmt.Fprintf(c.Writer, "event: finish\ndata: %s\n\n", data)
			flusher.Flush()
			return
		}
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case logMsg, chOk := <-logCh:
			if !chOk {
				continue
			}
			data, _ := json.Marshal(logMsg)
			_, _ = fmt.Fprintf(c.Writer, "event: log\ndata: %s\n\n", data)
			flusher.Flush()
		case finishMsg, chOk := <-finishCh:
			if chOk {
				data, _ := json.Marshal(finishMsg)
				_, _ = fmt.Fprintf(c.Writer, "event: finish\ndata: %s\n\n", data)
				flusher.Flush()
			}
			return
		}
	}
}

// RetryJob handles POST /api/v1/controller/jobs/:id/retry, re-enqueuing a failed job for admin.
func (h *JobHandler) RetryJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	if err := h.jobService.RetryJob(c.Request.Context(), id); err != nil {
		if errors.Is(err, consts.ErrJobNotFound) {
			response.AbortNotFound(c, consts.ErrJobNotFound.Error())
			return
		}
		logger.ErrorF(c.Request.Context(), "[JobHandler] retry job %d failed: %v", id, err)
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// BatchDeleteJobs handles POST /api/v1/jobs/batch-delete for authenticated users.
func (h *JobHandler) BatchDeleteJobs(c *gin.Context) {
	var req do.BatchDeleteJobsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	userID := GetCurrentUserID(c, h.authService)
	if userID == 0 {
		response.AbortUnauthorized(c, consts.ErrUnauthorized)
		return
	}

	deletedCount, err := h.jobService.DeleteJobs(c.Request.Context(), req.JobIDs, userID, false)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[JobHandler] user %d batch delete jobs failed: %v", userID, err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{"deleted_count": deletedCount}))
}

// BatchDeleteAllJobs handles POST /api/v1/controller/jobs/batch-delete for admin.
func (h *JobHandler) BatchDeleteAllJobs(c *gin.Context) {
	var req do.BatchDeleteJobsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	deletedCount, err := h.jobService.DeleteJobs(c.Request.Context(), req.JobIDs, 0, true)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[JobHandler] admin batch delete jobs failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{"deleted_count": deletedCount}))
}
