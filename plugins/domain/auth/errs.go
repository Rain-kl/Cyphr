// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

// OAuth and Auth error messages
const (
	errInvalidState                         = "非法登录请求"
	errIDTokenVerifyFailed                  = "ID Token 验证失败" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errIDTokenVerifyFailedFormat            = "%s: %w"
	errNonceMismatch                        = "nonce 不匹配，可能存在重放攻击"
	errNoActiveAuthSource                   = "未配置可用认证源"
	errServerAddressMissing                 = "服务器地址 (server_address) 未配置或配置为空，请在后台系统设置中配置后再试"
	errAuthSourceRequired                   = "认证源不能为空"
	errDiscoveryURLRequired                 = "OIDC 认证源必须配置 Discovery URL"
	errUsernameGenerateFailed               = "无法生成可用用户名"
	errUsernameFromSourceFailed             = "无法从认证源获取用户名"
	errAuthSourceDisabled                   = "认证源未启用"
	errInvalidExternalAccountBindingID      = "绑定记录 ID 无效"
	ErrTokenAuthNotAllowed                  = "该端点不允许使用访问令牌进行身份验证" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errOAuthStateRateLimited                = "请求授权过于频繁，请稍后重试"
	errAuthSourceNameRequired               = "认证源名称不能为空"
	errAuthSourceNameInvalid                = "认证源名称格式不正确"
	errAuthSourceTypeUnsupported            = "不支持的认证源类型"
	errAuthSourceDiscoveryURLRequired       = "Discovery URL 不能为空"
	//nolint:gosec // error message, not hardcoded credentials
	errAuthSourceClientCredentialsRequired  = "启用认证源时必须配置 Client ID 和 Client Secret"
	errAuthSourceIDRequired                 = "认证源 ID 不能为空"
	errUserIDRequired                       = "用户 ID 不能为空"
	errExternalAccountBindingIncomplete     = "外部帐号绑定信息不完整"
	errExternalAccountAlreadyBoundToAnother = "该外部帐号已被其他用户绑定"
	errExternalAccountBindingIDRequired     = "外部帐号绑定记录 ID 不能为空"
	errAdminRequired                        = "无权访问"
	//nolint:gosec // error message, not hardcoded credentials
	errTokenAdminRequired                   = "令牌无管理员权限"
)
