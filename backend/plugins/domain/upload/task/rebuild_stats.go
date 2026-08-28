// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"fmt"

	"github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/models"
	uploadstats "github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/stats"
	"github.com/Rain-kl/Wavelet/backend/plugins/drivers/driver_asynq_worker"
	database "github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
)

const (
	// RebuildUploadStatsTask is the Asynq task name for rebuilding upload stats.
	RebuildUploadStatsTask = "upload:rebuild_stats"
	// TaskTypeRebuildUploadStats is the admin-dispatchable task type.
	TaskTypeRebuildUploadStats = "rebuild_upload_stats"
)

// RebuildUploadStatsMeta describes the upload stats rebuild task.
var RebuildUploadStatsMeta = driver_asynq_worker.TaskMeta{
	Type:         TaskTypeRebuildUploadStats,
	AsynqTask:    RebuildUploadStatsTask,
	Name:         "重算文件存储统计",
	Description:  "根据当前 w_uploads 活跃记录全量重建 w_upload_stats（总量、类型、分类、趋势）",
	SupportsTime: false,
	MaxRetry:     driver_asynq_worker.DefaultMaxRetry,
	Queue:        driver_asynq_worker.QueueDefault,
	Retryable:    true,
}

// RebuildUploadStatsHandler rebuilds incremental upload stats from active upload records.
type RebuildUploadStatsHandler struct{}

// Execute scans active uploads and rebuilds all upload stat dimensions.
func (h *RebuildUploadStatsHandler) Execute(ctx context.Context, _ []byte) (*driver_asynq_worker.TaskResult, error) {
	var activeCount int64
	if err := database.DB(ctx).
		Model(&models.Upload{}).
		Where("status != ?", models.UploadStatusDeleted).
		Count(&activeCount).Error; err != nil {
		driver_asynq_worker.AppendLog(ctx, "统计活跃上传记录失败: %v", err)
		return nil, fmt.Errorf("count active uploads: %w", err)
	}

	driver_asynq_worker.AppendLog(ctx, "开始重算文件存储统计，活跃记录数: %d", activeCount)

	if err := uploadstats.RebuildUploadStats(ctx); err != nil {
		driver_asynq_worker.AppendLog(ctx, "重算文件存储统计失败: %v", err)
		return nil, fmt.Errorf("rebuild upload stats: %w", err)
	}

	var totalStat models.UploadStat
	if err := database.DB(ctx).
		Where("dimension = ? AND stat_key = ?", models.UploadStatDimensionTotal, "").
		First(&totalStat).Error; err != nil {
		driver_asynq_worker.AppendLog(ctx, "读取总量统计失败: %v", err)
		return nil, fmt.Errorf("load total upload stats: %w", err)
	}

	msg := fmt.Sprintf(
		"文件存储统计重算完成，活跃记录 %d 条，统计文件数 %d，总大小 %d 字节",
		activeCount,
		totalStat.FileCount,
		totalStat.FileSize,
	)
	driver_asynq_worker.AppendLog(ctx, "%s", msg)
	return &driver_asynq_worker.TaskResult{Message: msg}, nil
}
