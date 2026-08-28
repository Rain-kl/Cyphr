// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_inproc_worker provides the zero-dependency in-process async worker driver plugin for Cordis.
package driver_inproc_worker

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"
	"time"
)

const (
	defaultConcurrency     = 10
	defaultQueueCapacity   = 2000
	defaultShutdownTimeout = 5 * time.Second
)

var (
	globalMu    sync.RWMutex
	globalQueue *InprocQueue
)

// DispatchTask enqueues a background task to the active in-process worker queue.
func DispatchTask(_ context.Context, taskType string, payload []byte, source string) (string, error) {
	globalMu.RLock()
	q := globalQueue
	globalMu.RUnlock()

	if q == nil {
		return "", nil
	}
	return q.Enqueue(taskType, payload, source)
}

// Option configures the in-process worker driver plugin.
type Option func(*Plugin)

// WithConcurrency sets the number of concurrent worker goroutines.
func WithConcurrency(concurrency int) Option {
	return func(p *Plugin) {
		p.concurrency = concurrency
	}
}

// WithQueueCapacity sets the internal queue buffer capacity.
func WithQueueCapacity(capacity int) Option {
	return func(p *Plugin) {
		p.queueCapacity = capacity
	}
}

// WithShutdownTimeout sets the maximum duration to wait for in-flight tasks during graceful shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.shutdownTimeout = d
	}
}

// Plugin implements core.Plugin and core.Driver for in-process background worker execution.
type Plugin struct {
	concurrency     int
	queueCapacity   int
	shutdownTimeout time.Duration
	coreCtx         *core.Context
	queue           *InprocQueue
}

// New creates a new in-process worker driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		concurrency:     defaultConcurrency,
		queueCapacity:   defaultQueueCapacity,
		shutdownTimeout: defaultShutdownTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the plugin.
func (p *Plugin) Name() string {
	return "driver_inproc_worker"
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "driver_inproc_worker",
		Version:     "1.0.0",
		Description: "Zero-dependency in-process async worker driver plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers the worker driver and provides contracts.TaskService.
func (p *Plugin) Apply(ctx *core.Context) error {
	p.coreCtx = ctx

	taskSvc := newInprocTaskService(ctx.Tasks())
	core.Provide[contracts.TaskService](ctx, taskSvc)

	ctx.OnDispose(func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
		defer cancel()
		return p.Stop(shutdownCtx)
	})

	return ctx.RegisterDriver(p)
}

// Type returns DriverTypeWorker.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeWorker
}

// Start initiates task consumption.
func (p *Plugin) Start(_ context.Context) error {
	if p.queue == nil {
		p.queue = NewInprocQueue(p.concurrency, p.queueCapacity, p.coreCtx.Tasks())
	}

	globalMu.Lock()
	globalQueue = p.queue
	globalMu.Unlock()

	p.queue.Start()
	return nil
}

// Stop gracefully terminates worker processing.
func (p *Plugin) Stop(ctx context.Context) error {
	if p.queue != nil {
		err := p.queue.Stop(ctx)
		globalMu.Lock()
		if globalQueue == p.queue {
			globalQueue = nil
		}
		globalMu.Unlock()
		return err
	}
	return nil
}
