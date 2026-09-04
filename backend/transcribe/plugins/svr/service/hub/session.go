// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package hub provides thread-safe agent session registry and connection management.
package hub

import (
	"Wavelet/transcribe/plugins/svr/consts"
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

	mu                 sync.RWMutex
	writeMu            sync.Mutex
	workMode           string
	supportedModes     []string
	currentMode        string
	allowAutoLoad      bool
	autoUnloadMinutes  int
	maxConcurrentJobs  int
	advertisedCapacity int
	modelVramEstimates map[string]int
	idleSince          time.Time
	failedLoads        map[string]time.Time
	loadedModels       []string
	downloadedModels   []string
	runningJobs        int
	lastHeartbeat      time.Time
	systemStats        *do.SystemStatsDTO
}

// NewAgentSession creates a new AgentSession instance.
func NewAgentSession(nodeID uint64, nodeName, ip string, conn WSConn) *AgentSession {
	return &AgentSession{
		NodeID:             nodeID,
		NodeName:           nodeName,
		Conn:               conn,
		IP:                 ip,
		workMode:           "gpu",
		supportedModes:     []string{"cpu"},
		currentMode:        "cpu",
		allowAutoLoad:      true,
		autoUnloadMinutes:  0,
		maxConcurrentJobs:  consts.DefaultMaxConcurrentJobs,
		modelVramEstimates: make(map[string]int),
		failedLoads:        make(map[string]time.Time),
		lastHeartbeat:      time.Now(),
		loadedModels:       make([]string, 0),
		downloadedModels:   make([]string, 0),
	}
}

// SetConfig updates node configuration in session memory.
func (s *AgentSession) SetConfig(workMode string, allowAutoLoad bool, autoUnloadMinutes int, maxConcurrentJobs int, estimates map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workMode != "" {
		s.workMode = workMode
	}
	s.allowAutoLoad = allowAutoLoad
	s.autoUnloadMinutes = autoUnloadMinutes
	if maxConcurrentJobs == consts.DynamicMaxConcurrentJobs || maxConcurrentJobs > 0 {
		s.maxConcurrentJobs = maxConcurrentJobs
	}
	if estimates != nil {
		s.modelVramEstimates = make(map[string]int, len(estimates))
		for k, v := range estimates {
			s.modelVramEstimates[k] = v
		}
	} else {
		s.modelVramEstimates = make(map[string]int)
	}
}

// GetWorkMode returns the configured work mode (gpu or cpu).
func (s *AgentSession) GetWorkMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.workMode == "" {
		return "gpu"
	}
	return s.workMode
}

// SetModes updates supported and current modes reported by the agent.
func (s *AgentSession) SetModes(supported []string, current string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(supported) > 0 {
		s.supportedModes = make([]string, len(supported))
		copy(s.supportedModes, supported)
	}
	if current != "" {
		s.currentMode = current
	}
}

// GetSupportedModes returns available inference modes reported by agent.
func (s *AgentSession) GetSupportedModes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.supportedModes))
	copy(res, s.supportedModes)
	return res
}

// GetCurrentMode returns the current active inference mode on agent.
func (s *AgentSession) GetCurrentMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMode
}

// SupportsMode checks if the agent hardware supports the specified mode.
func (s *AgentSession) SupportsMode(mode string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.supportedModes) == 0 {
		return true
	}
	for _, m := range s.supportedModes {
		if m == mode {
			return true
		}
	}
	return false
}

// GetAllowAutoLoad returns whether auto-load is permitted on this node.
func (s *AgentSession) GetAllowAutoLoad() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowAutoLoad
}

// GetAutoUnloadMinutes returns the idle unload threshold in minutes.
func (s *AgentSession) GetAutoUnloadMinutes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autoUnloadMinutes
}

// GetModelVramEstimate returns the estimated VRAM (MB) for model, or 0 if unconfigured.
func (s *AgentSession) GetModelVramEstimate(modelName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelVramEstimates[modelName]
}

// GetModelVramEstimatesMap returns a copy of all configured model VRAM estimates.
func (s *AgentSession) GetModelVramEstimatesMap() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]int, len(s.modelVramEstimates))
	for k, v := range s.modelVramEstimates {
		res[k] = v
	}
	return res
}

// GetFreeVRAM returns available free GPU memory in MB, or 0 if GPU stats unavailable.
func (s *AgentSession) GetFreeVRAM() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.systemStats == nil || s.systemStats.GPUMemoryTotalMB == 0 {
		return 0
	}
	if s.systemStats.GPUMemoryTotalMB > s.systemStats.GPUMemoryUsedMB {
		return s.systemStats.GPUMemoryTotalMB - s.systemStats.GPUMemoryUsedMB
	}
	return 0
}

// UpdateHeartbeat updates session heartbeat timestamp, models, job count and system stats.
func (s *AgentSession) UpdateHeartbeat(models []string, runningJobs int, stats *do.SystemStatsDTO, downloadedModels ...[]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = time.Now()
	if models != nil {
		s.loadedModels = make([]string, len(models))
		copy(s.loadedModels, models)
	}
	if len(downloadedModels) > 0 && downloadedModels[0] != nil {
		s.downloadedModels = make([]string, len(downloadedModels[0]))
		copy(s.downloadedModels, downloadedModels[0])
	}
	s.runningJobs = runningJobs
	s.systemStats = stats

	// Maintain idle tracking
	if s.runningJobs == 0 && len(s.loadedModels) > 0 {
		if s.idleSince.IsZero() {
			s.idleSince = time.Now()
		}
	} else {
		s.idleSince = time.Time{}
	}
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

// GetDownloadedModels returns a copy of downloaded model names.
func (s *AgentSession) GetDownloadedModels() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.downloadedModels))
	copy(result, s.downloadedModels)
	return result
}

// SetDownloadedModels sets the downloaded model names.
func (s *AgentSession) SetDownloadedModels(models []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.downloadedModels = make([]string, len(models))
	copy(s.downloadedModels, models)
}

// IsDynamicMode reports whether this session uses agent-advertised capacity (-1).
func (s *AgentSession) IsDynamicMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxConcurrentJobs == consts.DynamicMaxConcurrentJobs
}

// SetAdvertisedCapacity stores the agent-reported capacity (clamped to sane range).
func (s *AgentSession) SetAdvertisedCapacity(cap int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cap < consts.DynamicCapacityMin {
		cap = consts.DynamicCapacityMin
	}
	if cap > consts.DynamicCapacityMax {
		cap = consts.DynamicCapacityMax
	}
	s.advertisedCapacity = cap
}

// GetAdvertisedCapacity returns the last agent-reported capacity (0 if never reported).
func (s *AgentSession) GetAdvertisedCapacity() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.advertisedCapacity
}

// effectiveCapacity returns dynamic advertised capacity or the static limit.
func (s *AgentSession) effectiveCapacity() int {
	if s.maxConcurrentJobs == consts.DynamicMaxConcurrentJobs {
		if s.advertisedCapacity > 0 {
			return s.advertisedCapacity
		}
		return consts.DefaultMaxConcurrentJobs
	}
	if s.maxConcurrentJobs <= 0 {
		return consts.DefaultMaxConcurrentJobs
	}
	return s.maxConcurrentJobs
}

// GetRemainingCapacity returns effective capacity minus running jobs.
func (s *AgentSession) GetRemainingCapacity() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.effectiveCapacity() - s.runningJobs
}

// isCapacityFreshLocked reports whether the last heartbeat is recent enough to trust.
func (s *AgentSession) isCapacityFreshLocked() bool {
	if s.lastHeartbeat.IsZero() {
		return false
	}
	return time.Since(s.lastHeartbeat) < time.Duration(consts.DynamicCapacityStaleSeconds)*time.Second
}

// CanAcceptJob reports whether the scheduler may dispatch one more job here.
func (s *AgentSession) CanAcceptJob() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.maxConcurrentJobs == consts.DynamicMaxConcurrentJobs {
		return s.isCapacityFreshLocked() && s.effectiveCapacity()-s.runningJobs > 0
	}
	return s.runningJobs < s.effectiveCapacity()
}

// GetMaxConcurrentJobs returns the maximum concurrent jobs limit for this session.
func (s *AgentSession) GetMaxConcurrentJobs() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.maxConcurrentJobs <= 0 {
		return consts.DefaultMaxConcurrentJobs
	}
	return s.maxConcurrentJobs
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

// IncrementRunningJobs increments running job count and resets idle timer.
func (s *AgentSession) IncrementRunningJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runningJobs++
	s.idleSince = time.Time{}
}

// DecrementRunningJobs decrements running job count and initializes idle timer if now idle.
func (s *AgentSession) DecrementRunningJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningJobs > 0 {
		s.runningJobs--
	}
	if s.runningJobs == 0 && len(s.loadedModels) > 0 {
		if s.idleSince.IsZero() {
			s.idleSince = time.Now()
		}
	}
}

// GetIdleSince returns the timestamp when this node entered idle state.
func (s *AgentSession) GetIdleSince() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idleSince
}

// SetIdleSince sets the idle since timestamp (useful for testing or resetting).
func (s *AgentSession) SetIdleSince(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idleSince = t
}

// CheckIdleTimeout returns true if the node has been idle for >= autoUnloadMinutes.
func (s *AgentSession) CheckIdleTimeout(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.autoUnloadMinutes <= 0 {
		return false
	}
	if s.runningJobs > 0 || len(s.loadedModels) == 0 {
		return false
	}
	if s.idleSince.IsZero() {
		return false
	}
	timeout := time.Duration(s.autoUnloadMinutes) * time.Minute
	return now.Sub(s.idleSince) >= timeout
}

// RecordModelLoadFailure records a failed model load timestamp for cooling off.
func (s *AgentSession) RecordModelLoadFailure(modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedLoads[modelName] = time.Now()
}

// IsModelCoolingOff checks if a model failed recently within the specified cooldown duration.
func (s *AgentSession) IsModelCoolingOff(modelName string, cooldown time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.failedLoads[modelName]
	if !ok {
		return false
	}
	return time.Since(t) < cooldown
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
