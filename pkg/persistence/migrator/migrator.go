// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package migrator 提供数据库迁移功能
package migrator

import (
	"context"
	"embed"
	"log"

	"github.com/Rain-kl/Wavelet/pkg/config"
	"github.com/Rain-kl/Wavelet/pkg/persistence"

	"github.com/pressly/goose/v3"
)

// migrationFS contains SQL migrations under goose/<dialect>.
//
//go:embed goose/postgres/*.sql goose/sqlite/*.sql
var migrationFS embed.FS

// dbType 返回当前数据库类型名称（用于日志输出）
func dbType() string {
	if !config.Config.Database.Enabled {
		return "SQLite"
	}
	return "PostgreSQL"
}

const (
	dialectSqlite   = "sqlite3"
	dialectPostgres = "postgres"
	cascadeSuffix   = " CASCADE"
)

// Report describes the database migration state observed during startup.
type Report struct {
	Backend string
	Enabled bool
	Version int64
	Applied bool
}

func gooseDialect() string {
	if !config.Config.Database.Enabled {
		return dialectSqlite
	}
	return dialectPostgres
}

func migrationDir() string {
	if !config.Config.Database.Enabled {
		return "goose/sqlite"
	}
	return "goose/postgres"
}

// Migrate 执行数据库迁移
func Migrate() Report {
	gormDB := db.DB(context.Background())
	if gormDB == nil {
		log.Fatalf("[%s] database not initialized\n", dbType())
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("[%s] load sql db failed: %v\n", dbType(), err)
	}

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect(gooseDialect()); err != nil {
		log.Fatalf("[%s] set goose dialect failed: %v\n", dbType(), err)
	}
	previousVersion, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		log.Fatalf("[%s] get goose version failed: %v\n", dbType(), err)
	}
	if err := goose.Up(sqlDB, migrationDir()); err != nil {
		log.Fatalf("[%s] goose migrate failed: %v\n", dbType(), err)
	}

	clearSystemConfigCache()
	currentVersion, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		log.Fatalf("[%s] get migrated goose version failed: %v\n", dbType(), err)
	}

	log.Printf("[%s] goose migrate success\n", dbType())
	return Report{
		Backend: dbType(),
		Enabled: true,
		Version: currentVersion,
		Applied: currentVersion != previousVersion,
	}
}

func clearSystemConfigCache() {
}
