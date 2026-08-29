// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_http 提供 HTTP 路由中间件与服务启动
package driver_http

import (
	"Wavelet/pkg/config"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	otel_trace "Wavelet/pkg/trace"
)

func loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 初始化 Trace
		ctx, span := otel_trace.Start(c.Request.Context(), "LoggerMiddleware")
		defer span.End()

		// 开始计时
		start := time.Now()

		// 记录请求路径和 Query
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		// 执行请求
		c.Next()

		// 停止计时
		end := time.Now()
		latency := end.Sub(start)

		// 打印日志
		// 排除健康检查接口
		healthPath := config.Config.App.APIPrefix + "/health"
		if c.Request.URL.Path != healthPath {
			logger.InfoF(
				ctx,
				"[LoggerMiddleware] %s %s\nStartTime: %s\nEndTime: %s\nLatency: %d\nClientIP: %s\nResponse: %d %d",
				c.Request.Method,
				path,
				start.Format(time.RFC3339),
				end.Format(time.RFC3339),
				latency.Milliseconds(),
				c.ClientIP(),
				c.Writer.Status(),
				c.Writer.Size(),
			)
		}

		// 设置 Span 状态
		if c.Writer.Status() >= http.StatusBadRequest {
			span := trace.SpanFromContext(ctx)
			span.SetStatus(codes.Error, strconv.Itoa(c.Writer.Status()))
		}
	}
}

const (
	// serverAddressConfigKey 是 admin 域声明的系统配置键（跨插件不能直接引用其常量）。
	serverAddressConfigKey = "server_address"
	// serverAddressCacheKey / serverAddressCacheTTL 把 CORS 允许来源的读取从每请求
	// 一次主库查询降为每 TTL 一次，TTL 与存储驱动的系统配置缓存保持一致。
	serverAddressCacheKey = "driver_http:cors:server_address"
	serverAddressCacheTTL = 5 * time.Second
)

// loadServerAddress reads the configured server address straight from system configs.
func loadServerAddress(ctx context.Context) (string, error) {
	db := getDB(ctx)
	if db == nil {
		return "", nil
	}
	var val string
	if err := db.Table("w_system_configs").Where("key = ?", serverAddressConfigKey).Pluck("value", &val).Error; err != nil {
		return "", err
	}
	return val, nil
}

// serverAddress returns the configured server address, served from the shared
// cache so CORS does not reach the database on every request.
func serverAddress(ctx context.Context) (string, error) {
	cacheSvc := getCache(ctx)
	if cacheSvc == nil {
		return loadServerAddress(ctx)
	}

	var val string
	if err := cacheSvc.GetOrSet(ctx, serverAddressCacheKey, &val, serverAddressCacheTTL, func() (any, error) {
		return loadServerAddress(ctx)
	}); err != nil {
		return "", err
	}
	return val, nil
}

func isOriginAllowed(ctx context.Context, origin string) bool {
	val, err := serverAddress(ctx)
	if err != nil || val == "" {
		return false
	}
	allowedOrigins := strings.Split(val, ",")
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimRight(strings.TrimSpace(allowed), "/")
		if allowed != "" && strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" && isOriginAllowed(c.Request.Context(), origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Access-Token, X-Cap-Token")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// errorHandlerMiddleware 委托给 response.ErrorHandlerMiddleware，保持路由层单一入口。
func errorHandlerMiddleware() gin.HandlerFunc {
	return response.ErrorHandlerMiddleware()
}
