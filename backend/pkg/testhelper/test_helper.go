// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package testhelper 提供测试辅助工具
package testhelper

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"gorm.io/gorm"

	cachepkg "Wavelet/plugins/infra/cache"
	db "Wavelet/plugins/infra/database"
)

// SystemConfig 测试用系统配置表
type SystemConfig struct {
	Key         string `gorm:"primaryKey;size:64;not null"`
	Value       string `gorm:"type:text;not null"`
	Type        string `gorm:"size:32;not null"`
	Visibility  string `gorm:"size:32;not null;default:'hidden'"`
	Description string `gorm:"size:255"`
}

// TableName 返回测试配置表表名
func (SystemConfig) TableName() string {
	return "w_system_configs"
}

type userHelper struct {
	ID                 uint64 `gorm:"primaryKey;autoIncrement"`
	Username           string `gorm:"size:64;uniqueIndex;not null"`
	Nickname           string `gorm:"size:64;not null;default:''"`
	Password           string `gorm:"size:255;not null;default:''"`
	Email              string `gorm:"size:128;index;default:''"`
	AvatarURL          string `gorm:"size:255;default:''"`
	IsAdmin            bool   `gorm:"default:false;not null"`
	IsActive           bool   `gorm:"default:true;not null"`
	NeedChangePassword bool   `gorm:"default:false;not null"`
	Bio                string `gorm:"size:500;default:''"`
	Phone              string `gorm:"size:32;default:''"`
	Gender             string `gorm:"size:16;default:''"`
	Website            string `gorm:"size:255;default:''"`
	Location           string `gorm:"size:255;default:''"`
	LastLoginAt        time.Time
	CreatedAt          time.Time `gorm:"autoCreateTime"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"`
}

func (userHelper) TableName() string { return "w_users" }

type accessTokenHelper struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `gorm:"not null;index"`
	Name        string    `gorm:"size:64;not null"`
	TokenHash   string    `gorm:"size:64;uniqueIndex;not null"`
	MaskedToken string    `gorm:"size:32;not null"`
	IsAdmin     bool      `gorm:"default:false;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (accessTokenHelper) TableName() string { return "w_access_tokens" }

type authSourceHelper struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Type      string    `gorm:"size:32;not null"`
	Name      string    `gorm:"size:64;not null"`
	Enabled   bool      `gorm:"default:false;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (authSourceHelper) TableName() string { return "w_auth_sources" }

type externalAccountHelper struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	UserID         uint64    `gorm:"not null;index"`
	AuthSourceType string    `gorm:"size:32;not null;index"`
	ExternalID     string    `gorm:"size:128;not null;index"`
	Username       string    `gorm:"size:128;default:''"`
	Email          string    `gorm:"size:128;default:''"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (externalAccountHelper) TableName() string { return "w_external_accounts" }

type uploadHelper struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	UserID     uint64    `gorm:"not null;index"`
	FileName   string    `gorm:"size:255;not null"`
	FilePath   string    `gorm:"size:500;not null"`
	FileSize   int64     `gorm:"not null"`
	MimeType   string    `gorm:"size:128;not null"`
	Extension  string    `gorm:"size:32;not null"`
	Hash       string    `gorm:"size:64;index;not null;default:''"`
	Type       string    `gorm:"size:50;not null;index"`
	Status     string    `gorm:"size:20;not null;default:'pending'"`
	AccessMode int       `gorm:"not null;default:0"`
	Metadata   string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
}

func (uploadHelper) TableName() string { return "w_uploads" }

type uploadStatHelper struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Dimension string    `gorm:"size:32;not null;uniqueIndex:idx_stat_dimension_key"`
	StatKey   string    `gorm:"size:100;not null;uniqueIndex:idx_stat_dimension_key"`
	FileCount int64     `gorm:"not null;default:0"`
	FileSize  int64     `gorm:"not null;default:0"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (uploadStatHelper) TableName() string { return "w_upload_stats" }

type messageChannelHelper struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Type        string `gorm:"size:32;not null"`
	Name        string `gorm:"size:64;not null"`
	OwnerScope  string `gorm:"size:32;not null;default:'system'"`
	OwnerID     *uint64
	Credentials string    `gorm:"type:text;not null"`
	Extra       string    `gorm:"type:text"`
	Enabled     bool      `gorm:"default:false;not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (messageChannelHelper) TableName() string { return "w_message_channels" }

type messageBindingHelper struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	ChannelID      uint64    `gorm:"not null;index"`
	PlatformUserID string    `gorm:"size:128;not null;index"`
	UserID         uint64    `gorm:"not null;index"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (messageBindingHelper) TableName() string { return "w_message_bindings" }

type messagePairingHelper struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement"`
	Code           string    `gorm:"size:32;uniqueIndex;not null"`
	ChannelID      uint64    `gorm:"not null;index"`
	PlatformUserID string    `gorm:"size:128;not null;index"`
	UserID         uint64    `gorm:"not null;index"`
	ExpiresAt      time.Time `gorm:"not null;index"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (messagePairingHelper) TableName() string { return "w_message_pairing_codes" }

type taskExecutionHelper struct {
	ID           uint64     `gorm:"primaryKey;autoIncrement"`
	TaskID       string     `gorm:"size:128;uniqueIndex;not null"`
	TaskType     string     `gorm:"size:64;index;not null"`
	TaskName     string     `gorm:"size:128"`
	Status       string     `gorm:"size:32;index;not null"`
	Retryable    bool       `gorm:"not null;default:false"`
	MaxRetry     int        `gorm:"not null;default:0"`
	RetryCount   int        `gorm:"not null;default:0"`
	Log          string     `gorm:"type:text"`
	ErrorMessage string     `gorm:"type:text"`
	Result       string     `gorm:"type:text"`
	StartedAt    *time.Time `gorm:"index"`
	FinishedAt   *time.Time
	Duration     int64
	Payload      string    `gorm:"type:text"`
	TriggeredBy  string    `gorm:"size:32;not null;default:system"`
	CreatedAt    time.Time `gorm:"autoCreateTime;index"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (taskExecutionHelper) TableName() string {
	return "w_task_executions"
}

type scheduleHelper struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	TaskType  string    `gorm:"size:64;uniqueIndex;not null"`
	TaskName  string    `gorm:"size:128;not null"`
	CronExpr  string    `gorm:"size:64;not null"`
	Payload   string    `gorm:"type:text"`
	Enabled   bool      `gorm:"default:true;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (scheduleHelper) TableName() string { return "w_schedules" }

const (
	configTypeSystem   = "system"
	configTypeBusiness = "business"
	configValueTrue    = "true"
	configValueFalse   = "false"
)

// SetupTestEnvironment initializes an in-memory SQLite DB, seeds default configurations,
// starts miniredis, and overrides the global db/Redis clients. It returns a cleanup function.
func SetupTestEnvironment(t *testing.T) (*gorm.DB, *miniredis.Miniredis, func()) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite db: %v", err)
	}

	if sqlDB, err := sqliteDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	// AutoMigrate all tables via internal test helpers to completely decouple testhelper from domain plugins
	err = sqliteDB.AutoMigrate(
		&userHelper{},
		&accessTokenHelper{},
		&authSourceHelper{},
		&externalAccountHelper{},
		&SystemConfig{},
		&uploadHelper{},
		&uploadStatHelper{},
		&taskExecutionHelper{},
		&scheduleHelper{},
		&messageChannelHelper{},
		&messageBindingHelper{},
		&messagePairingHelper{},
	)
	if err != nil {
		t.Fatalf("failed to auto migrate tables: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	db.SetDB(sqliteDB)
	cachepkg.Redis = redisClient

	seedDefaultConfigs(t, sqliteDB)

	cleanup := func() {
		runExtraCleanups()
		_ = redisClient.Close()
		mr.Close()
		db.SetDB(nil)
		cachepkg.Redis = nil
	}

	return sqliteDB, mr, cleanup
}

func getSeedConfigsPart1() []SystemConfig {
	return []SystemConfig{
		{Key: "upload_allowed_extensions", Value: `["jpg", "jpeg", "png", "gif", "webp", "txt", "pdf", "zip"]`, Type: configTypeSystem, Description: "允许上传的文件扩展名列表（JSON 字符串数组）"},
		{Key: "site_name", Value: "Wavelet", Type: configTypeSystem, Description: "站点名称"},
		{Key: "site_description", Value: "Lightweight and Modular Web Application Platform", Type: configTypeSystem, Description: "站点描述"},
		{Key: "password_login_enabled", Value: configValueTrue, Type: configTypeSystem, Description: "是否开启账号密码登录（true/false）"},
		{Key: "registration_enabled", Value: configValueTrue, Type: configTypeSystem, Description: "是否允许新用户注册（全局总开关，true/false）"},
		{Key: "password_register_enabled", Value: configValueTrue, Type: configTypeSystem, Description: "是否允许账号密码注册（true/false）"},
		{Key: "oidc_login_enabled", Value: configValueFalse, Type: configTypeSystem, Description: "是否开启 OIDC 登录（true/false）"},
		{Key: "server_address", Value: "http://localhost:8000", Type: configTypeSystem, Description: "服务端访问地址（用于生成绝对路径链接，多个地址用英文逗号分隔）"},
		{Key: "smtp_host", Value: "", Type: configTypeSystem, Description: "SMTP 服务器主机名或 IP"},
		{Key: "smtp_port", Value: "587", Type: configTypeSystem, Description: "SMTP 服务器端口（标准 STARTTLS 为 587，SMTPS 为 465）"},
		{Key: "smtp_username", Value: "", Type: configTypeSystem, Description: "SMTP 账户（如 sender@example.com）"},
		{Key: "smtp_password", Value: "", Type: configTypeSystem, Description: "SMTP 访问凭证（授权码/密码）"},
		{Key: "email_login_verification_enabled", Value: configValueFalse, Type: configTypeSystem, Description: "是否开启邮箱登录验证（true/false）"},
		{Key: "email_register_verification_enabled", Value: configValueFalse, Type: configTypeSystem, Description: "是否开启邮箱注册验证（true/false）"},
		{Key: "menu_display_config", Value: "{}", Type: configTypeSystem, Description: "目录显示配置（JSON 字符串，格式为 {url: enabled}）"},
		{Key: "search_engine_indexing_enabled", Value: configValueFalse, Type: configTypeSystem, Description: "是否允许搜索引擎检索"},
		{Key: "file_access_whitelist", Value: `["avatar"]`, Type: configTypeSystem, Description: "免登录访问的文件业务类型白名单"},
		{Key: "disk_cache_max_size_mb", Value: "100", Type: configTypeSystem, Description: "磁盘缓存最大空间大小 (MB)"},
		{Key: "disk_cache_ttl_minutes", Value: "60", Type: configTypeSystem, Description: "磁盘缓存默认有效期 (分钟)"},
		{Key: "disk_cache_lru_enabled", Value: configValueTrue, Type: configTypeSystem, Description: "是否启用 LRU 淘汰机制"},
		{Key: "login_session_ttl_hours", Value: "0", Type: configTypeSystem, Description: "登录会话过期时间 (小时，0表示浏览器关闭后自动退出，-1表示永不过期)"},
		{Key: "update_upstream_repository", Value: "Rain-kl/Wavelet", Type: configTypeSystem, Description: "GitHub Actions Release 上游仓库"},
		{Key: "storage_config", Value: `{"driver":"local","local":{"root":"."},"s3":{"region":"us-east-1"},"r2":{"region":"auto"},"minio":{"region":"us-east-1","path_style":true},"oss":{},"webdav":{}}`, Type: configTypeSystem, Description: "文件存储驱动及连接配置（JSON）"},
		{Key: "log_database", Value: "sqlite", Type: configTypeSystem, Description: "当前日志主库"},
		{Key: "log_db_migration", Value: "", Type: configTypeSystem, Description: "日志库迁移冻结标记"},
		{Key: "log_retention_days_postgres", Value: "30", Type: configTypeBusiness, Description: "PostgreSQL 用户访问日志保留天数"},
		{Key: "log_retention_days_sqlite", Value: "30", Type: configTypeBusiness, Description: "SQLite 用户访问日志保留天数"},
		{Key: "log_retention_days_clickhouse", Value: "30", Type: configTypeBusiness, Description: "ClickHouse 用户访问日志保留天数"},
	}
}

func seedDefaultConfigs(t *testing.T, tx *gorm.DB) {
	defaultConfigs := getSeedConfigsPart1()

	if err := tx.Create(&defaultConfigs).Error; err != nil {
		t.Fatalf("failed to seed default system configs: %v", err)
	}

	publicKeys := map[string]struct{}{
		"upload_allowed_extensions": {},
		"site_name":                 {},
		"password_login_enabled":    {},
		"registration_enabled":      {},
		"password_register_enabled": {},
		"oidc_login_enabled":        {},
	}

	keys := make([]string, 0, len(publicKeys))
	for key := range publicKeys {
		keys = append(keys, key)
	}
	if err := tx.Model(&SystemConfig{}).
		Where("key IN ?", keys).
		Update("visibility", "visible").Error; err != nil {
		t.Fatalf("failed to seed public system config visibility: %v", err)
	}

	for _, config := range defaultConfigs {
		if _, ok := publicKeys[config.Key]; ok {
			config.Visibility = "visible"
		}
		_ = cachepkg.HSetJSON(context.Background(), "system_configs", config.Key, &config)
	}
}
