// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"Wavelet/core/extpoints"
	"context"
	"errors"
	"reflect"
)

// Standard sentinel errors returned by core operations.
var (
	// ErrServiceNotFound is returned when a requested service is not registered in the IoC container.
	ErrServiceNotFound = errors.New("core: service not found")

	// ErrServiceNotReady is returned when one or more required services are not ready in Using/UsingN.
	ErrServiceNotReady = errors.New("core: service not ready")

	// ErrNilContext is returned when a nil Context is passed to an operation requiring a valid Context.
	ErrNilContext = errors.New("core: context is nil")

	// ErrNilService is returned when attempting to provide a nil service implementation.
	ErrNilService = errors.New("core: service is nil")

	// ErrInvalidManifest is returned when a plugin manifest fails validation.
	ErrInvalidManifest = errors.New("core: invalid manifest")

	// ErrInvalidManifestName is returned when a plugin manifest has an empty name.
	ErrInvalidManifestName = errors.New("core: manifest name is required")

	// ErrDriverNotFound is returned when a requested driver type is not registered.
	ErrDriverNotFound = errors.New("core: driver not found")

	// ErrAppRunning is returned when attempting to start an already running App.
	ErrAppRunning = errors.New("core: app is already running")

	// ErrAppNotRunning is returned when attempting to operate on an App that is not running.
	ErrAppNotRunning = errors.New("core: app is not running")
)

// Plugin is the unified contract for all core and downstream plugins.
type Plugin interface {
	// Name returns the globally unique identifier of the plugin (e.g. "auth", "database").
	Name() string
	// Apply is the core mounting entrypoint: provides services, registers routes, tasks, and event listeners.
	Apply(ctx *Context) error
}

// DependentPlugin is an optional extension interface for plugins that declare required service dependencies.
type DependentPlugin interface {
	Plugin
	Inject() []reflect.Type
}

// PluginWithManifest is an optional extension interface for plugins that declare metadata.
type PluginWithManifest interface {
	Plugin
	Manifest() Manifest
}

// DriverType identifies the category of a runtime driver engine.
type DriverType string

const (
	// DriverTypeHTTP represents HTTP web server drivers (e.g. Gin).
	DriverTypeHTTP DriverType = "http"

	// DriverTypeWorker represents asynchronous background worker drivers (e.g. Asynq worker server).
	DriverTypeWorker DriverType = "worker"

	// DriverTypeScheduler represents cron and timer schedule drivers (e.g. Asynq scheduler).
	DriverTypeScheduler DriverType = "schedule"
)

// Driver is a runtime engine that manages an event loop or listening port.
type Driver interface {
	// Type returns the category of this driver engine.
	Type() DriverType
	// Start starts the driver lifecycle loop.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the driver.
	Stop(ctx context.Context) error
}

// Profile identifies the runtime aspect or execution mode of an application.
type Profile string

const (
	// ProfileAPI runs HTTP API server drivers.
	ProfileAPI Profile = "api"

	// ProfileWorker runs asynchronous background worker drivers.
	ProfileWorker Profile = "worker"

	// ProfileSchedule runs cron and timer schedule drivers.
	ProfileSchedule Profile = "schedule"

	// ProfileAll runs all registered drivers concurrently in fused mode.
	ProfileAll Profile = "all"
)

// MigrationEngine is the interface for executing database migrations across registered plugins.
// The ctx parameter is the root micro-kernel Context, allowing the engine to resolve
// services from the IoC container via core.Inject or core.Using.
type MigrationEngine interface {
	Migrate(ctx *Context, entries []MigrationEntry) error
}

// MigrationRunner is a function adapter implementing MigrationEngine.
type MigrationRunner func(ctx *Context, entries []MigrationEntry) error

// Migrate calls the underlying migration function.
func (fn MigrationRunner) Migrate(ctx *Context, entries []MigrationEntry) error {
	return fn(ctx, entries)
}

// Disposer is a cleanup function executed when a Context is disposed.
type Disposer func() error

// RouterExtension re-exports extpoints.RouterExtension.
type RouterExtension = extpoints.RouterExtension

// RouteDefinition re-exports extpoints.RouteDefinition.
type RouteDefinition = extpoints.RouteDefinition

// MigrationExtension re-exports extpoints.MigrationExtension.
type MigrationExtension = extpoints.MigrationExtension

// MigrationEntry re-exports extpoints.MigrationEntry.
type MigrationEntry = extpoints.MigrationEntry

// TaskExtension re-exports extpoints.TaskExtension.
type TaskExtension = extpoints.TaskExtension

// TaskDefinition re-exports extpoints.TaskDefinition.
type TaskDefinition = extpoints.TaskDefinition

// TaskOption re-exports extpoints.TaskOption.
type TaskOption = extpoints.TaskOption

// ScheduleExtension re-exports extpoints.ScheduleExtension.
type ScheduleExtension = extpoints.ScheduleExtension

// ScheduleDefinition re-exports extpoints.ScheduleDefinition.
type ScheduleDefinition = extpoints.ScheduleDefinition

// ScheduleOption re-exports extpoints.ScheduleOption.
type ScheduleOption = extpoints.ScheduleOption

// SettingExtension re-exports extpoints.SettingExtension.
type SettingExtension = extpoints.SettingExtension

// SettingSchema re-exports extpoints.SettingSchema.
type SettingSchema = extpoints.SettingSchema

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
// configuration. The kernel evaluates the gate during reconciliation, before the
// plugin's Apply runs, so every key read by ConfigEnabled must be published through
// DeclareConfig.
type ConfigGatedPlugin interface {
	Plugin

	// DeclareConfig publishes the configuration bindings consumed by ConfigEnabled.
	DeclareConfig() []extpoints.ConfigBinding

	// ConfigEnabled reports whether this plugin should activate for the resolved values.
	ConfigEnabled(view extpoints.ConfigView) bool
}
