// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
	gwrunner "github.com/Rain-kl/Wavelet/internal/apps/message_gateway/runner"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/task/scheduler"
	"github.com/Rain-kl/Wavelet/internal/infra/task/worker"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/router"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"github.com/Rain-kl/Wavelet/plugins/domain/admin"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	"github.com/Rain-kl/Wavelet/plugins/domain/message_gateway"
	"github.com/Rain-kl/Wavelet/plugins/domain/risk_control"
	"github.com/Rain-kl/Wavelet/plugins/domain/user"
	"github.com/Rain-kl/Wavelet/plugins/infra/cache"
	"github.com/Rain-kl/Wavelet/plugins/infra/database"
	"github.com/Rain-kl/Wavelet/plugins/infra/logger"
	"github.com/Rain-kl/Wavelet/plugins/infra/storage"
	"github.com/hibiken/asynq"
)

// newWaveletApp creates a core.App wired with Wavelet platform infrastructure, domain plugins, and profile drivers.
//
//nolint:contextcheck
func newWaveletApp(profile core.Profile) *core.App {
	app := core.NewApp(
		core.WithProfile(profile),
		core.WithShutdownTimeout(time.Duration(config.Config.App.GracefulShutdownTimeout)*time.Second),
	)

	// Register standard infrastructure plugins
	app.Use(
		database.New(),
		cache.New(),
		logger.New(),
		storage.New(),
	)

	// Register domain plugins
	app.Use(
		auth.New(),
		user.New(),
		message_gateway.New(),
		risk_control.New(),
		admin.New(),
	)

	// Bind Goose migration runner
	app.SetMigrationRunner(func(_ context.Context, _ []extpoints.MigrationEntry) error {
		runMigrations()
		return nil
	})

	// Mount drivers for each aspect
	app.Use(
		newWaveletHTTPDriver(profile),
		newWaveletWorkerDriver(profile),
		newWaveletSchedulerDriver(profile),
	)

	return app
}

type waveletHTTPDriver struct {
	profile core.Profile
	server  *http.Server
	mu      sync.Mutex
	running bool
}

func newWaveletHTTPDriver(profile core.Profile) *waveletHTTPDriver {
	return &waveletHTTPDriver{profile: profile}
}

func (d *waveletHTTPDriver) Name() string {
	return "driver_wavelet_http"
}

func (d *waveletHTTPDriver) Apply(ctx *core.Context) error {
	return ctx.RegisterDriver(d)
}

func (d *waveletHTTPDriver) Type() core.DriverType {
	return core.DriverTypeHTTP
}

//nolint:contextcheck
func (d *waveletHTTPDriver) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	bootstrap.RegisterAPI()
	runBootstrap(bootstrap.Options{API: true})

	engine, err := router.BuildEngine()
	if err != nil {
		return fmt.Errorf("[API] build router engine failed: %w", err)
	}

	srv := &http.Server{
		Addr:              config.Config.App.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.Config.App.Addr)
	if err != nil {
		return fmt.Errorf("[API] listen on %s failed: %w", config.Config.App.Addr, err)
	}

	mode := "API"
	if d.profile == core.ProfileAll {
		mode = "API + Worker + Scheduler"
	}
	printStartupBanner(startupState{
		mode:           mode,
		relationalDB:   latestMigrationState.relationalDB,
		clickHouseDB:   latestMigrationState.clickHouseDB,
		listensForHTTP: true,
	})

	d.server = srv
	d.running = true

	util.Go(func() {
		log.Printf("[API] server listening on %s\n", config.Config.App.Addr)
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("[API] server failed: %v\n", serveErr)
		}
	})

	return nil
}

func (d *waveletHTTPDriver) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}
	d.running = false

	var err error
	if d.server != nil {
		err = d.server.Shutdown(ctx)
		d.server = nil
	}
	bootstrap.Stop(ctx)
	log.Println("[API] server exited")
	return err
}

type waveletWorkerDriver struct {
	profile core.Profile
	server  *asynq.Server
	mu      sync.Mutex
	running bool
}

func newWaveletWorkerDriver(profile core.Profile) *waveletWorkerDriver {
	return &waveletWorkerDriver{profile: profile}
}

func (d *waveletWorkerDriver) Name() string {
	return "driver_wavelet_worker"
}

func (d *waveletWorkerDriver) Apply(ctx *core.Context) error {
	return ctx.RegisterDriver(d)
}

func (d *waveletWorkerDriver) Type() core.DriverType {
	return core.DriverTypeWorker
}

//nolint:contextcheck
func (d *waveletWorkerDriver) Start(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	if d.profile == core.ProfileAll {
		bootstrap.RegisterAll()
	} else {
		bootstrap.RegisterWorker()
	}
	runBootstrap(bootstrap.Options{})

	if d.profile == core.ProfileWorker {
		printStartupBanner(startupState{
			mode:         "Worker",
			relationalDB: latestMigrationState.relationalDB,
			clickHouseDB: latestMigrationState.clickHouseDB,
		})
	}

	util.Go(func() {
		if err := gwrunner.Start(context.Background()); err != nil {
			log.Printf("[Worker] message gateway stopped: %v", err)
		}
	})

	log.Println("[Worker] 启动任务处理服务")
	srv, err := worker.StartWorkerServer()
	if err != nil {
		return fmt.Errorf("[Worker] 启动失败: %w", err)
	}

	d.server = srv
	d.running = true
	return nil
}

func (d *waveletWorkerDriver) Stop(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}
	d.running = false

	if d.server != nil {
		d.server.Stop()
		d.server.Shutdown()
		d.server = nil
	}
	log.Println("[Worker] 任务处理服务已退出")
	return nil
}

type waveletSchedulerDriver struct {
	profile core.Profile
	mu      sync.Mutex
	running bool
}

func newWaveletSchedulerDriver(profile core.Profile) *waveletSchedulerDriver {
	return &waveletSchedulerDriver{profile: profile}
}

func (d *waveletSchedulerDriver) Name() string {
	return "driver_wavelet_scheduler"
}

func (d *waveletSchedulerDriver) Apply(ctx *core.Context) error {
	return ctx.RegisterDriver(d)
}

func (d *waveletSchedulerDriver) Type() core.DriverType {
	return core.DriverTypeScheduler
}

//nolint:contextcheck
func (d *waveletSchedulerDriver) Start(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	if d.profile == core.ProfileAll {
		bootstrap.RegisterAll()
	} else {
		bootstrap.RegisterScheduler()
	}
	runBootstrap(bootstrap.Options{})

	if d.profile == core.ProfileSchedule {
		printStartupBanner(startupState{
			mode:         "Scheduler",
			relationalDB: latestMigrationState.relationalDB,
			clickHouseDB: latestMigrationState.clickHouseDB,
		})
	}

	log.Println("[Scheduler] 启动定时任务调度服务")
	if err := scheduler.ReloadScheduler(); err != nil {
		return fmt.Errorf("[Scheduler] 启动失败: %w", err)
	}

	d.running = true
	return nil
}

func (d *waveletSchedulerDriver) Stop(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}
	d.running = false

	scheduler.StopScheduler()
	log.Println("[Scheduler] 定时任务调度服务已退出")
	return nil
}
