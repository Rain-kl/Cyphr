// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"errors"
	"time"

	db "github.com/Rain-kl/Wavelet/pkg/persistence"
	"gorm.io/gorm"
)

// TaskExecutionStatus 任务执行状态
//
//nolint:revive
type TaskExecutionStatus string

// Task execution status constants.
const (
	TaskExecutionStatusPending   TaskExecutionStatus = "pending"
	TaskExecutionStatusRunning   TaskExecutionStatus = "running"
	TaskExecutionStatusSucceeded TaskExecutionStatus = "succeeded"
	TaskExecutionStatusFailed    TaskExecutionStatus = "failed"
)

// TaskExecution 任务执行记录
//
//nolint:revive
type TaskExecution struct {
	ID           uint64              `json:"id,string" gorm:"primaryKey"`
	TaskID       string              `json:"task_id" gorm:"size:128;uniqueIndex;not null"`
	TaskType     string              `json:"task_type" gorm:"size:64;index;not null"`
	TaskName     string              `json:"task_name" gorm:"size:128"`
	Status       TaskExecutionStatus `json:"status" gorm:"size:32;index;not null"`
	Retryable    bool                `json:"retryable" gorm:"not null;default:false"`
	MaxRetry     int                 `json:"max_retry" gorm:"not null;default:0"`
	RetryCount   int                 `json:"retry_count" gorm:"not null;default:0"`
	Log          string              `json:"log" gorm:"type:text"`
	ErrorMessage string              `json:"error_message" gorm:"type:text"`
	Result       string              `json:"result" gorm:"type:text"`
	StartedAt    *time.Time          `json:"started_at" gorm:"index"`
	FinishedAt   *time.Time          `json:"finished_at"`
	Duration     int64               `json:"duration" gorm:"comment:耗时毫秒"`
	Payload      string              `json:"payload" gorm:"type:text"`
	TriggeredBy  string              `json:"triggered_by" gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time           `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time           `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (TaskExecution) TableName() string {
	return "w_task_executions"
}

// CreateTaskExecution 创建任务执行记录
func CreateTaskExecution(ctx context.Context, exec *TaskExecution) error {
	return db.DB(ctx).Create(exec).Error
}

// GetTaskExecutionByTaskID 根据 TaskID 查询执行记录
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*TaskExecution, error) {
	var exec TaskExecution
	if err := db.DB(ctx).Where("task_id = ?", taskID).First(&exec).Error; err != nil {
		return nil, err
	}
	_ = loadTaskExecutionLog(ctx, &exec)
	return &exec, nil
}

// GetTaskExecutionByID 根据主键 ID 查询执行记录
func GetTaskExecutionByID(ctx context.Context, id uint64) (*TaskExecution, error) {
	var exec TaskExecution
	if err := db.DB(ctx).Where("id = ?", id).First(&exec).Error; err != nil {
		return nil, err
	}
	_ = loadTaskExecutionLog(ctx, &exec)
	return &exec, nil
}

// GetLatestTaskExecutionByTaskType 获取指定任务类型的最新执行记录
func GetLatestTaskExecutionByTaskType(ctx context.Context, taskType string) (*TaskExecution, bool, error) {
	var exec TaskExecution
	err := db.DB(ctx).Where("task_type = ?", taskType).Order("id DESC").First(&exec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &exec, true, nil
}

// TaskExecutionCleanupStats describes task execution log cleanup results.
//
//nolint:revive
type TaskExecutionCleanupStats struct {
	HighFrequencyDeleted int64
	LowFrequencyDeleted  int64
}

// CleanupTaskExecutionLogs removes finished task execution logs according to frequency-based retention.
func CleanupTaskExecutionLogs(ctx context.Context, now time.Time) (TaskExecutionCleanupStats, error) {
	const (
		frequencyWindowDays    = 30
		highFrequencyThreshold = frequencyWindowDays
	)

	frequencyWindowStart := now.AddDate(0, 0, -frequencyWindowDays)
	highFrequencyCutoff := now.AddDate(0, 0, -3)
	lowFrequencyCutoff := now.AddDate(0, 0, -30)
	terminalStatuses := []TaskExecutionStatus{TaskExecutionStatusSucceeded, TaskExecutionStatusFailed}

	var highFrequencyTaskTypes []string
	if err := db.DB(ctx).
		Model(&TaskExecution{}).
		Select("task_type").
		Where("created_at >= ?", frequencyWindowStart).
		Group("task_type").
		Having("COUNT(*) > ?", highFrequencyThreshold).
		Pluck("task_type", &highFrequencyTaskTypes).Error; err != nil {
		return TaskExecutionCleanupStats{}, err
	}

	var highFrequencyDeleted int64
	if len(highFrequencyTaskTypes) > 0 {
		highFrequencyResult := db.DB(ctx).
			Where("status IN ?", terminalStatuses).
			Where("created_at < ?", highFrequencyCutoff).
			Where("task_type IN ?", highFrequencyTaskTypes).
			Delete(&TaskExecution{})
		if highFrequencyResult.Error != nil {
			return TaskExecutionCleanupStats{}, highFrequencyResult.Error
		}
		highFrequencyDeleted = highFrequencyResult.RowsAffected
	}

	lowFrequencyQuery := db.DB(ctx).
		Where("status IN ?", terminalStatuses).
		Where("created_at < ?", lowFrequencyCutoff)
	if len(highFrequencyTaskTypes) > 0 {
		lowFrequencyQuery = lowFrequencyQuery.Where("task_type NOT IN ?", highFrequencyTaskTypes)
	}
	lowFrequencyResult := lowFrequencyQuery.Delete(&TaskExecution{})
	if lowFrequencyResult.Error != nil {
		return TaskExecutionCleanupStats{}, lowFrequencyResult.Error
	}

	return TaskExecutionCleanupStats{
		HighFrequencyDeleted: highFrequencyDeleted,
		LowFrequencyDeleted:  lowFrequencyResult.RowsAffected,
	}, nil
}
