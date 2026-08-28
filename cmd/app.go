// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	"github.com/Rain-kl/Wavelet/pkg/config"
	"github.com/Rain-kl/Wavelet/plugins/domain/admin"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	"github.com/Rain-kl/Wavelet/plugins/domain/cap"
	"github.com/Rain-kl/Wavelet/plugins/domain/message_gateway"
	"github.com/Rain-kl/Wavelet/plugins/domain/risk_control"
	"github.com/Rain-kl/Wavelet/plugins/domain/system"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload"
	"github.com/Rain-kl/Wavelet/plugins/domain/user"
	"github.com/Rain-kl/Wavelet/plugins/drivers/driver_asynq_cron"
	"github.com/Rain-kl/Wavelet/plugins/drivers/driver_asynq_worker"
	"github.com/Rain-kl/Wavelet/plugins/drivers/driver_http"
	"github.com/Rain-kl/Wavelet/plugins/infra/cache"
	"github.com/Rain-kl/Wavelet/plugins/infra/database"
	"github.com/Rain-kl/Wavelet/plugins/infra/logger"
	"github.com/Rain-kl/Wavelet/plugins/infra/storage"
)

// newWaveletApp creates a core.App wired with Wavelet platform infrastructure, domain plugins, and profile drivers.
//
//nolint:contextcheck
func newWaveletApp(profile core.Profile) *core.App {
	app := core.NewApp(
		core.WithProfile(profile),
		core.WithShutdownTimeout(time.Duration(config.Config.App.GracefulShutdownTimeout)*time.Second),
	)

	// 1. Register standard infrastructure plugins
	app.Use(
		database.New(),
		cache.New(),
		logger.New(),
		storage.New(),
	)

	// 2. Register all 8 domain business plugins
	app.Use(
		auth.New(),
		user.New(),
		message_gateway.New(),
		risk_control.New(),
		admin.New(),
		upload.New(),
		cap.New(),
		system.New(),
	)

	// 3. Bind Goose migration runner
	app.SetMigrationRunner(func(_ context.Context, _ []extpoints.MigrationEntry) error {
		runMigrations()
		return nil
	})

	// 4. Mount runtime drivers for each aspect
	app.Use(
		driver_http.New(driver_http.WithAddr(config.Config.App.Addr)),
		driver_asynq_worker.New(),
		driver_asynq_cron.New(),
	)

	return app
}
