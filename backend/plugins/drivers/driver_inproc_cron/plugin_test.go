// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_cron_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/plugins/drivers/driver_inproc_cron"
)

func TestInprocCronPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())

	cronPlugin := driver_inproc_cron.New()
	assert.Equal(t, "driver_inproc_cron", cronPlugin.Name())
	assert.Equal(t, core.DriverTypeScheduler, cronPlugin.Type())
	require.NoError(t, cronPlugin.Apply(ctx))

	var cronTriggered atomic.Int32
	ctx.Tasks().Register("cron:heartbeat", func(ctx context.Context) error {
		cronTriggered.Add(1)
		return nil
	})

	// Every second cron spec
	ctx.Schedules().RegisterCron("* * * * * *", "cron:heartbeat", nil)

	require.NoError(t, cronPlugin.Start(context.Background()))

	require.Eventually(t, func() bool {
		return cronTriggered.Load() >= 1
	}, 3*time.Second, 100*time.Millisecond)

	require.NoError(t, cronPlugin.Stop(context.Background()))
}
