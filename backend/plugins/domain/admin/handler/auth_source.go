// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListAuthSources 获取认证源列表
// @Summary 获取认证源列表
// @Description 返回所有已配置的 OAuth/OIDC 认证源列表，包括已启用和未启用的，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "认证源列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/auth-sources [get]
func ListAuthSources(c *gin.Context) {
	views, err := service.ListAuthSources(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(views))
}

// CreateAuthSource 创建认证源
// @Summary 创建认证源
// @Description 创建一个新的 OAuth/OIDC 认证源配置，认证源名称必须唯一且符合命名规范，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body contracts.AuthSourceDTO true "创建认证源参数"
// @Success 200 {object} response.Any "创建成功，返回认证源信息"
// @Failure 400 {object} response.Any "参数错误或验证失败"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/auth-sources [post]
func CreateAuthSource(c *gin.Context) {
	var source contracts.AuthSourceDTO
	if err := c.ShouldBindJSON(&source); err != nil {
		response.AbortBadRequest(c, errs.InvalidParams)
		return
	}

	created, err := service.CreateAuthSource(c.Request.Context(), source)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(created))
}

// UpdateAuthSource 更新认证源
// @Summary 更新认证源
// @Description 更新指定 ID 的认证源配置。若 client_secret 字段为空，则保留原有密钥不变，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "认证源 ID 或名称"
// @Param request body contracts.AuthSourceDTO true "更新认证源参数"
// @Success 200 {object} response.Any "更新成功，返回更新后的认证源信息"
// @Failure 400 {object} response.Any "参数错误或验证失败"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/auth-sources/{id} [put]
func UpdateAuthSource(c *gin.Context) {
	id, ok := parseAuthSourceID(c)
	if !ok {
		return
	}

	var req contracts.AuthSourceDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errs.InvalidParams)
		return
	}

	updated, err := service.UpdateAuthSource(c.Request.Context(), id, req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(updated))
}

// ToggleAuthSource 切换认证源启用状态
// @Summary 切换认证源启用状态
// @Description 启用或禁用指定认证源。尝试启用时将验证 Client ID 和 Client Secret 是否已配置，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "认证源 ID 或名称"
// @Success 200 {object} response.Any{data=string} "切换成功"
// @Failure 400 {object} response.Any "验证失败或认证源不存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/auth-sources/{id}/toggle [put]
func ToggleAuthSource(c *gin.Context) {
	id, ok := parseAuthSourceID(c)
	if !ok {
		return
	}

	toggled, err := service.ToggleAuthSource(c.Request.Context(), id)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{"is_active": toggled.IsActive}))
}

// DeleteAuthSource 删除认证源
// @Summary 删除认证源
// @Description 删除指定认证源及其关联的所有外部帐号绑定记录，警告：删除后相关用户将无法通过该源登录，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "认证源 ID 或名称"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "ID 无效或删除失败"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/auth-sources/{id} [delete]
func DeleteAuthSource(c *gin.Context) {
	id, ok := parseAuthSourceID(c)
	if !ok {
		return
	}

	if err := service.DeleteAuthSource(c.Request.Context(), id); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// parseAuthSourceID reads the numeric auth source path parameter.
func parseAuthSourceID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errs.ErrInvalidAuthSourceID)
		return 0, false
	}
	return id, true
}
