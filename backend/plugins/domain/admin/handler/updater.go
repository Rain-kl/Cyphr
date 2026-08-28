// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/admin/service"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetUpdateStatus 获取应用更新状态
// @Summary 获取应用更新状态
// @Description 从系统配置指定的 GitHub 上游仓库查询最新兼容 Release，并与当前服务版本比较
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.UpdaterStatus} "更新状态"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "查询失败"
// @Router /api/v1/admin/update [get]
func GetUpdateStatus(c *gin.Context) {
	status, err := service.GetUpdateStatus(c.Request.Context())
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[Updater] check release failed: %v", err)
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(status))
}

// ApplyUpdate 下载并应用应用更新
// @Summary 下载并应用应用更新
// @Description 下载当前平台对应的 GitHub Actions Release 资产，替换当前二进制并重启进程
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "升级已准备并即将重启"
// @Failure 400 {object} response.Any "当前版本不可升级"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "升级准备失败"
// @Router /api/v1/admin/update/apply [post]
func ApplyUpdate(c *gin.Context) {
	executable, stagedBinary, err := service.DefaultUpdaterManager.PrepareUpgrade(c.Request.Context())
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[Updater] prepare upgrade failed: %v", err)
		response.AbortBadRequest(c, err.Error())
		return
	}

	logger.InfoF(c.Request.Context(), "[Updater] upgrade prepared; restarting with %s", stagedBinary)
	c.JSON(http.StatusOK, response.OKNil())

	util.Go(func() {
		time.Sleep(time.Second)
		if err := service.ReplaceAndRestart(executable, stagedBinary); err != nil {
			service.DefaultUpdaterManager.FinishUpgrade()
			logger.ErrorF(context.Background(), "[Updater] replace and restart failed: %v", err)
		}
	})
}
