// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	goosedb "github.com/pressly/goose/v3/database"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/config"
	"Wavelet/plugins/domain/admin"
	"Wavelet/plugins/domain/auth"
	"Wavelet/plugins/domain/cap"
	"Wavelet/plugins/domain/message_gateway"
	"Wavelet/plugins/domain/risk_control"
	"Wavelet/plugins/domain/system"
	"Wavelet/plugins/domain/upload"
	"Wavelet/plugins/domain/user"
	"Wavelet/plugins/drivers/driver_asynq_cron"
	"Wavelet/plugins/drivers/driver_asynq_worker"
	"Wavelet/plugins/drivers/driver_http"
	"Wavelet/plugins/drivers/driver_inproc_cron"
	"Wavelet/plugins/drivers/driver_inproc_worker"
	"Wavelet/plugins/infra/cache"
	"Wavelet/plugins/infra/cache_memory"
	infradb "Wavelet/plugins/infra/database"
	"Wavelet/plugins/infra/logger"
	"Wavelet/plugins/infra/storage"
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
		infradb.New(),
		logger.New(),
		storage.New(),
	)

	// 2. Register Cache and Async/Cron Drivers based on Redis configuration
	if config.Config.Redis.Enabled {
		app.Use(
			cache.New(),
			driver_asynq_worker.New(),
			driver_asynq_cron.New(),
		)
	} else {
		app.Use(
			cache_memory.New(),
			driver_inproc_worker.New(),
			driver_inproc_cron.New(),
		)
	}

	// 3. Register all 8 domain business plugins (admin first to ensure schema and base config tables exist)
	app.Use(
		admin.New(),
		user.New(),
		auth.New(),
		message_gateway.New(),
		risk_control.New(),
		upload.New(),
		cap.New(),
		system.New(),
	)

	// 4. Bind Goose migration engine
	app.SetMigrationEngine(&gooseEngine{})

	// 5. Mount HTTP runtime driver
	app.Use(
		driver_http.New(driver_http.WithAddr(config.Config.App.Addr)),
	)

	return app
}

// ─── Schema Version Store ──────────────────────────────────────────────────────

// sharedStore implements database.Store using a single w_schema_versions table.
// All plugins share this table, with plugin_id as the discriminator.
//
// Schema:
//
//	w_schema_versions (
//	    plugin_id   VARCHAR(64) NOT NULL,
//	    version_id  BIGINT      NOT NULL,
//	    applied_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
//	    PRIMARY KEY (plugin_id, version_id)
//	)
type sharedStore struct {
	pluginID string
	dialect  string // "postgres" or "sqlite3"
}

func (s *sharedStore) Tablename() string { return "w_schema_versions" }

func (s *sharedStore) CreateVersionTable(ctx context.Context, db goosedb.DBTxConn) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS w_schema_versions (
		plugin_id   VARCHAR(64)  NOT NULL,
		version_id  BIGINT       NOT NULL,
		applied_at  TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (plugin_id, version_id)
	)`)
	return err
}

//nolint:mnd
func (s *sharedStore) Insert(ctx context.Context, db goosedb.DBTxConn, req goosedb.InsertRequest) error {
	p := s.placeholder
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("INSERT INTO w_schema_versions (plugin_id, version_id) VALUES (%s, %s) ON CONFLICT (plugin_id, version_id) DO NOTHING", p(1), p(2)),
		s.pluginID, req.Version)
	return err
}

//nolint:mnd
func (s *sharedStore) Delete(ctx context.Context, db goosedb.DBTxConn, version int64) error {
	p := s.placeholder
	_, err := db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM w_schema_versions WHERE plugin_id = %s AND version_id = %s", p(1), p(2)),
		s.pluginID, version)
	return err
}

//nolint:mnd
func (s *sharedStore) GetMigration(ctx context.Context, db goosedb.DBTxConn, version int64) (*goosedb.GetMigrationResult, error) {
	p := s.placeholder
	var t time.Time
	err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT applied_at FROM w_schema_versions WHERE plugin_id = %s AND version_id = %s", p(1), p(2)),
		s.pluginID, version).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, goosedb.ErrVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &goosedb.GetMigrationResult{Timestamp: t, IsApplied: true}, nil
}

func (s *sharedStore) GetLatestVersion(ctx context.Context, db goosedb.DBTxConn) (int64, error) {
	p := s.placeholder
	var version int64
	err := db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COALESCE(MAX(version_id), 0) FROM w_schema_versions WHERE plugin_id = %s", p(1)),
		s.pluginID).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (s *sharedStore) ListMigrations(ctx context.Context, db goosedb.DBTxConn) ([]*goosedb.ListMigrationsResult, error) {
	p := s.placeholder
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf("SELECT version_id, TRUE FROM w_schema_versions WHERE plugin_id = %s ORDER BY version_id DESC", p(1)),
		s.pluginID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*goosedb.ListMigrationsResult
	for rows.Next() {
		var r goosedb.ListMigrationsResult
		if err := rows.Scan(&r.Version, &r.IsApplied); err != nil {
			return nil, err
		}
		results = append(results, &r)
	}
	return results, rows.Err()
}

func (s *sharedStore) placeholder(n int) string {
	if s.dialect == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// ─── Migration Engine ──────────────────────────────────────────────────────────

// gooseEngine implements core.MigrationEngine by iterating all plugin-registered
// migration entries and applying each plugin's migrations against the shared DB.
//
// Each plugin owns its own `migrations/*.sql` directory, embedded via go:embed
// and registered via ctx.Migrations().Register(pluginID, embedFS).
//
// Version tracking: all plugins share a single w_schema_versions table with
// plugin_id as the discriminator column. Querying this table shows the current
// migration version of every plugin at a glance.
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
		return fmt.Errorf("migration: get underlying DB from GORM: %w", err)
	}

	dialect := gooseDialect()
	dialectStr := string(dialect)

	for _, entry := range entries {
		store := &sharedStore{
			pluginID: entry.PluginID,
			dialect:  dialectStr,
		}

		migrationFS := findMigrationFS(entry.FS, dialect)
		provider, err := goose.NewProvider(goose.DialectCustom, sqlDB, migrationFS, goose.WithStore(store))
		if err != nil {
			return fmt.Errorf("migration %s: create provider: %w", entry.PluginID, err)
		}

		results, err := provider.Up(context.Background())
		if err != nil {
			return fmt.Errorf("migration %s: apply %w", entry.PluginID, err)
		}

		if len(results) > 0 {
			log.Printf("[migrate] %s: applied %d migration(s)", entry.PluginID, len(results))
		} else {
			log.Printf("[migrate] %s: up to date", entry.PluginID)
		}
	}

	return nil
}

// gooseDialect returns the goose dialect based on the configured database engine.
func gooseDialect() goose.Dialect {
	if !config.Config.Database.Enabled {
		return goose.DialectSQLite3
	}
	return goose.DialectPostgres
}

func findMigrationFS(rootFS fs.FS, dialect goose.Dialect) fs.FS {
	dialectDir := "postgres"
	if dialect == goose.DialectSQLite3 {
		dialectDir = "sqlite"
	}

	// 1. Direct search for dialect folder (e.g., "sqlite", "migrations/sqlite", "logstore/migrations/sqlite")
	for _, subDir := range []string{
		dialectDir,
		"migrations/" + dialectDir,
		"logstore/migrations/" + dialectDir,
	} {
		if sub, err := fs.Sub(rootFS, subDir); err == nil {
			if matches, err := fs.Glob(sub, "*.sql"); err == nil && len(matches) > 0 {
				return sub
			}
		}
	}

	// 2. Recursive walk to find a directory named dialectDir with *.sql files
	var foundDir string
	_ = fs.WalkDir(rootFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && filepath.Base(path) == dialectDir {
			if sub, subErr := fs.Sub(rootFS, path); subErr == nil {
				if matches, globErr := fs.Glob(sub, "*.sql"); globErr == nil && len(matches) > 0 {
					foundDir = path
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	if foundDir != "" && foundDir != "." {
		if sub, err := fs.Sub(rootFS, foundDir); err == nil {
			return sub
		}
	}

	// 3. Fallback to generic migrations / root if dialect specific is not present
	for _, subDir := range []string{"migrations", "logstore/migrations"} {
		if sub, err := fs.Sub(rootFS, subDir); err == nil {
			if matches, err := fs.Glob(sub, "*.sql"); err == nil && len(matches) > 0 {
				return sub
			}
		}
	}

	return rootFS
}
