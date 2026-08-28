// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway_test

import (
	"Wavelet/core"
	"Wavelet/plugins/domain/message_gateway"
	"context"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageGatewayPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := message_gateway.New()
	assert.Equal(t, "message_gateway", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	// Verify migrations
	entry, ok := ctx.Migrations().Get("message_gateway")
	require.True(t, ok)
	entries, err := fs.ReadDir(entry.FS, entry.Dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Verify tasks
	task, ok := ctx.Tasks().Get("message_gateway:push_notification")
	require.True(t, ok)
	assert.Equal(t, 3, task.Retry)

	// Verify schedules
	sched, ok := ctx.Schedules().Get("message_gateway:cleanup_pairing_codes")
	require.True(t, ok)
	assert.Equal(t, "*/10 * * * *", sched.Spec)

	// Verify settings
	setting, ok := ctx.Settings().Get("message_gateway.max_bindings_per_user")
	require.True(t, ok)
	assert.Equal(t, 5, setting.Default)
}
