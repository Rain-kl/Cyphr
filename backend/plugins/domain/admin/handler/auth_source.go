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

// ListAuthSources lists all configured authentication sources.
func ListAuthSources(c *gin.Context) {
	views, err := service.ListAuthSources(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(views))
}

// CreateAuthSource creates a new authentication source.
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

// UpdateAuthSource updates an authentication source.
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

// ToggleAuthSource toggles the active state of an auth source.
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

// DeleteAuthSource deletes an authentication source.
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
