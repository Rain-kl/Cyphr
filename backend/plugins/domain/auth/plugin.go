// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package auth provides the authentication, OAuth, session management, and access token domain plugin for Cordis.
package auth

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"context"
	"embed"
	"reflect"
)

//go:embed migrations/*/*.sql
var authMigrations embed.FS

// Option configures the auth plugin.
type Option func(*Plugin)

// WithAuthService sets a custom AuthService implementation.
func WithAuthService(svc contracts.AuthService) Option {
	return func(p *Plugin) {
		p.authSvc = svc
	}
}

// WithAuthRegistry sets a custom AuthRegistry implementation.
func WithAuthRegistry(reg contracts.AuthRegistry) Option {
	return func(p *Plugin) {
		p.authRegistry = reg
	}
}

// Plugin implements core.Plugin to provide authentication and OAuth domain services.
type Plugin struct {
	authSvc      contracts.AuthService
	authRegistry contracts.AuthRegistry
}

// New creates a new auth domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the auth domain plugin.
func (p *Plugin) Name() string {
	return "auth"
}

// Inject declares required dependencies for the auth domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "auth",
		Version:     "1.0.0",
		Description: "Authentication, OAuth, Session and Passkey domain plugin",
		Author:      "Wavelet Team",
	}
}

// DeclareConfig declares configuration bindings for the auth plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "app", Target: &SessionConfig{}},
	}
}

// Apply registers the auth migrations, services, routes, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg SessionConfig
	if err := ctx.Config().Bind("app", &cfg); err == nil {
		SetSessionConfig(cfg)
	}

	core.Bind[contracts.DBService](ctx, setDBService)
	core.Bind[contracts.CacheService](ctx, setCacheService)
	core.Bind[contracts.LimiterService](ctx, setLimiterService)
	ctx.OnDispose(func() error {
		setDBService(nil)
		setCacheService(nil)
		setLimiterService(nil)
		return nil
	})

	// 1. Register migrations
	ctx.Migrations().Register("auth", authMigrations)

	// 2. Initialize and provide AuthService & AuthRegistry
	if p.authSvc == nil {
		p.authSvc = newAuthService()
	}
	if p.authRegistry == nil {
		p.authRegistry = newAuthRegistry()
	}

	core.Provide[contracts.AuthService](ctx, p.authSvc)
	core.Provide[contracts.AuthRegistry](ctx, p.authRegistry)

	// 2.1 Register Public / Auth Whitelist Endpoints
	publicEndpoints := []string{
		"/api/v1/oauth/sources",
		"/api/v1/oauth/login",
		"/api/v1/oauth/*/authorize",
		"/api/v1/oauth/:source/authorize",
		"/api/v1/oauth/callback",
		"/api/v1/user/login",
		"/api/v1/user/register",
		"/api/v1/user/send-email-code",
		"/api/v1/config/public",
		"/api/v1/cap/challenge",
		"/api/v1/cap/redeem",
		"/api/healthz",
		"/metrics",
	}
	RegisterWhitelist(publicEndpoints...)
	ctx.Router().RegisterWhitelist(publicEndpoints...)

	// 3. Register HTTP Routes
	oauthGroup := ctx.Router().Group("/api/v1/oauth")
	{
		oauthGroup.GET("/sources", GetLoginSources)
		oauthGroup.GET("/login", GetLoginURL)
		oauthGroup.GET("/:source/authorize", Authorize)
		oauthGroup.GET("/logout", Logout)
		oauthGroup.POST("/callback", Callback)
		oauthGroup.GET("/user-info", LoginRequired(), UserInfo)
		oauthGroup.GET("/external-accounts", LoginRequired(), ListExternalAccounts)
		oauthGroup.POST("/external-accounts/:id/delete", LoginRequired(), DeleteExternalAccount)
	}
	ctx.Router().GET("/api/v1/user-info", LoginRequired(), UserInfo)

	// 4. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "auth.session_age",
		Default:     86400 * 7,
		Description: "Default session lifetime in seconds",
		Type:        "integer",
		Category:    "security",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "auth.login_rate_limit_max_attempts",
		Default:     5,
		Description: "Max login failure attempts before temporary IP lock",
		Type:        "integer",
		Category:    "security",
	})

	// 5. Register Event Listeners for domain events
	ctx.Events().On(contracts.EventTopicUserStatusChanged, func(c context.Context, e contracts.UserStatusChangedEvent) error {
		InvalidateCachedUser(c, e.UserID)
		return nil
	})

	ctx.Events().On(contracts.EventTopicUserDeleted, func(c context.Context, e contracts.UserDeletedEvent) error {
		InvalidateCachedUser(c, e.TargetUserID)
		return nil
	})

	return nil
}
