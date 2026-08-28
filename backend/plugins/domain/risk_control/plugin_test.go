// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control_test

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/risk_control"
)

func TestRiskControlPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var customMWCalled bool
	customMW := func(c *gin.Context) {
		customMWCalled = true
		c.Next()
	}

	p := risk_control.New(risk_control.WithMiddleware(customMW))
	assert.Equal(t, "risk_control", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	// Verify middlewares registered
	mws := ctx.Router().Middlewares()
	assert.NotEmpty(t, mws)

	// Verify settings
	setting, ok := ctx.Settings().Get("risk_control.enable_access_log")
	require.True(t, ok)
	assert.Equal(t, true, setting.Default)

	require.NoError(t, ctx.Dispose())
	assert.False(t, customMWCalled) // not dispatched via gin engine here
}
