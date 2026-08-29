// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"Wavelet/pkg/trace"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	whitelistMu sync.RWMutex
	whitelist   []string
)

// RegisterWhitelist registers route patterns that bypass mandatory authentication.
func RegisterWhitelist(patterns ...string) {
	whitelistMu.Lock()
	defer whitelistMu.Unlock()
	whitelist = append(whitelist, patterns...)
}

// IsWhitelisted checks if the specified path matches the auth whitelist.
func IsWhitelisted(path string) bool {
	whitelistMu.RLock()
	defer whitelistMu.RUnlock()
	for _, pattern := range whitelist {
		if extpoints.MatchPathPattern(pattern, path) {
			return true
		}
	}
	return false
}

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// currentUserIDFromRequestContext 是接入层向 Service 层暴露的登录态桥接。
//
// Session 读取必须依赖 *gin.Context，而 Service 层禁止 import gin，
// 因此该类型断言收敛在本（接入层）文件中。ok 为 false 表示 ctx 不是 *gin.Context。
func currentUserIDFromRequestContext(ctx context.Context) (uint64, bool) {
	ginCtx, ok := ctx.(*gin.Context)
	if !ok {
		return 0, false
	}
	return GetUserIDFromContext(ginCtx), true
}

func getUserByToken(ctx context.Context, tokenStr string) (*contracts.UserDTO, *CachedToken, error) {
	tokenHash := hashToken(tokenStr)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err != nil || tokenRecord == nil {
		tokenRecord, err = GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, nil, err
		}
		SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || user == nil || !user.IsActive {
		user, err = GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, nil, err
		}
		SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	return user, tokenRecord, nil
}

// GetUserFromRequest 从请求中获取当前用户（优先 Access Token，其次 Session）
func GetUserFromRequest(c *gin.Context) (*contracts.UserDTO, error) {
	ctx := c.Request.Context()
	var tokenStr string

	tokenFromQuery := c.Query("token")
	if tokenFromQuery != "" {
		tokenStr = tokenFromQuery
	} else {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}

	// 优先使用 Access Token 鉴权
	if tokenStr != "" {
		if user, tokenRecord, err := getUserByToken(ctx, tokenStr); err == nil {
			if user.Username == SystemUsername {
				return nil, errors.New(errSystemUserLoginNotAllowed)
			}
			ginutil.SetToContext(c, contracts.AuthTokenAuthKey, true)
			ginutil.SetToContext(c, contracts.AuthTokenAdminKey, tokenRecord.IsAdmin)
			return user, nil
		}
	}

	// 降级使用 Session 鉴权
	userID := GetUserIDFromContext(c)
	if userID <= 0 {
		return nil, errors.New(errUnauthorizedInternal)
	}

	user, err := GetCachedUser(ctx, userID)
	if err != nil || user == nil || !user.IsActive {
		user, err = GetActiveUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		SetCachedUser(ctx, userID, user)
	}

	ginutil.SetToContext(c, contracts.AuthTokenAuthKey, false)
	ginutil.SetToContext(c, contracts.AuthTokenAdminKey, false)

	if user.Username == "system" {
		return nil, errors.New(errSystemUserLoginNotAllowed)
	}

	return user, nil
}

// LoginRequired 返回登录鉴权中间件，校验 Access Token 或 Session
func LoginRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsWhitelisted(c.Request.URL.Path) {
			c.Next()
			return
		}

		_, span := trace.Start(c.Request.Context(), "LoginRequired")
		defer span.End()

		user, err := GetUserFromRequest(c)
		if err != nil {
			response.AbortUnauthorized(c, errUnAuthorized)
			return
		}

		LogForAudit(c.Request.Context(), user, c)
		ginutil.SetToContext(c, contracts.AuthUserObjKey, user)
		c.Next()
	}
}

// AdminRequired 校验管理员权限（支持 Session 和 Token 鉴权）
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := trace.Start(c.Request.Context(), "AdminRequired")
		defer span.End()

		user, err := GetUserFromRequest(c)
		if err != nil {
			response.AbortUnauthorized(c, errUnAuthorized)
			return
		}

		isTokenAuth, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAuthKey)
		isTokenAdmin, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAdminKey)

		// 如果是通过 Token 鉴权，要求该 Token 具备管理员权限或者用户本身为管理员
		if isTokenAuth && !isTokenAdmin && !user.IsAdmin {
			response.AbortNotFound(c, errTokenAdminRequired)
			return
		}

		// 如果是通过 Session 鉴权，直接检查用户的 is_admin 属性
		if !isTokenAuth && !user.IsAdmin {
			response.AbortNotFound(c, errAdminRequired)
			return
		}

		LogForAudit(c.Request.Context(), user, c)
		ginutil.SetToContext(c, contracts.AuthUserObjKey, user)
		c.Next()
	}
}

// LoginAdminRequired is an alias for AdminRequired.
func LoginAdminRequired() gin.HandlerFunc {
	return AdminRequired()
}

// DisallowTokenAuth 拒绝使用 Access Token 进行身份验证的请求访问该端点
func DisallowTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenAuth, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAuthKey); tokenAuth {
			response.AbortForbidden(c, ErrTokenAuthNotAllowed)
			return
		}
		c.Next()
	}
}
