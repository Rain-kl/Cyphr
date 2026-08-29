// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Temporary migration harness: proves the kernel configuration engine resolves every
// key identically to pkg/config. Delete this file together with backend/pkg/config in P4.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core/extpoints"
)

// yamlSource is a test-local ConfigSource over the repository configuration file.
// It deliberately does not import plugins/infra/config: backend/pkg must not depend on
// upper layers even in tests, and the adapter has its own coverage in its package tests.
type yamlSource struct {
	v *viper.Viper
}

func newYAMLSource(t *testing.T, path string) *yamlSource {
	t.Helper()

	v := viper.New()
	v.SetConfigFile(path)
	require.NoError(t, v.ReadInConfig())
	return &yamlSource{v: v}
}

func (s *yamlSource) Lookup(path string) (any, bool) {
	if !s.v.IsSet(path) {
		return nil, false
	}
	return s.v.Get(path), true
}

func (s *yamlSource) LookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

func (s *yamlSource) Describe() string { return s.v.ConfigFileUsed() }

// The mirror declarations below reproduce the legacy loader's coverage exactly, which is
// why several keys declare no env: the legacy applyEnvOverrides only honoured an
// environment variable for a subset of fields. Widening that set is a declaration-time
// choice available to each plugin in P3, not an engine behaviour change.
type engineAppConfig struct {
	AppName                 string `config:"app_name" env:"APP_NAME"`
	Env                     string `config:"env" env:"APP_ENV"`
	Addr                    string `config:"addr" env:"APP_ADDR"`
	NodeID                  int64  `config:"node_id" env:"APP_NODE_ID"`
	APIPrefix               string `config:"api_prefix" env:"APP_API_PREFIX"`
	GracefulShutdownTimeout int    `config:"graceful_shutdown_timeout" env:"APP_GRACEFUL_SHUTDOWN_TIMEOUT"`
	SessionCookieName       string `config:"session_cookie_name" env:"APP_SESSION_COOKIE_NAME"`
	SessionSecret           string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
	SessionDomain           string `config:"session_domain" env:"APP_SESSION_DOMAIN"`
	SessionAge              int    `config:"session_age" env:"APP_SESSION_AGE" default:"86400"`
	SessionHTTPOnly         bool   `config:"session_http_only" env:"APP_SESSION_HTTP_ONLY"`
	SessionSecure           bool   `config:"session_secure" env:"APP_SESSION_SECURE"`
}

type engineReplicaConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Username string `config:"username"`
	Password string `config:"password"`
}

type engineDatabaseConfig struct {
	Enabled                bool                  `config:"enabled" env:"DB_ENABLED" default:"false" autoEnable:"DB_HOST"`
	SQLitePath             string                `config:"sqlite_path" env:"SQLITE_PATH"`
	Host                   string                `config:"host" env:"DB_HOST"`
	Port                   int                   `config:"port" env:"DB_PORT"`
	Username               string                `config:"username" env:"DB_USERNAME"`
	Password               string                `config:"password" env:"DB_PASSWORD" secret:"true"`
	Database               string                `config:"database" env:"DB_NAME"`
	MaxIdleConn            int                   `config:"max_idle_conn" env:"DB_MAX_IDLE_CONN"`
	MaxOpenConn            int                   `config:"max_open_conn" env:"DB_MAX_OPEN_CONN"`
	ConnMaxLifetime        int                   `config:"conn_max_lifetime"`
	ConnMaxIdleTime        int                   `config:"conn_max_idle_time"`
	LogLevel               string                `config:"log_level" env:"DB_LOG_LEVEL"`
	SSLMode                string                `config:"ssl_mode" env:"DB_SSL_MODE"`
	TimeZone               string                `config:"time_zone" env:"DB_TIMEZONE"`
	ApplicationName        string                `config:"application_name"`
	SearchPath             string                `config:"search_path"`
	PreferSimpleProtocol   bool                  `config:"prefer_simple_protocol"`
	StatementCacheCapacity int                   `config:"statement_cache_capacity"`
	DefaultQueryExecMode   string                `config:"default_query_exec_mode"`
	Replicas               []engineReplicaConfig `config:"replicas"`
	SlowThreshold          time.Duration         `config:"slow_threshold"`
}

type engineRedisConfig struct {
	Enabled            bool     `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs              []string `config:"addrs" env:"REDIS_ADDR"`
	Username           string   `config:"username" env:"REDIS_USERNAME"`
	Password           string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB                 int      `config:"db" env:"REDIS_DB"`
	ClusterMode        bool     `config:"cluster_mode"`
	MasterName         string   `config:"master_name"`
	KeyPrefix          string   `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	PoolSize           int      `config:"pool_size" env:"REDIS_POOL_SIZE"`
	MinIdleConn        int      `config:"min_idle_conn"`
	DialTimeout        int      `config:"dial_timeout"`
	ReadTimeout        int      `config:"read_timeout"`
	WriteTimeout       int      `config:"write_timeout"`
	MaxRetries         int      `config:"max_retries"`
	PoolTimeout        int      `config:"pool_timeout"`
	ConnMaxIdleTime    int      `config:"conn_max_idle_time"`
	MaintNotifications bool     `config:"maint_notifications" env:"REDIS_MAINT_NOTIFICATIONS"`
}

type engineClickHouseConfig struct {
	Enabled         bool     `config:"enabled" env:"CLICKHOUSE_ENABLED" default:"false" autoEnable:"CLICKHOUSE_HOST"`
	Hosts           []string `config:"hosts" env:"CLICKHOUSE_HOST"`
	Username        string   `config:"username" env:"CLICKHOUSE_USERNAME"`
	Password        string   `config:"password" env:"CLICKHOUSE_PASSWORD" secret:"true"`
	Database        string   `config:"database" env:"CLICKHOUSE_NAME"`
	MaxIdleConn     int      `config:"max_idle_conn"`
	MaxOpenConn     int      `config:"max_open_conn"`
	ConnMaxLifetime int      `config:"conn_max_lifetime"`
	DialTimeout     int      `config:"dial_timeout"`
	BlockBufferSize uint8    `config:"block_buffer_size"`
}

type engineLogConfig struct {
	Level      string `config:"level" env:"LOG_LEVEL"`
	Format     string `config:"format" env:"LOG_FORMAT"`
	Output     string `config:"output" env:"LOG_OUTPUT"`
	FilePath   string `config:"file_path"`
	MaxSize    int    `config:"max_size"`
	MaxAge     int    `config:"max_age"`
	MaxBackups int    `config:"max_backups"`
	Compress   bool   `config:"compress"`
}

type engineOtelConfig struct {
	SamplingRate float64 `config:"sampling_rate" env:"OTEL_SAMPLING_RATE"`
	TracerName   string  `config:"tracer_name" env:"OTEL_TRACER_NAME" default:"github.com/Rain-kl/Wavelet"`
}

type engineQueueConfig struct {
	Name     string `config:"name"`
	Priority int    `config:"priority"`
}

type engineWorkerConfig struct {
	Concurrency    int                 `config:"concurrency" env:"WORKER_CONCURRENCY"`
	StrictPriority bool                `config:"strict_priority" env:"WORKER_STRICT_PRIORITY"`
	Queues         []engineQueueConfig `config:"queues"`
}

// durationType mirrors the engine's own notion of a scalar duration field.
var durationType = reflect.TypeFor[time.Duration]()

// flatten exports a struct into dotted leaf paths rendered as text. Both sides of the
// parity assertion use distinct Go types for the same shape, so values are compared
// textually instead of handing cmp a cross-type diff.
func flatten(prefix string, v reflect.Value, out map[string]string) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fv := v.Field(i)
		path := prefix + "." + field.Name
		if fv.Kind() == reflect.Struct && fv.Type() != durationType {
			flatten(path, fv, out)
			continue
		}
		out[path] = fmt.Sprint(fv.Interface())
	}
}

// configCandidates returns the configuration files to run parity against. The tracked
// example file is mandatory so the proof never skips silently in a fresh clone. A local
// gitignored config.yaml is compared as an extra scenario only when it sits next to the
// example file: the upward search used to locate it can otherwise leave the current
// checkout and silently read a sibling worktree's configuration instead.
func configCandidates(t *testing.T) []string {
	t.Helper()

	example := findConfigPath("config.example.yaml")
	info, err := os.Stat(example)
	if err != nil || info.IsDir() {
		t.Fatalf("tracked config.example.yaml is unreachable from the test working directory: %v", err)
	}

	candidates := []string{example}
	local := filepath.Join(filepath.Dir(example), "config.yaml")
	if localInfo, localErr := os.Stat(local); localErr == nil && !localInfo.IsDir() {
		candidates = append(candidates, local)
	}
	return candidates
}

// bindSections declares every legacy section against the engine and binds them out.
func bindSections(t *testing.T, path string) map[string]string {
	t.Helper()

	engine := extpoints.NewConfigRegistry(newYAMLSource(t, path))
	require.NoError(t, engine.Declare("parity",
		extpoints.ConfigBinding{Prefix: "app", Target: &engineAppConfig{}},
		extpoints.ConfigBinding{Prefix: "database", Target: &engineDatabaseConfig{}},
		extpoints.ConfigBinding{Prefix: "redis", Target: &engineRedisConfig{}},
		extpoints.ConfigBinding{Prefix: "clickhouse", Target: &engineClickHouseConfig{}},
		extpoints.ConfigBinding{Prefix: "log", Target: &engineLogConfig{}},
		extpoints.ConfigBinding{Prefix: "otel", Target: &engineOtelConfig{}},
		extpoints.ConfigBinding{Prefix: "worker", Target: &engineWorkerConfig{}},
	))
	require.NoError(t, engine.Resolve())

	var (
		app        engineAppConfig
		database   engineDatabaseConfig
		redis      engineRedisConfig
		clickhouse engineClickHouseConfig
		log        engineLogConfig
		otel       engineOtelConfig
		worker     engineWorkerConfig
	)
	targets := []struct {
		prefix string
		target any
	}{
		{"app", &app}, {"database", &database}, {"redis", &redis}, {"clickhouse", &clickhouse},
		{"log", &log}, {"otel", &otel}, {"worker", &worker},
	}
	for _, item := range targets {
		require.NoError(t, engine.Bind(item.prefix, item.target))
	}

	flat := map[string]string{}
	flatten("app", reflect.ValueOf(app), flat)
	flatten("database", reflect.ValueOf(database), flat)
	flatten("redis", reflect.ValueOf(redis), flat)
	flatten("clickhouse", reflect.ValueOf(clickhouse), flat)
	flatten("log", reflect.ValueOf(log), flat)
	flatten("otel", reflect.ValueOf(otel), flat)
	flatten("worker", reflect.ValueOf(worker), flat)
	return flat
}

// legacySections resolves the same inputs with the legacy loader and flattens them.
func legacySections(t *testing.T, path string) map[string]string {
	t.Helper()

	legacy := load(path, false)

	flat := map[string]string{}
	flatten("app", reflect.ValueOf(legacy.App), flat)
	flatten("database", reflect.ValueOf(legacy.Database), flat)
	flatten("redis", reflect.ValueOf(legacy.Redis), flat)
	flatten("clickhouse", reflect.ValueOf(legacy.ClickHouse), flat)
	flatten("log", reflect.ValueOf(legacy.Log), flat)
	flatten("otel", reflect.ValueOf(legacy.Otel), flat)
	flatten("worker", reflect.ValueOf(legacy.Worker), flat)
	return flat
}

func TestEngineParityWithLegacyLoader(t *testing.T) {
	paths := configCandidates(t)

	scenarios := []struct {
		name string
		env  map[string]string
	}{
		{name: "file only", env: nil},
		{
			name: "implicit enable from hosts",
			env: map[string]string{
				"DB_HOST": "postgres", "REDIS_ADDR": "redis:6379", "CLICKHOUSE_HOST": "ch:9000",
			},
		},
		{
			name: "explicit flags win over implicit enable",
			env: map[string]string{
				"DB_HOST": "postgres", "DB_ENABLED": "false",
				"REDIS_ADDR": "redis:6379", "REDIS_ENABLED": "false",
				"CLICKHOUSE_HOST": "ch:9000", "CLICKHOUSE_ENABLED": "false",
			},
		},
		{
			name: "scalar overrides",
			env: map[string]string{
				"LOG_LEVEL": "debug", "APP_ADDR": ":9999", "DB_PORT": "6543",
				"SQLITE_PATH": "./data/parity.db", "REDIS_KEY_PREFIX": "parity:",
				"REDIS_MAINT_NOTIFICATIONS": "true", "OTEL_SAMPLING_RATE": "0.5",
				"WORKER_CONCURRENCY": "7", "APP_NODE_ID": "42",
			},
		},
	}

	for _, path := range paths {
		for _, scenario := range scenarios {
			t.Run(filepath.Base(path)+" "+scenario.name, func(t *testing.T) {
				for name, value := range scenario.env {
					t.Setenv(name, value)
				}

				assert.Empty(t, cmp.Diff(legacySections(t, path), bindSections(t, path)),
					"engine resolution drifted from the legacy loader")
			})
		}
	}
}

func TestEngineHonoursEnvOnlyForDeclaredKeys(t *testing.T) {
	// slow_threshold has no environment counterpart in either the legacy loader or this
	// mirror declaration, so setting one must leave the file value untouched.
	t.Setenv("DB_SLOW_THRESHOLD", "9s")

	for _, path := range configCandidates(t) {
		assert.Empty(t, cmp.Diff(legacySections(t, path), bindSections(t, path)))
	}
}
