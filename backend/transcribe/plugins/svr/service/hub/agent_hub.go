// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package hub

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"context"
	"errors"
	"sync"
	"time"
)

const (
	// DefaultHeartbeatTimeout is the threshold after which an agent is considered disconnected.
	DefaultHeartbeatTimeout = 15 * time.Second
	// DefaultWatchdogInterval is the frequency at which the watchdog scans for stale sessions.
	DefaultWatchdogInterval = 5 * time.Second
)

// AgentHub defines the interface for the in-memory agent session hub.
type AgentHub interface {
	RegisterSession(sess *AgentSession)
	UnregisterSession(nodeID uint64)
	GetSession(nodeID uint64) (*AgentSession, bool)
	ListActiveSessions() []*AgentSession
	BroadcastToNode(nodeID uint64, msg any) error
	StartWatchdog(ctx context.Context, checkInterval, timeout time.Duration)
	Stop()
	OnNodeDisconnected(fn func(nodeID uint64))
}

// DefaultAgentHub implements AgentHub with thread-safe session storage and watchdog.
type DefaultAgentHub struct {
	mu            sync.RWMutex
	sessions      map[uint64]*AgentSession
	jobDAO        dao.JobDAO
	disconnectFns []func(nodeID uint64)
	disconnectMu  sync.RWMutex
	stopCh        chan struct{}
	stopOnce      sync.Once
}

var _ AgentHub = (*DefaultAgentHub)(nil)

// NewAgentHub creates a new DefaultAgentHub instance.
func NewAgentHub(jobDAO dao.JobDAO) *DefaultAgentHub {
	return &DefaultAgentHub{
		sessions:      make(map[uint64]*AgentSession),
		jobDAO:        jobDAO,
		disconnectFns: make([]func(nodeID uint64), 0),
		stopCh:        make(chan struct{}),
	}
}

// RegisterSession registers a newly connected agent session. If a session for the node already exists,
// the old connection is closed and replaced.
func (h *DefaultAgentHub) RegisterSession(sess *AgentSession) {
	if sess == nil {
		return
	}

	var oldSess *AgentSession
	h.mu.Lock()
	if existing, found := h.sessions[sess.NodeID]; found {
		oldSess = existing
	}
	h.sessions[sess.NodeID] = sess
	h.mu.Unlock()

	if oldSess != nil {
		_ = oldSess.Close()
	}
}

// UnregisterSession removes a session, closes its connection, and resets any running jobs on that node back to pending.
func (h *DefaultAgentHub) UnregisterSession(nodeID uint64) {
	h.mu.Lock()
	sess, found := h.sessions[nodeID]
	if found {
		delete(h.sessions, nodeID)
	}
	h.mu.Unlock()

	if found && sess != nil {
		_ = sess.Close()
		h.handleNodeDisconnect(context.Background(), nodeID)
	}
}

// GetSession returns the active session for a node ID if present.
func (h *DefaultAgentHub) GetSession(nodeID uint64) (*AgentSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	sess, ok := h.sessions[nodeID]
	return sess, ok
}

// ListActiveSessions returns a snapshot slice of all currently registered agent sessions.
func (h *DefaultAgentHub) ListActiveSessions() []*AgentSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]*AgentSession, 0, len(h.sessions))
	for _, s := range h.sessions {
		list = append(list, s)
	}
	return list
}

// BroadcastToNode sends a message payload to a specific node over its WebSocket connection.
func (h *DefaultAgentHub) BroadcastToNode(nodeID uint64, msg any) error {
	sess, ok := h.GetSession(nodeID)
	if !ok {
		return errors.New(consts.ErrNodeOffline)
	}
	return sess.WriteJSON(msg)
}

// OnNodeDisconnected registers a callback to be invoked when a node is disconnected.
func (h *DefaultAgentHub) OnNodeDisconnected(fn func(nodeID uint64)) {
	if fn == nil {
		return
	}
	h.disconnectMu.Lock()
	defer h.disconnectMu.Unlock()
	h.disconnectFns = append(h.disconnectFns, fn)
}

func (h *DefaultAgentHub) handleNodeDisconnect(ctx context.Context, nodeID uint64) {
	if h.jobDAO != nil {
		if err := h.jobDAO.ResetRunningJobsToPending(ctx, nodeID); err != nil {
			logger.ErrorF(ctx, "[AgentHub] failed to reset running jobs for node %d: %v", nodeID, err)
		}
	}

	h.disconnectMu.RLock()
	callbacks := make([]func(nodeID uint64), len(h.disconnectFns))
	copy(callbacks, h.disconnectFns)
	h.disconnectMu.RUnlock()

	for _, fn := range callbacks {
		fn(nodeID)
	}
}

// StartWatchdog begins a periodic background watchdog to detect timed-out agent sessions.
func (h *DefaultAgentHub) StartWatchdog(ctx context.Context, checkInterval, timeout time.Duration) {
	if checkInterval <= 0 {
		checkInterval = DefaultWatchdogInterval
	}
	if timeout <= 0 {
		timeout = DefaultHeartbeatTimeout
	}

	util.Go(func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-h.stopCh:
				return
			case now := <-ticker.C:
				h.checkStaleSessions(ctx, now, timeout)
			}
		}
	})
}

func (h *DefaultAgentHub) checkStaleSessions(ctx context.Context, now time.Time, timeout time.Duration) {
	var timedOutSessions []*AgentSession

	h.mu.Lock()
	for nodeID, sess := range h.sessions {
		if now.Sub(sess.GetLastHeartbeat()) > timeout {
			timedOutSessions = append(timedOutSessions, sess)
			delete(h.sessions, nodeID)
		}
	}
	h.mu.Unlock()

	for _, sess := range timedOutSessions {
		logger.WarnF(ctx, "[AgentHub] agent node %d (%s) timed out, disconnecting", sess.NodeID, sess.NodeName)
		_ = sess.Close()
		h.handleNodeDisconnect(ctx, sess.NodeID)
	}
}

// Stop stops the hub watchdog and releases resources.
func (h *DefaultAgentHub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})
}
