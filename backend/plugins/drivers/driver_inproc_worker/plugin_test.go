// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker_test

import (
	"Wavelet/core"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/drivers/driver_inproc_worker"
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInprocWorkerPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := driver_inproc_worker.New(
		driver_inproc_worker.WithConcurrency(2),
		driver_inproc_worker.WithShutdownTimeout(time.Second),
	)

	assert.Equal(t, "driver_inproc_worker", p.Name())
	assert.Equal(t, core.DriverTypeWorker, p.Type())
	require.NoError(t, p.Apply(ctx))

	var executedCount atomic.Int32
	ctx.Tasks().Register("test:task", func(ctx context.Context, payload []byte) error {
		executedCount.Add(1)
		return nil
	}, extpoints.WithTaskTimeout(2*time.Second))

	// Start worker driver
	require.NoError(t, p.Start(context.Background()))

	// Enqueue tasks
	taskID, err := driver_inproc_worker.DispatchTask(context.Background(), "test:task", []byte("hello"), "test")
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// Wait for execution
	require.Eventually(t, func() bool {
		return executedCount.Load() == 1
	}, 2*time.Second, 20*time.Millisecond)

	// Stop driver
	require.NoError(t, p.Stop(context.Background()))
}
