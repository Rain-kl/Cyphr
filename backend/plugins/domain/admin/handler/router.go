// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package handler provides HTTP routing and handlers for the admin domain.
package handler

import (
	"Wavelet/core/extpoints"
)

// RegisterRoutes mounts all admin console endpoints under the provided admin router group.
func RegisterRoutes(adminRouter extpoints.RouterExtension) {
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
