// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package handler provides HTTP endpoints for message_gateway.
package handler

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/message_gateway/errs"
	"Wavelet/plugins/domain/message_gateway/model"
	"Wavelet/plugins/domain/message_gateway/service"
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func currentUser(c *gin.Context) (*contracts.UserDTO, bool) {
	return ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
}

// handleJSONRequest binds a JSON body, runs the service use case and writes the
// standard success envelope; any service error surfaces as a bad request.
func handleJSONRequest[Req any, Res any](c *gin.Context, handler func(ctx context.Context, req Req) (Res, error)) {
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	res, err := handler(c.Request.Context(), req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(res))
}

// handleEntityUpdate resolves a path identifier plus JSON body, runs the updater
// use case and writes the success envelope; error classification is delegated to onErr.
func handleEntityUpdate[Req any, Res any](
	c *gin.Context,
	parseID func(*gin.Context) (uint64, bool),
	updater func(ctx context.Context, id uint64, req Req) (Res, error),
	onErr func(*gin.Context, error),
) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	dto, err := updater(c.Request.Context(), id, req)
	if err != nil {
		onErr(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(dto))
}

// ListChannels lists enabled channels a user can bind.
// @Summary List enabled messaging channels
// @Description Returns enabled system bots the current user can pair with
// @Tags message-gateway
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=[]model.PublicChannelDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/channels [get]
func ListChannels(c *gin.Context) {
	if user, ok := currentUser(c); !ok || user == nil {
		response.AbortUnauthorized(c, errs.ErrLoginRequired)
		return
	}
	rows, err := service.ListEnabledPublicChannels(c.Request.Context())
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
// @Success 200 {object} response.Any{data=[]model.BindingDTO}
// @Failure 401 {object} response.Any
// @Router /api/v1/message-gateway/bindings [get]
func ListBindings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, errs.ErrLoginRequired)
		return
	}
	rows, err := service.ListUserBindings(c.Request.Context(), user.ID)
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
// @Param request body model.BindRequest true "bind body"
// @Success 200 {object} response.Any{data=model.BindingDTO}
// @Failure 400 {object} response.Any
// @Failure 409 {object} response.Any
// @Router /api/v1/message-gateway/bindings [post]
func BindBinding(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok || user == nil {
		response.AbortUnauthorized(c, errs.ErrLoginRequired)
		return
	}
	var req model.BindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	dto, err := service.BindChannel(c.Request.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, errs.ErrPlatformAlreadyBound) {
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
		response.AbortUnauthorized(c, errs.ErrLoginRequired)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errs.ErrInvalidBindingID)
		return
	}
	if err := service.UnbindChannel(c.Request.Context(), user.ID, id); err != nil {
		if errors.Is(err, errs.ErrBindingNotFound) {
			response.AbortNotFound(c, err.Error())
			return
		}
		if errors.Is(err, errs.ErrBindingForbidden) {
			response.AbortForbidden(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
