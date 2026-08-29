// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// DatabaseConfig holds database configuration needed by the admin plugin.
type DatabaseConfig struct {
	Enabled    bool   `config:"enabled" env:"DB_ENABLED" default:"false" autoEnable:"DB_HOST"`
	Host       string `config:"host" env:"DB_HOST"`
	Port       int    `config:"port" env:"DB_PORT" default:"5432"`
	Database   string `config:"database" env:"DB_DATABASE"`
	Username   string `config:"username" env:"DB_USERNAME"`
	Password   string `config:"password" env:"DB_PASSWORD" secret:"true"`
	SQLitePath string `config:"sqlite_path" env:"DB_SQLITE_PATH" default:"./data/wavelet.db"`
}

// ClickHouseConfig holds clickhouse enablement status needed by admin log queries/switching.
type ClickHouseConfig struct {
	Enabled bool `config:"enabled" env:"CLICKHOUSE_ENABLED" default:"false" autoEnable:"CLICKHOUSE_HOST"`
}
