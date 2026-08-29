// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// BuildEngine 构建并初始化 Gin 路由引擎及全部中间件和路由
func BuildEngine() (*gin.Engine, error) {
	return BuildEngineWithConfig(httpAppConfig{}, httpRedisConfig{})
}

// BuildEngineWithConfig constructs the Gin engine with explicitly injected configuration.
func BuildEngineWithConfig(appCfg httpAppConfig, redisCfg httpRedisConfig) (*gin.Engine, error) {
	// 运行模式
	if appCfg.Env == "production" || appCfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	setAPIPrefix(appCfg.APIPrefix)

	// 初始化路由
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	addrs := redisCfg.Addrs
	sessionAddr := "localhost:6379"
	if len(addrs) > 0 {
		sessionAddr = addrs[0]
	}

	sessionSecret := appCfg.SessionSecret
	if sessionSecret == "" {
		sessionSecret = "wavelet-default-session-secret"
	}

	sessionStore, err := redis.NewStoreWithDB(
		redisCfg.MinIdleConn,
		"tcp",
		sessionAddr,
		redisCfg.Username,
		redisCfg.Password,
		strconv.Itoa(redisCfg.DB),
		[]byte(sessionSecret),
	)
	if err != nil {
		return nil, err
	}

	// 设置 Session Redis Key 前缀
	if redisCfg.KeyPrefix != "" {
		if err := redis.SetKeyPrefix(sessionStore, redisCfg.KeyPrefix+"session:"); err != nil {
			log.Printf("[API] set session key prefix failed: %v\n", err)
		}
	}

	sessionCookieName := appCfg.SessionCookieName
	if sessionCookieName == "" {
		sessionCookieName = "wavelet_session"
	}

	sessionAge := appCfg.SessionAge
	if sessionAge <= 0 {
		sessionAge = 86400
	}

	sessionStore.Options(sessions.Options{
		Path:     "/",
		Domain:   appCfg.SessionDomain,
		MaxAge:   sessionAge,
		HttpOnly: appCfg.SessionHTTPOnly,
		Secure:   appCfg.SessionSecure,
		SameSite: http.SameSiteLaxMode,
	})

	r.Use(sessions.Sessions(sessionCookieName, sessionStore))

	appName := appCfg.AppName
	if appName == "" {
		appName = "Wavelet"
	}

	// 补充中间件
	r.Use(otelgin.Middleware(appName), errorHandlerMiddleware(), loggerMiddleware())

	return r, nil
}
