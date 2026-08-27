// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package risk_control provides the access control, IP rate limiting, and telemetry risk analysis domain plugin for Cordis.
package risk_control

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/gin-gonic/gin"
)

// Option configures the risk_control plugin.
type Option func(*Plugin)

// WithMiddleware configures a custom risk control middleware.
func WithMiddleware(mw gin.HandlerFunc) Option {
	return func(p *Plugin) {
		p.middleware = mw
	}
}

// Plugin implements core.Plugin to provide risk control and access logging middleware.
type Plugin struct {
	middleware gin.HandlerFunc
}

// New creates a new risk_control domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the risk_control domain plugin.
func (p *Plugin) Name() string {
	return "risk_control"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "risk_control",
		Version:     "1.0.0",
		Description: "Access control, IP rate limiting, and access log telemetry domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers risk control middlewares, settings, and cleanup hooks into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. Initialize LogWriter if needed
	InitLogWriter(ctx.GoContext())

	// 2. Register router middleware
	mw := p.middleware
	if mw == nil {
		mw = RiskControlMiddleware()
	}
	ctx.Router().Use(mw)

	// 3. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "risk_control.ip_rate_limit_per_minute",
		Default:     60,
		Description: "Maximum requests allowed per IP per minute",
		Type:        "integer",
		Category:    "security",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "risk_control.enable_access_log",
		Default:     true,
		Description: "Enable structured access log auditing and backpressure queueing",
		Type:        "boolean",
		Category:    "security",
	})

	// 4. Register lifecycle disposal cleanup
	ctx.OnDispose(func() error {
		return StopLogWriter(context.Background())
	})

	return nil
}
