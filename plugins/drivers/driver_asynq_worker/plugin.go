// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_asynq_worker provides the Asynq worker driver plugin for Cordis.
package driver_asynq_worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"github.com/Rain-kl/Wavelet/core"
)

const (
	defaultConcurrency     = 10
	defaultShutdownTimeout = 10 * time.Second
)

// Option configures the Asynq worker driver plugin.
type Option func(*Plugin)

// WithRedisOpt sets the Redis connection options for Asynq.
func WithRedisOpt(opt asynq.RedisConnOpt) Option {
	return func(p *Plugin) {
		p.redisOpt = opt
	}
}

// WithConcurrency sets the worker concurrency limit.
func WithConcurrency(concurrency int) Option {
	return func(p *Plugin) {
		p.concurrency = concurrency
	}
}

// WithQueues sets the queue priorities mapping.
func WithQueues(queues map[string]int) Option {
	return func(p *Plugin) {
		p.queues = queues
	}
}

// WithStrictPriority sets whether to process queues strictly in priority order.
func WithStrictPriority(strict bool) Option {
	return func(p *Plugin) {
		p.strictPriority = strict
	}
}

// WithShutdownTimeout sets the timeout for graceful worker shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.shutdownTimeout = d
	}
}

// WithServer injects an existing Asynq server instance.
func WithServer(srv *asynq.Server) Option {
	return func(p *Plugin) {
		p.server = srv
	}
}

// Plugin implements core.Plugin and core.Driver for Asynq background worker server.
type Plugin struct {
	mu              sync.RWMutex
	redisOpt        asynq.RedisConnOpt
	concurrency     int
	queues          map[string]int
	strictPriority  bool
	shutdownTimeout time.Duration
	server          *asynq.Server
	mux             *asynq.ServeMux
	running         bool
	coreCtx         *core.Context
}

// New creates a new Asynq Worker driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		redisOpt:        asynq.RedisClientOpt{Addr: "127.0.0.1:6379"},
		concurrency:     defaultConcurrency,
		shutdownTimeout: defaultShutdownTimeout,
		queues:          map[string]int{"default": 1},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}

	return p
}

// Name returns the unique plugin identifier.
func (p *Plugin) Name() string {
	return "driver_asynq_worker"
}

// Apply mounts the Asynq Worker driver into the micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	p.mu.Lock()
	p.coreCtx = ctx
	p.mu.Unlock()

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

// Start boots the Asynq worker server and starts processing background tasks.
func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	mux := asynq.NewServeMux()

	if p.coreCtx != nil && p.coreCtx.Tasks() != nil {
		for _, td := range p.coreCtx.Tasks().Tasks() {
			handler, err := toAsynqHandler(td.Handler)
			if err != nil {
				return fmt.Errorf("driver_asynq_worker: invalid handler for task pattern %q: %w", td.Pattern, err)
			}
			mux.Handle(td.Pattern, handler)
		}
	}

	if p.server == nil {
		p.server = asynq.NewServer(
			p.redisOpt,
			asynq.Config{
				Concurrency:     p.concurrency,
				Queues:          p.queues,
				StrictPriority:  p.strictPriority,
				ShutdownTimeout: p.shutdownTimeout,
			},
		)
	}

	if err := p.server.Start(mux); err != nil {
		return fmt.Errorf("driver_asynq_worker: start server failed: %w", err)
	}

	p.mux = mux
	p.running = true
	return nil
}

// Stop gracefully shuts down the Asynq worker server.
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false

	if p.server != nil {
		p.server.Stop()
		p.server.Shutdown()
		p.server = nil
	}

	return nil
}

// IsRunning returns whether the worker server is running.
func (p *Plugin) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Server returns the underlying Asynq server instance.
func (p *Plugin) Server() *asynq.Server {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.server
}

// Mux returns the underlying Asynq serve mux.
func (p *Plugin) Mux() *asynq.ServeMux {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mux
}

func toAsynqHandler(h any) (asynq.Handler, error) {
	if h == nil {
		return nil, errors.New("nil handler")
	}

	switch fn := h.(type) {
	case asynq.HandlerFunc:
		return fn, nil
	case asynq.Handler:
		return fn, nil
	case func(context.Context, *asynq.Task) error:
		return asynq.HandlerFunc(fn), nil
	case func(context.Context, []byte) error:
		return asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
			return fn(c, t.Payload())
		}), nil
	case func(context.Context) error:
		return asynq.HandlerFunc(func(c context.Context, _ *asynq.Task) error {
			return fn(c)
		}), nil
	case func([]byte) error:
		return asynq.HandlerFunc(func(_ context.Context, t *asynq.Task) error {
			return fn(t.Payload())
		}), nil
	case func() error:
		return asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
			return fn()
		}), nil
	default:
		return nil, fmt.Errorf("unsupported task handler type: %T", h)
	}
}
