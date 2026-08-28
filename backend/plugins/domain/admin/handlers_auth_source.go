// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListAuthSources lists all configured authentication sources.
func ListAuthSources(c *gin.Context) {
	authSvc := GetAuthService(c.Request.Context())
	if authSvc == nil {
		response.AbortInternal(c, "认证服务未就绪")
		return
	}

	views, err := authSvc.ListAuthSources(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, "获取认证源列表失败")
		return
	}

	c.JSON(http.StatusOK, response.OK(views))
}

// CreateAuthSource creates a new authentication source.
func CreateAuthSource(c *gin.Context) {
	var source contracts.AuthSourceDTO
	if err := c.ShouldBindJSON(&source); err != nil {
		response.AbortBadRequest(c, "无效的参数")
		return
	}

	authSvc := GetAuthService(c.Request.Context())
	if authSvc == nil {
		response.AbortInternal(c, "认证服务未就绪")
		return
	}

	created, err := authSvc.CreateAuthSource(c.Request.Context(), source)
	if err != nil {
		response.AbortBadRequest(c, "创建认证源失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(created))
}

// UpdateAuthSource updates an authentication source.
func UpdateAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	var req contracts.AuthSourceDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, "无效的参数")
		return
	}

	authSvc := GetAuthService(c.Request.Context())
	if authSvc == nil {
		response.AbortInternal(c, "认证服务未就绪")
		return
	}

	updated, err := authSvc.UpdateAuthSource(c.Request.Context(), id, req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(updated))
}

// ToggleAuthSource toggles the active state of an auth source.
func ToggleAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	authSvc := GetAuthService(c.Request.Context())
	if authSvc == nil {
		response.AbortInternal(c, "认证服务未就绪")
		return
	}

	toggled, err := authSvc.ToggleAuthSource(c.Request.Context(), id)
	if err != nil {
		response.AbortInternal(c, "切换认证源状态失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{"is_active": toggled.IsActive}))
}

// DeleteAuthSource deletes an authentication source.
func DeleteAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	authSvc := GetAuthService(c.Request.Context())
	if authSvc == nil {
		response.AbortInternal(c, "认证服务未就绪")
		return
	}

	if err := authSvc.DeleteAuthSource(c.Request.Context(), id); err != nil {
		response.AbortInternal(c, "删除认证源失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
