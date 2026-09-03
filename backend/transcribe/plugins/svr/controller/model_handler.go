// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"net/http"
	"strconv"

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

func toModelDTOs(models []entity.ModelEntity) []do.ModelDTO {
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
	return dtos
}

// ListModels handles GET /api/v1/models, returning active transcription models.
func (h *ModelHandler) ListModels(c *gin.Context) {
	models, err := h.modelDAO.ListActive(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[ModelHandler] list active models failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(toModelDTOs(models)))
}

// ListAllModels handles GET /api/v1/controller/models, returning all models for administrators.
func (h *ModelHandler) ListAllModels(c *gin.Context) {
	models, err := h.modelDAO.ListAll(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[ModelHandler] list all models failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(toModelDTOs(models)))
}

// ToggleModelStatus handles PUT /api/v1/controller/models/:id/status, toggling active status.
func (h *ModelHandler) ToggleModelStatus(c *gin.Context) {
	modelID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	var req do.ToggleModelStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	if err := h.modelDAO.UpdateStatus(c.Request.Context(), modelID, req.IsActive); err != nil {
		logger.ErrorF(c.Request.Context(), "[ModelHandler] update model status failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
