// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"reflect"
	"sync"
)

// FiberState represents the lifecycle status of a plugin instance in the Cordis micro-kernel.
type FiberState string

const (
	// FiberPending indicates the plugin is waiting for its required dependencies to be provided.
	FiberPending FiberState = "PENDING"

	// FiberLoading indicates the plugin is currently running its Apply mounting phase.
	FiberLoading FiberState = "LOADING"

	// FiberActive indicates the plugin is fully mounted, active, and operating without error.
	FiberActive FiberState = "ACTIVE"

	// FiberUnloading indicates the plugin is tearing down its scoped effects in LIFO order.
	FiberUnloading FiberState = "UNLOADING"

	// FiberDisposed indicates the plugin has been completely unmounted and its context disposed.
	FiberDisposed FiberState = "DISPOSED"
)

// Fiber wraps a Plugin instance with a dedicated scoped Context and manages its
// reactive lifecycle state machine according to Cordis spatiotemporal composability principles.
type Fiber struct {
	mu     sync.RWMutex
	plugin Plugin
	state  FiberState
	ctx    *Context
	deps   []reflect.Type
	err    error
}

// NewFiber creates a new Fiber for the specified plugin with a child scoped context.
func NewFiber(rootCtx *Context, plugin Plugin) *Fiber {
	var deps []reflect.Type
	if depPlugin, ok := plugin.(DependentPlugin); ok {
		deps = depPlugin.Inject()
	}

	return &Fiber{
		plugin: plugin,
		state:  FiberPending,
		ctx:    rootCtx.Fork(),
		deps:   deps,
	}
}

// Plugin returns the underlying Plugin instance.
func (f *Fiber) Plugin() Plugin {
	return f.plugin
}

// Name returns the unique identifier of the plugin.
func (f *Fiber) Name() string {
	if f.plugin == nil {
		return ""
	}
	return f.plugin.Name()
}

// State returns the current lifecycle state of this Fiber.
func (f *Fiber) State() FiberState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// Context returns the dedicated scoped Context for this Fiber.
func (f *Fiber) Context() *Context {
	return f.ctx
}

// Dependencies returns the list of required service reflect.Types.
func (f *Fiber) Dependencies() []reflect.Type {
	res := make([]reflect.Type, len(f.deps))
	copy(res, f.deps)
	return res
}

// Error returns the latest mounting or unmounting error, if any.
func (f *Fiber) Error() error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.err
}

// DependenciesSatisfied checks if all declared dependencies are present in the target Context container.
func (f *Fiber) DependenciesSatisfied(ctx *Context) bool {
	if len(f.deps) == 0 {
		return true
	}
	container := ctx.Container()
	for _, dep := range f.deps {
		if _, err := container.resolve(dep); err != nil {
			return false
		}
	}
	return true
}

// Load executes the plugin mounting lifecycle: PENDING -> LOADING -> ACTIVE.
func (f *Fiber) Load() error {
	f.mu.Lock()
	if f.state != FiberPending {
		f.mu.Unlock()
		return nil
	}
	f.state = FiberLoading
	f.err = nil
	f.mu.Unlock()

	if err := f.plugin.Apply(f.ctx); err != nil {
		f.mu.Lock()
		f.err = fmt.Errorf("fiber %q: apply failed: %w", f.Name(), err)
		f.state = FiberPending
		_ = f.ctx.Dispose()
		f.mu.Unlock()
		return err
	}

	f.mu.Lock()
	f.state = FiberActive
	f.mu.Unlock()
	return nil
}

// Unload tears down the plugin: ACTIVE -> UNLOADING -> DISPOSED.
func (f *Fiber) Unload() error {
	f.mu.Lock()
	if f.state != FiberActive && f.state != FiberLoading {
		f.mu.Unlock()
		return nil
	}
	f.state = FiberUnloading
	f.mu.Unlock()

	err := f.ctx.Dispose()

	f.mu.Lock()
	f.state = FiberDisposed
	if err != nil {
		f.err = err
	}
	f.mu.Unlock()

	return err
}
