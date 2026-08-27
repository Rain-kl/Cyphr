// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

// 管理后台公共错误常量
const (
	AdminRequired          = "未经授权访问"
	TokenAdminRequired     = "该访问令牌没有管理员权限，无法访问管理端点" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	InvalidAuthSourceID    = "认证源 ID 无效"
	InvalidCursorParam     = "无效的 cursor 参数"
	InvalidTaskExecutionID = "无效的任务执行记录 ID"
)

// 系统配置错误消息常量
const (
	SystemConfigNotFound                 = "系统配置不存在"
	ConfigKeyRequired                    = "配置键不能为空"
	ConfigValueRequired                  = "配置值不能为空"
	ConfigKeyExists                      = "配置键已存在"
	protectedConfigKeyMessage            = "该配置项由系统任务管理，禁止手动修改"
	StorageDriverSwitchRequiresMigration = "存在存量文件，请通过存储迁移任务切换存储引擎"
)

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
	InvalidCronExpression = "无效的 Cron 表达式"
	ScheduleNotFound      = "定时任务不存在"
	ScheduleSaveFailed    = "保存定时任务失败"
	ScheduleDeleteFailed  = "删除定时任务失败"
)

// 应用更新相关错误消息常量
const (
	errInvalidRepository       = "上游仓库地址无效"
	errReleaseRequestFailed    = "获取上游版本失败"
	errReleaseResponseInvalid  = "上游版本响应无效"
	errNoCompatibleRelease     = "未找到兼容的 Release"
	errNoCompatibleAsset       = "未找到当前系统对应的 Release 资产"
	errDevelopmentBuild        = "开发版本无法执行自动升级"
	errAlreadyUpToDate         = "当前已是最新版本"
	errUpgradeAlreadyRunning   = "已有升级任务正在执行"
	errAutomaticUpgradeBlocked = "当前平台暂不支持自动替换二进制"
)

// 用户管理（管理员视角）错误消息常量
const (
	userNotFound     = "用户不存在"
	cannotDisable    = "不能禁用管理员账号"
	cannotDelete     = "不能删除管理员账号"
	cannotDeleteSelf = "不能删除当前登录账号"
	usernameRequired = "用户名不能为空"
	emailRequired    = "邮箱不能为空"
	//nolint:gosec // error message, not hardcoded credentials
	passwordTooShort      = "密码长度不能少于 8 位"
	usernameExists        = "用户名已存在"
	emailExists           = "邮箱已被使用"
	cannotRevokeSelfAdmin = "不能取消自身的管理员权限"
	updateUserFailed      = "更新用户状态失败"
	deleteUserFailed      = "删除用户失败"
	updateUserInfoFailed  = "更新用户信息失败"
)
