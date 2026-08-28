// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"Wavelet/pkg/config"
	"Wavelet/pkg/trace"
	"Wavelet/pkg/util"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// BuildEngine 构建并初始化 Gin 路由引擎及全部中间件和路由
func BuildEngine() (*gin.Engine, error) {
	// 运行模式
	if config.Config.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	cfg := config.Config.Redis
	addrs := cfg.Addrs
	sessionAddr := "localhost:6379"
	if len(addrs) > 0 {
		sessionAddr = addrs[0]
	}

	sessionStore, err := redis.NewStoreWithDB(
		cfg.MinIdleConn,
		"tcp",
		sessionAddr,
		cfg.Username,
		cfg.Password,
		strconv.Itoa(cfg.DB),
		[]byte(config.Config.App.SessionSecret),
	)
	if err != nil {
		return nil, err
	}

	// 设置 Session Redis Key 前缀
	if cfg.KeyPrefix != "" {
		if err := redis.SetKeyPrefix(sessionStore, cfg.KeyPrefix+"session:"); err != nil {
			log.Printf("[API] set session key prefix failed: %v\n", err)
		}
	}

	sessionStore.Options(sessions.Options{
		Path:     "/",
		Domain:   config.Config.App.SessionDomain,
		MaxAge:   config.Config.App.SessionAge,
		HttpOnly: config.Config.App.SessionHTTPOnly,
		Secure:   config.Config.App.SessionSecure,
		SameSite: http.SameSiteLaxMode,
	})

	r.Use(sessions.Sessions(config.Config.App.SessionCookieName, sessionStore))

	// 补充中间件
	r.Use(otelgin.Middleware(config.Config.App.AppName), errorHandlerMiddleware(), loggerMiddleware())

	return r, nil
}

// Serve 启动 HTTP API 服务。onStarted 仅会在 HTTP 地址成功绑定后调用。
func Serve(onStarted func()) {
	r, err := BuildEngine()
	if err != nil {
		log.Fatalf("[API] init session store failed: %v\n", err)
	}

	srv := &http.Server{
		Addr:              config.Config.App.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", config.Config.App.Addr)
	if err != nil {
		log.Fatalf("[API] server failed to listen on %s: %v\n", config.Config.App.Addr, err)
	}
	if onStarted != nil {
		onStarted()
	}

	util.Go(func() {
		log.Printf("[API] server listening on %s\n", config.Config.App.Addr)
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[API] server failed: %v\n", err)
		}
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Config.App.GracefulShutdownTimeout)*time.Second)

	trace.Shutdown(shutdownCtx)

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[API] server forced to shutdown: %v\n", err)
		cancel()
		os.Exit(1)
	}
	cancel()

	log.Println("[API] server exited")
}
