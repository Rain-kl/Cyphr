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

	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	uploadcache "Wavelet/plugins/domain/upload/cache"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	uploadstats "Wavelet/plugins/domain/upload/stats"
	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

const (
	// SystemCleanupTask 系统定期垃圾清理任务标识
	SystemCleanupTask = "system:cleanup"
	// TaskTypeSystemCleanup 系统定期垃圾清理管理类型
	TaskTypeSystemCleanup = "system_cleanup"
)

// SystemCleanupMeta represents the task metadata.
var SystemCleanupMeta = contracts.TaskMetaDTO{
	Name:        SystemCleanupTask,
	DisplayName: "系统垃圾清理",
	Description: "定期清理未使用上传文件、历史推送记录和过期任务执行日志",
	Category:    "maintenance",
	MaxRetry:    3,
	Queue:       "default",
}

// SystemCleanupHandler 系统定期垃圾清理异步任务处理器
type SystemCleanupHandler struct{}

// Execute 执行系统清理（包含文件清理、历史推送日志和任务执行日志清理）
func (h *SystemCleanupHandler) Execute(ctx context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
	if uploadstorage.ReadOnly(ctx) {
		return nil, errors.New(shared.ErrStorageReadOnly)
	}
	const batchSize = 100
	var lastID uint64
	var totalProcessed int
	var totalDeleted int

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	logger.InfoF(ctx, "开始扫描未使用的待删除上传文件，阈值时间: %s", oneHourAgo.Format(time.RFC3339))

	db := shared.GetDB(ctx)
	if db == nil {
		return nil, errors.New("database service not available")
	}

	storageSvc := shared.GetStorage(ctx)

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("system cleanup canceled: %w", err)
		}

		var pendingUploads []models.Upload
		if err := db.
			Where("id > ? AND status = ? AND created_at < ?", lastID, models.UploadStatusPending, oneHourAgo).
			Order("id ASC").
			Limit(batchSize).
			Find(&pendingUploads).Error; err != nil {
			logger.ErrorF(ctx, "查询过期待使用上传文件失败: %v", err)
			return nil, fmt.Errorf("failed to query pending uploads: %w", err)
		}

		if len(pendingUploads) == 0 {
			break
		}

		for i := range pendingUploads {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("system cleanup canceled: %w", err)
			}

			upload := &pendingUploads[i]
			totalProcessed++
			lastID = upload.ID

			if storageSvc != nil {
				if err := storageSvc.Delete(ctx, upload.FilePath); err != nil {
					logger.WarnF(ctx, "清理过期未确认上传底层文件失败 [ID:%d, Path:%s]: %v", upload.ID, upload.FilePath, err)
				}
			}

			statsSnapshot := *upload
			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Delete(upload).Error; err != nil {
					return err
				}
				return uploadstats.ApplyUploadStatsDeltaTx(tx, &statsSnapshot, -1)
			}); err != nil {
				logger.ErrorF(ctx, "删除过期未确认上传记录失败 [ID:%d]: %v", upload.ID, err)
				continue
			}

			uploadcache.EvictUploadMeta(ctx, upload.ID)
			totalDeleted++
		}
	}

	// 清理过期任务执行记录
	var deletedExecutions int64
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	if err := db.
		Table("w_task_executions").
		Where("created_at < ?", sevenDaysAgo).
		Delete(&struct{}{}).Error; err != nil {
		logger.WarnF(ctx, "清理过期任务执行日志失败: %v", err)
	} else {
		deletedExecutions = db.RowsAffected
		logger.InfoF(ctx, "已清理 7 天前任务执行日志，共 %d 条", deletedExecutions)
	}

	// 清理已过期推送日志
	var deletedPushLogs int64
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)
	if err := db.
		Table("w_push_logs").
		Where("created_at < ?", thirtyDaysAgo).
		Delete(&struct{}{}).Error; err != nil {
		logger.WarnF(ctx, "清理历史推送日志失败: %v", err)
	} else {
		deletedPushLogs = db.RowsAffected
		logger.InfoF(ctx, "已清理 30 天前推送日志，共 %d 条", deletedPushLogs)
	}

	msg := fmt.Sprintf(
		"系统垃圾清理完成，处理未确认文件: %d 个，物理删除: %d 个，清理过期任务日志: %d 条，清理历史推送日志: %d 条",
		totalProcessed,
		totalDeleted,
		deletedExecutions,
		deletedPushLogs,
	)
	logger.InfoF(ctx, "%s", msg)
	return &contracts.TaskResultDTO{Message: msg}, nil
}
