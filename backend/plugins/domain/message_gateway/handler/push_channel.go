// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/message_gateway/errs"
	"Wavelet/plugins/domain/message_gateway/model"
	"Wavelet/plugins/domain/message_gateway/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListPushChannelDefinitions returns channel definitions.
func ListPushChannelDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(model.ListPushDefinitions()))
}

// ListPushChannels lists configured push channels.
func ListPushChannels(c *gin.Context) {
	channels, err := service.ListPushChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channels))
}

// parsePushChannelID reads the path identifier of a push channel.
func parsePushChannelID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errs.ErrInvalidChannelID)
		return 0, false
	}
	return id, true
}

// handlePushChannelNotFoundError maps a missing channel row to 404, others to fallback.
func handlePushChannelNotFoundError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, errs.ErrRecordNotFound) {
		response.AbortNotFound(c, errs.ErrChannelNotFound)
		return
	}
	fallback(c, err.Error())
}

// CreatePushChannel creates a push channel.
func CreatePushChannel(c *gin.Context) {
	handleJSONRequest(c, service.CreatePushChannel)
}

// UpdatePushChannel updates a push channel.
func UpdatePushChannel(c *gin.Context) {
	handleEntityUpdate(c, parsePushChannelID, service.UpdatePushChannel, func(c *gin.Context, err error) {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
	})
}

// DeletePushChannel deletes a push channel.
func DeletePushChannel(c *gin.Context) {
	id, ok := parsePushChannelID(c)
	if !ok {
		return
	}

	if err := service.DeletePushChannel(c.Request.Context(), id); err != nil {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TestPushChannel tests connectivity of a push channel.
func TestPushChannel(c *gin.Context) {
	var req model.TestPushChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	payload, err := service.PreparePushChannelTest(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if err := service.EnqueuePushTask(c.Request.Context(), payload); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
