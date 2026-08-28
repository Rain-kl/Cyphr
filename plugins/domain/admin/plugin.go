// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package admin provides the system management console, diagnostics, audit logging, and configuration hot-reloading domain plugin for Cordis.
package admin

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

// Option configures the admin plugin.
type Option func(*Plugin)

// Plugin implements core.Plugin to provide system administration and management APIs.
type Plugin struct{}

// New creates a new admin domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the admin domain plugin.
func (p *Plugin) Name() string {
	return "admin"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "admin",
		Version:     "1.0.0",
		Description: "System administration console, diagnostic monitoring, and configuration hot-reload plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers admin routes, tasks, schedules, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 0. Resolve auth service for middleware (via IoC, not direct import)
	var authSvc contracts.AuthService
	if err := core.Using[contracts.AuthService](ctx, func(svc contracts.AuthService) { authSvc = svc }); err != nil {
		return err
	}
	loginMW := authSvc.RequireAuthMiddleware().(gin.HandlerFunc)
	adminMW := authSvc.RequireAdminMiddleware().(gin.HandlerFunc)

	// 1. Register Admin HTTP Routes
	adminRouter := ctx.Router().Group("/api/v1/admin", loginMW, adminMW)
	{
		// Status & Diagnostics
		adminRouter.GET("/status", GetSystemStatus)
		adminRouter.GET("/status/log-database", GetLogDatabaseStatus)
		adminRouter.GET("/db-info", GetDatabaseInfo)
		adminRouter.GET("/db-export", ExportDatabase)

		// DB Management
		dbGroup := adminRouter.Group("/db-manage")
		{
			dbGroup.GET("/overview", GetDBOverview)
			dbGroup.GET("/tables", ListDBTables)
			dbGroup.GET("/table-data", GetDBTableData)
			dbGroup.POST("/query", ExecuteSQL)
		}

		// Cache Management
		cacheGroup := adminRouter.Group("/cache")
		{
			cacheGroup.GET("/status", GetCacheStatus)
			cacheGroup.POST("/config", UpdateCacheConfig)
			cacheGroup.POST("/clear", ClearCache)
		}

		// Updater
		updateGroup := adminRouter.Group("/update")
		{
			updateGroup.GET("", GetUpdateStatus)
			updateGroup.POST("/apply", ApplyUpdate)
		}

		// Logs
		logsGroup := adminRouter.Group("/logs")
		{
			logsGroup.GET("", GetLogs)
			logsGroup.GET("/access", GetAccessLogs)
			logsGroup.GET("/analytics", GetLogsAnalytics)
			logsGroup.GET("/ws", HandleLogWebSocket)
		}

		// Users
		usersGroup := adminRouter.Group("/users")
		{
			usersGroup.GET("", ListUsers)
			usersGroup.POST("", CreateUser)
			usersGroup.GET("/:id", GetUser)
			usersGroup.PUT("/:id/status", UpdateUserStatus)
			usersGroup.PUT("/:id", UpdateUser)
			usersGroup.DELETE("/:id", DeleteUser)
		}

		// Auth Sources
		authSourcesGroup := adminRouter.Group("/auth-sources")
		{
			authSourcesGroup.GET("", ListAuthSources)
			authSourcesGroup.POST("", CreateAuthSource)
			authSourcesGroup.PUT("/:id", UpdateAuthSource)
			authSourcesGroup.PUT("/:id/toggle", ToggleAuthSource)
			authSourcesGroup.DELETE("/:id", DeleteAuthSource)
		}

		// System Configs
		configGroup := adminRouter.Group("/system-configs")
		{
			configGroup.GET("", ListSystemConfigs)
			configGroup.POST("", CreateSystemConfig)
			configGroup.POST("/smtp/test", TestSMTP)

			keyGroup := configGroup.Group("/:key")
			{
				keyGroup.GET("", GetSystemConfig)
				keyGroup.PUT("", UpdateSystemConfig)
			}
		}

		// Templates
		templateGroup := adminRouter.Group("/templates")
		{
			templateGroup.GET("", ListTemplates)
			templateGroup.POST("", CreateTemplate)

			keyGroup := templateGroup.Group("/:key")
			{
				keyGroup.GET("", GetTemplate)
				keyGroup.PUT("", UpdateTemplate)
				keyGroup.DELETE("", DeleteTemplate)
			}
		}

		// Tasks
		taskGroup := adminRouter.Group("/tasks")
		{
			taskGroup.GET("/types", ListTaskTypes)
			taskGroup.POST("/dispatch", DispatchTask)

			executions := taskGroup.Group("/executions")
			{
				executions.GET("", ListTaskExecutions)
				executions.GET("/:id", GetTaskExecution)
				executions.POST("/:id/retry", RetryTask)
			}

			schedules := taskGroup.Group("/schedules")
			{
				schedules.GET("", ListSchedules)
				schedules.POST("", CreateSchedule)
				schedules.PUT("/:id", UpdateSchedule)
				schedules.DELETE("/:id", DeleteSchedule)
			}
		}
	}

	// 2. Register Background Tasks
	ctx.Task().Register("admin:system_cleanup", func(_ context.Context, _ *asynq.Task) error {
		return nil
	}, extpoints.WithTaskRetry(1))

	// 3. Register Cron Schedules
	ctx.Schedule().RegisterCron("0 4 * * *", "admin:system_cleanup", map[string]string{"type": "daily"})

	// 4. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "admin.system_cleanup_cron",
		Default:     "0 4 * * *",
		Description: "Cron expression for nightly system logs and expired tokens cleanup",
		Type:        "string",
		Category:    "maintenance",
	})

	return nil
}
