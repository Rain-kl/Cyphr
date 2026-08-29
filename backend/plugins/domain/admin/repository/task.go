// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	taskExecutionLogRedisKeyPrefix = "task:execution:log:"
	taskExecutionLogExpiration     = 24 * time.Hour
)

// CreateScheduleRecord 创建定时任务
func CreateScheduleRecord(ctx context.Context, schedule *model.Schedule) error {
	return GetDB(ctx).Create(schedule).Error
}

// UpdateScheduleRecord 更新定时任务
func UpdateScheduleRecord(ctx context.Context, schedule *model.Schedule) error {
	return GetDB(ctx).Save(schedule).Error
}

// DeleteScheduleRecord 删除定时任务
func DeleteScheduleRecord(ctx context.Context, id uint64) error {
	return GetDB(ctx).Delete(&model.Schedule{}, id).Error
}

// GetScheduleByID 根据 ID 获取定时任务
func GetScheduleByID(ctx context.Context, id uint64) (*model.Schedule, error) {
	var schedule model.Schedule
	if err := GetDB(ctx).Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListSchedulesRecord 获取所有定时任务
func ListSchedulesRecord(ctx context.Context) ([]model.Schedule, error) {
	var schedules []model.Schedule
	if err := GetDB(ctx).Order("id DESC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// ListActiveSchedules 获取所有启用的定时任务
func ListActiveSchedules(ctx context.Context) ([]model.Schedule, error) {
	var schedules []model.Schedule
	if err := GetDB(ctx).Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// CreateTaskExecutionRecord 创建任务执行记录
func CreateTaskExecutionRecord(ctx context.Context, execution *model.TaskExecution) error {
	execution.ID = idgen.NextUint64ID()
	return GetDB(ctx).Create(execution).Error
}

// UpdateTaskExecutionRecord 更新任务执行记录，忽略由 Redis 缓冲和归档流程管理的 log 字段。
func UpdateTaskExecutionRecord(ctx context.Context, execution *model.TaskExecution) error {
	return GetDB(ctx).Omit("log").Save(execution).Error
}

// GetTaskExecutionByTaskID 根据 TaskID 获取执行记录
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*model.TaskExecution, error) {
	var execution model.TaskExecution
	if err := GetDB(ctx).Where("task_id = ?", taskID).First(&execution).Error; err != nil {
		return nil, err
	}
	loadTaskExecutionLog(ctx, &execution)
	return &execution, nil
}

// GetTaskExecutionByID 根据 ID 获取执行记录
func GetTaskExecutionByID(ctx context.Context, id uint64) (*model.TaskExecution, error) {
	var execution model.TaskExecution
	if err := GetDB(ctx).Where("id = ?", id).First(&execution).Error; err != nil {
		return nil, err
	}
	loadTaskExecutionLog(ctx, &execution)
	return &execution, nil
}

// GetLatestTaskExecutionByTaskType returns the most recent execution for a task type.
func GetLatestTaskExecutionByTaskType(ctx context.Context, taskType string) (*model.TaskExecution, bool, error) {
	var execution model.TaskExecution
	err := GetDB(ctx).
		Where("task_type = ?", taskType).
		Order("id DESC").
		First(&execution).Error
	if err == nil {
		loadTaskExecutionLog(ctx, &execution)
		return &execution, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, err
}

// AppendTaskExecutionLog 将日志追加到缓冲，任务完成后再持久化到数据库。
func AppendTaskExecutionLog(ctx context.Context, taskID, logLine string) error {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil {
		return errors.New(errs.ErrCacheServiceNotInitialized)
	}

	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", now, logLine)
	key := TaskExecutionLogRedisKey(taskID)

	var existing string
	if err := cacheSvc.Get(ctx, key, &existing); err != nil {
		// 只有未命中才代表「尚无缓冲」；其余读取失败若被当作空缓冲继续写入，
		// 会用这一行覆盖掉整段已缓冲的任务日志。
		if !errors.Is(err, contracts.ErrCacheMiss) {
			return fmt.Errorf("load buffered task execution log: %w", err)
		}
	}
	return cacheSvc.Set(ctx, key, existing+line, taskExecutionLogExpiration)
}

// FlushTaskExecutionLog 将缓冲中的完整任务日志写入数据库，并在成功后清理缓存。
func FlushTaskExecutionLog(ctx context.Context, taskID string) error {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil {
		return errors.New(errs.ErrCacheServiceNotInitialized)
	}

	key := TaskExecutionLogRedisKey(taskID)
	var logText string
	if err := cacheSvc.Get(ctx, key, &logText); err != nil {
		// 缓存未命中属于正常情况（任务无输出），其余错误必须上抛，
		// 否则缓冲日志会被静默丢弃并误报持久化成功。
		if !errors.Is(err, contracts.ErrCacheMiss) {
			return fmt.Errorf("load buffered task execution log: %w", err)
		}
		return nil
	}
	if logText == "" {
		return nil
	}

	gormDB := GetDB(ctx)
	if gormDB == nil {
		return errors.New(errs.ErrDatabaseNotInitialized)
	}
	result := gormDB.Model(&model.TaskExecution{}).
		Where("task_id = ?", taskID).
		Update("log", logText)
	if result.Error != nil {
		return fmt.Errorf("persist task execution log: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("persist task execution log: task %q not found", taskID)
	}

	_ = cacheSvc.Delete(ctx, key)
	return nil
}

// ListTaskExecutionRecords 分页查询任务执行记录
func ListTaskExecutionRecords(ctx context.Context, req model.ListTaskExecutionsRequest) ([]model.TaskExecution, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := GetDB(ctx).Model(&model.TaskExecution{})

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.TaskType != "" {
		query = query.Where("task_type = ?", req.TaskType)
	} else if types := parseTaskTypesFilter(req.TaskTypes); len(types) > 0 {
		query = query.Where("task_type IN ?", types)
	} else if req.TaskTypePrefix != "" {
		query = query.Where("task_type LIKE ? ESCAPE '\\'", util.EscapeLike(req.TaskTypePrefix)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var executions []model.TaskExecution
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&executions).Error; err != nil {
		return nil, 0, err
	}
	loadTaskExecutionLogs(ctx, executions)

	return executions, total, nil
}

func parseTaskTypesFilter(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// MarkFailedTaskExecutionsSucceededTx marks failed executions of a task type as succeeded within a transaction.
func MarkFailedTaskExecutionsSucceededTx(
	tx *gorm.DB,
	taskType string,
	result string,
	finishedAt time.Time,
) error {
	return tx.Model(&model.TaskExecution{}).
		Where("task_type = ? AND status = ?", taskType, model.TaskExecutionStatusFailed).
		Updates(map[string]any{
			"status":      model.TaskExecutionStatusSucceeded,
			"result":      result,
			"finished_at": finishedAt,
		}).Error
}

// CleanupTaskExecutionLogs removes finished task execution logs according to frequency-based retention.
func CleanupTaskExecutionLogs(ctx context.Context, now time.Time) (model.TaskExecutionCleanupStats, error) {
	const (
		frequencyWindowDays    = 30
		highFrequencyThreshold = frequencyWindowDays
	)

	frequencyWindowStart := now.AddDate(0, 0, -frequencyWindowDays)
	highFrequencyCutoff := now.AddDate(0, 0, -3)
	lowFrequencyCutoff := now.AddDate(0, 0, -30)
	terminalStatuses := []model.TaskExecutionStatus{model.TaskExecutionStatusSucceeded, model.TaskExecutionStatusFailed}

	var highFrequencyTaskTypes []string
	if err := GetDB(ctx).
		Model(&model.TaskExecution{}).
		Select("task_type").
		Where("created_at >= ?", frequencyWindowStart).
		Group("task_type").
		Having("COUNT(*) > ?", highFrequencyThreshold).
		Pluck("task_type", &highFrequencyTaskTypes).Error; err != nil {
		return model.TaskExecutionCleanupStats{}, fmt.Errorf("query high-frequency task types: %w", err)
	}

	var highFrequencyDeleted int64
	if len(highFrequencyTaskTypes) > 0 {
		highFrequencyResult := GetDB(ctx).
			Where("status IN ?", terminalStatuses).
			Where("created_at < ?", highFrequencyCutoff).
			Where("task_type IN ?", highFrequencyTaskTypes).
			Delete(&model.TaskExecution{})
		if highFrequencyResult.Error != nil {
			return model.TaskExecutionCleanupStats{}, fmt.Errorf("delete high-frequency task execution logs: %w", highFrequencyResult.Error)
		}
		highFrequencyDeleted = highFrequencyResult.RowsAffected
	}

	lowFrequencyQuery := GetDB(ctx).
		Where("status IN ?", terminalStatuses).
		Where("created_at < ?", lowFrequencyCutoff)
	if len(highFrequencyTaskTypes) > 0 {
		lowFrequencyQuery = lowFrequencyQuery.Where("task_type NOT IN ?", highFrequencyTaskTypes)
	}
	lowFrequencyResult := lowFrequencyQuery.Delete(&model.TaskExecution{})
	if lowFrequencyResult.Error != nil {
		return model.TaskExecutionCleanupStats{}, fmt.Errorf("delete low-frequency task execution logs: %w", lowFrequencyResult.Error)
	}

	return model.TaskExecutionCleanupStats{
		HighFrequencyDeleted: highFrequencyDeleted,
		LowFrequencyDeleted:  lowFrequencyResult.RowsAffected,
	}, nil
}

// TaskExecutionLogRedisKey builds the Redis key for task execution logs.
func TaskExecutionLogRedisKey(taskID string) string {
	return taskExecutionLogRedisKeyPrefix + taskID
}

// loadTaskExecutionLog best-effort enriches an execution with its cached log;
// a cache miss or failure simply leaves the stored log column in place.
func loadTaskExecutionLog(ctx context.Context, execution *model.TaskExecution) {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil {
		return
	}

	var logText string
	if err := cacheSvc.Get(ctx, TaskExecutionLogRedisKey(execution.TaskID), &logText); err == nil && logText != "" {
		execution.Log = logText
	}
}

// loadTaskExecutionLogs best-effort enriches every execution with its cached log.
func loadTaskExecutionLogs(ctx context.Context, executions []model.TaskExecution) {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil || len(executions) == 0 {
		return
	}

	for i := range executions {
		var logText string
		if err := cacheSvc.Get(ctx, TaskExecutionLogRedisKey(executions[i].TaskID), &logText); err == nil && logText != "" {
			executions[i].Log = logText
		}
	}
}
