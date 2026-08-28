// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	cacheplugin "Wavelet/plugins/infra/cache"
)

// stubDBService 用内存 SQLite 满足 DBService 契约，隔离外部依赖。
type stubDBService struct{ db *gorm.DB }

func (s stubDBService) GORM() *gorm.DB { return s.db }

func (s stubDBService) DB(context.Context) *gorm.DB { return s.db }

func (s stubDBService) Named(string) *gorm.DB { return s.db }

// newFlushLogTestCache 构建真实多层缓存服务并注入 admin 插件上下文。
func newFlushLogTestCache(t *testing.T) (contracts.CacheService, *miniredis.Miniredis, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}})

	p := cacheplugin.New(cacheplugin.WithRedis(rdb), cacheplugin.WithRAMCapacity(64))
	ctx := core.NewContext(context.Background())
	require.NoError(t, p.Apply(ctx))
	svc, err := core.Inject[contracts.CacheService](ctx)
	require.NoError(t, err)

	repository.SetCacheService(svc)
	cleanup := func() {
		repository.SetCacheService(nil)
		_ = rdb.Close()
		mr.Close()
	}
	return svc, mr, cleanup
}

// TestFlushTaskExecutionLogPropagatesCacheError 回归：缓存读取失败（非未命中）时，
// FlushTaskExecutionLog 必须返回错误而不是静默吞掉日志并误报成功（nilerr 修复）。
func TestFlushTaskExecutionLogPropagatesCacheError(t *testing.T) {
	_, mr, cleanup := newFlushLogTestCache(t)
	defer cleanup()

	ctx := context.Background()
	const taskID = "flush-err-task"

	// 先缓冲一行日志
	require.NoError(t, repository.AppendTaskExecutionLog(ctx, taskID, "step-1 ok"))

	// 关闭 miniredis 模拟缓存基础设施故障（读取出错而非未命中）
	mr.Close()

	err := repository.FlushTaskExecutionLog(ctx, taskID)
	assert.Error(t, err, "缓存故障时必须返回错误，防止缓冲日志被静默丢弃")
}

// TestFlushTaskExecutionLogCacheMissIsNoop 回归：任务无缓冲日志（未命中）时应为空操作成功。
func TestFlushTaskExecutionLogCacheMissIsNoop(t *testing.T) {
	_, _, cleanup := newFlushLogTestCache(t)
	defer cleanup()

	ctx := context.Background()
	assert.NoError(t, repository.FlushTaskExecutionLog(ctx, "missing-task"))
}

// TestFlushTaskExecutionLogPersistsAndClears 验证正常路径：缓冲日志写入执行记录后清理缓存。
func TestFlushTaskExecutionLogPersistsAndClears(t *testing.T) {
	svc, _, cleanup := newFlushLogTestCache(t)
	defer cleanup()

	ctx := context.Background()
	const taskID = "flush-ok-task"
	require.NoError(t, repository.AppendTaskExecutionLog(ctx, taskID, "done"))

	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, sqliteDB.AutoMigrate(&model.TaskExecution{}))
	repository.SetDBService(stubDBService{db: sqliteDB})
	defer repository.SetDBService(nil)
	gormDB := sqliteDB
	exec := &model.TaskExecution{TaskID: taskID, TaskType: "upload:test", TaskName: "t", Status: model.TaskExecutionStatusSucceeded}
	require.NoError(t, gormDB.Create(exec).Error)

	require.NoError(t, repository.FlushTaskExecutionLog(ctx, taskID))

	var got model.TaskExecution
	require.NoError(t, gormDB.First(&got, exec.ID).Error)
	assert.Contains(t, got.Log, "done")

	// 缓存中的缓冲日志应已被清理
	var buf string
	err = svc.Get(ctx, repository.TaskExecutionLogRedisKey(taskID), &buf)
	assert.True(t, errors.Is(err, contracts.ErrCacheMiss), "flush 后缓存应清空, got %v", err)
}
