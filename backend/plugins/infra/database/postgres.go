// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"gorm.io/plugin/opentelemetry/tracing"
)

var db *gorm.DB

const sqliteDirMode = 0o750

// InitDB 初始化主数据库实例（支持 PostgreSQL / SQLite）
func InitDB() (*gorm.DB, error) {
	return InitDBWithConfig(Config{}, false)
}

// InitDBWithConfig initializes the main database with the provided config.
func InitDBWithConfig(cfg Config, isProd bool) (*gorm.DB, error) {
	if !cfg.Enabled {
		return initSQLiteWithConfig(cfg, isProd)
	}
	return initPostgresWithConfig(cfg, isProd)
}

func initSQLiteWithConfig(cfg Config, isProd bool) (*gorm.DB, error) {
	sqlitePath := cfg.SQLitePath
	if sqlitePath == "" {
		sqlitePath = "./data/wavelet.db"
	}

	if sqlitePath != ":memory:" && !strings.HasPrefix(sqlitePath, "file:") {
		if dir := filepath.Dir(sqlitePath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, sqliteDirMode); err != nil {
				return nil, fmt.Errorf("create sqlite directory %q failed: %w", dir, err)
			}
		}
	}

	targetDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: &gormZapLogger{
			logLevel:                  parseLogLevel(cfg.LogLevel),
			slowThreshold:             cfg.SlowThreshold,
			ignoreRecordNotFoundError: isProd,
		},
	})
	if err != nil {
		return nil, err
	}

	// Trace 注入
	if err = targetDB.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", sqlitePath),
				attribute.String("db.system", "SQLite"),
			),
		),
	); err != nil {
		return nil, err
	}

	db = targetDB
	log.Printf("[SQLite] initialized (path: %s)\n", sqlitePath)
	return targetDB, nil
}

func initPostgresWithConfig(cfg Config, isProd bool) (*gorm.DB, error) {
	// 构建主库 DSN 并连接
	primaryDSN := buildDSN(cfg, cfg.Host, cfg.Port, cfg.Username, cfg.Password)

	pgConfig := postgres.Config{
		DSN:                  primaryDSN,
		PreferSimpleProtocol: cfg.PreferSimpleProtocol,
	}

	targetDB, err := gorm.Open(postgres.New(pgConfig), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: &gormZapLogger{
			logLevel:                  parseLogLevel(cfg.LogLevel),
			slowThreshold:             cfg.SlowThreshold,
			ignoreRecordNotFoundError: isProd,
		},
	})
	if err != nil {
		return nil, err
	}

	// Trace 注入
	if err = targetDB.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", cfg.Database),
				attribute.String("db.ip", cfg.Host),
				attribute.String("server.address", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))),
				attribute.String("db.system", "PostgreSQL"),
			),
		),
	); err != nil {
		return nil, err
	}

	if len(cfg.Replicas) > 0 {
		var replicaDialectors []gorm.Dialector
		for _, replica := range cfg.Replicas {
			username := replica.Username
			if username == "" {
				username = cfg.Username
			}
			password := replica.Password
			if password == "" {
				password = cfg.Password
			}
			replicaDSN := buildDSN(cfg, replica.Host, replica.Port, username, password)
			replicaDialectors = append(replicaDialectors, postgres.New(postgres.Config{
				DSN:                  replicaDSN,
				PreferSimpleProtocol: cfg.PreferSimpleProtocol,
			}))
		}

		resolver := dbresolver.Register(dbresolver.Config{
			Replicas: replicaDialectors,
			Policy:   dbresolver.RandomPolicy{},
		})

		resolver.SetMaxIdleConns(cfg.MaxIdleConn).
			SetMaxOpenConns(cfg.MaxOpenConn).
			SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second).
			SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

		if err = targetDB.Use(resolver); err != nil {
			return nil, err
		}
		log.Printf("[PostgreSQL] initialized in Primary-Replica mode (%d replicas)\n", len(cfg.Replicas))
	} else {
		log.Println("[PostgreSQL] initialized in Standalone mode")
	}

	// 获取通用数据库对象设置连接池
	sqlDB, err := targetDB.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConn)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)

	db = targetDB
	return targetDB, nil
}

// buildDSN 构建 PostgreSQL DSN
func buildDSN(cfg Config, host string, port int, username, password string) string {
	pqURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   cfg.Database,
	}
	if username != "" {
		pqURL.User = url.UserPassword(username, password)
	}

	query := pqURL.Query()
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	query.Set("sslmode", sslMode)
	if cfg.ApplicationName != "" {
		query.Set("application_name", cfg.ApplicationName)
	}
	if cfg.SearchPath != "" {
		query.Set("search_path", cfg.SearchPath)
	}
	if cfg.DefaultQueryExecMode != "" {
		query.Set("default_query_exec_mode", cfg.DefaultQueryExecMode)
	}
	if cfg.StatementCacheCapacity > 0 {
		query.Set("statement_cache_capacity", strconv.Itoa(cfg.StatementCacheCapacity))
	}

	rawQuery := query.Encode()
	if cfg.TimeZone != "" {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += "TimeZone=" + cfg.TimeZone
	}
	pqURL.RawQuery = rawQuery

	return pqURL.String()
}

// DB 返回带上下文追踪的 GORM 数据库实例
func DB(ctx context.Context) *gorm.DB {
	if db == nil {
		return nil
	}
	return db.WithContext(ctx)
}

// SetDB sets the package-level database instance for testing.
func SetDB(d *gorm.DB) {
	db = d
}
