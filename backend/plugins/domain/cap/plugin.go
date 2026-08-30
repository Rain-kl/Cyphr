// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap provides the proof-of-work (PoW) CAPTCHA verification domain plugin for Cordis.
package cap

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"reflect"
)

// Plugin implements core.Plugin to provide CAPTCHA generation, validation, and route protection.
type Plugin struct{}

// New creates a new cap domain plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for the cap domain plugin.
func (p *Plugin) Name() string {
	return "cap"
}

// Inject declares required dependencies for the cap domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "cap",
		Version:     "1.0.0",
		Description: "Proof-of-work CAPTCHA challenge and verification domain plugin",
		Author:      "Wavelet Team",
	}
}

type capAppConfig struct {
	SessionSecret string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
}

// DeclareConfig declares configuration bindings for the cap plugin.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "app", Target: &capAppConfig{}},
	}
}

// Apply registers the cap routes and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var cfg capAppConfig
	if err := ctx.Config().Bind("app", &cfg); err == nil && cfg.SessionSecret != "" {
		SetSecret([]byte(cfg.SessionSecret))
	}

	// 0. Bind DBService from Context
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		setDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			setDBService(db)
		})
	}
	ctx.OnDispose(func() error {
		setDBService(nil)
		return nil
	})

	// Listen to system config changed events to invalidate cached settings
	ctx.Events().On(contracts.EventTopicConfigChanged, func(_ any) {
		InvalidateRuntimeSettings()
	})

	core.Provide[contracts.CaptchaService](ctx, captchaService{})

	// Register HTTP Routes
	capGroup := ctx.Router().Group("/api/v1/cap")
	{
		capGroup.GET("/challenge", Challenge)
		capGroup.POST("/challenge", Challenge)
		capGroup.POST("/redeem", Redeem)
	}

	legacy := ctx.Router().Group("/api/cap")
	legacy.POST("/challenge", Challenge)
	legacy.POST("/redeem", Redeem)
	ctx.Router().RegisterWhitelist("/api/cap/challenge", "/api/cap/redeem")

	// Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "cap.login_enabled",
		Default:     false,
		Description: "Whether to require CAPTCHA verification for user login",
		Type:        "boolean",
		Category:    "security",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "cap.challenge_count",
		Default:     1,
		Description: "Number of PoW puzzle challenges to solve",
		Type:        "integer",
		Category:    "security",
	})

	return nil
}

type captchaService struct{}

func (captchaService) VerifyMiddleware(scope string) any {
	return VerifyMiddleware(GetDefaultManager(), scope)
}

func (captchaService) ChallengeHandler() any { return Challenge }

func (captchaService) RedeemHandler() any { return Redeem }
