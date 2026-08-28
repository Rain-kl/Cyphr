// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
)

// Context is the central micro-kernel service bus and runtime lifecycle container.
// It embeds Go standard context.Context compatibility, hierarchical scoping,
// service resolution, and LIFO disposer teardown.
type Context struct {
	goCtx     context.Context
	cancel    context.CancelFunc
	parent    *Context
	container *Container

	events     *EventBus
	router     extpoints.RouterExtension
	migrations extpoints.MigrationExtension
	tasks      extpoints.TaskExtension
	schedules  extpoints.ScheduleExtension
	settings   extpoints.SettingExtension

	mu        sync.RWMutex
	children  []*Context
	disposers []Disposer
	drivers   []Driver
	values    map[any]any
	disposed  bool
}

// NewContext creates a new root Context wrapping a standard Go context.
// If base is nil, context.Background() is used by default.
//
//nolint:contextcheck
func NewContext(base context.Context) *Context {
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)

	return &Context{
		goCtx:      ctx,
		cancel:     cancel,
		container:  NewContainer(nil),
		events:     NewEventBus(),
		router:     extpoints.NewRouterRegistry(),
		migrations: extpoints.NewMigrationRegistry(),
		tasks:      extpoints.NewTaskRegistry(),
		schedules:  extpoints.NewScheduleRegistry(),
		settings:   extpoints.NewSettingRegistry(),
		values:     make(map[any]any),
	}
}

// Deadline returns the time when work done on behalf of this context should be canceled.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.goCtx.Deadline()
}

// Done returns a channel that's closed when work done on behalf of this context should be canceled.
func (c *Context) Done() <-chan struct{} {
	return c.goCtx.Done()
}

// Err returns a non-nil error value after Done is closed.
func (c *Context) Err() error {
	return c.goCtx.Err()
}

// Value returns the value associated with key, searching the local values map,
// the underlying Go context, and fallback parent Contexts.
func (c *Context) Value(key any) any {
	c.mu.RLock()
	if v, ok := c.values[key]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	if v := c.goCtx.Value(key); v != nil {
		return v
	}

	if c.parent != nil {
		return c.parent.Value(key)
	}

	return nil
}

// GoContext returns the underlying standard Go context.Context.
func (c *Context) GoContext() context.Context {
	return c.goCtx
}

// Set stores an arbitrary key-value pair in this Context's local storage.
func (c *Context) Set(key any, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[any]any)
	}
	c.values[key] = val
}

// Get retrieves a key-value pair from this Context's local storage.
func (c *Context) Get(key any) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.values == nil {
		return nil, false
	}
	v, ok := c.values[key]
	return v, ok
}

// Container returns the underlying IoC container for this Context.
func (c *Context) Container() *Container {
	return c.container
}

// Parent returns the parent Context, or nil if this is a root Context.
func (c *Context) Parent() *Context {
	return c.parent
}

// Fork creates a child Context with its own scoped IoC container and values,
// linked to this Context for hierarchical fallback resolution and cascading teardown.
func (c *Context) Fork() *Context {
	return c.ForkWithContext(c.goCtx)
}

// ForkWithContext creates a child Context using a specific standard Go context.
//
//nolint:contextcheck
func (c *Context) ForkWithContext(base context.Context) *Context {
	if base == nil {
		base = c.goCtx
	}
	ctx, cancel := context.WithCancel(base)

	child := &Context{
		goCtx:      ctx,
		cancel:     cancel,
		parent:     c,
		container:  NewContainer(c.container),
		events:     c.Events(),
		router:     c.Router(),
		migrations: c.Migrations(),
		tasks:      c.Tasks(),
		schedules:  c.Schedules(),
		settings:   c.Settings(),
		values:     make(map[any]any),
	}

	c.mu.Lock()
	c.children = append(c.children, child)
	c.mu.Unlock()

	return child
}

// Events returns the domain EventBus associated with this Context hierarchy.
func (c *Context) Events() *EventBus {
	return c.events
}

// Router returns the RouterExtension registry.
func (c *Context) Router() extpoints.RouterExtension {
	return c.router
}

// Migrations returns the MigrationExtension registry.
func (c *Context) Migrations() extpoints.MigrationExtension {
	return c.migrations
}

// Tasks returns the TaskExtension registry.
func (c *Context) Tasks() extpoints.TaskExtension {
	return c.tasks
}

// Task is an alias for Tasks().
func (c *Context) Task() extpoints.TaskExtension {
	return c.tasks
}

// Schedules returns the ScheduleExtension registry.
func (c *Context) Schedules() extpoints.ScheduleExtension {
	return c.schedules
}

// Schedule is an alias for Schedules().
func (c *Context) Schedule() extpoints.ScheduleExtension {
	return c.schedules
}

// Settings returns the SettingExtension registry.
func (c *Context) Settings() extpoints.SettingExtension {
	return c.settings
}

// Setting is an alias for Settings().
func (c *Context) Setting() extpoints.SettingExtension {
	return c.settings
}

// DB returns the contracts.DBService registered in the IoC container, or nil if not registered.
func (c *Context) DB() contracts.DBService {
	svc, err := Inject[contracts.DBService](c)
	if err != nil {
		return nil
	}
	return svc
}

// Cache returns the contracts.CacheService registered in the IoC container, or nil if not registered.
func (c *Context) Cache() contracts.CacheService {
	svc, err := Inject[contracts.CacheService](c)
	if err != nil {
		return nil
	}
	return svc
}

// OnDispose registers a cleanup callback function to be executed when this Context is disposed.
// It accepts func() error, func(), or Disposer.
func (c *Context) OnDispose(fn any) {
	if fn == nil {
		return
	}

	var d Disposer
	switch f := fn.(type) {
	case Disposer:
		d = f
	case func() error:
		d = f
	case func():
		d = func() error {
			f()
			return nil
		}
	default:
		panic(fmt.Sprintf("core: OnDispose expects func() error or func(), got %T", fn))
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.disposers = append(c.disposers, d)
}

// Dispose shuts down this Context and all child Contexts, running registered disposers in LIFO order.
func (c *Context) Dispose() error {
	c.mu.Lock()
	if c.disposed {
		c.mu.Unlock()
		return nil
	}
	c.disposed = true

	// Copy children and disposers under lock
	children := make([]*Context, len(c.children))
	copy(children, c.children)

	disposers := make([]Disposer, len(c.disposers))
	copy(disposers, c.disposers)
	c.mu.Unlock()

	var errs []error

	// 1. Dispose all child contexts in reverse order
	for i := len(children) - 1; i >= 0; i-- {
		if err := children[i].Dispose(); err != nil {
			errs = append(errs, err)
		}
	}

	// 2. Run local disposers in LIFO order
	for i := len(disposers) - 1; i >= 0; i-- {
		if err := disposers[i](); err != nil {
			errs = append(errs, err)
		}
	}

	// 3. Cancel the Go context
	if c.cancel != nil {
		c.cancel()
	}

	// 4. Detach from parent
	if c.parent != nil {
		c.parent.removeChild(c)
	}

	return errors.Join(errs...)
}

func (c *Context) removeChild(target *Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, child := range c.children {
		if child == target {
			c.children = append(c.children[:i], c.children[i+1:]...)
			break
		}
	}
}

// IsDisposed returns true if this Context has been disposed.
func (c *Context) IsDisposed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disposed
}

// RegisterDriver registers a runtime driver engine on this Context.
func (c *Context) RegisterDriver(d Driver) error {
	if d == nil {
		return ErrNilService
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.drivers = append(c.drivers, d)
	return nil
}

// Drivers returns a copy of all drivers registered on this Context.
func (c *Context) Drivers() []Driver {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Driver, len(c.drivers))
	copy(result, c.drivers)
	return result
}

// Driver looks up a registered driver by its driver type.
func (c *Context) Driver(driverType DriverType) (Driver, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, d := range c.drivers {
		if d.Type() == driverType {
			return d, true
		}
	}
	return nil, false
}
