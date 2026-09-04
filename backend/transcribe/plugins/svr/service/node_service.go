// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service provides business logic for worker nodes and transcription jobs.
package service

import (
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	tokenEntropyBytes = 16
	tokenPrefixLength = 12
)

// NodeService defines business logic for managing agent worker nodes.
type NodeService interface {
	CreateNode(ctx context.Context, name string) (*do.NodeDTO, string, error)
	VerifyNodeToken(ctx context.Context, rawToken string) (*do.NodeDTO, error)
	GetNode(ctx context.Context, id uint64) (*do.NodeDTO, error)
	ListNodes(ctx context.Context, keyword ...string) ([]do.NodeDTO, error)
	UpdateLastSeen(ctx context.Context, id uint64, ip string) error
	UpdateNodeConfig(ctx context.Context, id uint64, req do.UpdateNodeConfigRequest) (*do.NodeDTO, error)
	DeleteNode(ctx context.Context, id uint64) error
}

// DefaultNodeService implements NodeService.
type DefaultNodeService struct {
	nodeDAO  dao.NodeDAO
	agentHub hub.AgentHub
}

var _ NodeService = (*DefaultNodeService)(nil)

// NewNodeService creates a new DefaultNodeService instance.
func NewNodeService(nodeDAO dao.NodeDAO, agentHub hub.AgentHub) *DefaultNodeService {
	return &DefaultNodeService{
		nodeDAO:  nodeDAO,
		agentHub: agentHub,
	}
}

// CreateNode generates a high-entropy agent token, hashes it with SHA-256 for persistent storage,
// and returns the node DTO alongside the one-time raw token.
func (s *DefaultNodeService) CreateNode(ctx context.Context, name string) (*do.NodeDTO, string, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, "", errors.New(consts.ErrBindParamsFailed)
	}

	rawToken, tokenHash, tokenPrefix, err := generateAgentToken()
	if err != nil {
		return nil, "", err
	}

	node := &entity.NodeEntity{
		Name:        trimmedName,
		AgentToken:  rawToken,
		TokenHash:   tokenHash,
		TokenPrefix: tokenPrefix,
		IsActive:    true,
	}

	if err := s.nodeDAO.Create(ctx, node); err != nil {
		return nil, "", err
	}

	dto := s.toNodeDTO(node)
	return dto, rawToken, nil
}

// VerifyNodeToken verifies a raw agent token against stored SHA-256 token hashes.
func (s *DefaultNodeService) VerifyNodeToken(ctx context.Context, rawToken string) (*do.NodeDTO, error) {
	trimmed := strings.TrimSpace(rawToken)
	if trimmed == "" {
		return nil, errors.New(consts.ErrInvalidToken)
	}

	hash := sha256.Sum256([]byte(trimmed))
	tokenHash := hex.EncodeToString(hash[:])

	node, err := s.nodeDAO.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, errors.New(consts.ErrInvalidToken)
	}
	if !node.IsActive {
		return nil, errors.New(consts.ErrNodeInactive)
	}

	return s.toNodeDTO(node), nil
}

// GetNode retrieves a node by ID, decorated with in-memory status if online.
func (s *DefaultNodeService) GetNode(ctx context.Context, id uint64) (*do.NodeDTO, error) {
	node, err := s.nodeDAO.GetByID(ctx, id)
	if err != nil {
		return nil, consts.ErrNodeNotFound
	}
	return s.toNodeDTO(node), nil
}

// ListNodes lists all nodes, decorated with live session data (online, loaded models, running jobs, system stats).
func (s *DefaultNodeService) ListNodes(ctx context.Context, keyword ...string) ([]do.NodeDTO, error) {
	nodes, err := s.nodeDAO.ListAll(ctx, keyword...)
	if err != nil {
		return nil, err
	}

	dtos := make([]do.NodeDTO, 0, len(nodes))
	for i := range nodes {
		dto := s.toNodeDTO(&nodes[i])
		dtos = append(dtos, *dto)
	}
	return dtos, nil
}

// UpdateLastSeen updates the node's last active timestamp and IP address.
func (s *DefaultNodeService) UpdateLastSeen(ctx context.Context, id uint64, ip string) error {
	return s.nodeDAO.UpdateLastSeen(ctx, id, ip)
}

// DeleteNode removes a node by ID and unregisters active session if online.
func (s *DefaultNodeService) DeleteNode(ctx context.Context, id uint64) error {
	if s.agentHub != nil {
		s.agentHub.UnregisterSession(id)
	}
	return s.nodeDAO.Delete(ctx, id)
}

// UpdateNodeConfig updates node settings (work_mode, allow_auto_load, auto_unload_minutes, model_vram_estimates).
func (s *DefaultNodeService) UpdateNodeConfig(ctx context.Context, id uint64, req do.UpdateNodeConfigRequest) (*do.NodeDTO, error) {
	node, err := s.nodeDAO.GetByID(ctx, id)
	if err != nil {
		return nil, consts.ErrNodeNotFound
	}

	workMode := node.WorkMode
	if workMode == "" {
		workMode = consts.WorkModeGPU
	}
	if req.WorkMode != nil {
		mode, err := s.validateWorkMode(id, *req.WorkMode)
		if err != nil {
			return nil, err
		}
		workMode = mode
	}

	allowAutoLoad := node.AllowAutoLoad
	if req.AllowAutoLoad != nil {
		allowAutoLoad = *req.AllowAutoLoad
	}

	autoUnloadMinutes := node.AutoUnloadMinutes
	if req.AutoUnloadMinutes != nil {
		if *req.AutoUnloadMinutes < 0 {
			return nil, errors.New("auto_unload_minutes cannot be negative")
		}
		autoUnloadMinutes = *req.AutoUnloadMinutes
	}

	maxConcurrentJobs := node.MaxConcurrentJobs
	if req.MaxConcurrentJobs != nil {
		if *req.MaxConcurrentJobs <= 0 {
			return nil, errors.New("max_concurrent_jobs must be greater than 0")
		}
		maxConcurrentJobs = *req.MaxConcurrentJobs
	}

	vramEstimates := node.ModelVramEstimates
	var estimatesMap map[string]int
	if req.ModelVramEstimates != nil {
		bytes, err := json.Marshal(req.ModelVramEstimates)
		if err != nil {
			return nil, err
		}
		vramEstimates = string(bytes)
		estimatesMap = req.ModelVramEstimates
	} else if node.ModelVramEstimates != "" {
		_ = json.Unmarshal([]byte(node.ModelVramEstimates), &estimatesMap)
	}

	if err := s.nodeDAO.UpdateConfig(ctx, id, workMode, allowAutoLoad, autoUnloadMinutes, maxConcurrentJobs, vramEstimates); err != nil {
		return nil, err
	}

	node.WorkMode = workMode
	node.AllowAutoLoad = allowAutoLoad
	node.AutoUnloadMinutes = autoUnloadMinutes
	node.MaxConcurrentJobs = maxConcurrentJobs
	node.ModelVramEstimates = vramEstimates

	// Sync with active session in memory and notify agent
	if s.agentHub != nil {
		s.notifyAgentSession(id, workMode, allowAutoLoad, autoUnloadMinutes, maxConcurrentJobs, estimatesMap, req)
	}

	return s.toNodeDTO(node), nil
}

func (s *DefaultNodeService) notifyAgentSession(
	nodeID uint64,
	workMode string,
	allowAutoLoad bool,
	autoUnloadMinutes int,
	maxConcurrentJobs int,
	estimatesMap map[string]int,
	req do.UpdateNodeConfigRequest,
) {
	sess, ok := s.agentHub.GetSession(nodeID)
	if !ok {
		return
	}

	sess.SetConfig(workMode, allowAutoLoad, autoUnloadMinutes, maxConcurrentJobs, estimatesMap)

	if req.WorkMode != nil {
		setModeMsg := do.WSMessage{
			Type:   "command",
			Action: "set_work_mode",
			Seq:    time.Now().UnixNano(),
			Payload: do.SetWorkModePayload{
				Mode: workMode,
			},
		}
		_ = sess.WriteJSON(setModeMsg)
	}

	if req.MaxConcurrentJobs != nil {
		setCfgMsg := do.WSMessage{
			Type:   "command",
			Action: "set_config",
			Seq:    time.Now().UnixNano(),
			Payload: map[string]any{
				"max_concurrent_jobs": maxConcurrentJobs,
			},
		}
		_ = sess.WriteJSON(setCfgMsg)
	}
}

func (s *DefaultNodeService) toNodeDTO(node *entity.NodeEntity) *do.NodeDTO {
	workMode := node.WorkMode
	if workMode == "" {
		workMode = "gpu"
	}

	var vramEstimates map[string]int
	if node.ModelVramEstimates != "" {
		_ = json.Unmarshal([]byte(node.ModelVramEstimates), &vramEstimates)
	}
	if vramEstimates == nil {
		vramEstimates = make(map[string]int)
	}

	maxJobs := node.MaxConcurrentJobs
	if maxJobs <= 0 {
		maxJobs = 2
	}

	dto := &do.NodeDTO{
		ID:                 node.ID,
		Name:               node.Name,
		AgentToken:         node.AgentToken,
		TokenPrefix:        node.TokenPrefix,
		IsActive:           node.IsActive,
		WorkMode:           workMode,
		SupportedModes:     []string{"cpu"},
		CurrentMode:        "cpu",
		AllowAutoLoad:      node.AllowAutoLoad,
		AutoUnloadMinutes:  node.AutoUnloadMinutes,
		MaxConcurrentJobs:  maxJobs,
		ModelVramEstimates: vramEstimates,
		DownloadedModels:   []string{},
		LastIP:             node.LastIP,
		LastSeenAt:         node.LastSeenAt,
		CreatedAt:          node.CreatedAt,
	}

	if s.agentHub != nil {
		if sess, ok := s.agentHub.GetSession(node.ID); ok {
			dto.IsOnline = true
			dto.LoadedModels = sess.GetLoadedModels()
			dto.DownloadedModels = sess.GetDownloadedModels()
			dto.RunningJobs = sess.GetRunningJobs()
			dto.System = sess.GetSystemStats()
			dto.SupportedModes = sess.GetSupportedModes()
			dto.CurrentMode = sess.GetCurrentMode()
		}
	}

	return dto
}

func (s *DefaultNodeService) validateWorkMode(nodeID uint64, rawMode string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(rawMode))
	if mode != consts.WorkModeCPU && mode != consts.WorkModeGPU {
		return "", errors.New("invalid work mode: must be cpu or gpu")
	}

	if s.agentHub == nil {
		return mode, nil
	}

	sess, ok := s.agentHub.GetSession(nodeID)
	if !ok {
		return mode, nil
	}

	if !sess.SupportsMode(mode) {
		return "", fmt.Errorf("node does not support %s mode", mode)
	}

	return mode, nil
}

// generateAgentToken generates a random token prefixed with 'agt_' and its SHA-256 hash.
func generateAgentToken() (rawToken, tokenHash, tokenPrefix string, err error) {
	bytes := make([]byte, tokenEntropyBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", err
	}

	randomHex := hex.EncodeToString(bytes)
	rawToken = consts.AgentTokenPrefix + randomHex
	tokenPrefix = rawToken[:tokenPrefixLength]

	hash := sha256.Sum256([]byte(rawToken))
	tokenHash = hex.EncodeToString(hash[:])
	return rawToken, tokenHash, tokenPrefix, nil
}
