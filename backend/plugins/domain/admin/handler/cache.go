// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetCacheStatus 获取磁盘缓存状态与当前统计数据
// @Summary 获取缓存状态
// @Description 获取当前系统磁盘缓存的使用情况（已占用字节、Key 数量等）与策略配置
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=disk.Status} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/cache/status [get]
func GetCacheStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.DiskCacheStatus()))
}

// UpdateCacheConfig 更新磁盘缓存策略配置
// @Summary 更新缓存配置
// @Description 更改磁盘缓存最大容量限制、文件生存时间（TTL）以及是否启用 LRU 淘汰淘汰算法，并进行热更新
// @Tags admin
// @Accept json
// @Produce json
// @Param request body model.UpdateCacheConfigRequest true "缓存配置请求体"
// @Security SessionCookie
// @Success 200 {object} response.Any "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/admin/cache/config [post]
func UpdateCacheConfig(c *gin.Context) {
	var req model.UpdateCacheConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.UpdateDiskCachePolicy(c.Request.Context(), req); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// ClearCache 一键清空所有磁盘缓存数据
// @Summary 清空缓存
// @Description 清除系统磁盘缓存目录中的所有临时文件，并重置缓存容量和 Key 追踪数据
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "清理成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "服务内部错误"
// @Router /api/v1/admin/cache/clear [post]
func ClearCache(c *gin.Context) {
	if err := service.ClearDiskCache(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
