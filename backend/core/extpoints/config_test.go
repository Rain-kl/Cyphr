// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core/extpoints"
)

// fakeSource is an in-memory extpoints.ConfigSource used by configuration engine tests.
type fakeSource struct {
	values map[string]any
	env    map[string]string
}

func newFakeSource() *fakeSource {
	return &fakeSource{values: map[string]any{}, env: map[string]string{}}
}

func (f *fakeSource) Lookup(path string) (any, bool) {
	v, ok := f.values[path]
	return v, ok
}

func (f *fakeSource) LookupEnv(name string) (string, bool) {
	v, ok := f.env[name]
	return v, ok
}

func (f *fakeSource) Describe() string { return "fake" }

// redisConfig mirrors how a plugin declares the configuration it reads.
type redisConfig struct {
	Enabled   bool          `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs     []string      `config:"addrs" env:"REDIS_ADDR"`
	DB        int           `config:"db" env:"REDIS_DB"`
	KeyPrefix string        `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	Dial      time.Duration `config:"dial_timeout" env:"REDIS_DIAL_TIMEOUT"`
	Ignored   string        `config:"-"`
	private   string
}

func TestDeclareRegistersTaggedLeafKeys(t *testing.T) {
	r := extpoints.NewConfigRegistry(newFakeSource())

	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))

	keys := make([]string, 0)
	for _, e := range r.Entries() {
		keys = append(keys, e.Key)
	}
	assert.Equal(t, []string{
		"redis.addrs", "redis.db", "redis.dial_timeout", "redis.enabled", "redis.key_prefix",
	}, keys)
}

func TestDeclareRejectsNonStructPointerTarget(t *testing.T) {
	r := extpoints.NewConfigRegistry(newFakeSource())

	assert.ErrorIs(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: redisConfig{}}),
		extpoints.ErrConfigTarget)
	assert.ErrorIs(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: (*redisConfig)(nil)}),
		extpoints.ErrConfigTarget)
}

func TestDeclareAllowsIdenticalDuplicateAndRejectsConflictingMetadata(t *testing.T) {
	r := extpoints.NewConfigRegistry(newFakeSource())
	binding := extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}
	require.NoError(t, r.Declare("cache", binding))
	require.NoError(t, r.Declare("cache_memory", binding), "identical shared declarations must be allowed")

	type conflictingConfig struct {
		Enabled bool `config:"enabled" env:"REDIS_ON" default:"true"`
	}
	err := r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "redis", Target: &conflictingConfig{}})
	require.ErrorIs(t, err, extpoints.ErrConfigConflict)
	assert.Contains(t, err.Error(), "redis.enabled")
	assert.Contains(t, err.Error(), "cache")
	assert.Contains(t, err.Error(), "driver_http")
}

// queueConfig is a composite element mirroring worker.queues in config.yaml.
type queueConfig struct {
	Name     string `config:"name"`
	Priority int    `config:"priority"`
}

type workerConfig struct {
	Concurrency int           `config:"concurrency" env:"WORKER_CONCURRENCY"`
	Queues      []queueConfig `config:"queues"`
}

type sessionConfig struct {
	Secret string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
	Age    int    `config:"session_age" env:"APP_SESSION_AGE" default:"86400"`
}

func TestResolveScalarDurationAndSlice(t *testing.T) {
	src := newFakeSource()
	src.values["redis.db"] = 1
	src.values["redis.dial_timeout"] = "5s"
	src.values["redis.addrs"] = []any{"127.0.0.1:6379"}
	src.env["REDIS_KEY_PREFIX"] = "refresh:"

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r.Resolve())

	var got redisConfig
	require.NoError(t, r.Bind("redis", &got))
	assert.Equal(t, redisConfig{
		Addrs: []string{"127.0.0.1:6379"}, DB: 1, KeyPrefix: "refresh:", Dial: 5 * time.Second,
	}, got)
}

func TestResolveFillsSliceFromScalarEnvironmentValue(t *testing.T) {
	src := newFakeSource()
	src.env["REDIS_ADDR"] = "redis:6379"

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r.Resolve())

	var got redisConfig
	require.NoError(t, r.Bind("redis", &got))
	assert.Equal(t, []string{"redis:6379"}, got.Addrs)
}

func TestResolveCompositeSliceOfStructs(t *testing.T) {
	src := newFakeSource()
	src.values["worker.concurrency"] = 20
	src.values["worker.queues"] = []any{
		map[string]any{"name": "webhook", "priority": 10},
		map[string]any{"name": "default", "priority": 3},
	}

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("asynq_worker", extpoints.ConfigBinding{Prefix: "worker", Target: &workerConfig{}}))
	require.NoError(t, r.Resolve())

	var got workerConfig
	require.NoError(t, r.Bind("worker", &got))
	assert.Equal(t, workerConfig{
		Concurrency: 20,
		Queues:      []queueConfig{{Name: "webhook", Priority: 10}, {Name: "default", Priority: 3}},
	}, got)
}

func TestResolveReportsTypeMismatchOnBadEnvironmentValue(t *testing.T) {
	src := newFakeSource()
	src.env["WORKER_CONCURRENCY"] = "many"

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("asynq_worker", extpoints.ConfigBinding{Prefix: "worker", Target: &workerConfig{}}))

	err := r.Resolve()
	require.ErrorIs(t, err, extpoints.ErrConfigType)
	assert.Contains(t, err.Error(), "worker.concurrency")
	assert.Contains(t, err.Error(), "WORKER_CONCURRENCY")
}

func TestViewAccessorsAndOrigins(t *testing.T) {
	src := newFakeSource()
	src.values["redis.db"] = 1
	src.env["REDIS_ADDR"] = "redis:6379"
	src.env["REDIS_ENABLED"] = "false"

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r.Declare("auth", extpoints.ConfigBinding{Prefix: "app", Target: &sessionConfig{}}))
	require.NoError(t, r.Resolve())

	assert.Equal(t, extpoints.OriginEnv, r.Origin("redis.addrs"))
	assert.Equal(t, []string{"redis:6379"}, r.Strings("redis.addrs"))
	assert.False(t, r.Bool("redis.enabled", true))
	assert.Equal(t, 1, r.Int("redis.db", 0))
	assert.Equal(t, "86400", r.String("app.session_age", "0"))
	assert.Equal(t, "fallback", r.String("redis.missing", "fallback"))
	assert.Zero(t, r.Duration("redis.dial_timeout", 0))
	assert.True(t, r.WasSet("REDIS_ADDR"))
	assert.False(t, r.WasSet("REDIS_NOPE"))
}

func TestAutoEnableBeatsFileValueButLosesToExplicitEnv(t *testing.T) {
	src := newFakeSource()
	src.env["REDIS_ADDR"] = "redis:6379"
	src.values["redis.enabled"] = false

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r.Resolve())
	assert.True(t, r.Bool("redis.enabled", false), "REDIS_ADDR presence implies enabled")
	assert.Equal(t, extpoints.OriginAutoEnable, r.Origin("redis.enabled"))

	explicit := newFakeSource()
	explicit.env["REDIS_ADDR"] = "redis:6379"
	explicit.env["REDIS_ENABLED"] = "false"

	r2 := extpoints.NewConfigRegistry(explicit)
	require.NoError(t, r2.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r2.Resolve())
	assert.False(t, r2.Bool("redis.enabled", true), "explicit REDIS_ENABLED must win over auto-enable")
	assert.Equal(t, extpoints.OriginEnv, r2.Origin("redis.enabled"))
}

func TestEntriesRedactSecretsAndReportDefaults(t *testing.T) {
	r := extpoints.NewConfigRegistry(newFakeSource())
	require.NoError(t, r.Declare("auth", extpoints.ConfigBinding{Prefix: "app", Target: &sessionConfig{}}))
	require.NoError(t, r.Resolve())

	entries := map[string]extpoints.ConfigEntry{}
	for _, e := range r.Entries() {
		entries[e.Key] = e
	}

	assert.Equal(t, extpoints.RedactedValue, entries["app.session_secret"].Value)
	assert.Equal(t, extpoints.OriginDefault, entries["app.session_age"].Origin)
	assert.Equal(t, "86400", entries["app.session_age"].Value)
}

type optionalBoolConfig struct {
	RedirectTrailingSlash *bool `config:"redirect_trailing_slash" env:"APP_REDIRECT_TRAILING_SLASH"`
}

func TestBindBoolPointerFromFileAndEnv(t *testing.T) {
	t.Run("absent stays nil", func(t *testing.T) {
		r := extpoints.NewConfigRegistry(newFakeSource())
		require.NoError(t, r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "app", Target: &optionalBoolConfig{}}))
		require.NoError(t, r.Resolve())

		var got optionalBoolConfig
		require.NoError(t, r.Bind("app", &got))
		assert.Nil(t, got.RedirectTrailingSlash)
		assert.Equal(t, "", r.Origin("app.redirect_trailing_slash"))
	})

	t.Run("file false", func(t *testing.T) {
		src := newFakeSource()
		src.values["app.redirect_trailing_slash"] = false

		r := extpoints.NewConfigRegistry(src)
		require.NoError(t, r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "app", Target: &optionalBoolConfig{}}))
		require.NoError(t, r.Resolve())

		var got optionalBoolConfig
		require.NoError(t, r.Bind("app", &got))
		require.NotNil(t, got.RedirectTrailingSlash)
		assert.False(t, *got.RedirectTrailingSlash)
		assert.Equal(t, extpoints.OriginFile, r.Origin("app.redirect_trailing_slash"))
		assert.False(t, r.Bool("app.redirect_trailing_slash", true))
	})

	t.Run("env false", func(t *testing.T) {
		src := newFakeSource()
		src.env["APP_REDIRECT_TRAILING_SLASH"] = "false"

		r := extpoints.NewConfigRegistry(src)
		require.NoError(t, r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "app", Target: &optionalBoolConfig{}}))
		require.NoError(t, r.Resolve())

		var got optionalBoolConfig
		require.NoError(t, r.Bind("app", &got))
		require.NotNil(t, got.RedirectTrailingSlash)
		assert.False(t, *got.RedirectTrailingSlash)
		assert.Equal(t, extpoints.OriginEnv, r.Origin("app.redirect_trailing_slash"))
	})
}

func TestBindRejectsReadsBeforeSourceIsRegistered(t *testing.T) {
	r := extpoints.NewConfigRegistry(nil)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))

	assert.ErrorIs(t, r.Resolve(), extpoints.ErrConfigNoSource)

	var cfg redisConfig
	assert.ErrorIs(t, r.Bind("redis", &cfg), extpoints.ErrConfigNoSource)
}

func TestBindRejectsReadsBeforeResolution(t *testing.T) {
	r := extpoints.NewConfigRegistry(newFakeSource())
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))

	var cfg redisConfig
	err := r.Bind("redis", &cfg)
	require.ErrorIs(t, err, extpoints.ErrConfigNotResolved)
	assert.Contains(t, err.Error(), "App.Prepare")
}
