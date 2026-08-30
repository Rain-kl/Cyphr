// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AbortBadRequest 以 400 中断请求并将错误挂载到 Gin Error 链，供全局中间件统一记录 Trace 并响应。
func AbortBadRequest(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusBadRequest, msg)
}

// AbortUnauthorized 以 401 中断请求并将错误挂载到 Gin Error 链。
func AbortUnauthorized(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusUnauthorized, msg)
}

// AbortForbidden 以 403 中断请求并将错误挂载到 Gin Error 链。
func AbortForbidden(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusForbidden, msg)
}

// AbortNotFound 以 404 中断请求并将错误挂载到 Gin Error 链。
func AbortNotFound(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusNotFound, msg)
}

// AbortInternal 以 500 中断请求并将错误挂载到 Gin Error 链。
func AbortInternal(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusInternalServerError, msg)
}

// AbortTooManyRequests 以 429 中断请求并将错误挂载到 Gin Error 链。
func AbortTooManyRequests(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusTooManyRequests, msg)
}

// AbortConflict 以 409 中断请求并将错误挂载到 Gin Error 链。
func AbortConflict(c *gin.Context, msg string) {
	AbortWithError(c, http.StatusConflict, msg)
}

// AbortNotFoundIfMissing 在 err 非空时中断请求：gorm.ErrRecordNotFound 映射为 404（文案 notFoundMsg），其余映射为 400（文案 err.Error()）。
// 返回是否已中断。err 为 nil 时不写响应并返回 false。
func AbortNotFoundIfMissing(c *gin.Context, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		AbortNotFound(c, notFoundMsg)
		return true
	}
	AbortBadRequest(c, err.Error())
	return true
}

// AbortBadRequestOnError 在 err 非空时以 400 中断请求，文案为 err.Error()。
// 返回是否已中断。err 为 nil 时不写响应并返回 false。
func AbortBadRequestOnError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	AbortBadRequest(c, err.Error())
	return true
}
