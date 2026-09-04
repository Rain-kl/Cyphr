// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"sync"
	"time"
)

// Controller aggregates HTTP and WebSocket endpoints for the transcribe svr plugin.
type Controller struct {
	mu     sync.RWMutex
	OpenAI *OpenAIHandler
	Job    *JobHandler
	Agent  *AgentHandler
	Node   *NodeHandler
	Model  *ModelHandler

	nodeService service.NodeService
	authService contracts.AuthService
	scheduler   scheduler.Scheduler
}

// Option configures optional services on Controller.
type Option func(*Controller)

// WithScheduler configures the scheduler on the controller.
func WithScheduler(s scheduler.Scheduler) Option {
	return func(c *Controller) {
		c.SetScheduler(s)
	}
}

// WithStorageService configures contracts.StorageService on the controller.
func WithStorageService(s contracts.StorageService) Option {
	return func(c *Controller) {
		if c.OpenAI != nil {
			c.OpenAI.storageService = s
		}
		if c.Agent != nil {
			c.Agent.storageService = s
		}
	}
}

// WithUploadService configures contracts.UploadService on the controller.
func WithUploadService(s contracts.UploadService) Option {
	return func(c *Controller) {
		if c.OpenAI != nil {
			c.OpenAI.uploadService = s
		}
	}
}

// WithSyncTimeout configures synchronous wait timeout on OpenAIHandler.
func WithSyncTimeout(d time.Duration) Option {
	return func(c *Controller) {
		if c.OpenAI != nil {
			c.OpenAI.syncTimeout = d
		}
	}
}

// SetSyncTimeout sets synchronous wait timeout.
func (c *Controller) SetSyncTimeout(d time.Duration) {
	if c.OpenAI != nil {
		c.OpenAI.syncTimeout = d
	}
}

// SetStorageService updates the storage service reference across handlers.
func (c *Controller) SetStorageService(s contracts.StorageService) {
	if c.OpenAI != nil {
		c.OpenAI.storageService = s
	}
	if c.Agent != nil {
		c.Agent.storageService = s
	}
}

// SetUploadService updates the upload service reference on the OpenAI handler.
func (c *Controller) SetUploadService(s contracts.UploadService) {
	if c.OpenAI != nil {
		c.OpenAI.uploadService = s
	}
}

// GetAuthService returns the currently configured AuthService safely.
func (c *Controller) GetAuthService() contracts.AuthService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authService
}

// GetNodeService returns the currently configured NodeService safely.
func (c *Controller) GetNodeService() service.NodeService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodeService
}

// SetAuthService updates the auth service reference across handlers.
func (c *Controller) SetAuthService(a contracts.AuthService) {
	c.mu.Lock()
	c.authService = a
	c.mu.Unlock()
	if c.OpenAI != nil {
		c.OpenAI.authService = a
	}
	if c.Job != nil {
		c.Job.authService = a
	}
}

// SetJobService updates the JobService reference across handlers.
func (c *Controller) SetJobService(s service.JobService) {
	if c.OpenAI != nil {
		c.OpenAI.jobService = s
	}
	if c.Job != nil {
		c.Job.jobService = s
	}
	if c.Agent != nil {
		c.Agent.jobService = s
	}
}

// SetNodeService updates the NodeService reference across handlers.
func (c *Controller) SetNodeService(s service.NodeService) {
	c.mu.Lock()
	c.nodeService = s
	c.mu.Unlock()
	if c.Agent != nil {
		c.Agent.nodeService = s
	}
	if c.Node != nil {
		c.Node.nodeService = s
	}
}

// SetModelDAO updates the ModelDAO reference.
func (c *Controller) SetModelDAO(d dao.ModelDAO) {
	if c.Model != nil {
		c.Model.modelDAO = d
	}
}

// SetAgentHub updates the AgentHub reference across handlers.
func (c *Controller) SetAgentHub(h hub.AgentHub) {
	if c.Agent != nil {
		c.Agent.agentHub = h
	}
	if c.Node != nil {
		c.Node.agentHub = h
	}
}

// SetLogBroker updates the LogBroker reference across handlers.
func (c *Controller) SetLogBroker(b service.LogBroker) {
	if c.OpenAI != nil {
		c.OpenAI.logBroker = b
	}
	if c.Job != nil {
		c.Job.logBroker = b
	}
}

// SetScheduler updates the scheduler reference across handlers.
func (c *Controller) SetScheduler(s scheduler.Scheduler) {
	c.mu.Lock()
	c.scheduler = s
	c.mu.Unlock()
	if c.Agent != nil {
		c.Agent.SetScheduler(s)
	}
}

// New creates a new Controller aggregating all handlers.
func New(
	jobSvc service.JobService,
	nodeSvc service.NodeService,
	modelDAO dao.ModelDAO,
	agentHub hub.AgentHub,
	logBroker service.LogBroker,
	opts ...Option,
) *Controller {
	c := &Controller{
		OpenAI:      NewOpenAIHandler(jobSvc, logBroker),
		Job:         NewJobHandler(jobSvc, logBroker),
		Agent:       NewAgentHandler(agentHub, nodeSvc, jobSvc),
		Node:        NewNodeHandler(nodeSvc, agentHub),
		Model:       NewModelHandler(modelDAO),
		nodeService: nodeSvc,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	return c
}

// RegisterRoutes mounts all transcribe endpoints and registers whitelist paths.
func (c *Controller) RegisterRoutes(router extpoints.RouterExtension) {
	// 1. Register whitelist paths that bypass user login authentication
	router.RegisterWhitelist(
		"/api/v1/agent/*",
	)

	userAuthMW := UserAuthMiddleware(c.GetAuthService)
	agentAuthMW := RequireAgentToken(c.GetNodeService)

	// 2. Transcription endpoint (protected by user authentication)
	router.POST("/api/v1/audio/transcriptions", userAuthMW, c.OpenAI.HandleTranscription)

	// 3. Model listing endpoint (protected by user authentication)
	router.GET("/api/v1/models", userAuthMW, c.Model.ListModels)

	// 4. Job query and streaming endpoints
	jobGroup := router.Group("/api/v1/jobs", userAuthMW)
	{
		jobGroup.GET("", c.Job.ListJobs)
		jobGroup.GET("/:id", c.Job.GetJob)
		jobGroup.GET("/:id/stream", c.Job.StreamJob)
	}

	// 5. Agent WebSocket and worker HTTP endpoints (protected by Agent Token)
	agentGroup := router.Group("/api/v1/agent", agentAuthMW)
	{
		agentGroup.GET("/ws", c.Agent.HandleWS)
		agentGroup.GET("/jobs/:id/media", c.Agent.DownloadMedia)
		agentGroup.POST("/jobs/:id/logs", c.Agent.AppendLogs)
		agentGroup.POST("/jobs/:id/complete", c.Agent.CompleteJob)
	}

	// 6. Node management endpoints (protected by User / Admin Auth)
	nodeGroup := router.Group("/api/v1/controller/nodes", userAuthMW)
	{
		nodeGroup.GET("", c.Node.ListNodes)
		nodeGroup.POST("", c.Node.CreateNode)
		nodeGroup.GET("/:id", c.Node.GetNode)
		nodeGroup.DELETE("/:id", c.Node.DeleteNode)
		nodeGroup.PUT("/:id/config", c.Node.UpdateNodeConfig)
		nodeGroup.POST("/:id/load-model", c.Node.LoadModel)
		nodeGroup.POST("/:id/unload-model", c.Node.UnloadModel)
	}

	// 7. Controller model management endpoints
	modelCtrlGroup := router.Group("/api/v1/controller/models", userAuthMW)
	{
		modelCtrlGroup.GET("", c.Model.ListAllModels)
		modelCtrlGroup.PUT("/:id/status", c.Model.ToggleModelStatus)
	}

	// 8. Controller all jobs endpoint
	jobCtrlGroup := router.Group("/api/v1/controller/jobs", userAuthMW)
	{
		jobCtrlGroup.GET("", c.Job.ListAllJobs)
	}
}
