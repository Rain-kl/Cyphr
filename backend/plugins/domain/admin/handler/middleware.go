// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/trace"
	"Wavelet/plugins/domain/admin/errs"

	"github.com/gin-gonic/gin"
)

// LoginAdminRequired 返回管理员权限校验中间件
func LoginAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := trace.Start(c.Request.Context(), "LoginAdminRequired")
		defer span.End()

		user, _ := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
		if user == nil {
			response.AbortNotFound(c, errs.AdminRequired)
			return
		}

		// 如果是通过 Access Token 鉴权，需要检查令牌本身是否具有管理员权限
		if tokenAuth, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAuthKey); tokenAuth {
			tokenAdmin, _ := ginutil.GetFromContext[bool](c, contracts.AuthTokenAdminKey)
			if !tokenAdmin {
				response.AbortNotFound(c, errs.TokenAdminRequired)
				return
			}
		}

		if !user.IsAdmin {
			response.AbortNotFound(c, errs.AdminRequired)
			return
		}

		logger.InfoF(ctx, "[LoginAdminRequired] %d %s", user.ID, user.Username)
		c.Next()
	}
}
