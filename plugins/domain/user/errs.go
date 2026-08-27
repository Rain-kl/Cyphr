// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

const (
	errInvalidParams = "无效的请求参数"
	errUserNotFound  = "用户不存在"
	//nolint:gosec // error message, not hardcoded credentials
	errPasswordMismatch = "用户名或密码错误"
	//nolint:gosec // error message, not hardcoded credentials
	errOldPasswordIncorrect = "原密码不正确"
	//nolint:gosec // error message, not hardcoded credentials
	errTokenNotFound = "访问令牌不存在"
)
