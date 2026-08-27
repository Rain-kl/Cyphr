// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user provides the user profile, credential management, role management, and access token domain plugin for Cordis.
package user

import (
	"context"
	"embed"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	"github.com/hibiken/asynq"
)

//go:embed migrations/*.sql
var userMigrations embed.FS

// Option configures the user plugin.
type Option func(*Plugin)

// WithUserService sets a custom UserService implementation.
func WithUserService(svc contracts.UserService) Option {
	return func(p *Plugin) {
		p.userSvc = svc
	}
}

// Plugin implements core.Plugin to provide user account and credential domain services.
type Plugin struct {
	userSvc contracts.UserService
}

// New creates a new user domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the user domain plugin.
func (p *Plugin) Name() string {
	return "user"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "user",
		Version:     "1.0.0",
		Description: "User profiles, credentials, role management, and access token domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers user migrations, services, routes, tasks, schedules, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. Register migrations
	ctx.Migrations().Register("user", userMigrations)

	// 2. Initialize and provide UserService
	if p.userSvc == nil {
		p.userSvc = newUserService()
	}
	core.Provide[contracts.UserService](ctx, p.userSvc)

	// 3. Register HTTP Routes
	userGroup := ctx.Router().Group("/api/v1/user")
	{
		userGroup.POST("/login", Login)
		userGroup.POST("/register", Register)
		userGroup.GET("/logout", Logout)
		userGroup.POST("/send-email-code", SendEmailCode)
		userGroup.POST("/change-password", auth.LoginRequired(), ChangePassword)
		userGroup.PUT("/profile", auth.LoginRequired(), UpdateProfile)

		// Access Tokens
		tokensGroup := userGroup.Group("/access-tokens", auth.LoginRequired(), auth.DisallowTokenAuth())
		{
			tokensGroup.GET("", ListAccessTokens)
			tokensGroup.POST("", CreateAccessToken)
			tokensGroup.DELETE("/:id", DeleteAccessToken)
			tokensGroup.POST("/:id/rotate", RotateAccessToken)
		}
	}

	const defaultUserTaskRetry = 3

	// 4. Register Asynq background tasks
	ctx.Task().Register("user:send_email_code", func(_ context.Context, _ *asynq.Task) error {
		// Asynq background task handler
		return nil
	}, extpoints.WithTaskRetry(defaultUserTaskRetry))

	ctx.Task().Register("user:cleanup_inactive", func(_ context.Context, _ *asynq.Task) error {
		return nil
	})

	// 5. Register Cron Schedules
	ctx.Schedule().RegisterCron("0 3 * * *", "user:daily_audit", map[string]string{"type": "audit"})

	// 6. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.registration_enabled",
		Default:     true,
		Description: "Whether new user registration is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.password_login_enabled",
		Default:     true,
		Description: "Whether password login is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.min_password_length",
		Default:     8,
		Description: "Minimum password length required for user accounts",
		Type:        "integer",
		Category:    "security",
	})

	return nil
}
