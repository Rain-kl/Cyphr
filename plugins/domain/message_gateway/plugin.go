// Package message_gateway provides the Bot gateway, multi-channel notification dispatching, and asynchronous push worker domain plugin for Cordis.
package message_gateway

import (
	"context"
	"embed"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	appgw "github.com/Rain-kl/Wavelet/internal/apps/message_gateway"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/hibiken/asynq"
)

//go:embed migrations/*.sql
var mgMigrations embed.FS

// Option configures the message_gateway plugin.
type Option func(*Plugin)

// Plugin implements core.Plugin to provide Bot gateway and notification dispatch domain services.
type Plugin struct{}

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
	// 1. Register migrations
	ctx.Migrations().Register("message_gateway", mgMigrations)

	// 2. Register HTTP Routes
	mgGroup := ctx.Router().Group("/api/v1/message-gateway", oauth.LoginRequired())
	{
		mgGroup.GET("/channels", appgw.ListChannels)
		mgGroup.GET("/bindings", appgw.ListBindings)
		mgGroup.POST("/bindings", appgw.BindBinding)
		mgGroup.DELETE("/bindings/:id", appgw.UnbindBinding)
	}

	// 3. Register Asynq background tasks
	ctx.Task().Register("message_gateway:push_notification", func(c context.Context, t *asynq.Task) error {
		return nil
	}, extpoints.WithTaskRetry(3))

	ctx.Task().Register("message_gateway:dispatch_bot_msg", func(c context.Context, t *asynq.Task) error {
		return nil
	})

	// 4. Register Cron Schedules
	ctx.Schedule().RegisterCron("*/10 * * * *", "message_gateway:cleanup_pairing_codes", map[string]any{"action": "cleanup"})

	// 5. Register EventBus listeners for decoupled push triggers
	ctx.Events().On("notification:push", func(c context.Context, e PushNotificationEvent) error {
		// Event triggered push handling
		return nil
	})

	// 6. Register Settings Schemas
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

	return nil
}
