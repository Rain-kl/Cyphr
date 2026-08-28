// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package task provides upload-related async background task handlers.
package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/backend/pkg/logger"
	logstore "github.com/Rain-kl/Wavelet/backend/plugins/domain/risk_control/logstore"
	uploadcache "github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/cache"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/models"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/shared"
	uploadstats "github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/stats"
	uploadstorage "github.com/Rain-kl/Wavelet/backend/plugins/domain/upload/storage"
	"github.com/Rain-kl/Wavelet/backend/plugins/drivers/driver_asynq_worker"
	database "github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/storage/objectstore"
)

const (
	// SystemCleanupTask 系统定期垃圾清理任务标识
	SystemCleanupTask = "system:cleanup"
	// TaskTypeSystemCleanup 系统定期垃圾清理管理类型
	TaskTypeSystemCleanup = "system_cleanup"
)

// SystemCleanupMeta represents the task metadata.
var SystemCleanupMeta = driver_asynq_worker.TaskMeta{
	Type:         TaskTypeSystemCleanup,
	AsynqTask:    SystemCleanupTask,
	Name:         "系统垃圾清理",
	Description:  "定期清理未使用上传文件、历史推送记录和过期任务执行日志",
	SupportsTime: false,
	MaxRetry:     driver_asynq_worker.DefaultMaxRetry,
	Queue:        driver_asynq_worker.QueueDefault,
	Retryable:    true,
}

// SystemCleanupHandler 系统定期垃圾清理异步任务处理器
type SystemCleanupHandler struct{}

// Execute 执行系统清理（包含文件清理、历史推送日志和任务执行日志清理）
func (h *SystemCleanupHandler) Execute(ctx context.Context, _ []byte) (*driver_asynq_worker.TaskResult, error) {
	if uploadstorage.ReadOnly(ctx) {
		return nil, errors.New(shared.ErrStorageReadOnly)
	}
	const batchSize = 100
	var lastID uint64
	var totalProcessed int
	var totalDeleted int

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	driver_asynq_worker.AppendLog(ctx, "开始扫描未使用上传文件，阈值: %s", oneHourAgo.Format(time.RFC3339))

	for {
		var unusedUploads []models.Upload
		if err := database.DB(ctx).
			Where("id > ? AND status = ? AND created_at < ?", lastID, models.UploadStatusPending, oneHourAgo).
			Order("id ASC").
			Limit(batchSize).
			Find(&unusedUploads).Error; err != nil {
			driver_asynq_worker.AppendLog(ctx, "查询未使用的上传文件失败: %v", err)
			return nil, fmt.Errorf(shared.ErrQueryUnusedUploadsFailed, err)
		}

		if len(unusedUploads) == 0 {
			break
		}

		driver_asynq_worker.AppendLog(ctx, "本批次找到 %d 个需要清理的上传文件", len(unusedUploads))

		for _, u := range unusedUploads {
			totalProcessed++

			if err := database.DB(ctx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.Upload{}).
					Where("id = ? AND status = ?", u.ID, models.UploadStatusPending).
					Update("status", models.UploadStatusDeleted).Error; err != nil {
					return err
				}

				_, backend, err := objectstore.Active(ctx)
				if err != nil {
					return err
				}
				if err := backend.Delete(ctx, u.FilePath); err != nil {
					return err
				}

				return nil
			}); err != nil {
				driver_asynq_worker.AppendLog(ctx, "清理上传文件失败 [ID:%d]: %v", u.ID, err)
				lastID = u.ID
				continue
			}

			uploadstats.RecordUploadStatsRemove(ctx, &u)
			uploadcache.InvalidateUploadMetaCache(ctx, u.ID)
			totalDeleted++
			lastID = u.ID
		}
	}

	driver_asynq_worker.AppendLog(ctx, "开始清理历史推送审计日志，只保留最近7天数据...")
	cutoff := time.Now().AddDate(0, 0, -7)
	var pushHistoryCount int64
	if err := database.DB(ctx).Table("w_push_histories").Where("created_at < ?", cutoff).Count(&pushHistoryCount).Error; err != nil {
		driver_asynq_worker.AppendLog(ctx, "统计待清理的历史推送记录失败: %v", err)
	} else if pushHistoryCount > 0 {
		if err := database.DB(ctx).Table("w_push_histories").Where("created_at < ?", cutoff).Delete(map[string]any{}).Error; err != nil {
			driver_asynq_worker.AppendLog(ctx, "删除历史推送记录失败: %v", err)
		} else {
			driver_asynq_worker.AppendLog(ctx, "成功删除 %d 条历史推送记录 (截止时间: %s)", pushHistoryCount, cutoff.Format("2006-01-02 15:04:05"))
		}
	} else {
		driver_asynq_worker.AppendLog(ctx, "没有需要清理的历史推送记录 (截止时间: %s)", cutoff.Format("2006-01-02 15:04:05"))
	}

	driver_asynq_worker.AppendLog(ctx, "开始清理任务执行日志：高频任务保留最近3天，低频任务保留最近30天...")
	taskLogStats, err := driver_asynq_worker.CleanupTaskExecutionLogs(ctx, time.Now())
	if err != nil {
		driver_asynq_worker.AppendLog(ctx, "清理任务执行日志失败: %v", err)
		logger.ErrorF(ctx, "清理任务执行日志失败: %v", err)
	} else {
		driver_asynq_worker.AppendLog(ctx, "成功清理任务执行日志 %d 条（高频 %d 条，低频 %d 条）",
			taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
			taskLogStats.HighFrequencyDeleted,
			taskLogStats.LowFrequencyDeleted,
		)
	}

	var logDeleted int64
	logSummary, logErr := logstore.CleanupExpired(ctx)
	if logErr != nil {
		driver_asynq_worker.AppendLog(ctx, "清理过期用户访问日志失败: %v", logErr)
		logger.ErrorF(ctx, "清理过期用户访问日志失败: %v", logErr)
	} else {
		logDeleted = logSummary.Deleted
		driver_asynq_worker.AppendLog(ctx, "成功清理过期用户访问日志 %d 条（%s 保留 %d 天）",
			logSummary.Deleted, logSummary.ActiveDatabase, logSummary.RetentionDays)
	}

	msg := fmt.Sprintf("系统清理完成。成功清理未使用的上传文件 %d/%d 个；清理历史推送审计日志 %d 条；清理任务执行日志 %d 条；清理过期访问日志 %d 条。",
		totalDeleted,
		totalProcessed,
		pushHistoryCount,
		taskLogStats.HighFrequencyDeleted+taskLogStats.LowFrequencyDeleted,
		logDeleted,
	)
	driver_asynq_worker.AppendLog(ctx, "%s", msg)
	return &driver_asynq_worker.TaskResult{Message: msg}, nil
}
