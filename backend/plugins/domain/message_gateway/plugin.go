// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package message_gateway provides the Bot gateway, multi-channel notification dispatching, and asynchronous push worker domain plugin for Cordis.
package message_gateway

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/util"
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*/*.sql
var mgMigrations embed.FS

// Option configures the message_gateway plugin.
type Option func(*Plugin)

// WithAutoStartRunner enables automatic bot runner startup in the background.
func WithAutoStartRunner(enable bool) Option {
	return func(p *Plugin) {
		p.autoStartRunner = enable
	}
}

// Plugin implements core.Plugin to provide Bot gateway and notification dispatch domain services.
type Plugin struct {
	autoStartRunner bool
	cancelRunner    context.CancelFunc
}

// New creates a new message_gateway domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the message_gateway domain plugin.
func (p *Plugin) Name() string {
	return "message_gateway"
}

// Inject declares required dependencies for the message_gateway domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "message_gateway",
		Version:     "1.0.0",
		Description: "Bot gateway, multi-channel notification push, and async worker dispatch plugin",
		Author:      "Wavelet Team",
	}
}

// PushNotificationEvent defines the payload for eventbus notification trigger.
type PushNotificationEvent struct {
	UserID   uint64         `json:"user_id"`
	Channel  string         `json:"channel"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Apply registers message_gateway migrations, routes, tasks, schedules, events, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 0. Bind DBService, CacheService, TaskService
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		setDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			setDBService(db)
		})
	}
	if cache, err := core.Inject[contracts.CacheService](ctx); err == nil && cache != nil {
		setCacheService(cache)
	} else {
		core.When[contracts.CacheService](ctx, func(cache contracts.CacheService) {
			setCacheService(cache)
		})
	}
	if taskSvc, err := core.Inject[contracts.TaskService](ctx); err == nil && taskSvc != nil {
		setTaskService(taskSvc)
	} else {
		core.When[contracts.TaskService](ctx, func(taskSvc contracts.TaskService) {
			setTaskService(taskSvc)
		})
	}
	ctx.OnDispose(func() error {
		setDBService(nil)
		setCacheService(nil)
		setTaskService(nil)
		return nil
	})

	// 0. Resolve auth service for middleware (via IoC, not direct import)
	var loginMW gin.HandlerFunc = func(c *gin.Context) { c.Next() }
	var adminMW gin.HandlerFunc = func(c *gin.Context) { c.Next() }
	if authSvc, err := core.Inject[contracts.AuthService](ctx); err == nil && authSvc != nil {
		if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
			loginMW = mw
		}
		if mw, ok := authSvc.RequireAdminMiddleware().(gin.HandlerFunc); ok {
			adminMW = mw
		}
	}

	// 1. Register migrations
	ctx.Migrations().Register("message_gateway", mgMigrations)

	// 2. Register User HTTP Routes
	mgGroup := ctx.Router().Group("/api/v1/message-gateway", loginMW)
	{
		mgGroup.GET("/channels", ListChannels)
		mgGroup.GET("/bindings", ListBindings)
		mgGroup.POST("/bindings", BindBinding)
		mgGroup.DELETE("/bindings/:id", UnbindBinding)
	}

	// 3. Register Admin Message Gateway HTTP Routes
	adminMgGroup := ctx.Router().Group("/api/v1/admin/message-gateway", loginMW, adminMW)
	{
		adminMgGroup.GET("/channels/definitions", ListAdminChannelDefinitions)
		adminMgGroup.GET("/channels", ListAdminChannels)
		adminMgGroup.POST("/channels", CreateAdminChannel)
		adminMgGroup.PATCH("/channels/:id", UpdateAdminChannel)
		adminMgGroup.DELETE("/channels/:id", DeleteAdminChannel)
		adminMgGroup.POST("/channels/:id/test", TestAdminChannel)
	}

	// 4. Register Admin Push HTTP Routes
	adminPushGroup := ctx.Router().Group("/api/v1/admin/push", loginMW, adminMW)
	{
		events := adminPushGroup.Group("/events")
		{
			events.GET("", ListPushEvents)
			events.GET("/builtin", ListBuiltInPushEvents)
			events.POST("", CreatePushEvent)
			events.PUT("/:id", UpdatePushEvent)
			events.DELETE("/:id", DeletePushEvent)
			events.POST("/:id/toggle", TogglePushEvent)
		}

		adminPushGroup.GET("/histories", ListPushHistories)
		adminPushGroup.POST("/test", TestPush)

		channels := adminPushGroup.Group("/channels")
		{
			channels.GET("/definitions", ListPushChannelDefinitions)
			channels.GET("", ListPushChannels)
			channels.POST("", CreatePushChannel)
			channels.PUT("/:id", UpdatePushChannel)
			channels.DELETE("/:id", DeletePushChannel)
			channels.POST("/test", TestPushChannel)
		}
	}

	const defaultTaskRetry = 3
	pushHandler := &PushHandler{}

	// 5. Register background tasks
	ctx.Task().Register("message_gateway:push_notification", func(c context.Context, payload []byte) error {
		return pushHandler.Execute(c, payload)
	}, extpoints.WithTaskRetry(defaultTaskRetry))

	ctx.Task().Register(SendNotificationTask, func(c context.Context, payload []byte) error {
		return pushHandler.Execute(c, payload)
	}, extpoints.WithTaskRetry(defaultTaskRetry))

	ctx.Task().Register("message_gateway:dispatch_bot_msg", func(_ context.Context, _ []byte) error {
		return nil
	})

	// 6. Register Cron Schedules
	ctx.Schedule().RegisterCron("*/10 * * * *", "message_gateway:cleanup_pairing_codes", map[string]any{"action": "cleanup"})

	// 7. Register EventBus listeners for decoupled push triggers
	ctx.Events().On("notification:push", func(c context.Context, e PushNotificationEvent) error {
		meta := EventMetadata{
			Key:  "eventbus:" + e.Channel,
			Name: e.Title,
			DefaultTemplate: NotificationMessage{
				Title:   e.Title,
				Content: e.Content,
				Level:   defaultLevelInfo,
				Ext:     e.Metadata,
			},
			Description: "EventBus triggered notification",
		}
		DefaultTrigger.Trigger(c, meta, map[string]any{
			"user.id": e.UserID,
			"title":   e.Title,
			"content": e.Content,
		})
		return nil
	})

	// 8. Register task completed event listener
	ctx.Events().On(contracts.EventTopicTaskCompleted, func(c context.Context, e contracts.TaskCompletedEvent) error {
		handleTaskCompleted(c, e)
		return nil
	})

	// 9. Register built-in domain events
	RegisterCustomEvents()

	// 9. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "message_gateway.pairing_code_expiry_minutes",
		Default:     15,
		Description: "Expiry duration for bot pairing codes in minutes",
		Type:        "integer",
		Category:    "messaging",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "message_gateway.max_bindings_per_user",
		Default:     5,
		Description: "Maximum platform bot bindings per user",
		Type:        "integer",
		Category:    "messaging",
	})

	// 10. Optional runner start & lifecycle
	if p.autoStartRunner {
		runnerCtx, cancel := context.WithCancel(ctx.GoContext())
		p.cancelRunner = cancel
		util.Go(func() {
			_ = Start(runnerCtx)
		})
	}

	ctx.OnDispose(func() error {
		if p.cancelRunner != nil {
			p.cancelRunner()
		}
		return nil
	})

	return nil
}
