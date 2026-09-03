// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// AgentHandler handles communication with distributed worker agents.
type AgentHandler struct {
	mu             sync.RWMutex
	agentHub       hub.AgentHub
	nodeService    service.NodeService
	jobService     service.JobService
	storageService contracts.StorageService
	scheduler      scheduler.Scheduler
	upgrader       websocket.Upgrader
}

// AgentOption configures optional parameters for AgentHandler.
type AgentOption func(*AgentHandler)

// WithAgentStorageService configures the platform storage service for audio downloads.
func WithAgentStorageService(s contracts.StorageService) AgentOption {
	return func(h *AgentHandler) {
		h.storageService = s
	}
}

// WithAgentScheduler configures the task scheduler on the agent handler.
func WithAgentScheduler(s scheduler.Scheduler) AgentOption {
	return func(h *AgentHandler) {
		h.scheduler = s
	}
}

// SetScheduler updates the task scheduler reference.
func (h *AgentHandler) SetScheduler(s scheduler.Scheduler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scheduler = s
}

func (h *AgentHandler) getScheduler() scheduler.Scheduler {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scheduler
}

func (h *AgentHandler) triggerSchedule(ctx context.Context) {
	sched := h.getScheduler()
	if sched != nil {
		schedCtx := context.WithoutCancel(ctx)
		util.Go(func() {
			if err := sched.SchedulePendingJobs(schedCtx); err != nil {
				logger.ErrorF(schedCtx, "[AgentHandler] trigger schedule failed: %v", err)
			}
		})
	}
}

// NewAgentHandler creates a new AgentHandler instance.
func NewAgentHandler(
	agentHub hub.AgentHub,
	nodeSvc service.NodeService,
	jobSvc service.JobService,
	opts ...AgentOption,
) *AgentHandler {
	h := &AgentHandler{
		agentHub:    agentHub,
		nodeService: nodeSvc,
		jobService:  jobSvc,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	return h
}

// HandleWS handles GET /api/v1/agent/ws, upgrading connection to WebSocket and maintaining session.
func (h *AgentHandler) HandleWS(c *gin.Context) {
	node, ok := GetNodeFromContext(c)
	if !ok {
		token := c.Query("token")
		var err error
		node, err = h.nodeService.VerifyNodeToken(c.Request.Context(), token)
		if err != nil {
			response.AbortUnauthorized(c, consts.ErrInvalidToken)
			return
		}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[AgentHandler] ws upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	clientIP := c.ClientIP()
	sess := hub.NewAgentSession(node.ID, node.Name, clientIP, conn)
	h.agentHub.RegisterSession(sess)
	defer h.agentHub.UnregisterSession(node.ID)

	_ = h.nodeService.UpdateLastSeen(c.Request.Context(), node.ID, clientIP)
	h.triggerSchedule(c.Request.Context())

	for {
		var msg do.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "heartbeat":
			h.processHeartbeat(c.Request.Context(), sess, node.ID, clientIP, msg.Payload)
		case "model_status":
			h.processModelStatus(c.Request.Context(), sess, msg.Payload)
		}
	}
}

type heartbeatPayload struct {
	Models       []string           `json:"models"`
	LoadedModels []string           `json:"loaded_models"`
	RunningJobs  int                `json:"running_jobs"`
	System       *do.SystemStatsDTO `json:"system"`
}

func (h *AgentHandler) processHeartbeat(ctx context.Context, sess *hub.AgentSession, nodeID uint64, ip string, raw any) {
	var payload heartbeatPayload
	if raw != nil {
		if bytes, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(bytes, &payload)
		}
	}

	models := payload.LoadedModels
	if len(models) == 0 {
		models = payload.Models
	}

	sess.UpdateHeartbeat(models, payload.RunningJobs, payload.System)
	_ = h.nodeService.UpdateLastSeen(ctx, nodeID, ip)
	h.triggerSchedule(ctx)
}

type modelStatusPayload struct {
	Models       []string `json:"models"`
	LoadedModels []string `json:"loaded_models"`
}

func (h *AgentHandler) processModelStatus(ctx context.Context, sess *hub.AgentSession, raw any) {
	var payload modelStatusPayload
	if raw != nil {
		if bytes, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(bytes, &payload)
		}
	}

	models := payload.LoadedModels
	if len(models) == 0 {
		models = payload.Models
	}

	if len(models) > 0 {
		sess.UpdateHeartbeat(models, sess.GetRunningJobs(), sess.GetSystemStats())
	}
	h.triggerSchedule(ctx)
}

// DownloadMedia handles GET /api/v1/agent/jobs/:id/media, serving audio files to authorized agents.
func (h *AgentHandler) DownloadMedia(c *gin.Context) {
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

	if job.AudioStoragePath == "" {
		response.AbortNotFound(c, consts.ErrMediaNotFound)
		return
	}

	if h.storageService == nil {
		response.AbortNotFound(c, consts.ErrMediaNotFound)
		return
	}

	obj, err := h.storageService.Get(c.Request.Context(), job.AudioStoragePath)
	if err != nil || obj == nil || obj.Body == nil {
		logger.WarnF(c.Request.Context(), "[AgentHandler] failed to retrieve media from storage: %v", err)
		response.AbortNotFound(c, consts.ErrMediaNotFound)
		return
	}
	defer func() { _ = obj.Body.Close() }()

	contentType := obj.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.DataFromReader(http.StatusOK, obj.ContentLength, contentType, obj.Body, nil)
}

func parseJobIDParam(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return 0, false
	}
	return id, true
}

// AppendLogs handles POST /api/v1/agent/jobs/:id/logs, ingesting batch logs from an agent.
//
//nolint:dupl // batch log ingestion and job completion follow standard handler lifecycle
func (h *AgentHandler) AppendLogs(c *gin.Context) {
	jobID, ok := parseJobIDParam(c)
	if !ok {
		return
	}

	var req do.AgentLogBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	ctx := c.Request.Context()
	if err := h.jobService.AppendLogs(ctx, jobID, &req); err != nil {
		logger.ErrorF(ctx, "[AgentHandler] failed to append logs for job %d: %v", jobID, err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// CompleteJob handles POST /api/v1/agent/jobs/:id/complete, settling final job status and OpenAI results.
//
//nolint:dupl // batch log ingestion and job completion follow standard handler lifecycle
func (h *AgentHandler) CompleteJob(c *gin.Context) {
	jobID, ok := parseJobIDParam(c)
	if !ok {
		return
	}

	var req do.AgentCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	ctx := c.Request.Context()
	if err := h.jobService.CompleteJob(ctx, jobID, &req); err != nil {
		logger.ErrorF(ctx, "[AgentHandler] failed to complete job %d: %v", jobID, err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
