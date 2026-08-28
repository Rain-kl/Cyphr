// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"time"
)

// Session and Context Keys
const (
	UserNameKey     = "username"
	UserIDKey       = "user_id"
	UserObjKey      = "user_obj"
	TokenAuthKey    = "token_auth"          // 标记当前请求是否通过 Access Token 鉴权
	TokenAdminKey   = "token_admin"         // Access Token 本身是否具有管理员权限
	SessionTokenKey = "oauth_session_token" //nolint:gosec // false positive: this is a session key, not hardcoded credentials
	PasswordHashKey = "password_hash"
	SystemUsername  = "system"
)

// OAuth State Cache Keys and Expirations
const (
	OAuthStateCacheKeyFormat     = "oauth:state:%s"
	OAuthStateCacheKeyExpiration = 10 * time.Minute
	oauthStateLimitKeyFormat     = "oauth:state:limit:%s"
	oauthStateLimitMax           = 10
)

// OAuth Purpose Constants
const (
	OAuthPurposeLogin = "login"
	OAuthPurposeBind  = "bind"
)

// Auth Source Types
const (
	AuthSourceTypeOIDC = "oidc"
)
