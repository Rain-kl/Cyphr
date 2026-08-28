// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap provides the proof-of-work (PoW) CAPTCHA verification domain plugin for Cordis.
package cap

import (
	"Wavelet/core"
	"Wavelet/core/extpoints"
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

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "cap",
		Version:     "1.0.0",
		Description: "Proof-of-work CAPTCHA challenge and verification domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers the cap routes and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// Register HTTP Routes
	capGroup := ctx.Router().Group("/api/v1/cap")
	{
		capGroup.GET("/challenge", Challenge)
		capGroup.POST("/challenge", Challenge)
		capGroup.POST("/redeem", Redeem)
	}

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
