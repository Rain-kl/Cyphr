// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/pkg/response"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

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
