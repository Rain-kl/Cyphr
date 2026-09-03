// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package svr provides the Transcribe server controller plugin.
package svr

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/transcribe/plugins/svr/controller"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"context"
	"embed"
	"sync"
	"time"

	"gorm.io/gorm"
)

//go:embed migrations/*
var migrationsFS embed.FS

const (
	watchdogInterval = 5 * time.Second
	offlineTimeout   = 15 * time.Second
)

// Plugin implements core.Plugin for transcribe server plugin.
type Plugin struct {
	mu           sync.RWMutex
	watchdogOnce sync.Once
	appCtx       context.Context
	db           *gorm.DB
	storageSvc   contracts.StorageService
	uploadSvc    contracts.UploadService
	authSvc      contracts.AuthService
	agentHub     hub.AgentHub
	logBroker    service.LogBroker
	scheduler    scheduler.Scheduler
	jobService   service.JobService
	nodeService  service.NodeService
	modelDAO     dao.ModelDAO
	nodeDAO      dao.NodeDAO
	jobDAO       dao.JobDAO
	ctrl         *controller.Controller
}

// Option configures Plugin instances.
type Option func(*Plugin)

// WithDB allows injecting custom DB instance (useful for unit tests).
func WithDB(db *gorm.DB) Option {
	return func(p *Plugin) {
		p.db = db
	}
}

// WithAgentHub allows injecting custom AgentHub instance.
func WithAgentHub(h hub.AgentHub) Option {
	return func(p *Plugin) {
		p.agentHub = h
	}
}

// WithLogBroker allows injecting custom LogBroker instance.
func WithLogBroker(b service.LogBroker) Option {
	return func(p *Plugin) {
		p.logBroker = b
	}
}

// WithJobService allows injecting custom JobService instance.
func WithJobService(s service.JobService) Option {
	return func(p *Plugin) {
		p.jobService = s
	}
}

// WithNodeService allows injecting custom NodeService instance.
func WithNodeService(s service.NodeService) Option {
	return func(p *Plugin) {
		p.nodeService = s
	}
}

// WithModelDAO allows injecting custom ModelDAO instance.
func WithModelDAO(d dao.ModelDAO) Option {
	return func(p *Plugin) {
		p.modelDAO = d
	}
}

// WithScheduler allows injecting custom Scheduler instance.
func WithScheduler(s scheduler.Scheduler) Option {
	return func(p *Plugin) {
		p.scheduler = s
	}
}

// New creates a new transcribe_svr plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for this plugin.
func (p *Plugin) Name() string {
	return "transcribe_svr"
}

// Manifest returns plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "transcribe_svr",
		Version:     "1.0.0",
		Description: "Transcribe server controller, OpenAI audio handler, and agent WebSocket hub",
		Author:      "Arctel Team",
	}
}

func (p *Plugin) initDAOsAndServices(db *gorm.DB) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.db = db
	if p.jobDAO == nil {
		p.jobDAO = dao.NewJobDAO(db)
	}
	if p.nodeDAO == nil {
		p.nodeDAO = dao.NewNodeDAO(db)
	}
	if p.modelDAO == nil {
		p.modelDAO = dao.NewModelDAO(db)
	}
	if p.logBroker == nil {
		p.logBroker = service.NewLogBroker()
	}
	if p.agentHub == nil {
		p.agentHub = hub.NewAgentHub(p.jobDAO)
	}
	if p.nodeService == nil {
		p.nodeService = service.NewNodeService(p.nodeDAO, p.agentHub)
	}
	if p.scheduler == nil {
		p.scheduler = scheduler.NewScheduler(p.jobDAO, p.agentHub)
	}
	if p.jobService == nil {
		p.jobService = service.NewJobService(p.jobDAO, p.modelDAO, p.scheduler, p.logBroker, p.agentHub)
	}

	if p.ctrl != nil {
		p.ctrl.SetJobService(p.jobService)
		p.ctrl.SetNodeService(p.nodeService)
		p.ctrl.SetModelDAO(p.modelDAO)
		p.ctrl.SetAgentHub(p.agentHub)
		p.ctrl.SetLogBroker(p.logBroker)
		p.ctrl.SetScheduler(p.scheduler)
	}

	p.startWatchdog()
}

func (p *Plugin) startWatchdog() {
	if p.agentHub == nil {
		return
	}
	p.watchdogOnce.Do(func() {
		watchCtx := p.appCtx
		if watchCtx == nil {
			watchCtx = context.Background()
		}
		p.agentHub.StartWatchdog(watchCtx, watchdogInterval, offlineTimeout)
	})
}

// Apply registers routes, migrations and services into the Cordis micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	p.mu.Lock()
	p.appCtx = ctx.GoContext()
	p.mu.Unlock()

	// 1. Register Goose migrations embedded from migrations/
	ctx.Migrations().Register("transcribe_svr", migrationsFS)

	// 2. Register route whitelist so agent endpoints and public API are not intercepted with 401
	ctx.Router().RegisterWhitelist(
		"/api/v1/agent/*",
		"/api/v1/audio/transcriptions",
		"/api/v1/models",
	)

	// 3. Resolve or bind DBService
	if p.db != nil {
		p.initDAOsAndServices(p.db)
	} else if dbSvc, err := core.Inject[contracts.DBService](ctx); err == nil && dbSvc != nil && dbSvc.GORM() != nil {
		p.initDAOsAndServices(dbSvc.GORM())
	}

	core.Bind[contracts.DBService](ctx, func(svc contracts.DBService) {
		if svc != nil && svc.GORM() != nil {
			p.initDAOsAndServices(svc.GORM())
		}
	})

	p.bindExternalServices(ctx)

	// Ensure in-memory broker exists even if DB is bound asynchronously
	if p.logBroker == nil {
		p.logBroker = service.NewLogBroker()
	}

	// 6. Initialize Controller and mount routes
	p.ctrl = controller.New(
		p.jobService,
		p.nodeService,
		p.modelDAO,
		p.agentHub,
		p.logBroker,
	)
	if p.scheduler != nil {
		p.ctrl.SetScheduler(p.scheduler)
	}
	if p.storageSvc != nil {
		p.ctrl.SetStorageService(p.storageSvc)
	}
	if p.uploadSvc != nil {
		p.ctrl.SetUploadService(p.uploadSvc)
	}
	if p.authSvc != nil {
		p.ctrl.SetAuthService(p.authSvc)
	}
	p.ctrl.RegisterRoutes(ctx.Router())

	// 7. Start AgentHub watchdog
	p.startWatchdog()

	ctx.OnDispose(func() error {
		p.mu.RLock()
		hubInstance := p.agentHub
		p.mu.RUnlock()
		if hubInstance != nil {
			hubInstance.Stop()
		}
		return nil
	})

	return nil
}

// bindExternalServices resolves StorageService, UploadService and AuthService from
// the context eagerly (if already provided) and reactively via core.Bind.
func (p *Plugin) bindExternalServices(ctx *core.Context) {
	if svc, err := core.Inject[contracts.StorageService](ctx); err == nil && svc != nil {
		p.storageSvc = svc
	}
	core.Bind[contracts.StorageService](ctx, func(svc contracts.StorageService) {
		p.mu.Lock()
		p.storageSvc = svc
		p.mu.Unlock()
		if p.ctrl != nil {
			p.ctrl.SetStorageService(svc)
		}
	})

	if svc, err := core.Inject[contracts.UploadService](ctx); err == nil && svc != nil {
		p.uploadSvc = svc
	}
	core.Bind[contracts.UploadService](ctx, func(svc contracts.UploadService) {
		p.mu.Lock()
		p.uploadSvc = svc
		p.mu.Unlock()
		if p.ctrl != nil {
			p.ctrl.SetUploadService(svc)
		}
	})

	if svc, err := core.Inject[contracts.AuthService](ctx); err == nil && svc != nil {
		p.authSvc = svc
	}
	core.Bind[contracts.AuthService](ctx, func(svc contracts.AuthService) {
		p.mu.Lock()
		p.authSvc = svc
		p.mu.Unlock()
		if p.ctrl != nil {
			p.ctrl.SetAuthService(svc)
		}
	})
}
