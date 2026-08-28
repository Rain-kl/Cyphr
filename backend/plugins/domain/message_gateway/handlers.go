// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/pkg/response"
	"github.com/Rain-kl/Wavelet/backend/pkg/util"
	"github.com/gin-gonic/gin"
)

func currentUser(c *gin.Context) (*contracts.UserDTO, bool) {
	return util.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
}

// ListChannels lists enabled channels a user can bind.
// @Summary List enabled messaging channels
// @Description Returns enabled system bots the current user can pair with
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]PublicChannelDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/channels [get]
func ListChannels(c *gin.Context) {
	if user, ok := currentUser(c); !ok || user == nil {
		response.AbortUnauthorized(c, "login required")
		return
	}
	rows, err := listEnabledPublicChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(rows))
}

// ListBindings lists the current user's bot bindings.
// @Summary List message gateway bindings
// @Description Returns the current user's bound messaging channels
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]BindingDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/bindings [get]
func ListBindings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, "login required")
		return
	}
	rows, err := listUserBindings(c.Request.Context(), user.ID)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(rows))
}

// BindBinding consumes a pairing code and binds the platform identity.
// @Summary Bind a messaging channel
// @Description Binds the current user to a platform identity using a one-time pairing code
// @Tags message-gateway
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body BindRequest true "bind body"
// @Success 200 {object} response.Any{data=BindingDTO}
// @Failure 400 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/message-gateway/bindings [post]
func BindBinding(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, "login required")
		return
	}
	var req BindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	dto, err := bindChannel(c.Request.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, errPlatformAlreadyBound) {
			response.AbortConflict(c, err.Error())
			return
		}
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(dto))
}

// UnbindBinding removes the current user's binding.
// @Summary Unbind a messaging channel
// @Description Removes a binding owned by the current user
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Param id path int true "binding id"
// @Success 200 {object} response.Any
// @Failure 403 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/message-gateway/bindings/{id} [delete]
func UnbindBinding(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, "login required")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid binding id")
		return
	}
	if err := unbindChannel(c.Request.Context(), user.ID, id); err != nil {
		if errors.Is(err, errBindingNotFound) {
			response.AbortNotFound(c, err.Error())
			return
		}
		if errors.Is(err, errBindingForbidden) {
			response.AbortForbidden(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// RegisterUserRoutes mounts user-facing message gateway endpoints.
func RegisterUserRoutes(r *gin.RouterGroup, loginMW gin.HandlerFunc) {
	mg := r.Group("/message-gateway", loginMW)
	{
		mg.GET("/channels", ListChannels)
		mg.GET("/bindings", ListBindings)
		mg.POST("/bindings", BindBinding)
		mg.DELETE("/bindings/:id", UnbindBinding)
	}
}
