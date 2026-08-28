// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
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
	"github.com/pressly/goose/v3"
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

	// 3. Bind Goose migration engine
	app.SetMigrationEngine(&gooseEngine{})

	// 4. Mount runtime drivers for each aspect
	app.Use(
		driver_http.New(driver_http.WithAddr(config.Config.App.Addr)),
		driver_asynq_worker.New(),
		driver_asynq_cron.New(),
	)

	return app
}

// gooseEngine implements core.MigrationEngine by iterating all plugin-registered
// migration entries and applying each plugin's migrations against the shared DB.
//
// Each plugin owns its own `migrations/*.sql` directory, embedded via go:embed
// and registered via ctx.Migrations().Register(pluginID, embedFS). The engine
// resolves DBService from the IoC container and runs every entry in order.
type gooseEngine struct{}

func (e *gooseEngine) Migrate(ctx *core.Context, entries []core.MigrationEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Resolve DBService from the IoC container.
	var dbSvc contracts.DBService
	if err := core.Using[contracts.DBService](ctx, func(svc contracts.DBService) {
		dbSvc = svc
	}); err != nil {
		return fmt.Errorf("migration: resolve DBService: %w", err)
	}

	gormDB := dbSvc.GORM()
	if gormDB == nil {
		return fmt.Errorf("migration: DBService.GORM() returned nil")
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("migration: get underlying %s from GORM: %w", dialectName(), err)
	}

	dialect := dialectName()
	for _, entry := range entries {
		log.Printf("[migrate] applying %s migrations (%s)", entry.PluginID, entry.Dir)

		goose.SetBaseFS(entry.FS)
		if err := goose.SetDialect(dialect); err != nil {
			return fmt.Errorf("migration %s: set dialect %q: %w", entry.PluginID, dialect, err)
		}

		dir := entry.Dir
		if dir == "" {
			dir = "migrations"
		}

		if err := goose.Up(sqlDB, dir); err != nil {
			return fmt.Errorf("migration %s: apply %w", entry.PluginID, err)
		}

		log.Printf("[migrate] %s migrations applied", entry.PluginID)
	}

	return nil
}

// dialectName returns the goose dialect based on the configured database engine.
func dialectName() string {
	if !config.Config.Database.Enabled {
		return "sqlite3"
	}
	return "postgres"
}