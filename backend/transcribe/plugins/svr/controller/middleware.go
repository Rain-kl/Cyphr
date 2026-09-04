// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package controller provides HTTP and WebSocket handlers for the transcribe svr plugin.
package controller

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// ContextKeyNodeID stores the verified worker node ID in the Gin context.
	ContextKeyNodeID = "node_id"
	// ContextKeyNode stores the verified NodeDTO in the Gin context.
	ContextKeyNode = "node"
)

func resolveNodeService(provider any) service.NodeService {
	switch p := provider.(type) {
	case service.NodeService:
		return p
	case func() service.NodeService:
		if p != nil {
			return p()
		}
	}
	return nil
}

func resolveAuthService(provider any) contracts.AuthService {
	switch p := provider.(type) {
	case contracts.AuthService:
		return p
	case func() contracts.AuthService:
		if p != nil {
			return p()
		}
	}
	return nil
}

// RequireAgentToken verifies the agent token provided via "?token=" query parameter
// or "Authorization: Bearer <token>" header using NodeService.
// provider can be a service.NodeService or dynamic getter func() service.NodeService.
func RequireAgentToken(provider any) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeSvc := resolveNodeService(provider)
		if nodeSvc == nil {
			logger.ErrorF(c.Request.Context(), "[RequireAgentToken] nodeService unavailable")
			response.AbortInternal(c, consts.ErrInternal)
			return
		}

		token := strings.TrimSpace(c.Query("token"))
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			}
		}

		if token == "" {
			response.AbortUnauthorized(c, consts.ErrInvalidToken)
			return
		}

		node, err := nodeSvc.VerifyNodeToken(c.Request.Context(), token)
		if err != nil {
			if err.Error() == consts.ErrNodeInactive {
				response.AbortForbidden(c, consts.ErrNodeInactive)
				return
			}
			response.AbortUnauthorized(c, consts.ErrInvalidToken)
			return
		}

		c.Set(ContextKeyNodeID, node.ID)
		c.Set(ContextKeyNode, node)
		c.Next()
	}
}

// GetNodeFromContext extracts the authenticated worker NodeDTO from the Gin context.
func GetNodeFromContext(c *gin.Context) (*do.NodeDTO, bool) {
	if val, ok := c.Get(ContextKeyNode); ok {
		if node, ok := val.(*do.NodeDTO); ok {
			return node, true
		}
	}
	return nil, false
}

// GetNodeIDFromContext extracts the authenticated worker node ID from the Gin context.
func GetNodeIDFromContext(c *gin.Context) (uint64, bool) {
	if val, ok := c.Get(ContextKeyNodeID); ok {
		switch v := val.(type) {
		case uint64:
			return v, true
		case int64:
			if v >= 0 {
				return uint64(v), true
			}
		case int:
			if v >= 0 {
				return uint64(v), true
			}
		}
	}
	return 0, false
}

// UserAuthMiddleware provides user authentication by delegating to contracts.AuthService
// or falling back to pre-injected context user IDs (for testing).
// provider can be a contracts.AuthService or dynamic getter func() contracts.AuthService.
func UserAuthMiddleware(provider any) gin.HandlerFunc {
	return func(c *gin.Context) {
		authSvc := resolveAuthService(provider)
		if authSvc != nil {
			if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok && mw != nil {
				mw(c)
				return
			}
		}
		if uid := GetCurrentUserID(c, authSvc); uid > 0 {
			c.Next()
			return
		}
		response.AbortUnauthorized(c, consts.ErrUnauthorized)
	}
}

// GetCurrentUserID extracts the current authenticated user ID from context.
func GetCurrentUserID(c *gin.Context, authSvc contracts.AuthService) uint64 {
	if u, ok := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey); ok && u != nil && u.ID > 0 {
		return u.ID
	}
	if uid := getUserIDFromAuthService(c, authSvc); uid > 0 {
		return uid
	}
	return getUserIDFromGinKey(c)
}

func getUserIDFromAuthService(c *gin.Context, authSvc contracts.AuthService) uint64 {
	if authSvc == nil {
		return 0
	}
	if u, err := authSvc.GetCurrentUser(c); err == nil && u != nil && u.ID > 0 {
		return u.ID
	}
	if uid, err := authSvc.GetCurrentUserID(c); err == nil && uid > 0 {
		return uid
	}
	if c.Request == nil || c.Request.Context() == nil {
		return 0
	}
	reqCtx := c.Request.Context()
	if u, err := authSvc.GetCurrentUser(reqCtx); err == nil && u != nil && u.ID > 0 {
		return u.ID
	}
	if uid, err := authSvc.GetCurrentUserID(reqCtx); err == nil && uid > 0 {
		return uid
	}
	return 0
}

func getUserIDFromGinKey(c *gin.Context) uint64 {
	val, ok := c.Get(contracts.AuthUserIDKey)
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		if v >= 0 {
			return uint64(v)
		}
	case int:
		if v >= 0 {
			return uint64(v)
		}
	case string:
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			return id
		}
	}
	return 0
}
