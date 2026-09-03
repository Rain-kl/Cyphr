// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"Wavelet/pkg/logger"
	"fmt"
	"math"
	"time"
)

const (
	binaryKB        = 0
	binaryMB        = 1
	binaryGB        = 2
	valueThreshold  = 10
	maxStringLength = 200
)

// FormatBytes renders a byte count using binary units.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(bytes) / float64(div)
	var suffix string
	switch exp {
	case binaryKB:
		suffix = "KiB"
	case binaryMB:
		suffix = "MiB"
	case binaryGB:
		suffix = "GiB"
	default:
		suffix = "TiB"
	}

	if value == math.Trunc(value) {
		if value >= valueThreshold {
			return fmt.Sprintf("%.0f %s", value, suffix)
		}
		return fmt.Sprintf("%.1f %s", value, suffix)
	}
	return fmt.Sprintf("%.1f %s", value, suffix)
}

// TruncateDisplayValue caps oversized cell values before they reach the console UI.
func TruncateDisplayValue(value string) string {
	runes := []rune(value)
	if len(runes) > maxStringLength {
		return string(runes[:maxStringLength]) + "..."
	}
	return value
}

// DBOverviewResponse 数据库运行概览响应结构体
type DBOverviewResponse struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	Size        string `json:"size"`
	TableCount  int64  `json:"table_count"`
	Connections int64  `json:"connections"`
}

// GetTableDataRequest 分页拉取表数据请求结构体
type GetTableDataRequest struct {
	Table    string `form:"table" binding:"required"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"pageSize,default=10"`
}

// TableDataResponse 动态数据表响应结构体
type TableDataResponse struct {
	Columns []string                 `json:"columns"`
	Total   int64                    `json:"total"`
	Results []map[string]interface{} `json:"results"`
}

// ExecuteSQLRequest 执行自定义 SQL 请求结构体
type ExecuteSQLRequest struct {
	SQL string `json:"sql" binding:"required"`
}

// ExecuteSQLResponse 执行自定义 SQL 响应结构体
type ExecuteSQLResponse struct {
	Type            string                   `json:"type"` // "select" 或 "exec"
	Columns         []string                 `json:"columns,omitempty"`
	Results         []map[string]interface{} `json:"results,omitempty"`
	AffectedRows    int64                    `json:"affected_rows"`
	ExecutionTimeMs int64                    `json:"execution_time_ms"`
}

// DatabaseInfoResponse 数据库信息响应结构体
type DatabaseInfoResponse struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

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

// LogDatabaseStatus 日志库状态。
type LogDatabaseStatus struct {
	ActiveDatabase   string         `json:"active_database"`
	Migration        string         `json:"migration"`
	RetentionDays    map[string]int `json:"retention_days"`
	AvailableTargets []string       `json:"available_targets"`
}

// CreateSystemConfigRequest 创建系统配置请求
type CreateSystemConfigRequest struct {
	Key         string `json:"key" binding:"required,max=64"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=system business"`
	Visibility  int    `json:"visibility" binding:"oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateSystemConfigRequest 更新系统配置请求
type UpdateSystemConfigRequest struct {
	Value       string `json:"value" binding:"required"`
	Visibility  *int   `json:"visibility" binding:"omitempty,oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// TestSMTPRequest 测试 SMTP 配置请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host" binding:"required,max=255"`
	SMTPPort     int    `json:"smtp_port" binding:"required"`
	SMTPUsername string `json:"smtp_username" binding:"required,max=255"`
	SMTPPassword string `json:"smtp_password" binding:"required,max=255"`
	To           string `json:"to" binding:"required,email"`
}

// TestSMTPResponse 测试 SMTP 配置响应
type TestSMTPResponse struct {
	Success bool   `json:"success"`
	Log     string `json:"log"`
	Error   string `json:"error"`
}

// UpdateCacheConfigRequest 磁盘缓存策略更新请求
type UpdateCacheConfigRequest struct {
	MaxSizeMB  int64 `json:"max_size_mb" binding:"required,min=1"`
	TTLMinutes int64 `json:"ttl_minutes" binding:"required,min=0"`
	LRUEnabled bool  `json:"lru_enabled"`
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Key         string `json:"key" binding:"required,max=80"`
	Name        string `json:"name" binding:"required,max=100"`
	Type        string `json:"type" binding:"required,max=20"`
	Subject     string `json:"subject" binding:"max=255"`
	Content     string `json:"content" binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Type        string `json:"type" binding:"required,max=20"`
	Subject     string `json:"subject" binding:"max=255"`
	Content     string `json:"content" binding:"required"`
	Description string `json:"description" binding:"max=255"`
}

// DispatchTaskRequest 下发任务请求
type DispatchTaskRequest struct {
	TaskType  string     `json:"task_type" binding:"required"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	UserID    *uint64    `json:"user_id,string,omitempty"`
	Payload   string     `json:"payload"`
}

// CreateScheduleRequest 创建定时任务请求
type CreateScheduleRequest struct {
	Name     string `json:"name" binding:"required"`
	TaskType string `json:"task_type" binding:"required"`
	Cron     string `json:"cron" binding:"required"`
	Payload  string `json:"payload"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

// UpdateScheduleRequest 修改定时任务请求
type UpdateScheduleRequest struct {
	Name     string `json:"name" binding:"required"`
	TaskType string `json:"task_type" binding:"required"`
	Cron     string `json:"cron" binding:"required"`
	Payload  string `json:"payload"`
	IsActive *bool  `json:"is_active" binding:"required"`
}

// ListTaskExecutionsRequest 分页查询任务执行记录请求参数
type ListTaskExecutionsRequest struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"page_size"`
	Status         string `form:"status"`
	TaskType       string `form:"task_type"`
	TaskTypes      string `form:"task_types"`
	TaskTypePrefix string `form:"task_type_prefix"`
}

// ListUsersRequest 用户列表查询请求
type ListUsersRequest struct {
	Page     int     `form:"page" binding:"min=1"`
	PageSize int     `form:"page_size" binding:"min=1,max=100"`
	UserID   *uint64 `form:"user_id" binding:"omitempty,gt=0"`
	Username string  `form:"username"`
	Email    string  `form:"email"`
}

// UserResponse 用户资料响应
type UserResponse struct {
	ID          uint64    `json:"id,string"`
	Username    string    `json:"username"`
	Nickname    string    `json:"nickname"`
	Email       string    `json:"email"`
	AvatarURL   string    `json:"avatar_url"`
	IsActive    bool      `json:"is_active"`
	IsAdmin     bool      `json:"is_admin"`
	Bio         string    `json:"bio"`
	Phone       string    `json:"phone"`
	Gender      string    `json:"gender"`
	Website     string    `json:"website"`
	Location    string    `json:"location"`
	LastLoginAt time.Time `json:"last_login_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListUsersResponse 用户列表响应
type ListUsersResponse struct {
	Users []UserResponse `json:"users"`
	Total int64          `json:"total"`
}

// UpdateUserStatusRequest 更新用户状态请求
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	Nickname string `json:"nickname" binding:"omitempty,max=64"`
	Email    string `json:"email" binding:"required,email,max=255"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
}

// UpdateUserRequest 更新用户信息请求
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"max=64"`
	Email    string `json:"email" binding:"required,email,max=255"`
	IsAdmin  bool   `json:"is_admin"`
	Password string `json:"password" binding:"omitempty,min=8,max=64"`
}

// LogsResponse 历史日志查询响应
type LogsResponse struct {
	Lines      []logger.LogEntry `json:"lines"`
	HasMore    bool              `json:"has_more"`
	NextCursor int               `json:"next_cursor"` // 用于加载更早日志的 cursor
}

// AccessLogItem 访问日志单条数据
type AccessLogItem struct {
	ID        uint64 `json:"id,string"`
	TraceID   string `json:"trace_id"`
	UserID    uint64 `json:"user_id,string"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Path      string `json:"path"`
	Method    string `json:"method"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Headers   string `json:"headers"`
	Status    int32  `json:"status"`
	Latency   int64  `json:"latency"`
	CreatedAt string `json:"created_at"`
}

// AccessLogsResponse 访问日志查询响应
type AccessLogsResponse struct {
	Total uint64          `json:"total"`
	List  []AccessLogItem `json:"list"`
}

// TrendItem 趋势图数据点
type TrendItem struct {
	Date  string `json:"date"`
	Count uint64 `json:"count"`
}

// BrowserItem 浏览器占比排行
type BrowserItem struct {
	Browser string `json:"browser"`
	Count   uint64 `json:"count"`
}

// TopUserItem 活跃用户数据
type TopUserItem struct {
	UserID   uint64 `json:"user_id,string"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Count    uint64 `json:"count"`
}

// LogsAnalyticsResponse 访问日志数据分析结果
type LogsAnalyticsResponse struct {
	Trend    []TrendItem   `json:"trend"`
	Browsers []BrowserItem `json:"browsers"`
	TopUsers []TopUserItem `json:"top_users"`
}

// AccessLogQuery carries the raw console filters before they become a contract filter.
type AccessLogQuery struct {
	Username  string
	Path      string
	StartTime string
	EndTime   string
	Page      int
	PageSize  int
}

// UpdaterStatus describes the current build and the newest compatible upstream release.
type UpdaterStatus struct {
	CurrentVersion     string `json:"current_version"`
	BuildTime          string `json:"build_time"`
	LatestVersion      string `json:"latest_version"`
	UpdateAvailable    bool   `json:"update_available"`
	CanUpgrade         bool   `json:"can_upgrade"`
	Prerelease         bool   `json:"prerelease"`
	ReleaseName        string `json:"release_name"`
	ReleaseNotes       string `json:"release_notes"`
	ReleaseURL         string `json:"release_url"`
	PublishedAt        string `json:"published_at"`
	UpstreamRepository string `json:"upstream_repository"`
	AssetName          string `json:"asset_name"`
	Platform           string `json:"platform"`
}
