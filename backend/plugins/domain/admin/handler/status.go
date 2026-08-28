// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetSystemStatus 获取系统状态信息
// @Summary 获取系统状态信息
// @Description 获取后端服务运行状态、Goroutine、内存指标等详细统计数据，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.SystemStatusResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/status [get]
func GetSystemStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.CollectSystemStatus()))
}

// GetLogDatabaseStatus 返回当前日志库状态。
// @Summary 获取日志数据库状态
// @Description 返回当前日志主库、迁移状态、各库保留天数与合法迁移目标，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.LogDatabaseStatus} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/status/log-database [get]
func GetLogDatabaseStatus(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.LogDatabaseStatus(c.Request.Context())))
}
