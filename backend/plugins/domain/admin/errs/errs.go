// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package errs defines error constants, sentinels, and error helpers for the admin domain.
package errs

import (
	"errors"
	"strings"
)

// 管理后台公共错误常量
const (
	AdminRequired          = "未经授权访问"
	TokenAdminRequired     = "该访问令牌没有管理员权限，无法访问管理端点" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	InvalidAuthSourceID    = "认证源 ID 无效"
	ErrInvalidAuthSourceID = "无效的认证源 ID"
	InvalidCursorParam     = "无效的 cursor 参数"
	InvalidTaskExecutionID = "无效的任务执行记录 ID"
	InvalidParams          = "无效的参数"
	InternalServerError    = "内部服务器错误"
	InvalidScheduleID      = "无效的定时任务ID"
)

// 依赖服务未就绪错误常量
const (
	DatabaseNotInitialized         = "数据库未初始化"
	ErrDatabaseServiceNotAvailable = "database service not available"
	ErrDatabaseNotInitialized      = "database not initialized"
	ErrCacheServiceNotInitialized  = "cache service is not initialized"
	UserServiceUnavailable         = "用户服务未就绪"
	AuthServiceUnavailable         = "认证服务未就绪"
	TaskServiceUnavailable         = "task service not available"
	LogStoreUnavailable            = "日志存储服务未初始化"
)

// 系统配置错误消息常量
const (
	SystemConfigNotFound                 = "系统配置不存在"
	ConfigKeyRequired                    = "配置键不能为空"
	ConfigValueRequired                  = "配置值不能为空"
	ConfigKeyExists                      = "配置键已存在"
	ProtectedConfigKeyMessage            = "该配置项由系统任务管理，禁止手动修改"
	StorageDriverSwitchRequiresMigration = "存在存量文件，请通过存储迁移任务切换存储引擎"
	ErrConfigIntParseFailed              = "配置 %s 的值 '%s' 无法转换为整数: %w"
	ErrConfigDecimalParseFailed          = "配置 %s 的值 '%s' 无法转换为decimal: %w"
	ErrConfigBoolParseFailed             = "配置 %s 的值 '%s' 无法转换为布尔值: %w"
	ErrParseMenuDisplayConfigFailed      = "解析目录显示配置失败: %w"
	ErrCheckExistingUploadsFailed        = "检查存量文件失败: %w"
	ErrParseCurrentStorageConfigFailed   = "解析当前存储配置失败: %w"
	ErrParseTargetStorageConfigFailed    = "解析目标存储配置失败: %w"
	ErrSerializeStorageConfigFailed      = "序列化存储配置失败: %w"
	ErrAutoResolveMigrationTaskFailed    = "自动更新迁移任务状态失败: %v"
	StorageMigrationTaskType             = "storage:migrate"
	StorageDriverResolvedResult          = "存储配置直接更新，故障迁移任务自动标记为已解决"
)

// 存储配置校验错误前缀，用于区分参数校验失败与内部错误。
var storageValidationErrPrefixes = []string{
	"解析", "验证", "初始化测试", "存储连通性", "序列化", "检查存量文件",
}

// 模板管理相关错误消息常量
const (
	TemplateNotFound              = "模板不存在"
	TemplateKeyRequired           = "模板标识符不能为空"
	TemplateNameRequired          = "模板名称不能为空"
	TemplateContentRequired       = "模板内容不能为空"
	TemplateKeyExists             = "模板标识符已存在"
	SystemTemplateCannotDelete    = "系统预置模板不可删除"
	SystemTemplateCannotModifyKey = "系统预置模板不可修改标识符"
)

// 任务调度相关错误消息常量
const (
	InvalidTaskType       = "无效的任务类型"
	InvalidTimeRange      = "无效的时间范围"
	TaskDispatchFailed    = "任务下发失败"
	UserIDRequired        = "用户ID必填"
	TaskNotFound          = "任务执行记录不存在"
	TaskNotRetryable      = "该任务不支持重试"
	TaskNotFailed         = "只有失败的任务才能重试"
	TaskMaxRetryExceeded  = "已达到最大重试次数"
	TaskRetryFailed       = "任务重试失败"
	ScheduleSaveFailed    = "保存定时任务失败"
	ScheduleDeleteFailed  = "删除定时任务失败"
	InvalidCronExpression = "无效的 Cron 表达式"
	ScheduleNotFound      = "定时任务不存在"
	// 任务契约实现返回的远端错误文案，用于状态码归类。
	RemoteTaskNotFoundMsg     = "不存在"
	RemoteTaskNotFailedMsg    = "只有失败的任务"
	RemoteTaskNotRetryableMsg = "不支持重试"
	RemoteTaskMaxRetryMsg     = "已达到最大重试"
)

// 数据库管理相关错误消息常量
const (
	InvalidSQLStatement           = "SQL 语句不能为空"
	ErrOpenDatabaseFileFailed     = "无法打开数据库文件"
	ErrReadDatabaseFileInfoFailed = "无法读取数据库文件信息"
	ErrPgDumpUnavailable          = "pg_dump 不可用，请确保服务器已安装 PostgreSQL 客户端工具"
)

// 访问日志相关错误消息常量
const (
	ErrQueryUserFailed        = "查询用户信息失败: %w"
	ErrQueryAccessTrendFailed = "查询访问趋势失败: "
)

// 日志库切换相关错误消息常量
const (
	ErrReadLogDatabaseFailed   = "读取日志主库失败: %w"
	ErrLogDatabaseEmpty        = "日志主库配置为空"
	ErrSameLogTarget           = "目标日志库与当前日志库相同，无需迁移"
	ErrClickHouseNotEnabled    = "ClickHouse 未启用，无法迁移到 ClickHouse"
	ErrPostgresNotEnabled      = "PostgreSQL 未启用（当前主库为 SQLite），无法迁移到 PostgreSQL"
	ErrSQLiteNotAllowedAsLogDB = "当前主库为 PostgreSQL，日志库不能设置为 SQLite"
)

// 应用更新相关错误消息常量
const (
	ErrInvalidRepository           = "上游仓库地址无效"
	ErrReleaseRequestFailed        = "获取上游版本失败"
	ErrReleaseResponseInvalid      = "上游版本响应无效"
	ErrNoCompatibleRelease         = "未找到兼容的 Release"
	ErrNoCompatibleAsset           = "未找到当前系统对应的 Release 资产"
	ErrDevelopmentBuild            = "开发版本无法执行自动升级"
	ErrAlreadyUpToDate             = "当前已是最新版本"
	ErrUpgradeAlreadyRunning       = "已有升级任务正在执行"
	ErrAutomaticUpgradeBlocked     = "当前平台暂不支持自动替换二进制"
	ErrReleaseAssetSizeInvalid     = "release 资产大小无效: %d"
	ErrCreateUpgradeRequestFailed  = "创建升级下载请求失败: %w"
	ErrDownloadUpgradeAssetFailed  = "下载升级资产失败: %w"
	ErrUpgradeAssetHTTPFailed      = "下载升级资产失败: HTTP %d"
	ErrCreateUpgradeArchiveFailed  = "创建升级归档失败: %w"
	ErrWriteUpgradeArchiveFailed   = "写入升级归档失败: %w"
	ErrCloseUpgradeArchiveFailed   = "关闭升级归档失败: %w"
	ErrUpgradeArchiveSizeMismatch  = "升级归档大小不匹配: got %d, want %d"
	ErrArchiveContainsIllegalPath  = "归档包含非法路径: %s"
	ErrArchivePathOutOfDestination = "归档路径越界: %s"
	ErrExtractedBinaryTooLarge     = "解压后的程序文件超过大小限制"
	ErrLocateExecutableFailed      = "定位当前程序失败: %w"
	ErrResolveExecutablePathFailed = "解析当前程序路径失败: %w"
	ErrCreateUpgradeDirFailed      = "创建升级目录失败: %w"
	ErrExtractUpgradeAssetFailed   = "解压升级资产失败: %w"
)

// 用户管理（管理员视角）错误消息常量
const (
	UserNotFound          = "用户不存在"
	CannotDisable         = "不能禁用管理员账号"
	CannotDelete          = "不能删除管理员账号"
	CannotDeleteSelf      = "不能删除当前登录账号"
	UsernameRequired      = "用户名不能为空"
	EmailRequired         = "邮箱不能为空"
	PasswordTooShort      = "密码长度不能少于 8 位" //nolint:gosec // error message, not hardcoded credentials
	UsernameExists        = "用户名已存在"
	EmailExists           = "邮箱已被使用"
	CannotRevokeSelfAdmin = "不能取消自身的管理员权限"
	UpdateUserFailed      = "更新用户状态失败"
	DeleteUserFailed      = "删除用户失败"
	UpdateUserInfoFailed  = "更新用户信息失败"
	ListAdminUsersFailed  = "获取用户列表失败"
)

// 认证源管理相关错误消息常量
const (
	ListAuthSourcesFailed  = "获取认证源列表失败"
	CreateAuthSourceFailed = "创建认证源失败: "
	ToggleAuthSourceFailed = "切换认证源状态失败: "
	DeleteAuthSourceFailed = "删除认证源失败: "
)

// 层边界哨兵错误：Service/Repository 层返回、Handler 层据以选择信封状态码。
var (
	// ErrDatabaseUninitialized 表示数据库服务尚未注入。
	ErrDatabaseUninitialized = errors.New(DatabaseNotInitialized)
	// ErrSystemConfigNotFound 表示系统配置键不存在。
	ErrSystemConfigNotFound = errors.New(SystemConfigNotFound)
	// ErrConfigKeyExists 表示系统配置键已存在。
	ErrConfigKeyExists = errors.New(ConfigKeyExists)
	// ErrProtectedConfigKey 表示配置键由系统任务托管，禁止手动修改。
	ErrProtectedConfigKey = errors.New(ProtectedConfigKeyMessage)
	// ErrTemplateNotFound 表示模板标识符不存在。
	ErrTemplateNotFound = errors.New(TemplateNotFound)
	// ErrTemplateKeyExists 表示模板标识符已被占用。
	ErrTemplateKeyExists = errors.New(TemplateKeyExists)
	// ErrSystemTemplateCannotDelete 表示系统预置模板不可删除。
	ErrSystemTemplateCannotDelete = errors.New(SystemTemplateCannotDelete)
	// ErrUserNotFound 表示目标用户不存在。
	ErrUserNotFound = errors.New(UserNotFound)
	// ErrUserServiceUnavailable 表示用户契约服务尚未注入。
	ErrUserServiceUnavailable = errors.New(UserServiceUnavailable)
	// ErrAuthServiceUnavailable 表示认证契约服务尚未注入。
	ErrAuthServiceUnavailable = errors.New(AuthServiceUnavailable)
	// ErrTaskServiceUnavailable 表示任务契约服务尚未注入。
	ErrTaskServiceUnavailable = errors.New(TaskServiceUnavailable)
	// ErrLogStoreUnavailable 表示日志分析契约服务尚未注入。
	ErrLogStoreUnavailable = errors.New(LogStoreUnavailable)
	// ErrScheduleNotFound 表示定时任务不存在。
	ErrScheduleNotFound = errors.New(ScheduleNotFound)
	// ErrInvalidCronExpression 表示 Cron 表达式无法解析。
	ErrInvalidCronExpression = errors.New(InvalidCronExpression)
	// ErrInvalidTaskType 表示任务类型未在任务注册表中声明。
	ErrInvalidTaskType = errors.New(InvalidTaskType)
)

// InvalidInputError marks a failure caused by caller supplied content (an unusable SQL
// statement, a rejected task payload, ...). It carries no HTTP semantics; the handler
// layer decides how such errors surface to the client.
type InvalidInputError struct {
	Msg string
}

func (e *InvalidInputError) Error() string { return e.Msg }

// NewInvalidInputError builds an invalid input failure preserving the original message.
func NewInvalidInputError(msg string) error {
	return &InvalidInputError{Msg: msg}
}

// AsInvalidInput reports whether err was caused by rejected caller input.
func AsInvalidInput(err error) (string, bool) {
	var target *InvalidInputError
	if errors.As(err, &target) {
		return target.Msg, true
	}
	return "", false
}

// IsStorageConfigValidationError 判定错误是否属于存储配置参数校验失败。
func IsStorageConfigValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if msg == StorageDriverSwitchRequiresMigration {
		return true
	}
	for _, prefix := range storageValidationErrPrefixes {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}
