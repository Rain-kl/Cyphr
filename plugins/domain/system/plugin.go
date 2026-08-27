// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system provides core system health probes, public config endpoints, and static frontend assets dispatch plugin for Cordis.
package system

import (
	"net/http"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
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
	// 1. Health check
	ctx.Router().GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ctx.Router().GET("/api/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 2. Public config
	ctx.Router().GET("/api/v1/config/public", func(c *gin.Context) {
		configs, err := repository.ListVisibleSystemConfigs(c.Request.Context())
		if err != nil {
			response.AbortInternal(c, "获取公开配置失败")
			return
		}
		c.JSON(http.StatusOK, response.OK(gin.H{
			"configs": configs,
			"app": gin.H{
				"name": config.Config.App.AppName,
			},
		}))
	})

	// 3. Custom injection
	ctx.Router().GET("/custom", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.OK(gin.H{"custom": true}))
	})

	return nil
}
