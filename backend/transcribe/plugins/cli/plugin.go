// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package custom_example demonstrates how to build a downstream Cordis plugin.
// Copy this directory to create your own plugin.
package controller

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Plugin implements core.Plugin for the custom_example downstream plugin.
type Plugin struct{}

// New creates a new custom_example plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for this plugin.
func (p *Plugin) Name() string {
	return "custom_example"
}

// Apply registers routes and services into the Cordis micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// Resolve platform services via IoC container (no direct imports of domain plugins).
	var authSvc contracts.AuthService
	if err := core.Using[contracts.AuthService](ctx, func(svc contracts.AuthService) { authSvc = svc }); err != nil {
		return err
	}
	_ = authSvc

	// Register routes using the auth middleware obtained through the contract.
	g := ctx.Router().Group("/api/v1/custom", authSvc.RequireAuthMiddleware().(gin.HandlerFunc))
	g.GET("/hello", func(c *gin.Context) {
		user, err := authSvc.GetCurrentUser(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Hello " + user.Username})
	})

	return nil
}
