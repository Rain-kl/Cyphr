// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package system provides core system health probes, public config endpoints, and static frontend assets dispatch plugin for Cordis.
package system

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/config"
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
	// 1. Health check
	ctx.Router().GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	ctx.Router().GET("/api/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 2. Public config
	ctx.Router().GET("/api/v1/config/public", func(c *gin.Context) {
		type configItem struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		var configs []configItem
		if dbSvc, err := core.Inject[contracts.DBService](ctx); err == nil && dbSvc != nil {
			_ = dbSvc.DB(c.Request.Context()).Table("w_system_configs").Where("visibility = ?", "visible").Find(&configs).Error
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
