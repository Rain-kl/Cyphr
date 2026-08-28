// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/config"
)

func TestRedisPluggability_Simulation(t *testing.T) {
	origRedisEnabled := config.Config.Redis.Enabled
	defer func() { config.Config.Redis.Enabled = origRedisEnabled }()

	// ══════════════════════════════════════════════════════════════════════════
	// 场景 1: 拔出 Redis (Zero-Redis Monolith 模式)
	// ══════════════════════════════════════════════════════════════════════════
	t.Run("Scenario_Unplugged_ZeroRedis_Mode", func(t *testing.T) {
		config.Config.Redis.Enabled = false

		app := newWaveletApp(core.ProfileAll)
		require.NotNil(t, app)

		// 1. 验证插件挂载形态
		_, ok := app.Plugin("cache_memory")
		assert.True(t, ok, "cache_memory 必须挂载")
		_, ok = app.Plugin("driver_inproc_worker")
		assert.True(t, ok, "driver_inproc_worker 必须挂载")
		_, ok = app.Plugin("driver_inproc_cron")
		assert.True(t, ok, "driver_inproc_cron 必须挂载")

		_, ok = app.Plugin("cache")
		assert.False(t, ok, "分布式 cache 不得挂载")
		_, ok = app.Plugin("driver_asynq_worker")
		assert.False(t, ok, "asynq_worker 不得挂载")
		_, ok = app.Plugin("driver_asynq_cron")
		assert.False(t, ok, "asynq_cron 不得挂载")

		// 2. 注册测试任务与 Cron 定时
		var taskExecuted atomic.Int32
		var cronExecuted atomic.Int32

		app.Context().Tasks().Register("test:inproc_task", func(ctx context.Context, payload []byte) error {
			if string(payload) == "payload_unplugged" {
				taskExecuted.Add(1)
			}
			return nil
		}, extpoints.WithTaskTimeout(3*time.Second))

		app.Context().Schedules().RegisterCron("* * * * * *", "test:inproc_cron", []byte("cron_ping"))
		app.Context().Tasks().Register("test:inproc_cron", func(ctx context.Context, payload []byte) error {
			if string(payload) == "cron_ping" {
				cronExecuted.Add(1)
			}
			return nil
		})

		// 3. 启动应用
		bootCtx, bootCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bootCancel()
		require.NoError(t, app.Start(bootCtx))

		// 4. 验证 CacheService 操作
		cacheSvc, err := core.Inject[contracts.CacheService](app.Context())
		require.NoError(t, err)
		require.NotNil(t, cacheSvc)

		reqCtx := context.Background()
		require.NoError(t, cacheSvc.Set(reqCtx, "unplugged_key", "value_123", time.Minute))
		var val string
		require.NoError(t, cacheSvc.Get(reqCtx, "unplugged_key", &val))
		assert.Equal(t, "value_123", val)

		// 5. 验证异步 Worker 任务分发与执行
		taskSvc, err := core.Inject[contracts.TaskService](app.Context())
		require.NoError(t, err)
		require.NotNil(t, taskSvc)

		taskID, err := taskSvc.Dispatch(reqCtx, "test:inproc_task", []byte("payload_unplugged"), "unit_test")
		require.NoError(t, err)
		assert.NotEmpty(t, taskID)

		require.Eventually(t, func() bool {
			return taskExecuted.Load() >= 1
		}, 3*time.Second, 50*time.Millisecond, "内存 Worker 应在进程内顺利执行任务")

		// 6. 验证 Cron 定时触发
		require.Eventually(t, func() bool {
			return cronExecuted.Load() >= 1
		}, 3*time.Second, 100*time.Millisecond, "内存 Cron 驱动应成功触发定时任务")

		// 7. 优雅关闭
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		require.NoError(t, app.Stop(stopCtx))
	})

	// ══════════════════════════════════════════════════════════════════════════
	// 场景 2: 插入 Redis (Distributed Cluster 模式)
	// ══════════════════════════════════════════════════════════════════════════
	t.Run("Scenario_Plugged_Redis_Mode", func(t *testing.T) {
		config.Config.Redis.Enabled = true

		app := newWaveletApp(core.ProfileAll)
		require.NotNil(t, app)

		// 1. 验证插件挂载形态
		_, ok := app.Plugin("cache")
		assert.True(t, ok, "分布式 cache 必须挂载")
		_, ok = app.Plugin("driver_asynq_worker")
		assert.True(t, ok, "driver_asynq_worker 必须挂载")
		_, ok = app.Plugin("driver_asynq_cron")
		assert.True(t, ok, "driver_asynq_cron 必须挂载")

		_, ok = app.Plugin("cache_memory")
		assert.False(t, ok, "纯内存 cache 不得挂载")
		_, ok = app.Plugin("driver_inproc_worker")
		assert.False(t, ok, "inproc_worker 不得挂载")
		_, ok = app.Plugin("driver_inproc_cron")
		assert.False(t, ok, "inproc_cron 不得挂载")

		// 2. 注册测试任务
		var asynqTaskExecuted atomic.Int32
		app.Context().Tasks().Register("test:asynq_task", func(ctx context.Context, payload []byte) error {
			if string(payload) == "payload_plugged" {
				asynqTaskExecuted.Add(1)
			}
			return nil
		}, extpoints.WithTaskTimeout(3*time.Second))

		// 3. 启动应用 (连接真实运行中的 Redis 6379)
		bootCtx, bootCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bootCancel()
		require.NoError(t, app.Start(bootCtx))

		// 4. 验证 CacheService 操作 (L1 RAM + L2 Redis)
		cacheSvc, err := core.Inject[contracts.CacheService](app.Context())
		require.NoError(t, err)
		require.NotNil(t, cacheSvc)

		reqCtx := context.Background()
		testKey := fmt.Sprintf("plugged_key_%d", time.Now().UnixNano())
		require.NoError(t, cacheSvc.Set(reqCtx, testKey, "value_redis_cluster", time.Minute))

		var val string
		require.NoError(t, cacheSvc.Get(reqCtx, testKey, &val))
		assert.Equal(t, "value_redis_cluster", val)

		// 验证失效广播与删除
		require.NoError(t, cacheSvc.Delete(reqCtx, testKey))
		var valAfterDelete string
		err = cacheSvc.Get(reqCtx, testKey, &valAfterDelete)
		assert.ErrorIs(t, err, contracts.ErrCacheMiss)

		// 5. 验证 Asynq Worker 任务分发与消费
		taskSvc, err := core.Inject[contracts.TaskService](app.Context())
		require.NoError(t, err)
		require.NotNil(t, taskSvc)

		taskID, err := taskSvc.Dispatch(reqCtx, "test:asynq_task", []byte("payload_plugged"), "unit_test")
		require.NoError(t, err)
		assert.NotEmpty(t, taskID)

		require.Eventually(t, func() bool {
			return asynqTaskExecuted.Load() >= 1
		}, 5*time.Second, 100*time.Millisecond, "Asynq Worker 应从 Redis 队列中成功消费并执行任务")

		// 6. 优雅关闭
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		require.NoError(t, app.Stop(stopCtx))
	})

	// ══════════════════════════════════════════════════════════════════════════
	// 场景 3: 往复插拔连续切换 (拔出 → 插入 → 再拔出，验证时空可组合性与零残留)
	// ══════════════════════════════════════════════════════════════════════════
	t.Run("Scenario_Dynamic_Plug_Unplug_Sequence", func(t *testing.T) {
		for i := 1; i <= 2; i++ {
			// 1. 拔出 Redis 运行
			config.Config.Redis.Enabled = false
			appUnplugged := newWaveletApp(core.ProfileAll)
			require.NoError(t, appUnplugged.Start(context.Background()))

			cacheSvc1, err := core.Inject[contracts.CacheService](appUnplugged.Context())
			require.NoError(t, err)
			require.NoError(t, cacheSvc1.Set(context.Background(), fmt.Sprintf("seq_key_%d", i), "seq_val_unplugged", time.Minute))

			require.NoError(t, appUnplugged.Stop(context.Background()))

			// 2. 插入 Redis 运行
			config.Config.Redis.Enabled = true
			appPlugged := newWaveletApp(core.ProfileAll)
			require.NoError(t, appPlugged.Start(context.Background()))

			cacheSvc2, err := core.Inject[contracts.CacheService](appPlugged.Context())
			require.NoError(t, err)
			require.NoError(t, cacheSvc2.Set(context.Background(), fmt.Sprintf("seq_key_%d", i), "seq_val_plugged", time.Minute))

			require.NoError(t, appPlugged.Stop(context.Background()))
		}
	})
}
