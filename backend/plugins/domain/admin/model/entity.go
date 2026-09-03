// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package model contains database entities and data transfer objects for the admin domain.
package model

import (
	"Wavelet/plugins/domain/admin/errs"
	"bytes"
	"errors"
	"strings"
	"text/template"
	"time"
)

// 配置键常量 - 所有系统配置的 key 定义
const (
	ConfigKeyUploadAllowedExtensions          = "upload_allowed_extensions"           // 允许上传的文件扩展名，逗号分隔
	ConfigKeySiteName                         = "site_name"                           // 站点名称
	ConfigKeyPasswordLoginEnabled             = "password_login_enabled"              // 是否允许密码登录
	ConfigKeyRegistrationEnabled              = "registration_enabled"                // 是否允许注册
	ConfigKeyPasswordRegisterEnabled          = "password_register_enabled"           // 是否允许密码注册
	ConfigKeyOIDCLoginEnabled                 = "oidc_login_enabled"                  // 是否允许 OIDC 登录
	ConfigKeyMaxAPIKeysPerUser                = "max_api_keys_per_user"               //nolint:gosec // false positive: config key name, not credentials
	ConfigKeyCapLoginEnabled                  = "cap_login_enabled"                   // 是否启用登录人机验证
	ConfigKeyCapAutoSolve                     = "cap_auto_solve"                      // 打开页面后是否自动开始计算（false 则需用户手动点击）
	ConfigKeyCapChallengeCount                = "cap_challenge_count"                 // 客户端需求解的 PoW 难题总数，默认 1，推荐 1～5
	ConfigKeyCapChallengeSize                 = "cap_challenge_size"                  // 人机验证盐值长度
	ConfigKeyCapChallengeDifficulty           = "cap_challenge_difficulty"            // 人机验证 PoW 难度（目标前缀长度）
	ConfigKeyCapChallengeTTL                  = "cap_challenge_ttl_seconds"           // 人机验证难题有效时间（秒）
	ConfigKeyCapTokenTTL                      = "cap_token_ttl_seconds"               //nolint:gosec // false positive: config key name, not credentials
	ConfigKeyServerAddress                    = "server_address"                      // 服务器地址
	ConfigKeySMTPHost                         = "smtp_host"                           // SMTP 服务器地址
	ConfigKeySMTPPort                         = "smtp_port"                           // SMTP 端口
	ConfigKeySMTPUsername                     = "smtp_username"                       // SMTP 账户
	ConfigKeySMTPPassword                     = "smtp_password"                       // SMTP 访问凭证
	ConfigKeyEmailLoginVerificationEnabled    = "email_login_verification_enabled"    // 是否启用邮箱登录验证
	ConfigKeyEmailRegisterVerificationEnabled = "email_register_verification_enabled" // 是否启用邮箱注册验证
	ConfigKeyMenuDisplayConfig                = "menu_display_config"                 // 目录显示配置 (JSON 字符串)
	ConfigKeySearchEngineIndexingEnabled      = "search_engine_indexing_enabled"      // 是否允许搜索引擎检索
	ConfigKeyFileAccessWhitelist              = "file_access_whitelist"               // 免登录访问的文件业务类型白名单 (JSON 数组格式)
	ConfigKeyDiskCacheMaxSizeMB               = "disk_cache_max_size_mb"              // 磁盘缓存最大空间大小 (MB)
	ConfigKeyDiskCacheTTLMinutes              = "disk_cache_ttl_minutes"              // 磁盘缓存默认有效期 (分钟)
	ConfigKeyDiskCacheLRUEnabled              = "disk_cache_lru_enabled"              // 是否启用 LRU 淘汰机制
	ConfigKeyLoginSessionTTLHours             = "login_session_ttl_hours"             // 登录会话过期时间 (小时)
	ConfigKeyUpdateUpstreamRepository         = "update_upstream_repository"          // GitHub Actions Release 上游仓库
	ConfigKeyStorageConfig                    = "storage_config"                      // 文件存储配置 (JSON)
	ConfigKeyLogDatabase                      = "log_database"                        // 当前日志主库（postgres/sqlite/clickhouse），受保护
	ConfigKeyLogDBMigration                   = "log_db_migration"                    // 日志库迁移冻结标记（空/migrating），受保护
	ConfigKeyLogRetentionDaysPostgres         = "log_retention_days_postgres"         // PostgreSQL 用户访问日志保留天数
	ConfigKeyLogRetentionDaysSQLite           = "log_retention_days_sqlite"           // SQLite 用户访问日志保留天数
	ConfigKeyLogRetentionDaysClickHouse       = "log_retention_days_clickhouse"       // ClickHouse 用户访问日志保留天数
)

const (
	// ConfigVisibilityHidden 表示配置不通过公共配置接口暴露
	ConfigVisibilityHidden = 0
	// ConfigVisibilityVisible 表示配置通过公共配置接口暴露
	ConfigVisibilityVisible = 1
)

// SystemConfig 系统配置实体
type SystemConfig struct {
	Key         string    `json:"key" gorm:"primaryKey;size:64;not null"`
	Value       string    `json:"value" gorm:"type:text;not null"`
	Type        string    `json:"type" gorm:"size:32;not null;default:'system'"`
	Visibility  int       `json:"visibility" gorm:"not null;default:0"`
	Description string    `json:"description" gorm:"size:255"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (SystemConfig) TableName() string {
	return "w_system_configs"
}

// Template 邮件/消息模板实体
type Template struct {
	ID          uint64    `json:"id,string" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"uniqueIndex;size:80;not null"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Type        string    `json:"type" gorm:"size:20;not null;default:'email'"`
	Subject     string    `json:"subject" gorm:"size:255"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"size:255"`
	IsSystem    bool      `json:"is_system" gorm:"index;not null;default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 表名
func (Template) TableName() string {
	return "w_templates"
}

// TemplateTypeEmail 邮件模板类型
const TemplateTypeEmail = "email"

// Normalize 规范化模板字段
func (t *Template) Normalize() {
	t.Key = strings.TrimSpace(t.Key)
	t.Name = strings.TrimSpace(t.Name)
	t.Type = strings.ToLower(strings.TrimSpace(t.Type))
	t.Subject = strings.TrimSpace(t.Subject)
	t.Content = strings.TrimSpace(t.Content)
	t.Description = strings.TrimSpace(t.Description)
	if t.Type == "" {
		t.Type = TemplateTypeEmail
	}
}

// Validate 校验模板必填字段
func (t *Template) Validate() error {
	t.Normalize()
	if t.Key == "" {
		return errors.New(errs.TemplateKeyRequired)
	}
	if t.Name == "" {
		return errors.New(errs.TemplateNameRequired)
	}
	if t.Content == "" {
		return errors.New(errs.TemplateContentRequired)
	}
	return nil
}

// Render 渲染模板的 Subject 和 Content
func (t *Template) Render(data any) (string, string, error) {
	var subject string
	if t.Subject != "" {
		tmplSubject, err := template.New(t.Key + "_subject").Parse(t.Subject)
		if err != nil {
			return "", "", err
		}
		var subBuf bytes.Buffer
		if err := tmplSubject.Execute(&subBuf, data); err != nil {
			return "", "", err
		}
		subject = subBuf.String()
	}

	tmplContent, err := template.New(t.Key + "_content").Parse(t.Content)
	if err != nil {
		return "", "", err
	}
	var bodyBuf bytes.Buffer
	if err := tmplContent.Execute(&bodyBuf, data); err != nil {
		return "", "", err
	}

	return subject, bodyBuf.String(), nil
}

// Schedule 定时任务配置表
type Schedule struct {
	ID        uint64    `json:"id,string" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128;not null"`
	TaskType  string    `json:"task_type" gorm:"size:64;not null"`
	Cron      string    `json:"cron" gorm:"size:64;not null"`
	Payload   string    `json:"payload" gorm:"type:text"`
	IsActive  bool      `json:"is_active" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (Schedule) TableName() string {
	return "w_schedules"
}

// TaskExecutionStatus 任务执行状态
type TaskExecutionStatus string

// 任务执行状态
const (
	TaskExecutionStatusPending   TaskExecutionStatus = "pending"
	TaskExecutionStatusRunning   TaskExecutionStatus = "running"
	TaskExecutionStatusSucceeded TaskExecutionStatus = "succeeded"
	TaskExecutionStatusFailed    TaskExecutionStatus = "failed"
)

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID           uint64              `json:"id,string" gorm:"primaryKey"`
	TaskID       string              `json:"task_id" gorm:"size:128;uniqueIndex;not null"`
	TaskType     string              `json:"task_type" gorm:"size:64;index;not null"`
	TaskName     string              `json:"task_name" gorm:"size:128"`
	Status       TaskExecutionStatus `json:"status" gorm:"size:32;index;not null"`
	Retryable    bool                `json:"retryable" gorm:"not null;default:false"`
	MaxRetry     int                 `json:"max_retry" gorm:"not null;default:0"`
	RetryCount   int                 `json:"retry_count" gorm:"not null;default:0"`
	Log          string              `json:"log" gorm:"type:text"`
	ErrorMessage string              `json:"error_message" gorm:"type:text"`
	Result       string              `json:"result" gorm:"type:text"`
	StartedAt    *time.Time          `json:"started_at" gorm:"index"`
	FinishedAt   *time.Time          `json:"finished_at"`
	Duration     int64               `json:"duration" gorm:"comment:耗时毫秒"`
	Payload      string              `json:"payload" gorm:"type:text"`
	TriggeredBy  string              `json:"triggered_by" gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time           `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time           `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (TaskExecution) TableName() string {
	return "w_task_executions"
}

// TaskExecutionCleanupStats 任务日志清理结果统计
type TaskExecutionCleanupStats struct {
	HighFrequencyDeleted int64 `json:"high_frequency_deleted"`
	LowFrequencyDeleted  int64 `json:"low_frequency_deleted"`
}
