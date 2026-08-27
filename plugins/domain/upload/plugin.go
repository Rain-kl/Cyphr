// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package upload provides file uploading, storage abstraction, image transcoding, and caching domain plugin for Cordis.
package upload

import (
	"context"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/filesrv"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/handler"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/task"
	"github.com/hibiken/asynq"
)

// Plugin implements core.Plugin to provide file upload and media serving domain services.
type Plugin struct{}

// New creates a new upload domain plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier for the upload domain plugin.
func (p *Plugin) Name() string {
	return "upload"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "upload",
		Version:     "1.0.0",
		Description: "File upload, secure delivery, image transcoding, and storage management domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers upload routes, tasks, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 1. Register File Server Routes
	ctx.Router().GET("/f/:id", filesrv.ServeFileByID)

	// 2. Register User/Admin Upload HTTP Routes
	uploadGroup := ctx.Router().Group("/api/v1/upload", auth.LoginRequired())
	{
		uploadGroup.POST("", handler.UploadFile)
		uploadGroup.GET("", handler.ListFiles)
		uploadGroup.DELETE("/:id", handler.DeleteFile)
		uploadGroup.POST("/batch-download", handler.BatchDownloadFiles)
	}

	adminUploadGroup := ctx.Router().Group("/api/v1/admin/uploads", auth.LoginRequired())
	{
		adminUploadGroup.GET("", handler.ListFiles)
		adminUploadGroup.GET("/stats", handler.GetFileStats)
		adminUploadGroup.DELETE("/:id", handler.DeleteFile)
		adminUploadGroup.GET("/download/:id", handler.DownloadFile)
		adminUploadGroup.POST("/download/batch", handler.BatchDownloadFiles)
		adminUploadGroup.GET("/types", handler.GetDistinctUploadTypes)
	}

	const (
		defaultCleanupRetry = 3
		defaultStatsRetry   = 2
		defaultSingleRetry  = 1
	)

	// 3. Register Asynq tasks
	cleanupHandler := &task.SystemCleanupHandler{}
	ctx.Task().Register(task.SystemCleanupTask, func(c context.Context, t *asynq.Task) error {
		_, err := cleanupHandler.Execute(c, t.Payload())
		return err
	}, extpoints.WithTaskRetry(defaultCleanupRetry))

	rebuildStatsHandler := &task.RebuildUploadStatsHandler{}
	ctx.Task().Register(task.RebuildUploadStatsTask, func(c context.Context, t *asynq.Task) error {
		_, err := rebuildStatsHandler.Execute(c, t.Payload())
		return err
	}, extpoints.WithTaskRetry(defaultStatsRetry))

	migrationHandler := &task.MigrationHandler{}
	ctx.Task().Register(task.StorageMigrationTask, func(c context.Context, t *asynq.Task) error {
		_, err := migrationHandler.Execute(c, t.Payload())
		return err
	}, extpoints.WithTaskRetry(defaultSingleRetry))

	warmHandler := &task.WarmImageCacheHandler{}
	ctx.Task().Register(task.WarmImageCacheTask, func(c context.Context, t *asynq.Task) error {
		_, err := warmHandler.Execute(c, t.Payload())
		return err
	}, extpoints.WithTaskRetry(1))

	// 4. Register Cron Schedule
	ctx.Schedule().RegisterCron("0 3 * * *", task.SystemCleanupTask, nil)

	// 5. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "upload.max_file_size_mb",
		Default:     100,
		Description: "Maximum upload file size limit in MB",
		Type:        "integer",
		Category:    "storage",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "upload.allowed_extensions",
		Default:     "png,jpg,jpeg,gif,webp,svg,pdf,zip,tar,gz",
		Description: "Allowed upload file extensions separated by commas",
		Type:        "string",
		Category:    "storage",
	})

	return nil
}
