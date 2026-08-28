// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package drivers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/drivers/driver_asynq_cron"
	"Wavelet/plugins/drivers/driver_asynq_worker"
	"Wavelet/plugins/drivers/driver_http"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHTTPDriverLifecycle(t *testing.T) {
	ctx := core.NewContext(context.Background())

	var globalMiddlewareCalled atomic.Bool
	var groupMiddlewareCalled atomic.Bool

	// Register global middleware
	ctx.Router().Use(func(c *gin.Context) {
		globalMiddlewareCalled.Store(true)
		c.Next()
	})

	// Register standard gin handler
	ctx.Router().GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Register route group with middleware
	v1 := ctx.Router().Group("/api/v1", func(c *gin.Context) {
		groupMiddlewareCalled.Store(true)
		c.Next()
	})

	v1.POST("/echo", func(c *gin.Context) {
		var req map[string]any
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, req)
	})

	// Register standard http.HandlerFunc
	v1.GET("/legacy", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("legacy response"))
	}))

	// Create and apply HTTP driver with dynamic port
	httpPlugin := driver_http.New(driver_http.WithAddr("127.0.0.1:0"))
	require.Equal(t, "driver_http", httpPlugin.Name())

	err := httpPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered in context
	d, ok := ctx.Driver(core.DriverTypeHTTP)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeHTTP, d.Type())

	// Start HTTP Server
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, httpPlugin.IsRunning())

	addr := httpPlugin.Addr()
	require.NotEmpty(t, addr)

	// Verify GET /ping
	resp, err := http.Get(fmt.Sprintf("http://%s/ping", addr))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "pong")
	assert.True(t, globalMiddlewareCalled.Load())

	// Verify POST /api/v1/echo
	echoPayload := []byte(`{"title":"test-echo","value":42}`)
	respEcho, err := http.Post(fmt.Sprintf("http://%s/api/v1/echo", addr), "application/json", bytes.NewReader(echoPayload))
	require.NoError(t, err)
	defer respEcho.Body.Close()

	assert.Equal(t, http.StatusOK, respEcho.StatusCode)
	var echoResult map[string]any
	err = json.NewDecoder(respEcho.Body).Decode(&echoResult)
	require.NoError(t, err)
	assert.Equal(t, "test-echo", echoResult["title"])
	assert.Equal(t, float64(42), echoResult["value"])
	assert.True(t, groupMiddlewareCalled.Load())

	// Verify GET /api/v1/legacy
	respLegacy, err := http.Get(fmt.Sprintf("http://%s/api/v1/legacy", addr))
	require.NoError(t, err)
	defer respLegacy.Body.Close()

	assert.Equal(t, http.StatusOK, respLegacy.StatusCode)
	bodyLegacy, err := io.ReadAll(respLegacy.Body)
	require.NoError(t, err)
	assert.Equal(t, "legacy response", string(bodyLegacy))

	// Stop HTTP Server
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, httpPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestAsynqWorkerDriverLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())

	var emailTaskProcessed atomic.Bool
	var simpleTaskProcessed atomic.Bool

	// Register tasks with various handler signatures
	ctx.Tasks().Register("email:send", func(c context.Context, payload []byte) error {
		var data map[string]string
		if err := json.Unmarshal(payload, &data); err != nil {
			return err
		}
		if data["to"] == "user@example.com" {
			emailTaskProcessed.Store(true)
		}
		return nil
	}, extpoints.WithTaskConcurrency(2))

	ctx.Tasks().Register("maintenance:cleanup", func() error {
		simpleTaskProcessed.Store(true)
		return nil
	})

	workerPlugin := driver_asynq_worker.New(
		driver_asynq_worker.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}),
		driver_asynq_worker.WithConcurrency(2),
		driver_asynq_worker.WithShutdownTimeout(1*time.Second),
	)
	require.Equal(t, "driver_asynq_worker", workerPlugin.Name())

	err = workerPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered
	d, ok := ctx.Driver(core.DriverTypeWorker)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeWorker, d.Type())

	// Start Worker Driver
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, workerPlugin.IsRunning())

	// Enqueue tasks using asynq.Client
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close()

	emailPayload, _ := json.Marshal(map[string]string{"to": "user@example.com"})
	_, err = client.Enqueue(asynq.NewTask("email:send", emailPayload))
	require.NoError(t, err)

	_, err = client.Enqueue(asynq.NewTask("maintenance:cleanup", nil))
	require.NoError(t, err)

	// Wait for worker to consume tasks
	require.Eventually(t, func() bool {
		return emailTaskProcessed.Load() && simpleTaskProcessed.Load()
	}, 3*time.Second, 50*time.Millisecond)

	// Stop Worker Driver
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, workerPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestAsynqCronDriverLifecycle(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())

	// Register cron schedules
	ctx.Schedules().RegisterCron("@every 1s", "sync:stats", map[string]string{"type": "daily"},
		extpoints.WithScheduleOption("queue", "default"),
		extpoints.WithScheduleOption("retry", 3),
	)
	ctx.Schedules().RegisterCron("0 0 * * *", "report:generate", "raw_payload")

	cronPlugin := driver_asynq_cron.New(
		driver_asynq_cron.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}),
		driver_asynq_cron.WithLocation(time.UTC),
	)
	require.Equal(t, "driver_asynq_cron", cronPlugin.Name())

	err = cronPlugin.Apply(ctx)
	require.NoError(t, err)

	// Verify driver registered
	d, ok := ctx.Driver(core.DriverTypeScheduler)
	require.True(t, ok)
	require.Equal(t, core.DriverTypeScheduler, d.Type())

	// Start Cron Scheduler Driver
	err = d.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, cronPlugin.IsRunning())

	// Stop Cron Scheduler Driver
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = d.Stop(stopCtx)
	require.NoError(t, err)
	assert.False(t, cronPlugin.IsRunning())

	// Idempotent Stop
	err = d.Stop(stopCtx)
	require.NoError(t, err)
}

func TestMultipleDriversInContext(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := core.NewContext(context.Background())

	httpPlugin := driver_http.New(driver_http.WithAddr("127.0.0.1:0"))
	workerPlugin := driver_asynq_worker.New(driver_asynq_worker.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}))
	cronPlugin := driver_asynq_cron.New(driver_asynq_cron.WithRedisOpt(asynq.RedisClientOpt{Addr: mr.Addr()}))

	require.NoError(t, httpPlugin.Apply(ctx))
	require.NoError(t, workerPlugin.Apply(ctx))
	require.NoError(t, cronPlugin.Apply(ctx))

	drivers := ctx.Drivers()
	assert.Len(t, drivers, 3)

	_, ok := ctx.Driver(core.DriverTypeHTTP)
	assert.True(t, ok)

	_, ok = ctx.Driver(core.DriverTypeWorker)
	assert.True(t, ok)

	_, ok = ctx.Driver(core.DriverTypeScheduler)
	assert.True(t, ok)

	_, ok = ctx.Driver("non_existent")
	assert.False(t, ok)
}
