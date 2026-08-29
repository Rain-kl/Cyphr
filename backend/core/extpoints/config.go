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

	// ErrConfigUnknownKey is returned when a configuration key was never declared.
	ErrConfigUnknownKey = errors.New("extpoints: unknown configuration key")

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
// key is the fully qualified dotted path used for resolution; path is the raw `config`
// tag value used to locate the Go field again during Bind.
type configField struct {
	key        string
	path       string
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

	// SetSource installs the raw value source after construction, letting the composition
	// root build the adapter once the kernel Context already exists.
	SetSource(src ConfigSource)
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
			path:       path,
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
