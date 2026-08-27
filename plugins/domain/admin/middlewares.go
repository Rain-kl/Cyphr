// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	otel_trace "github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	"github.com/gin-gonic/gin"
)

// LoginAdminRequired 返回管理员权限校验中间件
func LoginAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := otel_trace.Start(c.Request.Context(), "LoginAdminRequired")
		defer span.End()

		user, _ := auth.GetFromContext[*model.User](c, auth.UserObjKey)
		if user == nil {
			response.AbortNotFound(c, AdminRequired)
			return
		}

		// 如果是通过 Access Token 鉴权，需要检查令牌本身是否具有管理员权限
		if tokenAuth, _ := auth.GetFromContext[bool](c, auth.TokenAuthKey); tokenAuth {
			tokenAdmin, _ := auth.GetFromContext[bool](c, auth.TokenAdminKey)
			if !tokenAdmin {
				response.AbortNotFound(c, TokenAdminRequired)
				return
			}
		}

		if !user.IsAdmin {
			response.AbortNotFound(c, AdminRequired)
			return
		}

		logger.InfoF(ctx, "[LoginAdminRequired] %d %s", user.ID, user.Username)
		c.Next()
	}
}
