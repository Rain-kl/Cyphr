// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_http provides the Gin HTTP web server driver plugin for Cordis.
package driver_http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/pkg/util"
)

const (
	defaultAddr              = ":8080"
	defaultReadHeaderTimeout = 10 * time.Second
	defaultShutdownTimeout   = 5 * time.Second
)

// Option configures the HTTP driver plugin.
type Option func(*Plugin)

// WithAddr sets the TCP address for the HTTP server to listen on.
func WithAddr(addr string) Option {
	return func(p *Plugin) {
		p.addr = addr
	}
}

// WithEngine sets a pre-configured Gin engine for the HTTP server.
func WithEngine(engine *gin.Engine) Option {
	return func(p *Plugin) {
		p.engine = engine
	}
}

// WithReadHeaderTimeout sets the ReadHeaderTimeout for http.Server.
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.readHeaderTimeout = d
	}
}

// WithShutdownTimeout sets the fallback timeout for graceful server shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.shutdownTimeout = d
	}
}

// Plugin implements core.Plugin and core.Driver for Gin HTTP Web Server.
type Plugin struct {
	mu                sync.RWMutex
	addr              string
	engine            *gin.Engine
	server            *http.Server
	listener          net.Listener
	running           bool
	readHeaderTimeout time.Duration
	shutdownTimeout   time.Duration
	coreCtx           *core.Context
}

// New creates a new Gin HTTP server driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		addr:              defaultAddr,
		readHeaderTimeout: defaultReadHeaderTimeout,
		shutdownTimeout:   defaultShutdownTimeout,
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
	return "driver_http"
}

// Apply mounts the HTTP driver plugin into the micro-kernel Context.
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

// Type returns DriverTypeHTTP.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeHTTP
}

// Start boots the Gin HTTP server, binds routes collected from ctx.Router(), and starts listening.
func (p *Plugin) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	if p.engine == nil {
		p.engine = gin.New()
	}

	// Mount routes collected in Context RouterExtension
	if p.coreCtx != nil && p.coreCtx.Router() != nil {
		for _, rd := range p.coreCtx.Router().Routes() {
			allHandlers := make([]gin.HandlerFunc, 0, len(rd.Middlewares)+len(rd.Handlers))

			for _, m := range rd.Middlewares {
				gh, err := toGinHandler(m)
				if err != nil {
					return fmt.Errorf("driver_http: invalid middleware for route %s %s: %w", rd.Method, rd.Path, err)
				}
				allHandlers = append(allHandlers, gh)
			}

			for _, h := range rd.Handlers {
				gh, err := toGinHandler(h)
				if err != nil {
					return fmt.Errorf("driver_http: invalid handler for route %s %s: %w", rd.Method, rd.Path, err)
				}
				allHandlers = append(allHandlers, gh)
			}

			p.engine.Handle(rd.Method, rd.Path, allHandlers...)
		}
	}

	p.server = &http.Server{
		Addr:              p.addr,
		Handler:           p.engine,
		ReadHeaderTimeout: p.readHeaderTimeout,
	}

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", p.addr)
	if err != nil {
		return fmt.Errorf("driver_http: listen on %s failed: %w", p.addr, err)
	}

	p.listener = listener
	p.addr = listener.Addr().String()
	p.running = true

	srv := p.server
	util.Go(func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			_ = serveErr
		}
	})

	return nil
}

// Stop gracefully stops the HTTP server.
//
//nolint:contextcheck
func (p *Plugin) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false

	var err error
	if p.server != nil {
		if ctx == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.Background(), p.shutdownTimeout)
			defer cancel()
		}
		err = p.server.Shutdown(ctx)
	}

	if p.listener != nil {
		_ = p.listener.Close()
		p.listener = nil
	}

	return err
}

// Addr returns the current listening address (or configured address if not yet started).
func (p *Plugin) Addr() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.addr
}

// Engine returns the underlying Gin engine.
func (p *Plugin) Engine() *gin.Engine {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.engine
}

// IsRunning returns whether the HTTP server is currently running.
func (p *Plugin) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

func toGinHandler(h any) (gin.HandlerFunc, error) {
	if h == nil {
		return nil, errors.New("nil handler")
	}

	switch fn := h.(type) {
	case gin.HandlerFunc:
		return fn, nil
	case func(*gin.Context):
		return gin.HandlerFunc(fn), nil
	case http.HandlerFunc:
		return gin.WrapF(fn), nil
	case func(http.ResponseWriter, *http.Request):
		return gin.WrapF(fn), nil
	case http.Handler:
		return gin.WrapH(fn), nil
	default:
		return nil, fmt.Errorf("unsupported handler type: %T", h)
	}
}
