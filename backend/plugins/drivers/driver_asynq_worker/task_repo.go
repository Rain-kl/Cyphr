// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Rain-kl/Wavelet/pkg/idgen"
	cachepkg "github.com/Rain-kl/Wavelet/plugins/infra/cache"
	db "github.com/Rain-kl/Wavelet/plugins/infra/database"
)

const (
	taskExecutionLogRedisKeyPrefix = "task:execution:log:"
	taskExecutionLogExpiration     = 24 * time.Hour
	taskExecutionLogMaxLines       = 1000
)

func taskExecutionLogRedisKey(taskID string) string {
	return cachepkg.PrefixedKey(taskExecutionLogRedisKeyPrefix + taskID)
}

func createTaskExecution(ctx context.Context, execution *TaskExecution) error {
	if execution.ID == 0 {
		execution.ID = idgen.NextUint64ID()
	}
	return db.DB(ctx).Create(execution).Error
}

func updateTaskExecution(ctx context.Context, execution *TaskExecution) error {
	return db.DB(ctx).Omit("log").Save(execution).Error
}

func getTaskExecutionByID(ctx context.Context, id uint64) (*TaskExecution, error) {
	var execution TaskExecution
	if err := db.DB(ctx).Where("id = ?", id).First(&execution).Error; err != nil {
		return nil, err
	}
	_ = loadTaskExecutionLog(ctx, &execution)
	return &execution, nil
}

func getTaskExecutionByTaskID(ctx context.Context, taskID string) (*TaskExecution, error) {
	var execution TaskExecution
	if err := db.DB(ctx).Where("task_id = ?", taskID).First(&execution).Error; err != nil {
		return nil, err
	}
	_ = loadTaskExecutionLog(ctx, &execution)
	return &execution, nil
}

func appendTaskExecutionLog(ctx context.Context, taskID string, logLine string) error {
	if cachepkg.Redis == nil {
		return errors.New("redis client is not initialized")
	}

	now := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s\n", now, logLine)
	key := taskExecutionLogRedisKey(taskID)

	_, err := cachepkg.Redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.RPush(ctx, key, line)
		pipe.LTrim(ctx, key, -taskExecutionLogMaxLines, -1)
		pipe.Expire(ctx, key, taskExecutionLogExpiration)
		return nil
	})
	if err != nil {
		return fmt.Errorf("append task execution log to redis: %w", err)
	}
	return nil
}

func flushTaskExecutionLog(ctx context.Context, taskID string) error {
	if cachepkg.Redis == nil {
		return errors.New("redis client is not initialized")
	}

	key := taskExecutionLogRedisKey(taskID)
	logLines, err := cachepkg.Redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("get task execution log from redis: %w", err)
	}
	if len(logLines) == 0 {
		return nil
	}
	logText := strings.Join(logLines, "")

	result := db.DB(ctx).Model(&TaskExecution{}).
		Where("task_id = ?", taskID).
		Update("log", logText)
	if result.Error != nil {
		return fmt.Errorf("persist task execution log: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("persist task execution log: task %q not found", taskID)
	}

	if err := cachepkg.Redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete persisted task execution log from redis: %w", err)
	}
	return nil
}

func loadTaskExecutionLog(ctx context.Context, execution *TaskExecution) error {
	if cachepkg.Redis == nil {
		return nil
	}

	logLines, err := cachepkg.Redis.LRange(ctx, taskExecutionLogRedisKey(execution.TaskID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("get task execution log from redis: %w", err)
	}
	if len(logLines) == 0 {
		return nil
	}

	execution.Log = strings.Join(logLines, "")
	return nil
}
