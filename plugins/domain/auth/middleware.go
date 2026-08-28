// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Rain-kl/Wavelet/core/contracts"
	db "github.com/Rain-kl/Wavelet/pkg/persistence"
	"github.com/Rain-kl/Wavelet/pkg/response"
	"github.com/Rain-kl/Wavelet/pkg/shared"
	otel_trace "github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/gin-gonic/gin"
)

// GetFromContext 从 Gin 请求上下文获取指定类型的值。
func GetFromContext[T any](c *gin.Context, key string) (T, bool) {
	value, exists := c.Get(key)
	if !exists {
		var zero T
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

// SetToContext 设置值到 Gin 请求上下文。
func SetToContext[T any](c *gin.Context, key string, value T) {
	c.Set(key, value)
}

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func getUserByToken(ctx context.Context, tokenStr string) (*contracts.UserDTO, *CachedToken, error) {
	tokenHash := hashToken(tokenStr)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err == nil {
		user, err := GetCachedUser(ctx, tokenRecord.UserID)
		if err == nil && user != nil && user.IsActive {
			return user, tokenRecord, nil
		}
	}

	var tokenRow struct {
		ID      uint64
		UserID  uint64
		IsAdmin bool
	}
	if err := db.DB(ctx).Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&tokenRow).Error; err != nil {
		return nil, nil, err
	}
	tokenRecord = &CachedToken{
		ID:      tokenRow.ID,
		UserID:  tokenRow.UserID,
		IsAdmin: tokenRow.IsAdmin,
	}
	SetCachedToken(ctx, tokenHash, tokenRecord)

	var userRow contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").Where("id = ? AND is_active = ?", tokenRow.UserID, true).First(&userRow).Error; err != nil {
		return nil, nil, err
	}
	SetCachedUser(ctx, userRow.ID, &userRow)
	return &userRow, tokenRecord, nil
}

// GetUserFromRequest 校验 Access Token 或 Session 并返回用户对象，如果未登录或用户失效则返回 error
func GetUserFromRequest(c *gin.Context) (*contracts.UserDTO, error) {
	ctx := c.Request.Context()

	// Check token in headers
	tokenStr := c.GetHeader("X-Access-Token")
	if tokenStr == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}

	// 优先使用 Access Token 鉴权
	if tokenStr != "" {
		if user, tokenRecord, err := getUserByToken(ctx, tokenStr); err == nil {
			if user.Username == SystemUsername {
				return nil, errors.New("system user is not allowed to login")
			}
			SetToContext(c, TokenAuthKey, true)
			SetToContext(c, TokenAdminKey, tokenRecord.IsAdmin)
			return user, nil
		}
	}

	// 降级使用 Session 鉴权
	userID := GetUserIDFromContext(c)
	if userID <= 0 {
		return nil, errors.New("unauthorized")
	}

	user, err := GetCachedUser(ctx, userID)
	if err != nil || user == nil || !user.IsActive {
		var dbUser contracts.UserDTO
		if err := db.DB(ctx).Table("w_users").Where("id = ? AND is_active = ?", userID, true).First(&dbUser).Error; err != nil {
			return nil, err
		}
		user = &dbUser
		SetCachedUser(ctx, userID, user)
	}

	SetToContext(c, TokenAuthKey, false)
	SetToContext(c, TokenAdminKey, false)

	if user.Username == "system" {
		return nil, errors.New("system user is not allowed to login")
	}

	return user, nil
}

// LoginRequired 返回登录鉴权中间件，校验 Access Token 或 Session
func LoginRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := otel_trace.Start(c.Request.Context(), "LoginRequired")
		defer span.End()

		user, err := GetUserFromRequest(c)
		if err != nil {
			response.AbortUnauthorized(c, shared.UnAuthorized)
			return
		}

		LogForAudit(ctx, user, c)
		SetToContext(c, UserObjKey, user)
		c.Next()
	}
}

// AdminRequired 校验管理员权限（支持 Session 和 Token 鉴权）
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := otel_trace.Start(c.Request.Context(), "AdminRequired")
		defer span.End()

		user, err := GetUserFromRequest(c)
		if err != nil {
			response.AbortUnauthorized(c, shared.UnAuthorized)
			return
		}

		isTokenAuth, _ := GetFromContext[bool](c, TokenAuthKey)
		isTokenAdmin, _ := GetFromContext[bool](c, TokenAdminKey)

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

		LogForAudit(ctx, user, c)
		SetToContext(c, UserObjKey, user)
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
		if tokenAuth, _ := GetFromContext[bool](c, TokenAuthKey); tokenAuth {
			response.AbortForbidden(c, ErrTokenAuthNotAllowed)
			return
		}
		c.Next()
	}
}
