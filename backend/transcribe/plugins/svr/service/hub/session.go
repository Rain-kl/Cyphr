// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package hub provides thread-safe agent session registry and connection management.
package hub

import (
	"Wavelet/transcribe/plugins/svr/model/do"
	"errors"
	"sync"
	"time"
)

// WSConn defines the minimal interface for a WebSocket connection.
type WSConn interface {
	WriteJSON(v any) error
	Close() error
}

// AgentSession represents an active connected agent node session in memory.
type AgentSession struct {
	NodeID   uint64
	NodeName string
	Conn     WSConn
	IP       string

	mu            sync.RWMutex
	writeMu       sync.Mutex
	loadedModels  []string
	runningJobs   int
	lastHeartbeat time.Time
	systemStats   *do.SystemStatsDTO
}

// NewAgentSession creates a new AgentSession instance.
func NewAgentSession(nodeID uint64, nodeName, ip string, conn WSConn) *AgentSession {
	return &AgentSession{
		NodeID:        nodeID,
		NodeName:      nodeName,
		Conn:          conn,
		IP:            ip,
		lastHeartbeat: time.Now(),
		loadedModels:  make([]string, 0),
	}
}

// UpdateHeartbeat updates session heartbeat timestamp, models, job count and system stats.
func (s *AgentSession) UpdateHeartbeat(models []string, runningJobs int, stats *do.SystemStatsDTO) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = time.Now()
	if models != nil {
		s.loadedModels = make([]string, len(models))
		copy(s.loadedModels, models)
	}
	s.runningJobs = runningJobs
	s.systemStats = stats
}

// GetLastHeartbeat returns the timestamp of the last received heartbeat.
func (s *AgentSession) GetLastHeartbeat() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastHeartbeat
}

// SetLastHeartbeat explicitly sets the last heartbeat timestamp (useful for testing timeout).
func (s *AgentSession) SetLastHeartbeat(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = t
}

// GetLoadedModels returns a copy of currently loaded model names.
func (s *AgentSession) GetLoadedModels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.loadedModels))
	copy(result, s.loadedModels)
	return result
}

// HasModel checks if a specific model is loaded in this session.
func (s *AgentSession) HasModel(modelName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.loadedModels {
		if m == modelName {
			return true
		}
	}
	return false
}

// GetRunningJobs returns the count of running jobs on this session.
func (s *AgentSession) GetRunningJobs() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runningJobs
}

// IncrementRunningJobs increments running job count.
func (s *AgentSession) IncrementRunningJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runningJobs++
}

// DecrementRunningJobs decrements running job count without going below zero.
func (s *AgentSession) DecrementRunningJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningJobs > 0 {
		s.runningJobs--
	}
}

// GetSystemStats returns the latest reported system utilization statistics.
func (s *AgentSession) GetSystemStats() *do.SystemStatsDTO {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.systemStats
}

// WriteJSON sends a JSON-serializable message over the WebSocket connection thread-safely.
func (s *AgentSession) WriteJSON(v any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.Conn == nil {
		return errors.New("connection is nil")
	}
	return s.Conn.WriteJSON(v)
}

// Close closes the underlying WebSocket connection thread-safely.
func (s *AgentSession) Close() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.Conn != nil {
		return s.Conn.Close()
	}
	return nil
}
