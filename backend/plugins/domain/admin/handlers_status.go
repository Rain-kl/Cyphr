// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/pkg/config"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var startTime = time.Now()

const (
	minutesInHour   = 60
	secondsInMinute = 60
	nanosPerSecond  = 1e9

	logDBNamePostgres       = "postgres"
	logDBNameSQLite         = "sqlite"
	logDBNameClickHouse     = "clickhouse"
	defaultLogRetentionDays = 30
)

// SystemStatusResponse 系统状态响应结构体
type SystemStatusResponse struct {
	Uptime       string `json:"uptime"`
	NumGoroutine int    `json:"num_goroutine"`
	Alloc        string `json:"alloc"`
	TotalAlloc   string `json:"total_alloc"`
	Sys          string `json:"sys"`
	Lookups      uint64 `json:"lookups"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    string `json:"heap_alloc"`
	HeapSys      string `json:"heap_sys"`
	HeapIdle     string `json:"heap_idle"`
	HeapInuse    string `json:"heap_inuse"`
	HeapReleased string `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   string `json:"stack_inuse"`
	StackSys     string `json:"stack_sys"`
	MSpanInuse   string `json:"mspan_inuse"`
	MSpanSys     string `json:"mspan_sys"`
	MCacheInuse  string `json:"mcache_inuse"`
	MCacheSys    string `json:"mcache_sys"`
	BuckHashSys  string `json:"buck_hash_sys"`
	GCSys        string `json:"gc_sys"`
	OtherSys     string `json:"other_sys"`
	NextGC       string `json:"next_gc"`
	LastGCTime   string `json:"last_gc_time"`
	PauseTotalNs string `json:"pause_total_ns"`
	LastPause    string `json:"last_pause"`
	NumGC        uint32 `json:"num_gc"`
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / hoursInDay
	hours := int(d.Hours()) % hoursInDay
	minutes := int(d.Minutes()) % minutesInHour
	seconds := int(d.Seconds()) % secondsInMinute

	var res string
	if days > 0 {
		res += fmt.Sprintf("%d天", days)
	}
	if hours > 0 {
		res += fmt.Sprintf("%d小时", hours)
	}
	if minutes > 0 {
		res += fmt.Sprintf("%d分钟", minutes)
	}
	if seconds > 0 || res == "" {
		res += fmt.Sprintf("%d秒钟", seconds)
	}
	return res
}

// GetSystemStatus 获取系统状态信息
// @Summary 获取系统状态信息
// @Description 获取后端服务运行状态、Goroutine、内存指标等详细统计数据，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=SystemStatusResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/status [get]
func GetSystemStatus(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := formatDuration(time.Since(startTime))
	numGoroutine := runtime.NumGoroutine()

	var lastGCTime string
	switch {
	case m.LastGC > 0 && m.LastGC <= math.MaxInt64:
		lastGCTime = formatDuration(time.Since(time.Unix(0, int64(m.LastGC))))
	case m.LastGC > 0:
		lastGCTime = "未知"
	default:
		lastGCTime = "无"
	}

	var lastPause string
	if m.NumGC > 0 {
		lastPause = fmt.Sprintf("%.3fs", float64(m.PauseNs[(m.NumGC-1)%256])/nanosPerSecond)
	} else {
		lastPause = "0.000s"
	}

	res := SystemStatusResponse{
		Uptime:       uptime,
		NumGoroutine: numGoroutine,
		Alloc:        formatBytes(m.Alloc),
		TotalAlloc:   formatBytes(m.TotalAlloc),
		Sys:          formatBytes(m.Sys),
		Lookups:      m.Lookups,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,
		HeapAlloc:    formatBytes(m.HeapAlloc),
		HeapSys:      formatBytes(m.HeapSys),
		HeapIdle:     formatBytes(m.HeapIdle),
		HeapInuse:    formatBytes(m.HeapInuse),
		HeapReleased: formatBytes(m.HeapReleased),
		HeapObjects:  m.HeapObjects,
		StackInuse:   formatBytes(m.StackInuse),
		StackSys:     formatBytes(m.StackSys),
		MSpanInuse:   formatBytes(m.MSpanInuse),
		MSpanSys:     formatBytes(m.MSpanSys),
		MCacheInuse:  formatBytes(m.MCacheInuse),
		MCacheSys:    formatBytes(m.MCacheSys),
		BuckHashSys:  formatBytes(m.BuckHashSys),
		GCSys:        formatBytes(m.GCSys),
		OtherSys:     formatBytes(m.OtherSys),
		NextGC:       formatBytes(m.NextGC),
		LastGCTime:   lastGCTime,
		PauseTotalNs: fmt.Sprintf("%.1fs", float64(m.PauseTotalNs)/nanosPerSecond),
		LastPause:    lastPause,
		NumGC:        m.NumGC,
	}

	c.JSON(http.StatusOK, response.OK(res))
}

// LogDatabaseStatus 日志库状态。
type LogDatabaseStatus struct {
	ActiveDatabase   string         `json:"active_database"`
	Migration        string         `json:"migration"`
	RetentionDays    map[string]int `json:"retention_days"`
	AvailableTargets []string       `json:"available_targets"`
}

// GetLogDatabaseStatus 返回当前日志库状态。
// @Summary 获取日志数据库状态
// @Description 返回当前日志主库、迁移状态、各库保留天数与合法迁移目标，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=LogDatabaseStatus} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/status/log-database [get]
func GetLogDatabaseStatus(c *gin.Context) {
	ctx := c.Request.Context()
	activeDB := "sqlite"
	migration := "idle"
	if rc := GetRiskControlService(); rc != nil {
		activeDB = rc.ActiveLogEngine(ctx)
		if rc.IsLogEngineMigrating(ctx) {
			migration = "migrating"
		}
	}
	c.JSON(http.StatusOK, response.OK(LogDatabaseStatus{
		ActiveDatabase: activeDB,
		Migration:      migration,
		RetentionDays: map[string]int{
			logDBNamePostgres:   retentionOr(ctx, ConfigKeyLogRetentionDaysPostgres),
			logDBNameSQLite:     retentionOr(ctx, ConfigKeyLogRetentionDaysSQLite),
			logDBNameClickHouse: retentionOr(ctx, ConfigKeyLogRetentionDaysClickHouse),
		},
		AvailableTargets: availableLogTargets(activeDB),
	}))
}

func retentionOr(ctx context.Context, key string) int {
	v, err := GetIntByKey(ctx, key)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.ErrorF(ctx, "读取日志保留天数配置失败 key=%s: %v", key, err)
		}
		return defaultLogRetentionDays
	}
	if v < 1 {
		return defaultLogRetentionDays
	}
	return v
}

func availableLogTargets(active string) []string {
	if active == logDBNameClickHouse {
		if config.Config.Database.Enabled {
			return []string{logDBNamePostgres}
		}
		return []string{logDBNameSQLite}
	}
	if config.Config.ClickHouse.Enabled {
		return []string{logDBNameClickHouse}
	}
	return []string{}
}
