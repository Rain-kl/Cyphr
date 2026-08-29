// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetDBOverview 获取数据库运行概览
// @Summary 获取数据库运行概览
// @Description 获取数据库类型、版本、名称、文件大小、表数量及当前连接数，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.DBOverviewResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/overview [get]
func GetDBOverview(c *gin.Context) {
	overview, err := service.DatabaseOverview(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(overview))
}

// ListDBTables 获取数据库所有表名
// @Summary 获取数据库所有表名
// @Description 返回当前数据库的所有用户自定义表名称列表，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]string} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/tables [get]
func ListDBTables(c *gin.Context) {
	tables, err := service.DatabaseTableNames(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(tables))
}

// GetDBTableData 获取数据表 data
func GetDBTableData(c *gin.Context) {
	var req model.GetTableDataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	data, err := service.DatabaseTableData(c.Request.Context(), req)
	if err != nil {
		if msg, ok := errs.AsInvalidInput(err); ok {
			response.AbortBadRequest(c, msg)
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(data))
}

// ExecuteSQL 执行 SQL 查询
// @Summary 执行 SQL 查询
// @Description 在当前数据库中执行任意自定义 SQL，如果是查询语句将返回格式化后的列与数据集，否则返回受影响行数，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body model.ExecuteSQLRequest true "SQL 请求参数"
// @Success 200 {object} response.Any{data=model.ExecuteSQLResponse} "执行完毕"
// @Failure 400 {object} response.Any "SQL 语句错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/db-manage/query [post]
func ExecuteSQL(c *gin.Context) {
	var req model.ExecuteSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	trimmedSQL := strings.TrimSpace(req.SQL)
	if trimmedSQL == "" {
		response.AbortBadRequest(c, errs.InvalidSQLStatement)
		return
	}

	resp, err := service.ExecuteCustomSQL(c.Request.Context(), trimmedSQL)
	if err != nil {
		if errors.Is(err, errs.ErrDatabaseUninitialized) {
			response.AbortInternal(c, err.Error())
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// GetDatabaseInfo 获取当前数据库类型及版本信息
// @Summary 获取数据库信息
// @Description 返回当前使用的数据库类型（sqlite/postgres）、名称/路径及版本字符串，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=model.DatabaseInfoResponse} "获取成功"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Router /api/v1/admin/db-info [get]
func GetDatabaseInfo(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.DatabaseInfo(c.Request.Context())))
}

// ExportDatabase 导出数据库
// @Summary 导出数据库
// @Description SQLite 时直接下载 .db 文件；PostgreSQL 时执行 pg_dump 并流式下载 .sql 文件，需要管理员权限
// @Tags admin
// @Produce application/octet-stream
// @Security SessionCookie
// @Success 200 {file} binary "数据库文件"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "导出失败"
// @Router /api/v1/admin/db-export [get]
func ExportDatabase(c *gin.Context) {
	if !service.GetDBConfig().Enabled {
		exportSQLite(c)
	} else {
		exportPostgres(c)
	}
}

func exportSQLite(c *gin.Context) {
	f, fi, err := service.OpenSQLiteExportFile()
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	defer func() {
		_ = f.Close()
	}()

	c.Header("Content-Disposition", `attachment; filename="wavelet.db"`)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fi.Size()))
	c.Status(http.StatusOK)
	http.ServeContent(c.Writer, c.Request, "wavelet.db", fi.ModTime(), f)
}

func exportPostgres(c *gin.Context) {
	cmd, fileName, err := service.NewPgDumpCommand(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
	c.Header("Content-Type", "application/octet-stream")
	c.Status(http.StatusOK)

	cmd.Stdout = c.Writer
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		logger.ErrorF(c.Request.Context(), "[db-export] pg_dump failed: %v", err)
	}
}
