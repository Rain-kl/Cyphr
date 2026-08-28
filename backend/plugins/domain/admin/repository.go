// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/pkg/cache/ram"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	configTypeSystem                = "system"
	errDatabaseNotInitialized       = "database not initialized"
	errConfigIntParseFailed         = "配置 %s 的值 '%s' 无法转换为整数: %w"
	errConfigDecimalParseFailed     = "配置 %s 的值 '%s' 无法转换为decimal: %w"
	errConfigBoolParseFailed        = "配置 %s 的值 '%s' 无法转换为布尔值: %w"
	errParseMenuDisplayConfigFailed = "解析目录显示配置失败: %w"

	taskExecutionLogRedisKeyPrefix = "task:execution:log:"
	taskExecutionLogExpiration     = 24 * time.Hour
	taskExecutionLogMaxLines       = 1000
)

// PreheatSystemConfigs loads all system configs from database.
func PreheatSystemConfigs(ctx context.Context) ([]SystemConfig, error) {
	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []SystemConfig
	if err := database.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// PreheatSystemConfigByKey loads a single config key from database.
func PreheatSystemConfigByKey(ctx context.Context, key string) (SystemConfig, error) {
	database := GetDB(ctx)
	if database == nil {
		return SystemConfig{}, errors.New(errDatabaseNotInitialized)
	}

	var sc SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return SystemConfig{}, err
	}
	return sc, nil
}

// GetSystemConfigByGroup queries a configuration by Type and Key.
func GetSystemConfigByGroup(ctx context.Context, configType, key string) (SystemConfig, error) {
	ensureSystemConfigCacheListener()

	if item, ok := ram.Get(configType, key); ok {
		var sc SystemConfig
		if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
			return sc, nil
		}
	}

	database := GetDB(ctx)
	if database == nil {
		return SystemConfig{}, errors.New(errDatabaseNotInitialized)
	}

	var sc SystemConfig
	if err := database.Where("key = ?", key).First(&sc).Error; err != nil {
		return SystemConfig{}, err
	}

	valBytes, err := json.Marshal(sc)
	if err == nil {
		ram.Set(ram.CacheItem{
			Key:   sc.Key,
			Value: string(valBytes),
			Type:  configType,
			TTL:   determineTTL(sc.Key),
		})
	}

	return sc, nil
}

// GetSystemConfigByKey queries config by key.
func GetSystemConfigByKey(ctx context.Context, key string) (SystemConfig, error) {
	return GetSystemConfigByGroup(ctx, ConfigCacheType, key)
}

// ListSystemConfigsByKeys loads multiple config keys.
func ListSystemConfigsByKeys(ctx context.Context, keys []string) (map[string]SystemConfig, error) {
	if len(keys) == 0 {
		return map[string]SystemConfig{}, nil
	}

	ensureSystemConfigCacheListener()

	result := make(map[string]SystemConfig, len(keys))
	missing := make([]string, 0, len(keys))

	for _, key := range keys {
		if item, ok := ram.Get(ConfigCacheType, key); ok {
			var sc SystemConfig
			if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
				result[key] = sc
				continue
			}
		}
		missing = append(missing, key)
	}

	if len(missing) == 0 {
		return result, nil
	}

	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []SystemConfig
	if err := database.Where("key IN ?", missing).Find(&configs).Error; err != nil {
		return nil, err
	}

	for i := range configs {
		valBytes, err := json.Marshal(configs[i])
		if err == nil {
			ram.Set(ram.CacheItem{
				Key:   configs[i].Key,
				Value: string(valBytes),
				Type:  ConfigCacheType,
				TTL:   determineTTL(configs[i].Key),
			})
		}
		result[configs[i].Key] = configs[i]
	}

	return result, nil
}

// InvalidateVisibleSystemConfigsCache clears the cached public config list.
func InvalidateVisibleSystemConfigsCache(ctx context.Context) error {
	return InvalidateAllSystemConfigCaches(ctx)
}

// ListVisibleSystemConfigs queries visible configs using local cache store.
func ListVisibleSystemConfigs(ctx context.Context) ([]SystemConfig, error) {
	ensureSystemConfigCacheListener()

	items := ram.GetTypeItems(ConfigCacheType)
	if len(items) > 0 {
		var list []SystemConfig
		for _, item := range items {
			var sc SystemConfig
			if err := json.Unmarshal([]byte(item.Value), &sc); err == nil {
				if sc.Visibility == ConfigVisibilityVisible {
					list = append(list, sc)
				}
			}
		}
		return list, nil
	}

	database := GetDB(ctx)
	if database == nil {
		return nil, errors.New(errDatabaseNotInitialized)
	}

	var configs []SystemConfig
	if err := database.Where("visibility = ?", ConfigVisibilityVisible).Find(&configs).Error; err != nil {
		return nil, err
	}

	for _, cfg := range configs {
		valBytes, err := json.Marshal(cfg)
		if err == nil {
			ram.Set(ram.CacheItem{
				Key:   cfg.Key,
				Value: string(valBytes),
				Type:  ConfigCacheType,
				TTL:   determineTTL(cfg.Key),
			})
		}
	}

	return configs, nil
}

// GetIntByKey queries config and converts to int.
func GetIntByKey(ctx context.Context, key string) (int, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(sc.Value)
	if err != nil {
		return 0, fmt.Errorf(errConfigIntParseFailed, key, sc.Value, err)
	}

	return value, nil
}

// GetDecimalByKey queries config and converts to decimal.Decimal.
func GetDecimalByKey(ctx context.Context, key string, precision int32) (decimal.Decimal, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return decimal.Zero, err
	}

	value, err := decimal.NewFromString(sc.Value)
	if err != nil {
		return decimal.Zero, fmt.Errorf(errConfigDecimalParseFailed, key, sc.Value, err)
	}

	return value.Truncate(precision), nil
}

// GetBoolByKey queries config and converts to bool.
func GetBoolByKey(ctx context.Context, key string) (bool, error) {
	sc, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return false, err
	}

	value, err := strconv.ParseBool(sc.Value)
	if err != nil {
		return false, fmt.Errorf(errConfigBoolParseFailed, key, sc.Value, err)
	}

	return value, nil
}

// GetMenuDisplayConfig queries and parses menu config.
func GetMenuDisplayConfig(ctx context.Context) (map[string]bool, error) {
	sc, err := GetSystemConfigByKey(ctx, ConfigKeyMenuDisplayConfig)
	if err != nil {
		return nil, err
	}

	config := make(map[string]bool)
	if sc.Value == "" || sc.Value == "{}" {
		return config, nil
	}

	if err := json.Unmarshal([]byte(sc.Value), &config); err != nil {
		return nil, fmt.Errorf(errParseMenuDisplayConfigFailed, err)
	}

	return config, nil
}

// ListAdminSystemConfigs returns all configs, optionally filtered by type.
func ListAdminSystemConfigs(ctx context.Context, configType string) ([]SystemConfig, error) {
	query := GetDB(ctx).Order("created_at DESC")
	if configType != "" {
		query = query.Where("type = ?", configType)
	}
	var configs []SystemConfig
	if err := query.Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetAdminSystemConfigByKey loads a config directly from DB.
func GetAdminSystemConfigByKey(ctx context.Context, key string) (SystemConfig, error) {
	var config SystemConfig
	if err := GetDB(ctx).Where("key = ?", key).First(&config).Error; err != nil {
		return SystemConfig{}, err
	}
	return config, nil
}

// SystemConfigExists reports whether a config key already exists.
func SystemConfigExists(ctx context.Context, key string) (bool, error) {
	var existing SystemConfig
	err := GetDB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSystemConfigRecord persists a new system config row.
func CreateSystemConfigRecord(ctx context.Context, config *SystemConfig) error {
	return GetDB(ctx).Create(config).Error
}

// UpdateSystemConfigFields applies partial updates to a system config row.
func UpdateSystemConfigFields(ctx context.Context, config *SystemConfig, updates map[string]any) error {
	return GetDB(ctx).Model(config).Updates(updates).Error
}

// SaveOrUpdateSystemConfig creates or updates a config row and invalidates cache.
func SaveOrUpdateSystemConfig(ctx context.Context, key, value string) error {
	var sc SystemConfig
	err := GetDB(ctx).Where("key = ?", key).First(&sc).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		sc = SystemConfig{
			Key:        key,
			Value:      value,
			Type:       configTypeSystem,
			Visibility: ConfigVisibilityHidden,
		}
		if err := GetDB(ctx).Create(&sc).Error; err != nil {
			return err
		}
	} else {
		sc.Value = value
		if err := GetDB(ctx).Save(&sc).Error; err != nil {
			return err
		}
	}
	return InvalidateSystemConfigCache(ctx, key)
}

// ListTemplatesRecord returns all templates ordered by system flag and creation time.
func ListTemplatesRecord(ctx context.Context) ([]Template, error) {
	var templates []Template
	if err := GetDB(ctx).Order("is_system DESC, created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplateByKey loads a template by its key.
func GetTemplateByKey(ctx context.Context, key string) (Template, error) {
	var tmpl Template
	if err := GetDB(ctx).Where("key = ?", key).First(&tmpl).Error; err != nil {
		return Template{}, err
	}
	return tmpl, nil
}

// TemplateExistsByKey reports whether a template key is already taken.
func TemplateExistsByKey(ctx context.Context, key string) (bool, error) {
	var existing Template
	err := GetDB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateTemplateRecord persists a new template.
func CreateTemplateRecord(ctx context.Context, tmpl *Template) error {
	return GetDB(ctx).Create(tmpl).Error
}

// SaveTemplateRecord updates an existing template.
func SaveTemplateRecord(ctx context.Context, tmpl *Template) error {
	return GetDB(ctx).Save(tmpl).Error
}

// DeleteTemplateRecord removes a template record.
func DeleteTemplateRecord(ctx context.Context, tmpl *Template) error {
	return GetDB(ctx).Delete(tmpl).Error
}

// CreateScheduleRecord 创建定时任务
func CreateScheduleRecord(ctx context.Context, schedule *Schedule) error {
	return GetDB(ctx).Create(schedule).Error
}

// UpdateScheduleRecord 更新定时任务
func UpdateScheduleRecord(ctx context.Context, schedule *Schedule) error {
	return GetDB(ctx).Save(schedule).Error
}

// DeleteScheduleRecord 删除定时任务
func DeleteScheduleRecord(ctx context.Context, id uint64) error {
	return GetDB(ctx).Delete(&Schedule{}, id).Error
}

// GetScheduleByID 根据 ID 获取定时任务
func GetScheduleByID(ctx context.Context, id uint64) (*Schedule, error) {
	var schedule Schedule
	if err := GetDB(ctx).Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListSchedulesRecord 获取所有定时任务
func ListSchedulesRecord(ctx context.Context) ([]Schedule, error) {
	var schedules []Schedule
	if err := GetDB(ctx).Order("id DESC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// ListActiveSchedules 获取所有启用的定时任务
func ListActiveSchedules(ctx context.Context) ([]Schedule, error) {
	var schedules []Schedule
	if err := GetDB(ctx).Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// CreateTaskExecutionRecord 创建任务执行记录
func CreateTaskExecutionRecord(ctx context.Context, execution *TaskExecution) error {
	execution.ID = idgen.NextUint64ID()
	return GetDB(ctx).Create(execution).Error
}

// UpdateTaskExecutionRecord 更新任务执行记录，忽略由 Redis 缓冲和归档流程管理的 log 字段。
func UpdateTaskExecutionRecord(ctx context.Context, execution *TaskExecution) error {
	return GetDB(ctx).Omit("log").Save(execution).Error
}

// GetTaskExecutionByTaskID 根据 TaskID 获取执行记录
func GetTaskExecutionByTaskID(ctx context.Context, taskID string) (*TaskExecution, error) {
	var execution TaskExecution
	if err := GetDB(ctx).Where("task_id = ?", taskID).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetTaskExecutionByID 根据 ID 获取执行记录
func GetTaskExecutionByID(ctx context.Context, id uint64) (*TaskExecution, error) {
	var execution TaskExecution
	if err := GetDB(ctx).Where("id = ?", id).First(&execution).Error; err != nil {
		return nil, err
	}
	if err := loadTaskExecutionLog(ctx, &execution); err != nil {
		return nil, err
	}
	return &execution, nil
}

// GetLatestTaskExecutionByTaskType returns the most recent execution for a task type.
func GetLatestTaskExecutionByTaskType(ctx context.Context, taskType string) (*TaskExecution, bool, error) {
	var execution TaskExecution
	err := GetDB(ctx).
		Where("task_type = ?", taskType).
		Order("id DESC").
		First(&execution).Error
	if err == nil {
		if loadErr := loadTaskExecutionLog(ctx, &execution); loadErr != nil {
			return nil, false, loadErr
		}
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
		return errors.New("cache service is not initialized")
	}

	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", now, logLine)
	key := taskExecutionLogRedisKey(taskID)

	var existing string
	_ = cacheSvc.Get(ctx, key, &existing)
	return cacheSvc.Set(ctx, key, existing+line, taskExecutionLogExpiration)
}

// FlushTaskExecutionLog 将缓冲中的完整任务日志写入数据库，并在成功后清理缓存。
func FlushTaskExecutionLog(ctx context.Context, taskID string) error {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil {
		return errors.New("cache service is not initialized")
	}

	key := taskExecutionLogRedisKey(taskID)
	var logText string
	if err := cacheSvc.Get(ctx, key, &logText); err != nil || logText == "" {
		return nil
	}

	gormDB := GetDB(ctx)
	if gormDB == nil {
		return errors.New(errDatabaseNotInitialized)
	}
	result := gormDB.Model(&TaskExecution{}).
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
func ListTaskExecutionRecords(ctx context.Context, req ListTaskExecutionsRequest) ([]TaskExecution, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	query := GetDB(ctx).Model(&TaskExecution{})

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

	var executions []TaskExecution
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&executions).Error; err != nil {
		return nil, 0, err
	}
	if err := loadTaskExecutionLogs(ctx, executions); err != nil {
		return nil, 0, err
	}

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
	return tx.Model(&TaskExecution{}).
		Where("task_type = ? AND status = ?", taskType, TaskExecutionStatusFailed).
		Updates(map[string]any{
			"status":      TaskExecutionStatusSucceeded,
			"result":      result,
			"finished_at": finishedAt,
		}).Error
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
	if err := GetDB(ctx).
		Model(&TaskExecution{}).
		Select("task_type").
		Where("created_at >= ?", frequencyWindowStart).
		Group("task_type").
		Having("COUNT(*) > ?", highFrequencyThreshold).
		Pluck("task_type", &highFrequencyTaskTypes).Error; err != nil {
		return TaskExecutionCleanupStats{}, fmt.Errorf("query high-frequency task types: %w", err)
	}

	var highFrequencyDeleted int64
	if len(highFrequencyTaskTypes) > 0 {
		highFrequencyResult := GetDB(ctx).
			Where("status IN ?", terminalStatuses).
			Where("created_at < ?", highFrequencyCutoff).
			Where("task_type IN ?", highFrequencyTaskTypes).
			Delete(&TaskExecution{})
		if highFrequencyResult.Error != nil {
			return TaskExecutionCleanupStats{}, fmt.Errorf("delete high-frequency task execution logs: %w", highFrequencyResult.Error)
		}
		highFrequencyDeleted = highFrequencyResult.RowsAffected
	}

	lowFrequencyQuery := GetDB(ctx).
		Where("status IN ?", terminalStatuses).
		Where("created_at < ?", lowFrequencyCutoff)
	if len(highFrequencyTaskTypes) > 0 {
		lowFrequencyQuery = lowFrequencyQuery.Where("task_type NOT IN ?", highFrequencyTaskTypes)
	}
	lowFrequencyResult := lowFrequencyQuery.Delete(&TaskExecution{})
	if lowFrequencyResult.Error != nil {
		return TaskExecutionCleanupStats{}, fmt.Errorf("delete low-frequency task execution logs: %w", lowFrequencyResult.Error)
	}

	return TaskExecutionCleanupStats{
		HighFrequencyDeleted: highFrequencyDeleted,
		LowFrequencyDeleted:  lowFrequencyResult.RowsAffected,
	}, nil
}

func taskExecutionLogRedisKey(taskID string) string {
	return taskExecutionLogRedisKeyPrefix + taskID
}

func loadTaskExecutionLog(ctx context.Context, execution *TaskExecution) error {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil {
		return nil
	}

	var logText string
	if err := cacheSvc.Get(ctx, taskExecutionLogRedisKey(execution.TaskID), &logText); err == nil && logText != "" {
		execution.Log = logText
	}
	return nil
}

func loadTaskExecutionLogs(ctx context.Context, executions []TaskExecution) error {
	cacheSvc := GetCache(ctx)
	if cacheSvc == nil || len(executions) == 0 {
		return nil
	}

	for i := range executions {
		var logText string
		if err := cacheSvc.Get(ctx, taskExecutionLogRedisKey(executions[i].TaskID), &logText); err == nil && logText != "" {
			executions[i].Log = logText
		}
	}
	return nil
}
