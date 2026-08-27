// Package auth provides the authentication, OAuth, session management, and access token domain plugin for Cordis.
package auth

import (
	"embed"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
)

//go:embed migrations/*.sql
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

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "auth",
		Version:     "1.0.0",
		Description: "Authentication, OAuth, Session and Passkey domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers the auth migrations, services, routes, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
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

	// 3. Register HTTP Routes
	oauthGroup := ctx.Router().Group("/api/v1/oauth")
	{
		oauthGroup.GET("/sources", oauth.GetLoginSources)
		oauthGroup.GET("/login", oauth.GetLoginURL)
		oauthGroup.GET("/:source/authorize", oauth.Authorize)
		oauthGroup.GET("/logout", oauth.Logout)
		oauthGroup.POST("/callback", oauth.Callback)
		oauthGroup.GET("/user-info", oauth.LoginRequired(), oauth.UserInfo)
		oauthGroup.GET("/external-accounts", oauth.LoginRequired(), oauth.ListExternalAccounts)
		oauthGroup.POST("/external-accounts/:id/delete", oauth.LoginRequired(), oauth.DeleteExternalAccount)
	}
	ctx.Router().GET("/api/v1/user-info", oauth.LoginRequired(), oauth.UserInfo)

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

	return nil
}
