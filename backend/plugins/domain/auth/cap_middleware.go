// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/pkg/response"

	"github.com/gin-gonic/gin"
)

// VerifyCaptchaMiddleware returns a Gin middleware that checks and consumes the X-Cap-Token header.
func VerifyCaptchaMiddleware(mgr *CaptchaManager, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !CapProtectionEnabled(c.Request.Context()) {
			c.Next()
			return
		}
		if mgr == nil {
			response.AbortBadRequest(c, errCapTokenInvalidOrExpired)
			return
		}

		token := c.GetHeader("X-Cap-Token")
		if token == "" {
			response.AbortBadRequest(c, errCapTokenMissing)
			return
		}

		valid, err := mgr.VerifyToken(c.Request.Context(), token, scope)
		if err != nil || !valid {
			response.AbortBadRequest(c, errCapTokenInvalidOrExpired)
			return
		}

		c.Next()
	}
}
