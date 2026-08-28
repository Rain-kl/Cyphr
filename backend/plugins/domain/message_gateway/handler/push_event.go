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

// ListPushEvents lists configured push events.
func ListPushEvents(c *gin.Context) {
	ctx := c.Request.Context()
	events, err := service.ListPushEvents(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(events))
}

// ListBuiltInPushEvents lists system built-in push event definitions.
func ListBuiltInPushEvents(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(service.GetBuiltInEvents()))
}

// parsePushEventID reads the path identifier of a push event.
func parsePushEventID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errs.ErrInvalidEventID)
		return 0, false
	}
	return id, true
}

// handlePushEventNotFoundError maps a missing event row to 404, others to fallback.
func handlePushEventNotFoundError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, errs.ErrRecordNotFound) {
		response.AbortNotFound(c, errs.ErrEventNotFound)
		return
	}
	fallback(c, err.Error())
}

// CreatePushEvent creates a new push event configuration.
func CreatePushEvent(c *gin.Context) {
	handleJSONRequest(c, service.CreatePushEvent)
}

// DeletePushEvent deletes a push event configuration by ID.
func DeletePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	if err := service.DeletePushEvent(c.Request.Context(), id); err != nil {
		handlePushEventNotFoundError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// UpdatePushEvent updates an existing push event.
func UpdatePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	var req model.UpdatePushEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.UpdatePushEvent(c.Request.Context(), id, req); err != nil {
		handlePushEventNotFoundError(c, err, response.AbortBadRequest)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TogglePushEvent toggles the enabled state of a push event.
func TogglePushEvent(c *gin.Context) {
	id, ok := parsePushEventID(c)
	if !ok {
		return
	}

	enabled, err := service.TogglePushEvent(c.Request.Context(), id)
	if err != nil {
		handlePushEventNotFoundError(c, err, response.AbortBadRequest)
		return
	}
	c.JSON(http.StatusOK, response.OK(enabled))
}

// ListPushHistories returns paginated push notification delivery histories.
func ListPushHistories(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	total, results, err := service.ListPushHistories(c.Request.Context(), model.PushHistoryListFilter{
		EventKey: c.Query("event_key"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(map[string]any{
		"total":   total,
		"results": results,
	}))
}

// TestPush executes a synchronous push test using the specified config.
func TestPush(c *gin.Context) {
	var req model.TestPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := service.RunPushTest(c.Request.Context(), req.Config, req.Target); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
