// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// NodeHandler handles node management and manual model loading/unloading.
type NodeHandler struct {
	nodeService service.NodeService
	agentHub    hub.AgentHub
}

// NewNodeHandler creates a new NodeHandler instance.
func NewNodeHandler(nodeSvc service.NodeService, agentHub hub.AgentHub) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeSvc,
		agentHub:    agentHub,
	}
}

// ListNodes handles GET /api/v1/controller/nodes, returning worker nodes with live session metrics.
func (h *NodeHandler) ListNodes(c *gin.Context) {
	keyword := c.Query("keyword")

	nodes, err := h.nodeService.ListNodes(c.Request.Context(), keyword)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[NodeHandler] list nodes failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(nodes))
}

// CreateNode handles POST /api/v1/controller/nodes, creating a node and returning its one-time raw token.
func (h *NodeHandler) CreateNode(c *gin.Context) {
	var req do.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	nodeDTO, rawToken, err := h.nodeService.CreateNode(c.Request.Context(), req.Name)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "[NodeHandler] create node failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(do.NodeCreatedDTO{
		ID:          nodeDTO.ID,
		Name:        nodeDTO.Name,
		AgentToken:  rawToken,
		TokenPrefix: nodeDTO.TokenPrefix,
		CreatedAt:   nodeDTO.CreatedAt,
	}))
}

// GetNode handles GET /api/v1/controller/nodes/:id, returning detailed node info.
func (h *NodeHandler) GetNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	nodeDTO, err := h.nodeService.GetNode(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, consts.ErrNodeNotFound) {
			response.AbortNotFound(c, consts.ErrNotFound)
			return
		}
		logger.ErrorF(c.Request.Context(), "[NodeHandler] get node failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OK(nodeDTO))
}

// DeleteNode handles DELETE /api/v1/controller/nodes/:id, deleting a node and evicting sessions.
func (h *NodeHandler) DeleteNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	if err := h.nodeService.DeleteNode(c.Request.Context(), nodeID); err != nil {
		logger.ErrorF(c.Request.Context(), "[NodeHandler] delete node failed: %v", err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

func (h *NodeHandler) sendModelCommand(c *gin.Context, action string) {
	nodeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	var req do.LoadModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	if h.agentHub == nil {
		response.AbortBadRequest(c, consts.ErrNodeOffline)
		return
	}

	var payload any
	if action == "load_model" {
		payload = do.LoadModelPayload(req)
	} else {
		payload = do.UnloadModelPayload(req)
	}

	msg := do.WSMessage{
		Type:    "command",
		Action:  action,
		Payload: payload,
	}

	if err := h.agentHub.BroadcastToNode(nodeID, msg); err != nil {
		if err.Error() == consts.ErrNodeOffline {
			response.AbortBadRequest(c, consts.ErrNodeOffline)
			return
		}
		logger.ErrorF(c.Request.Context(), "[NodeHandler] %s command failed for node %d: %v", action, nodeID, err)
		response.AbortInternal(c, consts.ErrInternal)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// LoadModel handles POST /api/v1/controller/nodes/:id/load-model, commanding an online agent to load a model.
func (h *NodeHandler) LoadModel(c *gin.Context) {
	h.sendModelCommand(c, "load_model")
}

// UnloadModel handles POST /api/v1/controller/nodes/:id/unload-model, commanding an online agent to unload a model.
func (h *NodeHandler) UnloadModel(c *gin.Context) {
	h.sendModelCommand(c, "unload_model")
}

// UpdateNodeConfig handles PUT /api/v1/controller/nodes/:id/config, updating node settings.
func (h *NodeHandler) UpdateNodeConfig(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	var req do.UpdateNodeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, consts.ErrBindParamsFailed)
		return
	}

	updatedDTO, err := h.nodeService.UpdateNodeConfig(c.Request.Context(), nodeID, req)
	if err != nil {
		if errors.Is(err, consts.ErrNodeNotFound) {
			response.AbortNotFound(c, consts.ErrNotFound)
			return
		}
		logger.ErrorF(c.Request.Context(), "[NodeHandler] update node config failed: %v", err)
		response.AbortBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(updatedDTO))
}
