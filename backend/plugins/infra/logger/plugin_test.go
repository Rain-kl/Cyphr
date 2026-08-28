// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logger_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/infra/logger"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerPluginOperations(t *testing.T) {
	p := logger.New()
	assert.Equal(t, "logger", p.Name())

	ctx := core.NewContext(context.Background())
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[contracts.LoggerService](ctx)
	require.NoError(t, err)
	require.NotNil(t, svc)

	testCtx := context.Background()

	svc.Debug(testCtx, "debug msg", "key", "val")
	svc.Info(testCtx, "info msg", "user", 1)
	svc.Warn(testCtx, "warn msg", "odd_field")
	svc.Error(testCtx, "error msg")

	svc.Debugf(testCtx, "debug %s", "formatted")
	svc.Infof(testCtx, "info %d", 100)
	svc.Warnf(testCtx, "warn %v", true)
	svc.Errorf(testCtx, "error %s", "fail")

	withLogger := svc.With("trace", "t-123", "span", "s-456")
	require.NotNil(t, withLogger)
	withLogger.Info(testCtx, "enriched message")
}
