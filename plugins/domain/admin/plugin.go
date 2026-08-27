// Package admin provides the system management console, diagnostics, audit logging, and configuration hot-reloading domain plugin for Cordis.
package admin

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	admin_auth_source "github.com/Rain-kl/Wavelet/internal/apps/admin/auth_source"
	admin_cache "github.com/Rain-kl/Wavelet/internal/apps/admin/cache"
	admin_db_manage "github.com/Rain-kl/Wavelet/internal/apps/admin/db_manage"
	admin_logs "github.com/Rain-kl/Wavelet/internal/apps/admin/logs"
	admin_push "github.com/Rain-kl/Wavelet/internal/apps/admin/push"
	admin_status "github.com/Rain-kl/Wavelet/internal/apps/admin/status"
	"github.com/Rain-kl/Wavelet/internal/apps/admin/system_config"
	admin_task "github.com/Rain-kl/Wavelet/internal/apps/admin/task"
	admin_template "github.com/Rain-kl/Wavelet/internal/apps/admin/template"
	admin_updater "github.com/Rain-kl/Wavelet/internal/apps/admin/updater"
	admin_user "github.com/Rain-kl/Wavelet/internal/apps/admin/user"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/apps/upload"
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
	// 1. Register Admin HTTP Routes
	adminRouter := ctx.Router().Group("/api/v1/admin", oauth.LoginRequired(), admin.LoginAdminRequired())
	{
		// Status & Diagnostics
		adminRouter.GET("/status", admin_status.GetSystemStatus)
		adminRouter.GET("/status/log-database", admin_status.GetLogDatabaseStatus)
		adminRouter.GET("/db-info", admin_status.GetDatabaseInfo)
		adminRouter.GET("/db-export", admin_status.ExportDatabase)

		// DB Management
		dbGroup := adminRouter.Group("/db-manage")
		{
			dbGroup.GET("/overview", admin_db_manage.GetDBOverview)
			dbGroup.GET("/tables", admin_db_manage.ListDBTables)
			dbGroup.GET("/table-data", admin_db_manage.GetDBTableData)
			dbGroup.POST("/query", admin_db_manage.ExecuteSQL)
		}

		// Cache Management
		cacheGroup := adminRouter.Group("/cache")
		{
			cacheGroup.GET("/status", admin_cache.GetCacheStatus)
			cacheGroup.POST("/config", admin_cache.UpdateCacheConfig)
			cacheGroup.POST("/clear", admin_cache.ClearCache)
		}

		// Updater
		updateGroup := adminRouter.Group("/update")
		{
			updateGroup.GET("", admin_updater.GetUpdateStatus)
			updateGroup.POST("/apply", admin_updater.ApplyUpdate)
		}

		// Logs
		logsGroup := adminRouter.Group("/logs")
		{
			logsGroup.GET("", admin_logs.GetLogs)
			logsGroup.GET("/access", admin_logs.GetAccessLogs)
			logsGroup.GET("/analytics", admin_logs.GetLogsAnalytics)
			logsGroup.GET("/ws", admin_logs.HandleLogWebSocket)
		}

		// Users
		usersGroup := adminRouter.Group("/users")
		{
			usersGroup.GET("", admin_user.ListUsers)
			usersGroup.POST("", admin_user.CreateUser)
			usersGroup.GET("/:id", admin_user.GetUser)
			usersGroup.PUT("/:id/status", admin_user.UpdateUserStatus)
			usersGroup.PUT("/:id", admin_user.UpdateUser)
			usersGroup.DELETE("/:id", admin_user.DeleteUser)
		}

		// Auth Sources
		authSourcesGroup := adminRouter.Group("/auth-sources")
		{
			authSourcesGroup.GET("", admin_auth_source.ListAuthSources)
			authSourcesGroup.POST("", admin_auth_source.CreateAuthSource)
			authSourcesGroup.PUT("/:id", admin_auth_source.UpdateAuthSource)
			authSourcesGroup.PUT("/:id/toggle", admin_auth_source.ToggleAuthSource)
			authSourcesGroup.DELETE("/:id", admin_auth_source.DeleteAuthSource)
		}

		// System Configs
		configGroup := adminRouter.Group("/system-configs")
		{
			configGroup.GET("", system_config.ListSystemConfigs)
			configGroup.POST("", system_config.CreateSystemConfig)
			configGroup.POST("/smtp/test", system_config.TestSMTP)

			keyGroup := configGroup.Group("/:key")
			{
				keyGroup.GET("", system_config.GetSystemConfig)
				keyGroup.PUT("", system_config.UpdateSystemConfig)
			}
		}

		// Templates
		templateGroup := adminRouter.Group("/templates")
		{
			templateGroup.GET("", admin_template.ListTemplates)
			templateGroup.POST("", admin_template.CreateTemplate)

			keyGroup := templateGroup.Group("/:key")
			{
				keyGroup.GET("", admin_template.GetTemplate)
				keyGroup.PUT("", admin_template.UpdateTemplate)
				keyGroup.DELETE("", admin_template.DeleteTemplate)
			}
		}

		// Uploads Management
		uploadGroup := adminRouter.Group("/uploads")
		{
			uploadGroup.GET("", upload.ListFiles)
			uploadGroup.GET("/stats", upload.GetFileStats)
			uploadGroup.DELETE("/:id", upload.DeleteFile)
			uploadGroup.GET("/download/:id", upload.DownloadFile)
			uploadGroup.POST("/download/batch", upload.BatchDownloadFiles)
			uploadGroup.GET("/types", upload.GetDistinctUploadTypes)
		}

		// Tasks
		taskGroup := adminRouter.Group("/tasks")
		{
			taskGroup.GET("/types", admin_task.ListTaskTypes)
			taskGroup.POST("/dispatch", admin_task.DispatchTask)

			executions := taskGroup.Group("/executions")
			{
				executions.GET("", admin_task.ListTaskExecutions)
				executions.GET("/:id", admin_task.GetTaskExecution)
				executions.POST("/:id/retry", admin_task.RetryTask)
			}

			schedules := taskGroup.Group("/schedules")
			{
				schedules.GET("", admin_task.ListSchedules)
				schedules.POST("", admin_task.CreateSchedule)
				schedules.PUT("/:id", admin_task.UpdateSchedule)
				schedules.DELETE("/:id", admin_task.DeleteSchedule)
			}
		}

		// Push & Notifications
		pushGroup := adminRouter.Group("/push")
		{
			events := pushGroup.Group("/events")
			{
				events.GET("", admin_push.ListEvents)
				events.GET("/builtin", admin_push.ListBuiltInEvents)
				events.POST("", admin_push.CreateEvent)
				events.PUT("/:id", admin_push.UpdateEvent)
				events.DELETE("/:id", admin_push.DeleteEvent)
				events.POST("/:id/toggle", admin_push.ToggleEvent)
			}

			pushGroup.GET("/histories", admin_push.ListHistories)
			pushGroup.POST("/test", admin_push.TestPush)

			channels := pushGroup.Group("/channels")
			{
				channels.GET("/definitions", admin_push.ListChannelDefinitions)
				channels.GET("", admin_push.ListChannels)
				channels.POST("", admin_push.CreateChannel)
				channels.PUT("/:id", admin_push.UpdateChannel)
				channels.DELETE("/:id", admin_push.DeleteChannel)
				channels.POST("/test", admin_push.TestChannel)
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
