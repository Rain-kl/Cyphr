// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package scheduler implements task assignment and dispatching algorithms for transcribe jobs.
package scheduler

import (
	"Wavelet/pkg/logger"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"context"
	"fmt"
	"sync"
	"time"
)

// Scheduler defines the contract for scheduling pending transcription tasks to nodes.
type Scheduler interface {
	SchedulePendingJobs(ctx context.Context) error
}

// DefaultScheduler coordinates pending jobs with active agent sessions.
type DefaultScheduler struct {
	mu       sync.Mutex
	jobDAO   dao.JobDAO
	agentHub hub.AgentHub
}

var _ Scheduler = (*DefaultScheduler)(nil)

// NewScheduler creates a new DefaultScheduler instance.
func NewScheduler(jobDAO dao.JobDAO, agentHub hub.AgentHub) *DefaultScheduler {
	return &DefaultScheduler{
		jobDAO:   jobDAO,
		agentHub: agentHub,
	}
}

// SchedulePendingJobs executes a pass over all pending jobs:
// 1. Prioritizes nodes that already have the required model loaded and have the lowest running job count.
// 2. If no node has the model loaded, sends a load_model command to the least loaded idle node.
// 3. If no active nodes are available, leaves jobs in pending state.
func (s *DefaultScheduler) SchedulePendingJobs(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pendingJobs, err := s.jobDAO.ListPendingJobs(ctx)
	if err != nil {
		return err
	}
	if len(pendingJobs) == 0 {
		return nil
	}

	activeSessions := s.agentHub.ListActiveSessions()
	if len(activeSessions) == 0 {
		return nil
	}

	modelsRequested := make(map[string]bool)

	for _, job := range pendingJobs {
		// 1. Search for nodes with the required model already loaded
		var modelNodes []*hub.AgentSession
		for _, sess := range activeSessions {
			if sess.HasModel(job.ModelName) {
				modelNodes = append(modelNodes, sess)
			}
		}

		if len(modelNodes) > 0 {
			bestNode := findLeastLoadedNode(modelNodes)
			if err := s.dispatchJob(ctx, job.ID, job.ModelName, job.TaskType, bestNode); err != nil {
				logger.ErrorF(ctx, "[Scheduler] failed to dispatch job %d to node %d: %v", job.ID, bestNode.NodeID, err)
				continue
			}
			bestNode.IncrementRunningJobs()
			continue
		}

		// 2. No node has this model loaded: pick the least loaded node to instruct load_model
		if !modelsRequested[job.ModelName] {
			idleNode := findLeastLoadedNode(activeSessions)
			if idleNode != nil {
				loadMsg := do.WSMessage{
					Type:   "command",
					Action: "load_model",
					Seq:    time.Now().UnixNano(),
					Payload: do.LoadModelPayload{
						ModelName: job.ModelName,
					},
				}
				if err := s.agentHub.BroadcastToNode(idleNode.NodeID, loadMsg); err != nil {
					logger.ErrorF(ctx, "[Scheduler] failed to send load_model for %s to node %d: %v", job.ModelName, idleNode.NodeID, err)
				} else {
					modelsRequested[job.ModelName] = true
					logger.InfoF(ctx, "[Scheduler] sent load_model for %s to node %d", job.ModelName, idleNode.NodeID)
				}
			}
		}
		// Job remains in pending status
	}

	return nil
}

func (s *DefaultScheduler) dispatchJob(ctx context.Context, jobID uint64, modelName, taskType string, targetNode *hub.AgentSession) error {
	// First update job status and node assignment in DB
	if err := s.jobDAO.UpdateNodeID(ctx, jobID, targetNode.NodeID, consts.StatusRunning); err != nil {
		return err
	}

	dispatchMsg := do.WSMessage{
		Type:   "command",
		Action: "dispatch_job",
		Seq:    time.Now().UnixNano(),
		Payload: do.DispatchJobPayload{
			JobID:     jobID,
			ModelName: modelName,
			TaskType:  taskType,
			MediaPath: fmt.Sprintf("/api/v1/agent/jobs/%d/media", jobID),
		},
	}

	if err := s.agentHub.BroadcastToNode(targetNode.NodeID, dispatchMsg); err != nil {
		// Roll back job status to pending if broadcast failed
		_ = s.jobDAO.UpdateNodeID(ctx, jobID, 0, consts.StatusPending)
		return err
	}

	logger.InfoF(ctx, "[Scheduler] dispatched job %d (model: %s) to node %d (%s)", jobID, modelName, targetNode.NodeID, targetNode.NodeName)
	return nil
}

// findLeastLoadedNode selects the session with the lowest running jobs; ties are broken by CPU percent.
func findLeastLoadedNode(sessions []*hub.AgentSession) *hub.AgentSession {
	if len(sessions) == 0 {
		return nil
	}

	best := sessions[0]
	bestJobs := best.GetRunningJobs()
	bestCPU := getCPUPercent(best)

	for i := 1; i < len(sessions); i++ {
		curr := sessions[i]
		currJobs := curr.GetRunningJobs()
		currCPU := getCPUPercent(curr)

		if currJobs < bestJobs || (currJobs == bestJobs && currCPU < bestCPU) {
			best = curr
			bestJobs = currJobs
			bestCPU = currCPU
		}
	}

	return best
}

func getCPUPercent(sess *hub.AgentSession) float64 {
	stats := sess.GetSystemStats()
	if stats != nil {
		return stats.CPUPercent
	}
	return 0.0
}
