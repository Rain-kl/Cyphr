// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	otel_trace "github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/gin-contrib/sessions"
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

func getUserByToken(ctx context.Context, tokenStr string) (*model.User, *model.AccessToken, error) {
	tokenHash := model.HashToken(tokenStr)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err != nil {
		dbToken, err := repository.GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, nil, err
		}
		tokenRecord = &dbToken
		SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || !user.IsActive {
		dbUser, err := repository.GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, nil, err
		}
		user = &dbUser
		SetCachedUser(ctx, tokenRecord.UserID, user)
	}
	return user, tokenRecord, nil
}

// GetUserFromRequest 校验 Access Token 或 Session 并返回用户对象，如果未登录或用户失效则返回 error
func GetUserFromRequest(c *gin.Context) (*model.User, error) {
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
	if err != nil || !user.IsActive {
		dbUser, loadErr := repository.GetActiveUserByID(ctx, userID)
		if loadErr != nil {
			return nil, loadErr
		}
		user = &dbUser
		SetCachedUser(ctx, userID, user)
	}

	// 密码哈希校验：当用户存在本地密码时，要求 Session 中的密码哈希必须与当前数据库中一致
	if user.Password != "" {
		session := sessions.Default(c)
		sessionHash, _ := session.Get(PasswordHashKey).(string)
		if sessionHash != user.Password {
			return nil, errors.New("session expired due to password change")
		}
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
