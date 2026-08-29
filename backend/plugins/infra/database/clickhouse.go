// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package database 提供数据库连接与基础设施
package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel/attribute"
	clickhouseDriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

const (
	clickhouseMaxExecTime       = 60 // ClickHouse 最大执行时间（秒）
	clickhouseReadTimeoutFactor = 2  // ReadTimeout 为 DialTimeout 的倍数
)

var (
	// ChConn ClickHouse 原生连接实例，用于批量写入
	ChConn driver.Conn

	chDB *gorm.DB
)

// InitClickHouseWithConfig initializes the ClickHouse connection using the provided configuration.
func InitClickHouseWithConfig(cfg ClickHouseConfig) error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.Database == "" {
		return fmt.Errorf("[ClickHouse] database name is required (expected: wavelet)")
	}

	opts := buildClickHouseOptions(cfg)

	var err error
	ChConn, err = clickhouse.Open(opts)
	if err != nil {
		return fmt.Errorf("[ClickHouse] init connection failed: %w", err)
	}

	if err = ChConn.Ping(context.Background()); err != nil {
		return fmt.Errorf("[ClickHouse] ping failed: %w", err)
	}

	chDB, err = gorm.Open(clickhouseDriver.New(clickhouseDriver.Config{
		DSN: buildClickHouseDSN(cfg),
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return fmt.Errorf("[ClickHouse] init gorm connection failed: %w", err)
	}

	if err = chDB.Use(
		tracing.NewPlugin(
			tracing.WithoutMetrics(),
			tracing.WithAttributes(
				attribute.String("db.instance", cfg.Database),
				attribute.String("db.system", "ClickHouse"),
			),
		),
	); err != nil {
		return fmt.Errorf("[ClickHouse] init trace failed: %w", err)
	}

	sqlDB, err := chDB.DB()
	if err != nil {
		return fmt.Errorf("[ClickHouse] load sql db failed: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConn)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	log.Println("[ClickHouse] connection established successfully")
	return nil
}

func buildClickHouseOptions(cfg ClickHouseConfig) *clickhouse.Options {
	return &clickhouse.Options{
		Addr: cfg.Hosts,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": clickhouseMaxExecTime,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		MaxOpenConns:    cfg.MaxOpenConn,
		MaxIdleConns:    cfg.MaxIdleConn,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifetime) * time.Second,
		ReadTimeout:     time.Duration(cfg.DialTimeout*clickhouseReadTimeoutFactor) * time.Second,
		BlockBufferSize: cfg.BlockBufferSize,
	}
}

func buildClickHouseDSN(cfg ClickHouseConfig) string {
	chURL := &url.URL{
		Scheme: "clickhouse",
		Host:   strings.Join(cfg.Hosts, ","),
		Path:   "/" + cfg.Database,
	}
	if cfg.Username != "" || cfg.Password != "" {
		chURL.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	query := chURL.Query()
	query.Set("dial_timeout", fmt.Sprintf("%ds", cfg.DialTimeout))
	query.Set("read_timeout", fmt.Sprintf("%ds", cfg.DialTimeout*clickhouseReadTimeoutFactor))
	query.Set("max_execution_time", strconv.Itoa(clickhouseMaxExecTime))
	chURL.RawQuery = query.Encode()

	return chURL.String()
}

// ChDB returns a context-aware GORM ClickHouse instance.
func ChDB(ctx context.Context) *gorm.DB {
	if chDB == nil {
		return nil
	}
	return chDB.WithContext(ctx)
}

// SetChDBForTest sets the package-level ClickHouse GORM instance for testing.
func SetChDBForTest(d *gorm.DB) {
	chDB = d
}

// SetChConnForTest sets the package-level native ClickHouse connection for testing.
func SetChConnForTest(c driver.Conn) {
	ChConn = c
}
