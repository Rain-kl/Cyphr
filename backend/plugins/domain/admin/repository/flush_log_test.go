// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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
	ctx.Config().SetSource(core.NewMapSource(map[string]any{
		"redis": map[string]any{
			"enabled": true,
			"addrs":   []string{mr.Addr()},
		},
	}))
	require.NoError(t, ctx.Config().Resolve())
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

// readFailCache 读取永远报错而写入成功，用于区分「未命中」与「缓存故障」两种语义。
type readFailCache struct {
	writes []string
}

func (c *readFailCache) Get(context.Context, string, any) error {
	return errors.New("cache unavailable")
}

func (c *readFailCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	c.writes = append(c.writes, fmt.Sprintf("%s=%v", key, value))
	return nil
}

func (c *readFailCache) Delete(context.Context, string) error { return nil }

func (c *readFailCache) GetOrSet(context.Context, string, any, time.Duration, func() (any, error)) error {
	return errors.New("cache unavailable")
}

func (c *readFailCache) Invalidate(context.Context, string) error { return nil }

// TestAppendTaskExecutionLogKeepsBufferOnCacheReadError 回归：缓存读取失败（而非未命中）时
// 不得把「读不到」当成「没有缓冲」继续写入，否则整段任务日志会被最新一行覆盖丢失。
func TestAppendTaskExecutionLogKeepsBufferOnCacheReadError(t *testing.T) {
	fake := &readFailCache{}
	repository.SetCacheService(fake)
	defer repository.SetCacheService(nil)

	err := repository.AppendTaskExecutionLog(context.Background(), "append-err-task", "step-2")
	assert.Error(t, err, "缓存故障必须上抛，而不是覆盖缓冲")
	assert.Empty(t, fake.writes, "读取失败时不得写入，避免覆盖已缓冲日志")
}
