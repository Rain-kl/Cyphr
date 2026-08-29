# Cordis 配置扩展点（框架与门禁）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在微内核中落地"插件声明配置字段、内核按声明解析、门禁决定插件激活"的配置扩展点，并证明其解析结果与现有 `backend/pkg/config` 逐 key 等价。

**Architecture:** `core/extpoints` 实现纯 stdlib 的配置引擎（声明注册表 + 优先级解析 + 冲突校验 + 脱敏导出），viper 装载隔离在 `plugins/infra/config` 适配器内；`App.Prepare()` 作为解析屏障，`Fiber` 新增 `FiberSkipped` 态承载门禁结果。

**Tech Stack:** Go 1.25.7、`github.com/spf13/viper v1.21.0`（仅适配器）、`github.com/stretchr/testify`（测试）、`github.com/google/go-cmp`（对拍）、golangci-lint（gofumpt + cyclop/funlen/mnd/revive/dupl/gosec）。

**Spec:** `docs/superpowers/specs/2026-08-29-cordis-config-extension-design.md`

---

## 本计划范围（对应 spec §7.3 的 P1 + P2）

本计划交付**内核能力**，不改动任何业务插件与 `cmd`：完成后旧的全局单例 `config.Config` 仍然是生产路径的唯一配置来源，应用行为零变化，新增能力由单测与新旧对拍证明。

**下一个计划（P3 + P4，本计划完成后另行编写）** 才做 27 个消费文件的迁移、`pkg/idgen` 解耦与 `backend/pkg/config` 删除。分期理由：迁移的声明结构体写法依赖本计划定稿的 tag 与 API 形状，先写会在实现过程中失真。

## 文件结构

| 文件 | 职责 | 动作 |
| :--- | :--- | :--- |
| `backend/core/extpoints/config.go` | 配置引擎的抽象与声明注册表：`ConfigSource`、`ConfigBinding`、`ConfigEntry`、`ConfigView`、`ConfigExtension`、`ConfigRegistry.Declare` + tag 遍历 + 冲突校验 | Create |
| `backend/core/extpoints/config_value.go` | 值解码：`convertValue` 及其 bool/int/uint/float/string/duration/slice/struct 分支 | Create |
| `backend/core/extpoints/config_resolve.go` | `Resolve` 优先级链、`Bind` 赋值、只读访问器、`Entries` 脱敏导出 | Create |
| `backend/core/extpoints/config_test.go` | 引擎单测（外部测试包 `extpoints_test`，fake source） | Create |
| `backend/core/config.go` | `ConfigGet[T]` 泛型读取入口 | Create |
| `backend/core/config_test.go` | 泛型读取与 `Context.Config()` 接入测试 | Create |
| `backend/core/types.go` | `ConfigExtension`/`ConfigBinding`/`ConfigEntry`/`ConfigSource` 别名 + `ConfigGatedPlugin` 可选接口 | Modify |
| `backend/core/context.go` | `config` 字段、`NewContext` 初始化、`Fork` 共享、`Config()` 访问器 | Modify |
| `backend/core/fiber.go` | `FiberSkipped` 状态与 `Skip()` | Modify |
| `backend/core/app.go` | `WithConfigSource`/`WithConfigDecl`/`Prepare`/`SetShutdownTimeout`、`Use` 收集声明、`reconcileLocked` 门禁求值 | Modify |
| `backend/plugins/infra/config/source.go` | viper + yaml 适配器，实现 `core.ConfigSource`，保留 `CONFIG_PATH` 与向上查找语义 | Create |
| `backend/plugins/infra/config/source_test.go` | 适配器单测（`t.TempDir()` + `t.Setenv`） | Create |
| `backend/pkg/config/config.go` | 抽出可重入 `load(configPath string, testMode bool)`（仅重构，行为不变） | Modify |
| `backend/pkg/config/parity_test.go` | 新旧解析对拍（临时文件，P4 随旧包删除） | Create then Delete in P4 |
| `scripts/check_cordis_architecture.sh` | 微内核禁 viper 检查项 | Modify |

**约束提示：** 所有新增导出符号必须带符合 `go-documentation` 规范的文档注释（`revive` 会检查）；测试统一用 `t.TempDir()`，禁止相对路径创建临时目录。

---

## Task 1: 配置引擎的抽象与声明注册表

**Files:**
- Create: `backend/core/extpoints/config.go`
- Test: `backend/core/extpoints/config_test.go`

- [ ] **Step 1: 写失败的测试**

创建 `backend/core/extpoints/config_test.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints_test

import (
	"Wavelet/core/extpoints"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/extpoints/ -run 'TestDeclare' -v`
Expected: 编译失败，报 `undefined: extpoints.NewConfigRegistry`、`undefined: extpoints.ConfigBinding`、`undefined: extpoints.ErrConfigTarget`、`undefined: extpoints.ErrConfigConflict`。

- [ ] **Step 3: 写最小实现**

创建 `backend/core/extpoints/config.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Sentinel errors returned by the configuration extension point.
var (
	// ErrConfigConflict is returned when the same key is declared with disagreeing metadata.
	ErrConfigConflict = errors.New("extpoints: conflicting configuration declarations")

	// ErrConfigType is returned when a value cannot be converted to the declared type.
	ErrConfigType = errors.New("extpoints: configuration value type mismatch")

	// ErrConfigInvalid is returned when a resolved value violates a declared value range.
	ErrConfigInvalid = errors.New("extpoints: invalid configuration value")

	// ErrConfigNotResolved is returned when typed reads happen before resolution.
	ErrConfigNotResolved = errors.New("extpoints: configuration not resolved; run App.Prepare first")

	// ErrConfigTarget is returned when a binding target is not an addressable struct pointer.
	ErrConfigTarget = errors.New("extpoints: configuration binding target must be a non-nil struct pointer")

	// ErrConfigNoSource is returned when resolution is attempted without a registered source.
	ErrConfigNoSource = errors.New("extpoints: no configuration source registered")
)

// Configuration origin labels reported by ConfigView.Origin and ConfigEntry.Origin.
const (
	// OriginEnv marks a value that came from an environment variable.
	OriginEnv = "env"
	// OriginAutoEnable marks a boolean enabled by the presence of another environment variable.
	OriginAutoEnable = "auto-enable"
	// OriginFile marks a value that came from the configuration file.
	OriginFile = "file"
	// OriginDefault marks a value that came from a declaration default.
	OriginDefault = "default"
)

// redactedValue replaces the printed value of keys declared with secret:"true".
const redactedValue = "******"

// durationType distinguishes time.Duration from plain int64 during tag walking and decoding.
var durationType = reflect.TypeFor[time.Duration]()

// ConfigSource abstracts where raw configuration values come from, keeping the
// micro-kernel free of concrete loaders such as viper.
type ConfigSource interface {
	// Lookup returns the raw value stored at a dotted path in the configuration file.
	Lookup(path string) (any, bool)
	// LookupEnv returns the raw value of an environment variable.
	LookupEnv(name string) (string, bool)
	// Describe returns a human readable identity for the source, used in diagnostics.
	Describe() string
}

// ConfigBinding declares that a plugin reads every `config` tagged field of Target
// under a dotted configuration prefix.
type ConfigBinding struct {
	// Prefix is the dotted configuration path, e.g. "redis". An empty prefix means
	// each field's `config` tag is already a full path.
	Prefix string
	// Target must be a non-nil pointer to a struct carrying `config` tags.
	Target any
}

// configField is a single leaf discovered while walking a binding struct's tags.
type configField struct {
	key        string
	env        string
	autoEnable string
	def        string
	secret     bool
	typ        reflect.Type
}

// configDecl is the registered form of a configField, attributed to its declaring plugin.
type configDecl struct {
	key        string
	pluginID   string
	env        string
	autoEnable string
	def        string
	secret     bool
	typ        reflect.Type
}

// ConfigEntry is a redacted, self-describing view of one effective configuration key.
type ConfigEntry struct {
	Key      string
	PluginID string
	Env      string
	Origin   string
	Value    string
}

// ConfigView is the read-only surface over effective configuration values.
// Keys are dotted paths such as "redis.enabled".
type ConfigView interface {
	Value(key string) (any, bool)
	String(key, fallback string) string
	Bool(key string, fallback bool) bool
	Int(key string, fallback int) int
	Duration(key string, fallback time.Duration) time.Duration
	Strings(key string) []string
	WasSet(envName string) bool
	Origin(key string) string
}

// ConfigExtension is the plugin-facing configuration extension point mounted on the
// root Context and shared by every forked plugin scope.
type ConfigExtension interface {
	ConfigView

	// Declare registers plugin-owned configuration bindings before Apply runs.
	Declare(pluginID string, bindings ...ConfigBinding) error
	// Bind resolves and assigns the configuration values for a tagged struct.
	Bind(prefix string, target any) error
	// Resolve computes the effective value of every declared key once.
	Resolve() error
	// Resolved reports whether Resolve has already run.
	Resolved() bool
	// Entries returns the redacted effective configuration ordered by key.
	Entries() []ConfigEntry
}

// ConfigRegistry implements ConfigExtension. Declarations are additive; values are
// computed once by Resolve and reused by every later read.
type ConfigRegistry struct {
	mu       sync.RWMutex
	src      ConfigSource
	decls    map[string]*configDecl
	order    []string
	values   map[string]any
	origins  map[string]string
	resolved bool
}

// NewConfigRegistry creates an empty configuration registry. A nil src is allowed so
// that the kernel can construct the registry before the composition root injects one.
func NewConfigRegistry(src ConfigSource) *ConfigRegistry {
	return &ConfigRegistry{
		src:     src,
		decls:   make(map[string]*configDecl),
		values:  make(map[string]any),
		origins: make(map[string]string),
	}
}

// SetSource installs the raw value source. It is intended for the composition root,
// which builds the adapter after the kernel Context already exists.
func (r *ConfigRegistry) SetSource(src ConfigSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.src = src
}

// Declare registers every `config` tagged leaf of each binding's target struct.
// Repeated declarations of the same key are accepted only when their env, default,
// auto-enable and secret metadata agree; disagreement is ErrConfigConflict.
func (r *ConfigRegistry) Declare(pluginID string, bindings ...ConfigBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range bindings {
		if err := r.declareBinding(pluginID, b); err != nil {
			return err
		}
	}
	return nil
}

func (r *ConfigRegistry) declareBinding(pluginID string, b ConfigBinding) error {
	target, err := bindingStruct(b.Target, b.Prefix)
	if err != nil {
		return err
	}

	fields, err := walkConfigFields(target.Type(), b.Prefix)
	if err != nil {
		return err
	}
	for _, f := range fields {
		if err := r.addDecl(pluginID, f); err != nil {
			return err
		}
	}
	return nil
}

// bindingStruct validates that a binding or bind target is a usable struct pointer.
func bindingStruct(target any, prefix string) (reflect.Value, error) {
	rv := reflect.ValueOf(target)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%w: prefix %q received %T", ErrConfigTarget, prefix, target)
	}
	return rv.Elem(), nil
}

// walkConfigFields collects leaf configuration declarations from `config` tagged fields.
// A field without a `config` tag is skipped, except for embedded structs which are
// recursed into so their own tags resolve under the same prefix.
func walkConfigFields(t reflect.Type, prefix string) ([]configField, error) {
	var out []configField

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}

		path := sf.Tag.Get("config")
		if path == "-" {
			continue
		}
		if path == "" {
			if sf.Type.Kind() == reflect.Struct && sf.Type != durationType {
				nested, err := walkConfigFields(sf.Type, prefix)
				if err != nil {
					return nil, err
				}
				out = append(out, nested...)
			}
			continue
		}

		out = append(out, configField{
			key:        joinKey(prefix, path),
			env:        sf.Tag.Get("env"),
			autoEnable: sf.Tag.Get("autoEnable"),
			def:        sf.Tag.Get("default"),
			secret:     strings.EqualFold(sf.Tag.Get("secret"), "true"),
			typ:        sf.Type,
		})
	}

	return out, nil
}

func joinKey(prefix, path string) string {
	if prefix == "" {
		return path
	}
	return prefix + "." + path
}

// addDecl records one leaf, enforcing the shared-declaration consistency rule.
func (r *ConfigRegistry) addDecl(pluginID string, f configField) error {
	if existing, ok := r.decls[f.key]; ok {
		if existing.env != f.env || existing.def != f.def ||
			existing.autoEnable != f.autoEnable || existing.secret != f.secret {
			return fmt.Errorf(
				"%w: key %q declared by plugin %q and plugin %q with disagreeing env/default/autoEnable/secret metadata",
				ErrConfigConflict, f.key, existing.pluginID, pluginID)
		}
		return nil
	}

	r.decls[f.key] = &configDecl{
		key: f.key, pluginID: pluginID, env: f.env,
		autoEnable: f.autoEnable, def: f.def, secret: f.secret, typ: f.typ,
	}
	r.order = append(r.order, f.key)
	return nil
}
```

- [ ] **Step 4: 补齐测试所需的占位实现**

此时 `Entries`、`Resolve`、`Bind`、访问器尚未实现，Step 1 的测试用到 `Entries`。先创建 `backend/core/extpoints/config_resolve.go` 骨架，仅让 `Entries` 返回声明清单（值与来源在 Task 3/4 填充）：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import "sort"

// Entries returns the effective configuration as redacted, key-sorted entries.
func (r *ConfigRegistry) Entries() []ConfigEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := append([]string(nil), r.order...)
	sort.Strings(keys)

	out := make([]ConfigEntry, 0, len(keys))
	for _, key := range keys {
		d := r.decls[key]
		out = append(out, ConfigEntry{
			Key: d.key, PluginID: d.pluginID, Env: d.env,
			Origin: r.origins[key], Value: "pending",
		})
	}
	return out
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./core/extpoints/ -run 'TestDeclare' -v`
Expected: `--- PASS: TestDeclareRegistersTaggedLeafKeys`、`--- PASS: TestDeclareRejectsNonStructPointerTarget`、`--- PASS: TestDeclareAllowsIdenticalDuplicateAndRejectsConflictingMetadata`，`ok Wavelet/core/extpoints`。

- [ ] **Step 6: 格式与静态检查**

Run: `cd backend && golangci-lint fmt ./core/extpoints/ && golangci-lint run ./core/extpoints/`
Expected: 无告警输出，退出码 0。

- [ ] **Step 7: 提交**

```bash
git add backend/core/extpoints/config.go backend/core/extpoints/config_resolve.go backend/core/extpoints/config_test.go
git commit -m "feat(core): add configuration declaration registry"
```

---

## Task 2: 值解码（标量、时长、切片、结构体）

**Files:**
- Create: `backend/core/extpoints/config_value.go`
- Test: `backend/core/extpoints/config_test.go`（追加）

- [ ] **Step 1: 追加失败的测试**

在 `backend/core/extpoints/config_test.go` 末尾追加（`fakeSource`、`redisConfig` 复用 Task 1 的定义）：

```go
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

	assert.Equal(t, []string{"redis:6379"}, r.Strings("redis.addrs"))
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/extpoints/ -run 'TestResolve' -v`
Expected: 编译失败，报 `r.Resolve undefined`、`r.Bind undefined`、`r.Strings undefined`。

- [ ] **Step 3: 写实现**

创建 `backend/core/extpoints/config_value.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// convertValue coerces a raw value coming from the configuration file or an
// environment variable into the declared Go type.
func convertValue(raw any, typ reflect.Type) (any, error) {
	if typ == durationType {
		return convertDuration(raw)
	}

	switch typ.Kind() {
	case reflect.Bool:
		return convertBool(raw)
	case reflect.String:
		return convertString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return convertInt(raw, typ)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return convertUint(raw, typ)
	case reflect.Float32, reflect.Float64:
		return convertFloat(raw, typ)
	case reflect.Slice:
		return convertSlice(raw, typ)
	case reflect.Struct:
		return convertStruct(raw, typ)
	default:
		return nil, fmt.Errorf("%w: %s is not a supported configuration type", ErrConfigType, typ)
	}
}

func convertBool(raw any) (any, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a boolean", ErrConfigType, v)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("%w: %v is not a boolean", ErrConfigType, raw)
	}
}

func convertString(raw any) (any, error) {
	switch v := raw.(type) {
	case string:
		return v, nil
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v), nil
	default:
		return nil, fmt.Errorf("%w: %v is not a string", ErrConfigType, raw)
	}
}

// numericString extracts the textual form of a value so environment overrides,
// which always arrive as strings, share one parsing path with file values.
func numericString(raw any) (string, bool) {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v), true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v), true
	default:
		return "", false
	}
}

func convertInt(raw any, typ reflect.Type) (any, error) {
	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not an integer", ErrConfigType, raw)
	}
	parsed, err := strconv.ParseInt(text, 10, typ.Bits())
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid %s", ErrConfigType, text, typ)
	}
	out := reflect.New(typ).Elem()
	out.SetInt(parsed)
	return out.Interface(), nil
}

func convertUint(raw any, typ reflect.Type) (any, error) {
	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not an unsigned integer", ErrConfigType, raw)
	}
	parsed, err := strconv.ParseUint(text, 10, typ.Bits())
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid %s", ErrConfigType, text, typ)
	}
	out := reflect.New(typ).Elem()
	out.SetUint(parsed)
	return out.Interface(), nil
}

func convertFloat(raw any, typ reflect.Type) (any, error) {
	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a float", ErrConfigType, raw)
	}
	parsed, err := strconv.ParseFloat(text, typ.Bits())
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid %s", ErrConfigType, text, typ)
	}
	out := reflect.New(typ).Elem()
	out.SetFloat(parsed)
	return out.Interface(), nil
}

// convertDuration accepts both Go duration strings such as "200ms" and integer
// nanoseconds, mirroring what the previous viper based decoding supported.
func convertDuration(raw any) (any, error) {
	switch v := raw.(type) {
	case time.Duration:
		return v, nil
	}

	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a duration", ErrConfigType, raw)
	}
	if parsed, err := time.ParseDuration(text); err == nil {
		return parsed, nil
	}
	nanos, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid duration", ErrConfigType, text)
	}
	return time.Duration(nanos), nil
}

func convertSlice(raw any, typ reflect.Type) (any, error) {
	items, ok := sliceItems(raw)
	if !ok {
		// Scalar-to-single-element promotion keeps REDIS_ADDR populating redis.addrs.
		items = []any{raw}
	}

	out := reflect.MakeSlice(typ, 0, len(items))
	for _, item := range items {
		converted, err := convertValue(item, typ.Elem())
		if err != nil {
			return nil, err
		}
		out = reflect.Append(out, reflect.ValueOf(converted))
	}
	return out.Interface(), nil
}

// sliceItems normalises the several slice shapes a loader may produce.
func sliceItems(raw any) ([]any, bool) {
	switch v := raw.(type) {
	case []any:
		return v, true
	case []string:
		items := make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
		return items, true
	}

	rv := reflect.ValueOf(raw)
	if rv.IsValid() && rv.Kind() == reflect.Slice {
		items := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			items[i] = rv.Index(i).Interface()
		}
		return items, true
	}
	return nil, false
}

func convertStruct(raw any, typ reflect.Type) (any, error) {
	table, ok := asStringMap(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a mapping, cannot decode into %s", ErrConfigType, raw, typ)
	}

	fields, err := walkConfigFields(typ, "")
	if err != nil {
		return nil, err
	}

	out := reflect.New(typ).Elem()
	for _, f := range fields {
		item, present := table[f.key]
		if !present || item == nil {
			continue
		}
		converted, err := convertValue(item, f.typ)
		if err != nil {
			return nil, fmt.Errorf("%w: %s.%s: %w", ErrConfigType, typ.Name(), f.key, err)
		}
		out.FieldByName(indexFieldName(typ, f.key)).Set(reflect.ValueOf(converted))
	}
	return out.Interface(), nil
}

// asStringMap normalises the two map shapes produced by YAML decoders.
func asStringMap(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			name, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[name] = val
		}
		return out, true
	default:
		return nil, false
	}
}

// indexFieldName maps a declared config path back to the Go struct field carrying it.
func indexFieldName(t reflect.Type, key string) string {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("config") == key {
			return t.Field(i).Name
		}
	}
	return ""
}
```

- [ ] **Step 4: 写解析与绑定实现**

把 `backend/core/extpoints/config_resolve.go` 的骨架替换为完整实现：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
)

// Resolve computes the effective value of every declared key. Priority is, in order:
// an explicit environment override, an auto-enable trigger, the configuration file,
// then the declared default. Resolution is idempotent; later declarations resolve lazily.
func (r *ConfigRegistry) Resolve() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.src == nil {
		return ErrConfigNoSource
	}

	var errs []error
	for _, key := range r.order {
		if _, done := r.values[key]; done {
			continue
		}
		if err := r.resolveLocked(key); err != nil {
			errs = append(errs, err)
		}
	}
	r.resolved = true

	return errors.Join(errs...)
}

// Resolved reports whether Resolve has already run.
func (r *ConfigRegistry) Resolved() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resolved
}

// resolveLocked computes one key. The caller must hold r.mu.
func (r *ConfigRegistry) resolveLocked(key string) error {
	d, ok := r.decls[key]
	if !ok {
		return fmt.Errorf("%w: %s", ErrConfigUnknownKey, key)
	}

	if d.env != "" {
		if raw, found := r.src.LookupEnv(d.env); found {
			value, err := convertValue(raw, d.typ)
			if err != nil {
				return fmt.Errorf("%w: key %q from environment %s: %w", ErrConfigType, key, d.env, err)
			}
			r.values[key], r.origins[key] = value, OriginEnv
			return nil
		}
	}

	if d.autoEnable != "" && d.typ.Kind() == reflect.Bool {
		if _, found := r.src.LookupEnv(d.autoEnable); found {
			r.values[key], r.origins[key] = true, OriginAutoEnable
			return nil
		}
	}

	if raw, found := r.src.Lookup(key); found {
		value, err := convertValue(raw, d.typ)
		if err != nil {
			return fmt.Errorf("%w: key %q from %s: %w", ErrConfigType, key, r.src.Describe(), err)
		}
		r.values[key], r.origins[key] = value, OriginFile
		return nil
	}

	if d.def != "" {
		value, err := convertValue(d.def, d.typ)
		if err != nil {
			return fmt.Errorf("%w: default %q for key %q: %w", ErrConfigType, d.def, key, err)
		}
		r.values[key], r.origins[key] = value, OriginDefault
		return nil
	}

	r.values[key] = reflect.New(d.typ).Elem().Interface()
	r.origins[key] = ""
	return nil
}

// Bind resolves the tagged fields of target and assigns them in place. Prefixes that
// were never declared self-register, so only gates need DeclareConfig.
func (r *ConfigRegistry) Bind(prefix string, target any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.src == nil {
		return ErrConfigNoSource
	}

	elem, err := bindingStruct(target, prefix)
	if err != nil {
		return err
	}
	fields, err := walkConfigFields(elem.Type(), prefix)
	if err != nil {
		return err
	}

	for _, f := range fields {
		if _, declared := r.decls[f.key]; !declared {
			if err := r.addDecl("bind:"+prefix, f); err != nil {
				return err
			}
		}
		if _, done := r.values[f.key]; !done {
			if err := r.resolveLocked(f.key); err != nil {
				return err
			}
		}
	}

	for _, f := range fields {
		value := r.values[f.key]
		field := elem.FieldByName(indexFieldName(elem.Type(), f.key))
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%w: field for key %q is not settable", ErrConfigTarget, f.key)
		}
		rv := reflect.ValueOf(value)
		if !rv.Type().AssignableTo(field.Type()) {
			return fmt.Errorf("%w: key %q resolves to %s, field expects %s",
				ErrConfigType, f.key, rv.Type(), field.Type())
		}
		field.Set(rv)
	}
	return nil
}
```

注意：`ErrConfigUnknownKey` 与后续访问器在 Task 3 引入；本 Step 先加哨兵错误到 `config.go` 的错误块，避免编译失败：

```go
	// ErrConfigUnknownKey is returned when a configuration key was never declared.
	ErrConfigUnknownKey = errors.New("extpoints: unknown configuration key")
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./core/extpoints/ -run 'TestResolve|TestDeclare' -v`
Expected: 全部 `--- PASS`，`ok Wavelet/core/extpoints`。若 `sync`/`time` 报未使用 import，说明骨架残留 import，删除即可。

- [ ] **Step 6: 格式与静态检查**

Run: `cd backend && golangci-lint fmt ./core/extpoints/ && golangci-lint run ./core/extpoints/`
Expected: 无告警。（`convertValue` 保持 8 个分支，若 `cyclop` 仍报复杂度过高，把 `Slice`/`Struct` 两分支拆成独立函数，不要放宽 lint 配置。）

- [ ] **Step 7: 提交**

```bash
git add backend/core/extpoints/
git commit -m "feat(core): resolve declared configuration with env and file precedence"
```

---

## Task 3: 只读访问器、泛型读取与脱敏导出

**Files:**
- Modify: `backend/core/extpoints/config_resolve.go`（追加访问器）
- Modify: `backend/core/extpoints/config.go`（`Entries` 用到的 secret 判断）
- Create: `backend/core/config.go`
- Test: `backend/core/extpoints/config_test.go`（追加）、`backend/core/config_test.go`

- [ ] **Step 1: 追加失败的测试**

在 `backend/core/extpoints/config_test.go` 末尾追加：

```go
func TestViewAccessorsAndOrigins(t *testing.T) {
	src := newFakeSource()
	src.values["redis.db"] = 1
	src.env["REDIS_ADDR"] = "redis:6379"
	src.env["REDIS_ENABLED"] = "false"

	r := extpoints.NewConfigRegistry(src)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))
	require.NoError(t, r.Declare("auth", extpoints.ConfigBinding{Prefix: "app", Target: &sessionConfig{}}))
	require.NoError(t, r.Resolve())

	assert.Equal(t, extpoints.OriginEnv, r.Origin("redis.addr"))
	assert.Equal(t, "redis:6379", r.Strings("redis.addrs")[0])
	assert.False(t, r.Bool("redis.enabled", true))
	assert.Equal(t, 1, r.Int("redis.db", 0))
	assert.Equal(t, "86400", r.String("redis.missing", "86400"))
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

func TestBindRejectsReadsBeforeSourceIsRegistered(t *testing.T) {
	r := extpoints.NewConfigRegistry(nil)
	require.NoError(t, r.Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &redisConfig{}}))

	assert.ErrorIs(t, r.Resolve(), extpoints.ErrConfigNoSource)

	var cfg redisConfig
	assert.ErrorIs(t, r.Bind("redis", &cfg), extpoints.ErrConfigNoSource)
}
```

导出脱敏常量：在 `backend/core/extpoints/config.go` 中把 `redactedValue` 改名导出为 `RedactedValue`（同文件内 `Entries` 与测试同步引用）：

```go
// RedactedValue replaces the printed value of keys declared with secret:"true".
const RedactedValue = "******"
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/extpoints/ -run 'TestView|TestAutoEnable|TestEntries|TestBindRejects' -v`
Expected: 编译失败，报 `r.Bool undefined`、`extpoints.RedactedValue undefined` 等；`Entries` 已有但返回 `"pending"` 占位，故 `TestEntriesRedactSecretsAndReportDefaults` 亦失败。

- [ ] **Step 3: 实现访问器**

在 `backend/core/extpoints/config_resolve.go` 末尾追加：

```go
// Value returns the resolved value for key, lazily resolving it when a source is
// available. Unresolvable and missing keys report false rather than an error so
// gates and diagnostics can keep using fallback accessors.
func (r *ConfigRegistry) Value(key string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.src == nil {
		value, ok := r.values[key]
		return value, ok
	}
	if _, done := r.values[key]; !done {
		if _, declared := r.decls[key]; !declared {
			return nil, false
		}
		if err := r.resolveLocked(key); err != nil {
			return nil, false
		}
	}
	value, ok := r.values[key]
	return value, ok
}

// String returns the string value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) String(key, fallback string) string {
	if value, ok := r.Value(key); ok {
		if converted, err := convertString(value); err == nil {
			return converted.(string)
		}
	}
	return fallback
}

// Bool returns the boolean value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Bool(key string, fallback bool) bool {
	if value, ok := r.Value(key); ok {
		if converted, err := convertBool(value); err == nil {
			return converted.(bool)
		}
	}
	return fallback
}

// Int returns the int value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Int(key string, fallback int) int {
	if value, ok := r.Value(key); ok {
		if converted, err := convertInt(value, reflect.TypeFor[int]()); err == nil {
			return int(converted.(int))
		}
	}
	return fallback
}

// Duration returns the time.Duration value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Duration(key string, fallback time.Duration) time.Duration {
	if value, ok := r.Value(key); ok {
		if converted, err := convertDuration(value); err == nil {
			return converted.(time.Duration)
		}
	}
	return fallback
}

// Strings returns the []string value of key, or nil when absent.
func (r *ConfigRegistry) Strings(key string) []string {
	value, ok := r.Value(key)
	if !ok {
		return nil
	}
	converted, err := convertSlice(value, reflect.TypeFor[[]string]())
	if err != nil {
		return nil
	}
	list, _ := converted.([]string)
	return list
}

// WasSet reports whether an environment variable is present, regardless of its value.
func (r *ConfigRegistry) WasSet(envName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.src == nil {
		return false
	}
	_, found := r.src.LookupEnv(envName)
	return found
}

// Origin reports where a key's effective value came from; "" means the zero value.
func (r *ConfigRegistry) Origin(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.origins[key]
}
```

把 `Entries` 的占位实现替换为真实值与脱敏：

```go
// Entries returns the effective configuration as redacted, key-sorted entries.
func (r *ConfigRegistry) Entries() []ConfigEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	keys := append([]string(nil), r.order...)
	sort.Strings(keys)

	out := make([]ConfigEntry, 0, len(keys))
	for _, key := range keys {
		d := r.decls[key]
		if _, done := r.values[key]; !done && r.src != nil {
			_ = r.resolveLocked(key)
		}
		out = append(out, ConfigEntry{
			Key:      d.key,
			PluginID: d.pluginID,
			Env:      d.env,
			Origin:   r.origins[key],
			Value:    formatEntryValue(r.values[key], d.secret),
		})
	}
	return out
}

// formatEntryValue renders one effective value for diagnostics, masking secrets.
func formatEntryValue(value any, secret bool) string {
	if secret {
		return RedactedValue
	}
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
```

- [ ] **Step 4: 实现泛型读取入口**

创建 `backend/core/config.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"

	"Wavelet/core/extpoints"
)

// ConfigGet reads one resolved configuration value with its declared type. It is the
// generic counterpart of the fallback accessors on ConfigView, and returns
// ErrConfigNotResolved when the key has neither been declared nor resolved.
func ConfigGet[T any](view extpoints.ConfigView, key string) (T, error) {
	var zero T
	if view == nil {
		return zero, extpoints.ErrConfigNotResolved
	}

	raw, ok := view.Value(key)
	if !ok {
		return zero, fmt.Errorf("%w: %s", extpoints.ErrConfigUnknownKey, key)
	}

	value, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%w: key %q holds %T, want %T", extpoints.ErrConfigType, key, raw, zero)
	}
	return value, nil
}
```

创建 `backend/core/config_test.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"Wavelet/core"
	"Wavelet/core/extpoints"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type otelConfig struct {
	SamplingRate float64 `config:"sampling_rate" env:"OTEL_SAMPLING_RATE"`
}

func TestConfigGetReturnsDeclaredType(t *testing.T) {
	r := extpoints.NewConfigRegistry(nil)
	require.NoError(t, r.Declare("host", extpoints.ConfigBinding{Prefix: "otel", Target: &otelConfig{}}))

	rate, err := core.ConfigGet[float64](r, "otel.sampling_rate")
	require.ErrorIs(t, err, extpoints.ErrConfigUnknownKey)
	assert.Equal(t, 0.0, rate)
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd backend && go test ./core/... -run 'TestView|TestAutoEnable|TestEntries|TestBindRejects|TestConfigGet' -v`
Expected: 全部 `--- PASS`。

- [ ] **Step 6: 格式与静态检查**

Run: `cd backend && golangci-lint fmt ./core/... && golangci-lint run ./core/...`
Expected: 无告警。若 `dupl` 因 `convertInt`/`convertUint` 结构相似报警，为其中之一加注释说明类型不同不可合并，或拆出公共 reflect 设置函数；不得关闭 `dupl`。

- [ ] **Step 7: 提交**

```bash
git add backend/core/
git commit -m "feat(core): add read-only config view accessors and generic getter"
```

---

## Task 4: 把配置注册表挂到 Context 并在 types.go 导出别名

**Files:**
- Modify: `backend/core/context.go`（`config` 字段、`NewContext`、`Fork`、`Config()`）
- Modify: `backend/core/types.go`（别名）
- Test: `backend/core/config_test.go`（追加）、`backend/core/context_test.go`（追加断言）

- [ ] **Step 1: 追加失败的测试**

在 `backend/core/config_test.go` 末尾追加：

```go
func TestContextConfigIsSharedAcrossForks(t *testing.T) {
	ctx := core.NewContext(nil)
	child := ctx.Fork()

	require.NoError(t, child.Config().Declare("cache", extpoints.ConfigBinding{Prefix: "redis", Target: &otelConfig{}}))

	rate, err := core.ConfigGet[float64](ctx.Config(), "redis.sampling_rate")
	require.ErrorIs(t, err, extpoints.ErrConfigUnknownKey)
	assert.Zero(t, rate)
	assert.False(t, ctx.Config().Resolved())
}
```

在 `backend/core/context_test.go` 中已有的 Context 构造测试里追加一行断言（沿用该文件现有测试函数与变量名）：

```go
	require.NotNil(t, ctx.Config(), "every Context must expose the configuration extension")
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/ -run 'TestContextConfigIsSharedAcrossForks' -v`
Expected: 编译失败，报 `ctx.Config undefined`、`core.ConfigBinding undefined`（测试里改用 `extpoints.ConfigBinding` 后该项消除）。

- [ ] **Step 3: 写实现**

`backend/core/context.go` 的 `Context` 结构体字段中，在 `settings` 之后加一行：

```go
	settings   extpoints.SettingExtension
	config     extpoints.ConfigExtension
```

`NewContext` 的返回字面量中，在 `settings: extpoints.NewSettingRegistry(),` 之后加：

```go
		config:     extpoints.NewConfigRegistry(nil),
```

`ForkWithContext` 的 child 字面量中，在 `settings: c.settings,` 之后加：

```go
		config:     c.config,
```

在 `Setting()` 别名方法之后加访问器：

```go
// Config returns the process-level configuration extension point. The registry is
// shared by every fork because configuration declarations are global facts, and it
// intentionally carries no per-scope disposers: values are resolved once before Apply.
func (c *Context) Config() extpoints.ConfigExtension {
	return c.config
}
```

`backend/core/types.go` 末尾追加别名与新接口：

```go
// ConfigExtension re-exports extpoints.ConfigExtension.
type ConfigExtension = extpoints.ConfigExtension

// ConfigSource re-exports extpoints.ConfigSource.
type ConfigSource = extpoints.ConfigSource

// ConfigBinding re-exports extpoints.ConfigBinding.
type ConfigBinding = extpoints.ConfigBinding

// ConfigView re-exports extpoints.ConfigView.
type ConfigView = extpoints.ConfigView

// ConfigEntry re-exports extpoints.ConfigEntry.
type ConfigEntry = extpoints.ConfigEntry

// ConfigGatedPlugin is an optional interface for plugins whose activation depends on
// configuration. The kernel evaluates the gate before any Apply runs, so keys read by
// ConfigEnabled must be published through DeclareConfig.
type ConfigGatedPlugin interface {
	Plugin

	// DeclareConfig publishes the configuration bindings consumed by ConfigEnabled.
	DeclareConfig() []extpoints.ConfigBinding

	// ConfigEnabled reports whether this plugin should activate for the resolved values.
	ConfigEnabled(view extpoints.ConfigView) bool
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./core/... -v -run 'TestContextConfig|TestFiber|TestApp|TestConfigGet'`
Expected: 新增用例 `--- PASS`，既有用例无回归。

- [ ] **Step 5: 提交**

```bash
git add backend/core/context.go backend/core/types.go backend/core/config_test.go backend/core/context_test.go
git commit -m "feat(core): mount the configuration extension point on the kernel Context"
```

---

## Task 5: Fiber 门禁跳过态

**Files:**
- Modify: `backend/core/fiber.go`
- Test: `backend/core/fiber_test.go`（追加）

- [ ] **Step 1: 追加失败的测试**

在 `backend/core/fiber_test.go` 末尾追加：

```go
// gatedPlugin is a minimal plugin used to exercise configuration gating.
type gatedPlugin struct {
	name    string
	enabled bool
	applied bool
}

func (g *gatedPlugin) Name() string { return g.name }
func (g *gatedPlugin) Apply(ctx *core.Context) error {
	g.applied = true
	return nil
}
func (g *gatedPlugin) DeclareConfig() []extpoints.ConfigBinding {
	return []extpoints.ConfigBinding{{Prefix: "gate", Target: &gateConfig{}}}
}
func (g *gatedPlugin) ConfigEnabled(view extpoints.ConfigView) bool {
	return view.Bool("gate.enabled", false) == g.enabled
}

type gateConfig struct {
	Enabled bool `config:"enabled" env:"GATE_ENABLED"`
}

func TestFiberSkipMovesToSkippedStateAndDisposesScope(t *testing.T) {
	root := core.NewContext(nil)
	plugin := &gatedPlugin{name: "cache", enabled: true}
	f := core.NewFiber(root, plugin)
	require.Equal(t, core.FiberPending, f.State())

	require.NoError(t, f.Skip())

	assert.Equal(t, core.FiberSkipped, f.State())
	assert.False(t, plugin.applied, "a skipped plugin must never reach Apply")
	assert.False(t, f.DependenciesSatisfied(root), "skipped fibers must not satisfy dependents")
	assert.NoError(t, f.Unload(), "unloading a skipped fiber is a no-op")
}

func TestFiberSkipIsIdempotentForActiveFibers(t *testing.T) {
	root := core.NewContext(nil)
	f := core.NewFiber(root, &gatedPlugin{name: "cache", enabled: true})
	require.NoError(t, f.Load())

	require.NoError(t, f.Skip())
	assert.Equal(t, core.FiberActive, f.State(), "Skip only applies to pending fibers")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/ -run 'TestFiberSkip' -v`
Expected: 编译失败，报 `undefined: core.FiberSkipped`、`f.Skip undefined`、`f.DependenciesSatisfied` 参数类型不符（该断言要求 skipped 判定，实现后通过）。

- [ ] **Step 3: 写实现**

`backend/core/fiber.go` 中 `FiberDisposed` 常量之后追加状态：

```go
	// FiberSkipped indicates the plugin never activated because its configuration gate
	// evaluated to false, so an alternative provider took over.
	FiberSkipped
```

`FiberState` 常量块内的字符串值需与注释一致，把该常量的取值写为 `"SKIPPED"`：

```go
	// FiberSkipped indicates the plugin never activated because its configuration gate
	// evaluated to false, so an alternative provider took over.
	FiberSkipped FiberState = "SKIPPED"
```

`DependenciesSatisfied` 改为短路跳过被门禁排除的插件：

```go
// DependenciesSatisfied checks if all declared dependencies are present in the target
// Context container. A skipped plugin provides nothing, so it can never satisfy dependents.
func (f *Fiber) DependenciesSatisfied(ctx *Context) bool {
	if f.State() == FiberSkipped {
		return false
	}
	if len(f.deps) == 0 {
		return true
	}
	// ...existing resolution loop unchanged
}
```

`Load` 之后追加 `Skip`：

```go
// Skip transitions a pending plugin to FiberSkipped and releases its scoped Context.
// Active plugins are left untouched, making the call safe to replay during reconcile.
func (f *Fiber) Skip() error {
	f.mu.Lock()
	if f.state != FiberPending {
		f.mu.Unlock()
		return nil
	}
	f.state = FiberSkipped
	f.mu.Unlock()

	return f.ctx.Dispose()
}
```

`Load` 开头补一条守卫，防止已跳过插件被重新加载：

```go
	f.mu.Lock()
	if f.state != FiberPending {
		f.mu.Unlock()
		return nil
	}
```

（该守卫已存在，无需改动；`Skip` 依赖同一不变式。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./core/ -run 'TestFiber' -v`
Expected: 新增两个用例 `--- PASS`，既有 `TestFiber_ConfluenceAndReactiveActivation`、`TestFiber_UnsatisfiedDependencyReturnsError` 无回归。

- [ ] **Step 5: 提交**

```bash
git add backend/core/fiber.go backend/core/fiber_test.go
git commit -m "feat(core): add skipped fiber state for configuration gates"
```

---

## Task 6: App 装配选项、解析屏障与门禁求值

**Files:**
- Modify: `backend/core/app.go`
- Test: `backend/core/app_test.go`（追加）

- [ ] **Step 1: 追加失败的测试**

在 `backend/core/app_test.go` 末尾追加（复用 Task 5 的 `gatedPlugin`/`gateConfig`；`mapSource` 为本测试自备的内存源）：

```go
// mapSource implements core.ConfigSource over static maps.
type mapSource struct {
	values map[string]any
	env    map[string]string
}

func (m *mapSource) Lookup(path string) (any, bool) {
	v, ok := m.values[path]
	return v, ok
}
func (m *mapSource) LookupEnv(name string) (string, bool) {
	v, ok := m.env[name]
	return v, ok
}
func (m *mapSource) Describe() string { return "map" }

func newGateSource(enabled bool) *mapSource {
	src := &mapSource{values: map[string]any{"gate.enabled": enabled}, env: map[string]string{}}
	return src
}

func TestAppPrepareResolvesAndGatesPlugins(t *testing.T) {
	redisLike := &gatedPlugin{name: "cache", enabled: true}
	redisAlt := &gatedPlugin{name: "cache_memory", enabled: false}

	app := core.NewApp(
		core.WithProfile(core.ProfileAPI),
		core.WithConfigSource(newGateSource(true)),
	)
	app.Use(redisLike, redisAlt)
	require.NoError(t, app.Prepare())

	cacheFiber, ok := app.Fiber("cache")
	require.True(t, ok)
	require.Equal(t, core.FiberActive, cacheFiber.State())

	memoryFiber, ok := app.Fiber("cache_memory")
	require.True(t, ok)
	require.Equal(t, core.FiberSkipped, memoryFiber.State())
	assert.False(t, redisAlt.applied)
}

func TestAppStartRunsPrepareWhenCallerSkipsIt(t *testing.T) {
	redisLike := &gatedPlugin{name: "cache", enabled: true}
	app := core.NewApp(core.WithConfigSource(newGateSource(false)))
	app.Use(redisLike)

	require.NoError(t, app.ApplyPlugins())

	fiber, ok := app.Fiber("cache")
	require.True(t, ok)
	assert.Equal(t, core.FiberSkipped, fiber.State(), "ApplyPlugins must resolve and gate implicitly")
}

func TestAppPrepareReportsConfigurationErrors(t *testing.T) {
	src := &mapSource{values: map[string]any{"gate.enabled": "yes"}, env: map[string]string{}}
	app := core.NewApp(core.WithConfigSource(src))
	app.Use(&gatedPlugin{name: "cache", enabled: true})

	err := app.Prepare()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gate.enabled")
}

func TestAppSetShutdownTimeoutOverridesDefault(t *testing.T) {
	app := core.NewApp()
	app.SetShutdownTimeout(0)
	assert.NotZero(t, app.ShutdownTimeout(), "zero durations must not shrink the kernel fallback")

	app.SetShutdownTimeout(45 * time.Second)
	assert.Equal(t, 45*time.Second, app.ShutdownTimeout())
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./core/ -run 'TestAppPrepare|TestAppStartRunsPrepare|TestAppSetShutdown' -v`
Expected: 编译失败，报 `core.WithConfigSource undefined`、`app.Prepare undefined`、`app.ShutdownTimeout undefined`。

- [ ] **Step 3: 写实现**

`backend/core/app.go` 中，`App` 结构体追加两个字段：

```go
	migrationEngine MigrationEngine
	shutdownTimeout time.Duration
	configSource    ConfigSource
	prepared        bool
```

`AppOption` 区追加选项（放在 `WithShutdownTimeout` 之后）：

```go
// WithConfigSource installs the raw configuration source adapter, typically built by
// an infrastructure package outside the kernel, before any plugin is applied.
func WithConfigSource(src ConfigSource) AppOption {
	return func(a *App) {
		if src == nil {
			return
		}
		a.configSource = src
		if setter, ok := a.ctx.Config().(interface{ SetSource(ConfigSource) }); ok {
			setter.SetSource(src)
		}
	}
}

// WithConfigDecl lets the composition root declare the configuration it reads itself,
// so host-level values participate in conflict validation and redacted reporting.
func WithConfigDecl(pluginID string, bindings ...ConfigBinding) AppOption {
	return func(a *App) {
		if len(bindings) == 0 {
			return
		}
		if err := a.ctx.Config().Declare(pluginID, bindings...); err != nil {
			a.applyErr = err
		}
	}
}
```

`App` 结构体再加 `applyErr error` 字段，并在 `NewApp` 末尾返回前保持原逻辑（`applyErr` 由 `Prepare` 首次上报）。

追加 `Prepare`、`ShutdownTimeout`、`SetShutdownTimeout`：

```go
// Prepare resolves declared configuration and evaluates plugin gates. It is idempotent
// and runs implicitly from ApplyPlugins, so callers that need resolved values earlier
// (for example to size a shutdown budget) can invoke it explicitly.
func (a *App) Prepare() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.applyErr; err != nil {
		return err
	}
	return a.prepareLocked()
}

func (a *App) prepareLocked() error {
	if a.prepared {
		return nil
	}

	if err := a.ctx.Config().Resolve(); err != nil {
		return err
	}
	a.prepared = true

	return a.evaluateGatesLocked()
}

// evaluateGatesLocked skips every plugin whose configuration gate is closed.
func (a *App) evaluateGatesLocked() error {
	for _, f := range a.fibers {
		if f.State() != FiberPending {
			continue
		}
		gated, ok := f.plugin.(ConfigGatedPlugin)
		if !ok || gated.ConfigEnabled(a.ctx.Config()) {
			continue
		}
		if err := f.Skip(); err != nil {
			return fmt.Errorf("core: skip gated plugin %q: %w", f.Name(), err)
		}
	}
	return nil
}

// ShutdownTimeout returns the graceful shutdown budget for the application.
func (a *App) ShutdownTimeout() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.shutdownTimeout
}

// SetShutdownTimeout replaces the graceful shutdown budget, ignoring non-positive values.
func (a *App) SetShutdownTimeout(timeout time.Duration) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	if timeout > 0 {
		a.shutdownTimeout = timeout
	}
	return a
}
```

`Use` 中，为每个成功登记的插件收集声明（放在 `a.pluginMap[name] = p` 之前）：

```go
		if gated, ok := p.(ConfigGatedPlugin); ok {
			if err := a.ctx.Config().Declare(name, gated.DeclareConfig()...); err != nil {
				if a.applyErr == nil {
					a.applyErr = err
				}
			}
		}
```

`ApplyPlugins` 与 `reconcileLocked` 接入屏障（`ApplyPlugins` 已持锁，改调用 `prepareLocked`）：

```go
func (a *App) ApplyPlugins() error {
	a.mu.Lock()
	if a.applied {
		a.mu.Unlock()
		return nil
	}
	a.applied = true

	if err := a.applyErr; err != nil {
		a.mu.Unlock()
		return err
	}
	if err := a.prepareLocked(); err != nil {
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()

	return a.Reconcile()
}
```

`reconcileLocked` 的加载循环中，在 `f.DependenciesSatisfied(a.ctx)` 判定之前加门禁守卫，确保未经 `Prepare` 的路径也不会激活门禁关闭的插件：

```go
		for _, f := range a.fibers {
			if f.State() == FiberSkipped {
				continue
			}
			if f.State() == FiberPending && f.DependenciesSatisfied(a.ctx) {
```

同时把 `unsatisfied` 收集循环改为跳过 `FiberSkipped`，避免把被门禁排除的插件误报为依赖缺失：

```go
	for _, f := range a.fibers {
		if f.State() == FiberPending {
			unsatisfied = append(unsatisfied, fmt.Sprintf("%s (waiting for %v)", f.Name(), f.Dependencies()))
		}
	}
```

（`FiberPending` 判定已天然排除 `FiberSkipped`，无需改动。）

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./core/ -v`
Expected: 全部 `--- PASS`，包括既有 `TestApp*`、`TestFiber*`、`TestContext*`。

- [ ] **Step 5: 全量回归（应用行为必须不变）**

Run: `cd backend && go test ./... && go build -o /dev/null ./...`
Expected: 全绿。此时业务插件与 `cmd` 仍走旧的全局单例，因此运行行为与迁移前完全一致——这是本计划的关键安全属性。

- [ ] **Step 6: 格式与静态检查**

Run: `cd backend && golangci-lint fmt ./core/... && golangci-lint run ./core/...`
Expected: 无告警。

- [ ] **Step 7: 提交**

```bash
git add backend/core/app.go backend/core/app_test.go
git commit -m "feat(core): add config resolution barrier and plugin gating to App"
```

---

## Task 7: viper 配置源适配器（plugins/infra/config）

**Files:**
- Create: `backend/plugins/infra/config/source.go`
- Create: `backend/plugins/infra/config/source_test.go`

- [ ] **Step 1: 写失败的测试**

创建 `backend/plugins/infra/config/source_test.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/plugins/infra/config"
)

const sampleYAML = "" +
	"app:\n  addr: \":8000\"\n  node_id: 1\n" +
	"database:\n  enabled: false\n  port: 5432\n  slow_threshold: 200ms\n" +
	"redis:\n  addrs:\n    - \"127.0.0.1:6379\"\n"

func writeConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(sampleYAML), 0o600))
	return path
}

func TestSourceLooksUpNestedPaths(t *testing.T) {
	src, err := config.NewSource(config.WithPath(writeConfig(t)))
	require.NoError(t, err)

	value, ok := src.Lookup("database.port")
	require.True(t, ok)
	assert.Equal(t, 5432, value)

	_, ok = src.Lookup("database.missing")
	assert.False(t, ok)
}

func TestSourceTreatsUnsetFileAsEnvOnly(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.yaml")

	src, err := config.NewSource(config.WithPath(missing))
	require.NoError(t, err, "a missing configuration file must fall back to environment values")

	_, ok := src.Lookup("app.addr")
	assert.False(t, ok)
	assert.Equal(t, config.EnvOnlyOrigin, src.Describe())
}

func TestSourceRejectsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("app: [unclosed\n"), 0o600))

	_, err := config.NewSource(config.WithPath(path))
	require.Error(t, err)
}

func TestSourceLookupEnvReadsProcessEnvironment(t *testing.T) {
	t.Setenv("WAVELET_SOURCE_PROBE", "present")

	src, err := config.NewSource(config.WithPath(writeConfig(t)))
	require.NoError(t, err)

	value, ok := src.LookupEnv("WAVELET_SOURCE_PROBE")
	require.True(t, ok)
	assert.Equal(t, "present", value)

	_, ok = src.LookupEnv("WAVELET_SOURCE_ABSENT")
	assert.False(t, ok)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./plugins/infra/config/ -v`
Expected: 编译失败，报 `package Wavelet/plugins/infra/config is not in std` / `undefined: config.NewSource`。

- [ ] **Step 3: 写实现**

创建 `backend/plugins/infra/config/source.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config adapts viper to the kernel configuration source contract. It is a
// runtime adapter rather than a core.Plugin: it owns no routes, services or tasks, and
// therefore never appears in app.Use. Keeping it out of core preserves the micro-kernel
// rule against importing concrete runtime dependencies.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// DefaultFileName is the configuration file looked up when CONFIG_PATH is unset.
const DefaultFileName = "config.yaml"

// EnvOnlyOrigin is reported by Describe when no configuration file was loaded.
const EnvOnlyOrigin = "<env only>"

// maxSearchDepth bounds the upward directory walk so a misconfigured working set
// cannot make the loader scan the whole filesystem.
const maxSearchDepth = 5

// Option configures a Source.
type Option func(*Source)

// WithPath pins the configuration file, bypassing CONFIG_PATH and the upward search.
func WithPath(path string) Option {
	return func(s *Source) {
		s.path = path
	}
}

// Source implements core.ConfigSource over a configuration file plus the process environment.
type Source struct {
	v     *viper.Viper
	path  string
	found bool
}

// NewSource loads the configuration file. A missing file is not an error: the source
// then serves environment values only, matching the previous pkg/config behaviour.
func NewSource(opts ...Option) (*Source, error) {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}

	if s.path == "" {
		s.path = os.Getenv("CONFIG_PATH")
	}
	if s.path == "" {
		s.path = findConfigPath(DefaultFileName)
	}

	v := viper.New()
	v.SetConfigFile(s.path)

	err := v.ReadInConfig()
	switch {
	case err == nil:
		s.found = true
	case isNotFound(err):
		// fall through to environment-only lookups
	default:
		if _, statErr := os.Stat(s.path); statErr == nil { //nolint:gosec // s.path comes from CONFIG_PATH or a bounded upward search
			return nil, fmt.Errorf("infra/config: read %s: %w", s.path, err)
		}
	}

	s.v = v
	return s, nil
}

func isNotFound(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist)
}

// Lookup returns the raw value stored at a dotted path, or false when the file was not
// loaded or the path is absent.
func (s *Source) Lookup(path string) (any, bool) {
	if !s.found || !s.v.IsSet(path) {
		return nil, false
	}
	return s.v.Get(path), true
}

// LookupEnv reads a process environment variable.
func (s *Source) LookupEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

// Describe returns the loaded file path, or EnvOnlyOrigin when running on environment values.
func (s *Source) Describe() string {
	if !s.found {
		return EnvOnlyOrigin
	}
	return s.path
}

// findConfigPath searches upward from the working directory so tests and binaries run
// from backend/ still find the repository-root configuration file.
func findConfigPath(configPath string) string {
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}

	dir := "."
	for range maxSearchDepth {
		dir += "/.."
		path := dir + "/" + configPath
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return configPath
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./plugins/infra/config/ -v`
Expected: 四个用例全部 `--- PASS`。

- [ ] **Step 5: 格式与静态检查**

Run: `cd backend && golangci-lint fmt ./plugins/infra/config/ && golangci-lint run ./plugins/infra/config/`
Expected: 无告警。

- [ ] **Step 6: 提交**

```bash
git add backend/plugins/infra/config/
git commit -m "feat(infra): add viper backed configuration source adapter"
```

---

## Task 8: 旧配置包可重入重构 + 新旧对拍

**Files:**
- Modify: `backend/pkg/config/config.go:57-108`
- Create: `backend/pkg/config/parity_test.go`（临时文件，P4 随旧包一并删除）

- [ ] **Step 1: 重构旧加载器为可重入函数**

把 `backend/pkg/config/config.go` 的 `init()` 拆成 `load` + `init`。原实现使用包级 `viper` 全局并在 `init` 里内联全部步骤，对拍需要能反复调用且不受 `isTest()` 干扰：

```go
// load reads configuration from configPath, applies defaults and environment overrides,
// and optionally disables external services for in-test runs.
func load(configPath string, testMode bool) *configModel {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			if _, statErr := os.Stat(configPath); statErr == nil { //nolint:gosec // configPath is loaded from CONFIG_PATH environment variable
				log.Fatalf("[Config] read config failed: %v\n", err)
			}
		}
		log.Println("[Config] no config file found, using environment variables only")
		v.SetConfigType("yaml")
		if err := v.ReadConfig(strings.NewReader("")); err != nil {
			log.Fatalf("[Config] failed to init empty config: %v\n", err)
		}
	}

	var c configModel
	if err := v.Unmarshal(&c); err != nil {
		log.Fatalf("[Config] parse config failed: %v\n", err)
	}

	applyDefaults(&c)
	applyEnvOverrides(&c)
	applyDefaults(&c)

	if testMode {
		c.Database.Enabled = false
		c.Database.SQLitePath = ":memory:"
		c.Redis.Enabled = false
		c.ClickHouse.Enabled = false
	}

	return &c
}

func init() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = findConfigPath("config.yaml")
	}

	Config = load(configPath, isTest())

	printConfig(Config)
}
```

同步调整：`import` 增加 `"errors"`；删除原 `init` 里的 `viper.SetConfigFile`/`viper.AutomaticEnv`/`viper.ReadInConfig` 等包级调用与 `if _, ok := err.(viper.ConfigFileNotFoundError); !ok` 断言（改用上面的 `errors.As`）。

> **行为等价说明（评审时核对）**：`viper.AutomaticEnv()` 只影响按键读取，`Unmarshal` 走的是 `AllKeys`，因此去掉 `AutomaticEnv` 不改变解析结果；环境变量覆盖仍由 `applyEnvOverrides` 负责。

- [ ] **Step 2: 运行旧测试确认无回归**

Run: `cd backend && go test ./pkg/config/ ./cmd/ -v -run 'TestApplyEnvOverrides|Test' 2>&1 | tail -30`
Expected: `pkg/config` 的 `TestApplyEnvOverridesRedisMaintNotifications` 通过；`cmd` 包既有用例结果与改动前一致（先运行一次改动前的 `go test ./cmd/` 记录基线）。

- [ ] **Step 3: 写对拍测试**

创建 `backend/pkg/config/parity_test.go`：

```go
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Temporary migration harness: proves the new kernel configuration engine resolves
// every key identically to pkg/config before the legacy singleton is deleted in P4.
// Delete this file together with backend/pkg/config.
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"Wavelet/core/extpoints"
	"Wavelet/plugins/infra/config"
)

// engineAppConfig mirrors appConfig with engine tags.
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

type engineDatabaseConfig struct {
	Enabled                bool          `config:"enabled" env:"DB_ENABLED" default:"false" autoEnable:"DB_HOST"`
	SQLitePath             string        `config:"sqlite_path" env:"SQLITE_PATH"`
	Host                   string        `config:"host" env:"DB_HOST"`
	Port                   int           `config:"port" env:"DB_PORT"`
	Username               string        `config:"username" env:"DB_USERNAME"`
	Password               string        `config:"password" env:"DB_PASSWORD" secret:"true"`
	Database               string        `config:"database" env:"DB_NAME"`
	MaxIdleConn            int           `config:"max_idle_conn" env:"DB_MAX_IDLE_CONN"`
	MaxOpenConn            int           `config:"max_open_conn" env:"DB_MAX_OPEN_CONN"`
	ConnMaxLifetime        int           `config:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime        int           `config:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME"`
	LogLevel               string        `config:"log_level" env:"DB_LOG_LEVEL"`
	SSLMode                string        `config:"ssl_mode" env:"DB_SSL_MODE"`
	TimeZone               string        `config:"time_zone" env:"DB_TIMEZONE"`
	ApplicationName        string        `config:"application_name" env:"DB_APPLICATION_NAME"`
	SearchPath             string        `config:"search_path" env:"DB_SEARCH_PATH"`
	PreferSimpleProtocol   bool          `config:"prefer_simple_protocol" env:"DB_PREFER_SIMPLE_PROTOCOL"`
	StatementCacheCapacity int           `config:"statement_cache_capacity" env:"DB_STATEMENT_CACHE_CAPACITY"`
	DefaultQueryExecMode   string        `config:"default_query_exec_mode" env:"DB_DEFAULT_QUERY_EXEC_MODE"`
	SlowThreshold          time.Duration `config:"slow_threshold" env:"DB_SLOW_THRESHOLD"`
}

type engineRedisConfig struct {
	Enabled            bool     `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs              []string `config:"addrs" env:"REDIS_ADDR"`
	Username           string   `config:"username" env:"REDIS_USERNAME"`
	Password           string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB                 int      `config:"db" env:"REDIS_DB"`
	ClusterMode        bool     `config:"cluster_mode" env:"REDIS_CLUSTER_MODE"`
	MasterName         string   `config:"master_name" env:"REDIS_MASTER_NAME"`
	KeyPrefix          string   `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	PoolSize           int      `config:"pool_size" env:"REDIS_POOL_SIZE"`
	MinIdleConn        int      `config:"min_idle_conn" env:"REDIS_MIN_IDLE_CONN"`
	DialTimeout        int      `config:"dial_timeout" env:"REDIS_DIAL_TIMEOUT"`
	ReadTimeout        int      `config:"read_timeout" env:"REDIS_READ_TIMEOUT"`
	WriteTimeout       int      `config:"write_timeout" env:"REDIS_WRITE_TIMEOUT"`
	MaxRetries         int      `config:"max_retries" env:"REDIS_MAX_RETRIES"`
	PoolTimeout        int      `config:"pool_timeout" env:"REDIS_POOL_TIMEOUT"`
	ConnMaxIdleTime    int      `config:"conn_max_idle_time" env:"REDIS_CONN_MAX_IDLE_TIME"`
	MaintNotifications bool     `config:"maint_notifications" env:"REDIS_MAINT_NOTIFICATIONS"`
}

type engineClickHouseConfig struct {
	Enabled         bool     `config:"enabled" env:"CLICKHOUSE_ENABLED" default:"false" autoEnable:"CLICKHOUSE_HOST"`
	Hosts           []string `config:"hosts" env:"CLICKHOUSE_HOST"`
	Username        string   `config:"username" env:"CLICKHOUSE_USERNAME"`
	Password        string   `config:"password" env:"CLICKHOUSE_PASSWORD" secret:"true"`
	Database        string   `config:"database" env:"CLICKHOUSE_NAME"`
	MaxIdleConn     int      `config:"max_idle_conn" env:"CLICKHOUSE_MAX_IDLE_CONN"`
	MaxOpenConn     int      `config:"max_open_conn" env:"CLICKHOUSE_MAX_OPEN_CONN"`
	ConnMaxLifetime int      `config:"conn_max_lifetime" env:"CLICKHOUSE_CONN_MAX_LIFETIME"`
	DialTimeout     int      `config:"dial_timeout" env:"CLICKHOUSE_DIAL_TIMEOUT"`
	BlockBufferSize uint8    `config:"block_buffer_size" env:"CLICKHOUSE_BLOCK_BUFFER_SIZE"`
}

type engineLogConfig struct {
	Level      string `config:"level" env:"LOG_LEVEL"`
	Format     string `config:"format" env:"LOG_FORMAT"`
	Output     string `config:"output" env:"LOG_OUTPUT"`
	FilePath   string `config:"file_path" env:"LOG_FILE_PATH"`
	MaxSize    int    `config:"max_size" env:"LOG_MAX_SIZE"`
	MaxAge     int    `config:"max_age" env:"LOG_MAX_AGE"`
	MaxBackups int    `config:"max_backups" env:"LOG_MAX_BACKUPS"`
	Compress   bool   `config:"compress" env:"LOG_COMPRESS"`
}

type engineOtelConfig struct {
	SamplingRate float64 `config:"sampling_rate" env:"OTEL_SAMPLING_RATE"`
	TracerName   string  `config:"tracer_name" env:"OTEL_TRACER_NAME" default:"github.com/Rain-kl/Wavelet"`
}

func repositoryConfig(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	path := filepath.Join(dir, "..", "..", "config.yaml")
	if _, statErr := os.Stat(path); statErr != nil {
		t.Skip("repository root config.yaml is unavailable")
	}
	return path
}

// copyLike copies same-named, same-typed exported fields from src into dst recursively.
func copyLike(dst, src reflect.Value) {
	for i := 0; i < dst.NumField(); i++ {
		dstField := dst.Type().Field(i)
		if dstField.PkgPath != "" {
			continue
		}
		srcField := src.FieldByName(dstField.Name)
		if !srcField.IsValid() {
			continue
		}
		if srcField.Kind() == reflect.Struct && srcField.Type() != reflect.TypeFor[time.Duration]() {
			copyLike(dst.Field(i), srcField)
			continue
		}
		if srcField.Type() == dst.Field(i).Type() {
			dst.Field(i).Set(srcField)
		}
	}
}

func TestEngineParityWithLegacyLoader(t *testing.T) {
	path := repositoryConfig(t)

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
			name: "scalar overrides and duration parsing",
			env: map[string]string{
				"LOG_LEVEL": "debug", "APP_ADDR": ":9999", "DB_SLOW_THRESHOLD": "1s",
				"REDIS_MAINT_NOTIFICATIONS": "true", "OTEL_SAMPLING_RATE": "0.5",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			for name, value := range scenario.env {
				t.Setenv(name, value)
			}

			legacy := load(path, false)

			src, err := config.NewSource(config.WithPath(path))
			require.NoError(t, err)

			engine := extpoints.NewConfigRegistry(src)
			require.NoError(t, engine.Declare("parity",
				extpoints.ConfigBinding{Prefix: "app", Target: &engineAppConfig{}},
				extpoints.ConfigBinding{Prefix: "database", Target: &engineDatabaseConfig{}},
				extpoints.ConfigBinding{Prefix: "redis", Target: &engineRedisConfig{}},
				extpoints.ConfigBinding{Prefix: "clickhouse", Target: &engineClickHouseConfig{}},
				extpoints.ConfigBinding{Prefix: "log", Target: &engineLogConfig{}},
				extpoints.ConfigBinding{Prefix: "otel", Target: &engineOtelConfig{}},
			))
			require.NoError(t, engine.Resolve())

			var app engineAppConfig
			var database engineDatabaseConfig
			var redis engineRedisConfig
			var clickhouse engineClickHouseConfig
			var log engineLogConfig
			var otel engineOtelConfig
			require.NoError(t, engine.Bind("app", &app))
			require.NoError(t, engine.Bind("database", &database))
			require.NoError(t, engine.Bind("redis", &redis))
			require.NoError(t, engine.Bind("clickhouse", &clickhouse))
			require.NoError(t, engine.Bind("log", &log))
			require.NoError(t, engine.Bind("otel", &otel))

			// The legacy loader keeps one code-level default outside the tags; the
			// engine expresses it as a declared default, so normalise before diffing.
			if legacy.App.SessionAge <= 0 {
				legacy.App.SessionAge = 86400
			}
			if legacy.Otel.TracerName == "" {
				legacy.Otel.TracerName = "github.com/Rain-kl/Wavelet"
			}

			copyLike(reflect.ValueOf(&app).Elem(), reflect.ValueOf(legacy.App).Elem())
			copyLike(reflect.ValueOf(&database).Elem(), reflect.ValueOf(legacy.Database).Elem())
			copyLike(reflect.ValueOf(&redis).Elem(), reflect.ValueOf(legacy.Redis).Elem())
			copyLike(reflect.ValueOf(&clickhouse).Elem(), reflect.ValueOf(legacy.ClickHouse).Elem())
			copyLike(reflect.ValueOf(&log).Elem(), reflect.ValueOf(legacy.Log).Elem())
			copyLike(reflect.ValueOf(&otel).Elem(), reflect.ValueOf(legacy.Otel).Elem())

			assert.Empty(t, cmp.Diff(app, legacy.App), "app config drifted")
			assert.Empty(t, cmp.Diff(database, legacy.Database), "database config drifted")
			assert.Empty(t, cmp.Diff(redis, legacy.Redis), "redis config drifted")
			assert.Empty(t, cmp.Diff(clickhouse, legacy.ClickHouse), "clickhouse config drifted")
			assert.Empty(t, cmp.Diff(log, legacy.Log), "log config drifted")
			assert.Empty(t, cmp.Diff(otel, legacy.Otel), "otel config drifted")
		})
	}
}
```

`require` 需从 testify 引入：在 import 块加 `"github.com/stretchr/testify/require"`。

- [ ] **Step 4: 运行对拍确认等价**

Run: `cd backend && go test ./pkg/config/ -run TestEngineParityWithLegacyLoader -v`
Expected: 四个场景全部 `--- PASS`，无任何 `drifted` 断言输出。若出现 drift，逐项核对是否为 spec §4.3 登记的 C1–C5 有意差异：是则在该场景补注释说明差异来源，否则按 drift 修复引擎。

- [ ] **Step 5: 确认既有测试与构建仍全绿**

Run: `cd backend && go test ./... && go build -o /dev/null ./...`
Expected: 全绿。

- [ ] **Step 6: 提交**

```bash
git add backend/pkg/config/
git commit -m "refactor(config): make legacy loader reentrant and add engine parity test"
```

---

## Task 9: 架构门禁脚本禁止 core 引入 viper

**Files:**
- Modify: `scripts/check_cordis_architecture.sh:57-65`

- [ ] **Step 1: 扩展检查项**

把 `CORE_FRAMEWORK_IMPORTS` 的匹配式加入 viper、mapstructure 与 gin 的兄弟框架，使配置装载依赖无法渗进内核：

```bash
CORE_FRAMEWORK_IMPORTS=$(rg -n '"github.com/gin-gonic/gin"|"gorm.io/gorm"|"github.com/hibiken/asynq"|"github.com/robfig/cron|"github.com/spf13/viper"|"github.com/mitchellh/mapstructure"' \
    "${BACKEND_DIR}/core/" --glob '*.go' -g '!*contracts*' -g '!*_test.go' || true)
```

同步更新失败提示文案，列出新增的两个包：

```bash
    log_fail "backend/core/ 严禁导入具体 Web/ORM/Worker/Config 运行时框架 (gin, gorm, asynq, cron, viper, mapstructure):"
```

- [ ] **Step 2: 运行脚本确认通过**

Run: `./scripts/check_cordis_architecture.sh`
Expected: `✓ backend/core/ 无重型框架依赖`，末行 `✓ 所有 Cordis 架构合规性检查全部通过 (0 Violations)!`，退出码 0。

- [ ] **Step 3: 反向验证检查有效性**

Run: `printf 'package core\n\nimport _ "github.com/spf13/viper"\n' > backend/core/zz_probe_tmp.go && ./scripts/check_cordis_architecture.sh; STATUS=$?; rm backend/core/zz_probe_tmp.go; exit $STATUS`
Expected: 报 `✗ [FAIL] backend/core/ 严禁导入具体 Web/ORM/Worker/Config 运行时框架`，退出码非 0。（临时文件必须删除，用后确认 `git status --short` 干净。）

- [ ] **Step 4: 提交**

```bash
git add scripts/check_cordis_architecture.sh
git commit -m "chore(arch): forbid viper and mapstructure inside the micro-kernel"
```

---

## Task 10: 收尾验证与文档同步

**Files:**
- Modify: `AGENTS.md`（Cordis 分层清单补配置扩展点条目）
- Modify: `docs/WAVELET_WHITE_PAPER.md`（§5.1 顶级分包说明）

- [ ] **Step 1: 写内核侧用法文档**

在 `AGENTS.md` 的“严格遵循事项 (Guardrails)”中，`扩展点自包含注册` 列表内 `动态配置` 一条之后补充：

```markdown
  - **静态配置声明**：插件在 `Apply` 中通过 `ctx.Config().Bind("<prefix>", &cfg)` 读取自己声明的配置（字段用 `config` / `env` / `default` / `autoEnable` / `secret` tag 声明）；需要在 `Apply` 之前被门禁求值的键，必须在 `DeclareConfig()` 中提前声明。**严禁**新增全局配置单例或在 `backend/pkg/` 读取配置。
```

- [ ] **Step 2: 修正白皮书漂移**

在 `docs/WAVELET_WHITE_PAPER.md` §5.1 的顶级分包说明后追加一条：

```markdown
- **配置所有权下沉**：`backend/pkg/config` 全局单例已废除。`core/extpoints` 只提供配置声明与解析引擎（不 import viper），`plugins/infra/config` 承担文件与环境装载，读哪些字段由各插件自行声明；组合根不再跨插件判断配置选实现，改由 `ConfigGatedPlugin` 门禁 + `FiberSkipped` 决定激活方。
```

- [ ] **Step 3: 全量门禁**

Run: `make code-check`
Expected: 架构脚本 0 Violations；`golangci-lint run` 无输出；前端 tsc 与 eslint 无新增错误（本计划未触碰前端，若报既有错误需确认为基线问题）。

- [ ] **Step 4: 格式化**

Run: `make format`
Expected: gofumpt 无 diff 或自动格式化；`git status --short` 中出现的文件需一并纳入下一步。

- [ ] **Step 5: 实跑确认应用行为未变**

Run: `cd backend && go run main.go all 2>&1 | head -40`
Expected: banner 正常输出、迁移日志正常、`[Config] loaded configuration` 出现，进程正常启动后 Ctrl-C 优雅退出。**这是 P1/P2 的验收线：新能力已就位，生产路径仍走旧单例，行为必须与改动前一致。**

- [ ] **Step 6: 提交**

```bash
git add AGENTS.md docs/WAVELET_WHITE_PAPER.md
git commit -m "docs(config): record the configuration extension point ownership rules"
```

---

## 完成标准（本计划）

1. `cd backend && go test ./...` 与 `go build ./...` 全绿；`make code-check` 与 `./scripts/check_cordis_architecture.sh` 零违规。
2. 引擎单测覆盖：优先级四档、`autoEnable` 与显式 env 的相对优先、标量 env→切片、`time.Duration`、结构体切片、冲突校验、脱敏导出、无 source 时的错误路径。
3. `TestEngineParityWithLegacyLoader` 四个场景零 drift，证明除 spec §4.3 的 C1–C5 外解析结果与旧实现逐 key 等价。
4. 同时挂载门禁谓词相反的两个插件时，恰好一个 `FiberActive`、一个 `FiberSkipped`，且被跳过者 `Apply` 从未执行。
5. `core/` 不出现 viper/mapstructure import，且架构脚本能主动拦截该违规。
6. 生产启动路径行为未变（仍由 `config.Config` 供值），业务插件与 `cmd` 零改动。
