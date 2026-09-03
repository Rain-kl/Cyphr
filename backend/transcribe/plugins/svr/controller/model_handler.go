// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ModelHandler handles queries for available transcription models.
type ModelHandler struct {
	modelDAO dao.ModelDAO
}

// NewModelHandler creates a new ModelHandler instance.
func NewModelHandler(modelDAO dao.ModelDAO) *ModelHandler {
	return &ModelHandler{
		modelDAO: modelDAO,
	}
}

// ListModels handles GET /api/v1/models, returning active transcription models.
func (h *ModelHandler) ListModels(c *gin.Context) {
	keyword := c.Query("keyword")

	models, err := h.modelDAO.ListActive(c.Request.Context(), keyword)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[ModelHandler] list active models failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	dtos := make([]do.ModelDTO, 0, len(models))
	for _, m := range models {
		dtos = append(dtos, do.ModelDTO{
			ID:          m.ID,
			Name:        m.Name,
			TaskType:    m.TaskType,
			Description: m.Description,
			IsActive:    m.IsActive,
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, response.OK(dtos))
}
