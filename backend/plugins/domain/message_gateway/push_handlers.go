// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	pkgpush "github.com/Rain-kl/Wavelet/backend/plugins/domain/message_gateway/push"

	"github.com/Rain-kl/Wavelet/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpdatePushEventRequest is the request body for updating a push event.
type UpdatePushEventRequest struct {
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template" binding:"required"`
	Enabled  bool     `json:"enabled"`
}

// CreatePushEventRequest is the request body for creating a push event.
type CreatePushEventRequest struct {
	EventKey string   `json:"event_key"`
	TaskType string   `json:"task_type"`
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template"`
	Enabled  bool     `json:"enabled"`
}

// TestPushRequest is the request body for testing push config.
type TestPushRequest struct {
	Config pkgpush.Config `json:"config" binding:"required"`
	Target string         `json:"target"`
}

// ListPushEvents lists configured push events.
func ListPushEvents(c *gin.Context) {
	ctx := c.Request.Context()
	events, err := listPushEvents(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(events))
}

// ListBuiltInPushEvents lists system built-in push event definitions.
func ListBuiltInPushEvents(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(GetBuiltInEvents()))
}

// CreatePushEvent creates a new push event configuration.
func CreatePushEvent(c *gin.Context) {
	var req CreatePushEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	event, err := createPushEvent(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(event))
}

// DeletePushEvent deletes a push event configuration by ID.
func DeletePushEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	if err := deletePushEvent(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// UpdatePushEvent updates an existing push event.
func UpdatePushEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	var req UpdatePushEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if err := updatePushEvent(c.Request.Context(), id, req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// TogglePushEvent toggles the enabled state of a push event.
func TogglePushEvent(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid event id")
		return
	}

	enabled, err := togglePushEvent(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, "notification event not found")
			return
		}
		response.AbortBadRequest(c, err.Error())
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

	total, results, err := listPushHistories(c.Request.Context(), PushHistoryListFilter{
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
	var req TestPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	pusher, err := pkgpush.GetPusher(req.Config.Channel)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if err := pusher.ValidateConfig(req.Config); err != nil {
		response.AbortBadRequest(c, fmt.Sprintf("validation failed: %v", err))
		return
	}

	applySMTPFallbackToPushConfig(c.Request.Context(), &req.Config)

	testBody := map[string]any{
		keyTitle:   "测试通道推送",
		keyContent: "当您收到这条消息，说明当前渠道连通性测试通过。",
		keyLevel:   defaultLevelInfo,
	}
	if _, err := pusher.Send(c.Request.Context(), req.Config, req.Target, testBody, "", nil); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
