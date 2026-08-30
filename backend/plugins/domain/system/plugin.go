// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system provides core system health probes, public config endpoints, and static frontend assets dispatch plugin for Cordis.
package system

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Plugin implements core.Plugin to provide system-level basic routes.
type Plugin struct{}

// New creates a new system domain plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for the system domain plugin.
func (p *Plugin) Name() string {
	return "system"
}

// Inject declares required dependencies for the system domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "system",
		Version:     "1.0.0",
		Description: "System health check, public config, and assets domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers system routes.
func (p *Plugin) Apply(ctx *core.Context) error {
	appName := ctx.Config().String("app.app_name", "Wavelet")

	// 1. Health check
	ctx.Router().GET("/api/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ctx.Router().RegisterWhitelist("/api/healthz")

	// 2. Public config
	ctx.Router().GET("/api/v1/config/public", func(c *gin.Context) {
		if p, err := core.Inject[contracts.PublicConfigProvider](ctx); err == nil && p != nil {
			data, err := p.PublicConfig(c.Request.Context())
			if err != nil {
				logger.ErrorF(c.Request.Context(), "[System] public config provider failed: %v", err)
				response.AbortInternal(c, "public config unavailable")
				return
			}
			c.JSON(http.StatusOK, response.OK(data))
			return
		}
		configs, err := listPublicSystemConfigs(c.Request.Context(), ctx)
		if err != nil {
			logger.ErrorF(c.Request.Context(), "[System] query public system configs failed: %v", err)
		}
		c.JSON(http.StatusOK, response.OK(gin.H{
			"configs": configs,
			"app": gin.H{
				"name": appName,
			},
		}))
	})

	// 3. Custom injection
	ctx.Router().GET("/custom", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK(gin.H{"custom": true}))
	})

	return nil
}
