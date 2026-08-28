// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/backend/pkg/response"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/auth"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
	"github.com/gin-gonic/gin"
)

// ListAuthSources lists all configured authentication sources.
func ListAuthSources(c *gin.Context) {
	var sources []auth.AuthSource
	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Order("id ASC").Find(&sources).Error; err != nil {
		response.AbortInternal(c, "获取认证源列表失败")
		return
	}

	views := make([]auth.AuthSourceView, len(sources))
	for i := range sources {
		views[i] = auth.AuthSourceView{
			ID:                     sources[i].ID,
			Name:                   sources[i].Name,
			Type:                   sources[i].Type,
			DisplayName:            sources[i].DisplayName,
			IsActive:               sources[i].IsActive,
			IconURL:                sources[i].IconURL,
			ClientSecretConfigured: sources[i].ClientSecret != "",
		}
	}

	c.JSON(http.StatusOK, response.OK(views))
}

// CreateAuthSource creates a new authentication source.
func CreateAuthSource(c *gin.Context) {
	var source auth.AuthSource
	if err := c.ShouldBindJSON(&source); err != nil {
		response.AbortBadRequest(c, "无效的参数")
		return
	}

	if err := source.Validate(); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Create(&source).Error; err != nil {
		response.AbortBadRequest(c, "创建认证源失败: "+err.Error())
		return
	}

	source.Sanitize()
	c.JSON(http.StatusOK, response.OK(source))
}

// UpdateAuthSource updates an authentication source.
func UpdateAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	gormDB := database.DB(c.Request.Context())
	var existing auth.AuthSource
	if err := gormDB.First(&existing, id).Error; err != nil {
		response.AbortNotFound(c, "认证源不存在")
		return
	}

	var req auth.AuthSource
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, "无效的参数")
		return
	}

	existing.DisplayName = req.DisplayName
	existing.ClientID = req.ClientID
	if req.ClientSecret != "" {
		existing.ClientSecret = req.ClientSecret
	}
	existing.OpenIDDiscoveryURL = req.OpenIDDiscoveryURL
	existing.Scopes = req.Scopes
	existing.IconURL = req.IconURL

	if err := existing.Validate(); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := gormDB.Save(&existing).Error; err != nil {
		response.AbortInternal(c, "更新认证源失败")
		return
	}

	existing.Sanitize()
	c.JSON(http.StatusOK, response.OK(existing))
}

// ToggleAuthSource toggles the active state of an auth source.
func ToggleAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	gormDB := database.DB(c.Request.Context())
	var existing auth.AuthSource
	if err := gormDB.First(&existing, id).Error; err != nil {
		response.AbortNotFound(c, "认证源不存在")
		return
	}

	existing.IsActive = !existing.IsActive
	if existing.IsActive {
		if err := existing.Validate(); err != nil {
			response.AbortBadRequest(c, err.Error())
			return
		}
	}

	if err := gormDB.Model(&existing).Update("is_active", existing.IsActive).Error; err != nil {
		response.AbortInternal(c, "切换认证源状态失败")
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{"is_active": existing.IsActive}))
}

// DeleteAuthSource deletes an authentication source.
func DeleteAuthSource(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "无效的认证源 ID")
		return
	}

	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Delete(&auth.AuthSource{}, id).Error; err != nil {
		response.AbortInternal(c, "删除认证源失败")
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
