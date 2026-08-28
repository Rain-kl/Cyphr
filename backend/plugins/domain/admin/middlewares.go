// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/trace"
	"Wavelet/pkg/util"

	"github.com/gin-gonic/gin"
)

// LoginAdminRequired 返回管理员权限校验中间件
func LoginAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := trace.Start(c.Request.Context(), "LoginAdminRequired")
		defer span.End()

		user, _ := util.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
		if user == nil {
			response.AbortNotFound(c, AdminRequired)
			return
		}

		// 如果是通过 Access Token 鉴权，需要检查令牌本身是否具有管理员权限
		if tokenAuth, _ := util.GetFromContext[bool](c, contracts.AuthTokenAuthKey); tokenAuth {
			tokenAdmin, _ := util.GetFromContext[bool](c, contracts.AuthTokenAdminKey)
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
