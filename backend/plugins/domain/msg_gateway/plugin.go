// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package msg_gateway provides the Bot gateway, multi-channel notification dispatching, and asynchronous push worker domain plugin for Cordis.
package msg_gateway

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/channels/qq"
	"Wavelet/plugins/domain/msg_gateway/channels/telegram"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/controller"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"Wavelet/plugins/domain/msg_gateway/service"
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*/*.sql
var mgMigrations embed.FS

// Option configures the msg_gateway plugin.
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

// New creates a new msg_gateway domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the msg_gateway domain plugin.
func (p *Plugin) Name() string {
	return "msg_gateway"
}

// Inject declares required dependencies for the msg_gateway domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		// AuthService is captured as a middleware value in Apply, so it cannot
		// be late-bound with core.When like the other services below; the
		// kernel must mount auth first or the routes get a pass-through guard.
		reflect.TypeFor[contracts.AuthService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "msg_gateway",
		Version:     "1.0.0",
		Description: "Bot gateway, multi-channel notification push, and async worker dispatch plugin",
		Author:      "Wavelet Team",
	}
}

type mgAppConfig struct {
	SessionSecret string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
}

// DeclareConfig declares configuration bindings for the msg_gateway plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "app", Target: &mgAppConfig{}},
	}
}

// Apply registers msg_gateway migrations, routes, tasks, schedules, events, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg mgAppConfig
	if err := ctx.Config().Bind("app", &cfg); err == nil && cfg.SessionSecret != "" {
		service.SetCredentialSecret(cfg.SessionSecret)
	}
	core.Bind[contracts.DBService](ctx, dao.SetDBService)
	core.Bind[contracts.CacheService](ctx, func(cache contracts.CacheService) {
		dao.SetCacheService(cache)
		service.SetCacheService(cache)
	})
	core.Bind[contracts.TaskService](ctx, service.SetTaskService)
	core.Bind[contracts.UserService](ctx, service.SetUserService)
	ctx.OnDispose(func() error {
		dao.SetDBService(nil)
		dao.SetCacheService(nil)
		service.SetCacheService(nil)
		service.SetTaskService(nil)
		service.SetUserService(nil)
		return nil
	})

	// 0. Resolve auth service for middleware (via IoC, not direct import)
	denyAuth := ginutil.AuthUnavailable()
	loginMW := denyAuth
	adminMW := denyAuth
	if authSvc, err := core.Inject[contracts.AuthService](ctx); err == nil && authSvc != nil {
		if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
			loginMW = mw
		}
		if mw, ok := authSvc.RequireAdminMiddleware().(gin.HandlerFunc); ok {
			adminMW = mw
		}
	}

	// 1. Register migrations
	ctx.Migrations().Register("msg_gateway", mgMigrations)

	// 2. Register User HTTP Routes
	controller.RegisterUserRoutes(ctx.Router().Group("/api/v1"), loginMW)

	// 3. Register Admin Message Gateway HTTP Routes
	controller.RegisterAdminRoutes(ctx.Router().Group("/api/v1/admin"), loginMW, adminMW)

	// 4. Register Admin Push HTTP Routes
	controller.RegisterAdminPushRoutes(ctx.Router().Group("/api/v1/admin"), loginMW, adminMW)

	service.Register(consts.MessageChannelTypeTelegram, telegram.New)
	service.Register(consts.MessageChannelTypeQQ, qq.New)

	const defaultTaskRetry = 3
	pushHandler := &service.PushHandler{}

	// 5. Register background tasks
	ctx.Task().Register(consts.TaskPushNotification, func(c context.Context, payload []byte) error {
		return pushHandler.Execute(c, payload)
	},
		extpoints.WithTaskType("push_notification"),
		extpoints.WithTaskName("消息网关推送通知"),
		extpoints.WithTaskDescription("异步执行系统通知的多渠道派发与推送"),
		extpoints.WithTaskCategory("push"),
		extpoints.WithTaskRetry(defaultTaskRetry),
		extpoints.WithTaskQueue("default"),
		extpoints.WithTaskRetryable(true),
	)

	ctx.Task().Register(service.SendNotificationTask, func(c context.Context, payload []byte) error {
		return pushHandler.Execute(c, payload)
	}, extpoints.WithTaskMeta(service.SendNotificationMeta), extpoints.WithTaskRetry(defaultTaskRetry))

	ctx.Task().Register(service.TaskDispatchBotMsg, &service.BotDispatchHandler{},
		extpoints.WithTaskMeta(service.BotDispatchMeta))

	ctx.Task().Register(consts.TaskCleanupPairingCodes, func(c context.Context, _ []byte) error {
		return dao.DeleteExpiredPairingCodes(c)
	},
		extpoints.WithTaskType("cleanup_pairing_codes"),
		extpoints.WithTaskName("清理过期配对码"),
		extpoints.WithTaskDescription("定时清理已过期的平台 Bot 配对码"),
		extpoints.WithTaskCategory("messaging"),
		extpoints.WithTaskRetry(defaultTaskRetry),
		extpoints.WithTaskQueue("default"),
		extpoints.WithTaskRetryable(true),
	)

	// 6. Register Cron Schedules
	ctx.Schedule().RegisterCron("*/10 * * * *", consts.TaskCleanupPairingCodes, map[string]any{"action": "cleanup"})

	// 7. Register EventBus listeners for decoupled push triggers
	ctx.Events().On("notification:push", func(c context.Context, e do.PushNotificationEvent) error {
		meta := do.EventMetadata{
			Key:  "eventbus:" + e.Channel,
			Name: e.Title,
			DefaultTemplate: do.NotificationMessage{
				Title:   e.Title,
				Content: e.Content,
				Level:   consts.DefaultLevelInfo,
				Ext:     e.Metadata,
			},
			Description: "EventBus triggered notification",
		}
		service.DefaultTrigger.Trigger(c, meta, map[string]any{
			"user.id": e.UserID,
			"title":   e.Title,
			"content": e.Content,
		})
		return nil
	})

	// 8. Register task completed event listener
	ctx.Events().On(contracts.EventTopicTaskCompleted, func(c context.Context, e contracts.TaskCompletedEvent) error {
		service.HandleTaskCompleted(c, e)
		return nil
	})

	// 9. Register built-in domain events and provide PushRegistry
	service.RegisterCustomEvents()
	core.Provide[contracts.PushRegistry](ctx, service.PushRegistryAdapter{})

	// 10. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "msg_gateway.pairing_code_expiry_minutes",
		Default:     15,
		Description: "Expiry duration for bot pairing codes in minutes",
		Type:        "integer",
		Category:    "messaging",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "msg_gateway.max_bindings_per_user",
		Default:     5,
		Description: "Maximum platform bot bindings per user",
		Type:        "integer",
		Category:    "messaging",
	})

	// 11. Optional runner start & lifecycle
	if p.autoStartRunner {
		runnerCtx, cancel := context.WithCancel(ctx.GoContext())
		p.cancelRunner = cancel
		util.Go(func() {
			_ = service.Start(runnerCtx)
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

// Re-exported constants.
const (
	CodeAlphabet = service.CodeAlphabet
	CodeLength   = service.CodeLength
)

// MessageChannel is an alias for entity.MessageChannel.
type MessageChannel = entity.MessageChannel

// MessageBinding is an alias for entity.MessageBinding.
type MessageBinding = entity.MessageBinding

// MessagePairingCode is an alias for entity.MessagePairingCode.
type MessagePairingCode = entity.MessagePairingCode

// PushChannel is an alias for entity.PushChannel.
type PushChannel = entity.PushChannel

// PushEvent is an alias for entity.PushEvent.
type PushEvent = entity.PushEvent

// PushHistory is an alias for entity.PushHistory.
type PushHistory = entity.PushHistory

// PushNotificationEvent is an alias for do.PushNotificationEvent.
type PushNotificationEvent = do.PushNotificationEvent

// ChannelConfig is an alias for do.ChannelConfig.
type ChannelConfig = do.ChannelConfig

// Capability is an alias for do.Capability.
type Capability = do.Capability

// Recipient is an alias for do.Recipient.
type Recipient = do.Recipient

// Attachment is an alias for do.Attachment.
type Attachment = do.Attachment

// InboundMessage is an alias for do.InboundMessage.
type InboundMessage = do.InboundMessage

// OutboundMessage is an alias for do.OutboundMessage.
type OutboundMessage = do.OutboundMessage

// BindingDTO is an alias for do.BindingDTO.
type BindingDTO = do.BindingDTO

// PublicChannelDTO is an alias for do.PublicChannelDTO.
type PublicChannelDTO = do.PublicChannelDTO

// Definition is an alias for do.Definition.
type Definition = do.Definition

// ChannelDTO is an alias for do.ChannelDTO.
type ChannelDTO = do.ChannelDTO

// CreateChannelRequest is an alias for do.CreateChannelRequest.
type CreateChannelRequest = do.CreateChannelRequest

// UpdateChannelRequest is an alias for do.UpdateChannelRequest.
type UpdateChannelRequest = do.UpdateChannelRequest

// PushDefinition is an alias for do.PushDefinition.
type PushDefinition = do.PushDefinition

// PushField is an alias for do.PushField.
type PushField = do.PushField

// NotificationMessage is an alias for do.NotificationMessage.
type NotificationMessage = do.NotificationMessage

// EventMetadata is an alias for do.EventMetadata.
type EventMetadata = do.EventMetadata

// SendPayload is an alias for do.SendPayload.
type SendPayload = do.SendPayload

// Handler is an alias for service.Handler.
type Handler = service.Handler

// Factory is an alias for service.Factory.
type Factory = service.Factory

// Channel is an alias for service.Channel.
type Channel = service.Channel

// Runner is an alias for service.Runner.
type Runner = service.Runner

// EventTrigger is an alias for service.EventTrigger.
type EventTrigger = service.EventTrigger

// PushHandler is an alias for service.PushHandler.
type PushHandler = service.PushHandler

// Re-exported variables and functions.
var (
	SetDBServiceForTest = dao.SetDBServiceForTest
	UpsertPairingCode   = dao.UpsertPairingCode
	Register            = service.Register
	Lookup              = service.Lookup
	GenerateCode        = service.GenerateCode
	NormalizeCode       = service.NormalizeCode
	FormatCode          = service.FormatCode
	Start               = service.Start
	Stop                = service.Stop
	GlobalRunner        = service.GlobalRunner
	DefaultTrigger      = service.DefaultTrigger
	SyncEvents          = service.SyncEvents
	AdminLogin          = service.AdminLogin
	HandleAdminLoggedIn = service.HandleAdminLoggedIn
)
