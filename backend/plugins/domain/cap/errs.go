// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cap 提供人机验证中间件
package cap

// HTTP 响应错误文案
const (
	errCapTokenMissing          = "验证码验证失败，缺少验证码凭证" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errCapTokenInvalidOrExpired = "验证码校验失败或已过期，请重试" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errCapNotConfigured         = "captcha is not configured"
	errChallengeGenerateFailed  = "生成验证难题失败，请稍后再试"
	errInvalidRequestParams     = "无效的参数"
	errSolutionVerifyFailed     = "校验验证解答失败，请稍后再试"
)

// Redeem 结果码，属于 redeem 响应 JSON 的对外契约取值，禁止改写取值
const (
	redeemErrInvalidToken     = "invalid_token"
	redeemErrNonceStoreFailed = "nonce_store_error"
	redeemErrAlreadyRedeemed  = "already_redeemed"
	redeemErrSettingsLoad     = "settings_load_error"
	redeemErrTokenStoreFailed = "token_store_error" //nolint:gosec // error code, not hardcoded credentials
)
