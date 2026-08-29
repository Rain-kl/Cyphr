// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package database

import "time"

// ReplicaConfig declares a single read-replica configuration for PostgreSQL.
type ReplicaConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Username string `config:"username"`
	Password string `config:"password" secret:"true"`
}

// Config declares PostgreSQL and SQLite database configuration.
type Config struct {
	Enabled                bool            `config:"enabled" env:"DB_ENABLED" default:"false" autoEnable:"DB_HOST"`
	SQLitePath             string          `config:"sqlite_path" env:"SQLITE_PATH"`
	Host                   string          `config:"host" env:"DB_HOST"`
	Port                   int             `config:"port" env:"DB_PORT" default:"5432"`
	Username               string          `config:"username" env:"DB_USERNAME"`
	Password               string          `config:"password" env:"DB_PASSWORD" secret:"true"`
	Database               string          `config:"database" env:"DB_NAME" default:"wavelet"`
	MaxIdleConn            int             `config:"max_idle_conn" env:"DB_MAX_IDLE_CONN" default:"10"`
	MaxOpenConn            int             `config:"max_open_conn" env:"DB_MAX_OPEN_CONN" default:"100"`
	ConnMaxLifetime        int             `config:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" default:"3600"`
	ConnMaxIdleTime        int             `config:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" default:"600"`
	LogLevel               string          `config:"log_level" env:"DB_LOG_LEVEL" default:"warn"`
	SSLMode                string          `config:"ssl_mode" env:"DB_SSL_MODE" default:"disable"`
	TimeZone               string          `config:"time_zone" env:"DB_TIMEZONE" default:"UTC"`
	ApplicationName        string          `config:"application_name" env:"DB_APPLICATION_NAME" default:"wavelet"`
	SearchPath             string          `config:"search_path" env:"DB_SEARCH_PATH" default:"public"`
	PreferSimpleProtocol   bool            `config:"prefer_simple_protocol" env:"DB_PREFER_SIMPLE_PROTOCOL"`
	StatementCacheCapacity int             `config:"statement_cache_capacity" env:"DB_STATEMENT_CACHE_CAPACITY"`
	DefaultQueryExecMode   string          `config:"default_query_exec_mode" env:"DB_DEFAULT_QUERY_EXEC_MODE"`
	Replicas               []ReplicaConfig `config:"replicas"`
	SlowThreshold          time.Duration   `config:"slow_threshold" env:"DB_SLOW_THRESHOLD" default:"200ms"`
}

// ClickHouseConfig declares the configuration for ClickHouse analytical storage.
type ClickHouseConfig struct {
	Enabled         bool     `config:"enabled" env:"CLICKHOUSE_ENABLED" default:"false" autoEnable:"CLICKHOUSE_HOST"`
	Hosts           []string `config:"hosts" env:"CLICKHOUSE_HOST"`
	Username        string   `config:"username" env:"CLICKHOUSE_USERNAME"`
	Password        string   `config:"password" env:"CLICKHOUSE_PASSWORD" secret:"true"`
	Database        string   `config:"database" env:"CLICKHOUSE_NAME" default:"wavelet"`
	MaxIdleConn     int      `config:"max_idle_conn" env:"CLICKHOUSE_MAX_IDLE_CONN" default:"10"`
	MaxOpenConn     int      `config:"max_open_conn" env:"CLICKHOUSE_MAX_OPEN_CONN" default:"50"`
	ConnMaxLifetime int      `config:"conn_max_lifetime" env:"CLICKHOUSE_CONN_MAX_LIFETIME" default:"3600"`
	DialTimeout     int      `config:"dial_timeout" env:"CLICKHOUSE_DIAL_TIMEOUT" default:"10"`
	BlockBufferSize uint8    `config:"block_buffer_size" env:"CLICKHOUSE_BLOCK_BUFFER_SIZE" default:"10"`
}

type appEnvConfig struct {
	Env string `config:"env" env:"APP_ENV" default:"development"`
}
