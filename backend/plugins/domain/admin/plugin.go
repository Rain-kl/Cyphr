// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package admin provides the system management console, diagnostics, audit logging, and configuration hot-reloading domain plugin for Cordis.
package admin

import (
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
)

//go:embed migrations/*.sql
var adminMigrations embed.FS

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

// Inject declares required dependencies for the admin domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
		reflect.TypeFor[contracts.UserService](),
		reflect.TypeFor[contracts.AuthService](),
	}
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
	// 0. Bind Services reactively
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		SetDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			SetDBService(db)
		})
	}
	if cache, err := core.Inject[contracts.CacheService](ctx); err == nil && cache != nil {
		SetCacheService(cache)
	} else {
		core.When[contracts.CacheService](ctx, func(cache contracts.CacheService) {
			SetCacheService(cache)
		})
	}
	if user, err := core.Inject[contracts.UserService](ctx); err == nil && user != nil {
		SetUserService(user)
	} else {
		core.When[contracts.UserService](ctx, func(user contracts.UserService) {
			SetUserService(user)
		})
	}
	if auth, err := core.Inject[contracts.AuthService](ctx); err == nil && auth != nil {
		SetAuthService(auth)
	} else {
		core.When[contracts.AuthService](ctx, func(auth contracts.AuthService) {
			SetAuthService(auth)
		})
	}
	if task, err := core.Inject[contracts.TaskService](ctx); err == nil && task != nil {
		SetTaskService(task)
	} else {
		core.When[contracts.TaskService](ctx, func(task contracts.TaskService) {
			SetTaskService(task)
		})
	}
	if storage, err := core.Inject[contracts.StorageService](ctx); err == nil && storage != nil {
		SetStorageService(storage)
	} else {
		core.When[contracts.StorageService](ctx, func(storage contracts.StorageService) {
			SetStorageService(storage)
		})
	}
	if rc, err := core.Inject[contracts.RiskControlService](ctx); err == nil && rc != nil {
		SetRiskControlService(rc)
	} else {
		core.When[contracts.RiskControlService](ctx, func(rc contracts.RiskControlService) {
			SetRiskControlService(rc)
		})
	}
	SetEventEmitter(ctx.Events().Emit)

	ctx.OnDispose(func() error {
		ResetServices()
		return nil
	})

	// 0a. Dynamic Auth Middlewares
	var loginMW gin.HandlerFunc = func(c *gin.Context) {
		if authSvc := GetAuthService(c.Request.Context()); authSvc != nil {
			if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
				mw(c)
				return
			}
		}
		c.Next()
	}
	var adminMW gin.HandlerFunc = func(c *gin.Context) {
		if authSvc := GetAuthService(c.Request.Context()); authSvc != nil {
			if mw, ok := authSvc.RequireAdminMiddleware().(gin.HandlerFunc); ok {
				mw(c)
				return
			}
		}
		c.Next()
	}

	// 0b. Register migrations
	ctx.Migrations().Register("admin", adminMigrations)

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
