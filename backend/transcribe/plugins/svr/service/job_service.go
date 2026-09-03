// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service provides business operations and domain services for transcribe.
package service

import (
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/dao"
	"Wavelet/transcribe/plugins/svr/model/do"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"Wavelet/transcribe/plugins/svr/service/hub"
	"Wavelet/transcribe/plugins/svr/service/scheduler"
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// JobService defines business operations for transcription jobs.
type JobService interface {
	CreateJob(ctx context.Context, req *do.CreateJobRequest) (*do.JobDTO, error)
	GetJobDetail(ctx context.Context, id uint64) (*do.JobDTO, error)
	ListJobs(ctx context.Context, uid uint64, page, size int, status ...string) (*do.JobListDTO, error)
	AppendLogs(ctx context.Context, jobID uint64, req *do.AgentLogBatchRequest) error
	CompleteJob(ctx context.Context, jobID uint64, req *do.AgentCompleteRequest) error
	CancelJob(ctx context.Context, id uint64) error
}

// DefaultJobService implements JobService.
type DefaultJobService struct {
	jobDAO    dao.JobDAO
	modelDAO  dao.ModelDAO
	scheduler scheduler.Scheduler
	logBroker LogBroker
	agentHub  hub.AgentHub
}

var _ JobService = (*DefaultJobService)(nil)

// NewJobService creates a new DefaultJobService instance.
func NewJobService(jobDAO dao.JobDAO, modelDAO dao.ModelDAO, sched scheduler.Scheduler, logBroker LogBroker, agentHub ...hub.AgentHub) *DefaultJobService {
	svc := &DefaultJobService{
		jobDAO:    jobDAO,
		modelDAO:  modelDAO,
		scheduler: sched,
		logBroker: logBroker,
	}
	if len(agentHub) > 0 {
		svc.agentHub = agentHub[0]
	}
	return svc
}

// CreateJob validates the request, records a pending job in the database, and kicks off the scheduler.
func (s *DefaultJobService) CreateJob(ctx context.Context, req *do.CreateJobRequest) (*do.JobDTO, error) {
	if req == nil {
		return nil, errors.New(consts.ErrBindParamsFailed)
	}

	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = consts.DefaultModelName
	}

	taskType := strings.TrimSpace(req.TaskType)
	if taskType == "" {
		taskType = consts.TaskTypeASR
	}

	storagePath := strings.TrimSpace(req.AudioStoragePath)
	if storagePath == "" {
		return nil, errors.New(consts.ErrBindParamsFailed)
	}

	// Validate model exists and is active if modelDAO is provided
	if s.modelDAO != nil {
		model, err := s.modelDAO.GetByName(ctx, modelName)
		if err != nil || !model.IsActive {
			return nil, errors.New(consts.ErrModelUnavailable)
		}
	}

	job := &entity.JobEntity{
		UserID:           req.UserID,
		ModelName:        modelName,
		TaskType:         taskType,
		AudioStoragePath: storagePath,
		OriginalFileName: req.OriginalFileName,
		Status:           consts.StatusPending,
		Progress:         0,
	}

	if err := s.jobDAO.Create(ctx, job); err != nil {
		return nil, err
	}

	// Trigger asynchronous scheduler pass
	if s.scheduler != nil {
		schedCtx := context.WithoutCancel(ctx)
		util.Go(func() {
			if err := s.scheduler.SchedulePendingJobs(schedCtx); err != nil {
				logger.ErrorF(schedCtx, "[JobService] scheduler pass failed: %v", err)
			}
		})
	}

	return s.toJobDTO(job), nil
}

// GetJobDetail fetches a job by ID, unpacking its OpenAI response JSON if completed.
func (s *DefaultJobService) GetJobDetail(ctx context.Context, id uint64) (*do.JobDTO, error) {
	job, err := s.jobDAO.GetByID(ctx, id)
	if err != nil {
		return nil, consts.ErrJobNotFound
	}
	return s.toJobDTO(job), nil
}

// ListJobs returns a paginated list of jobs for a user, optionally filtered by status.
func (s *DefaultJobService) ListJobs(ctx context.Context, uid uint64, page, size int, status ...string) (*do.JobListDTO, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	var st string
	if len(status) > 0 {
		st = status[0]
	}

	jobs, total, err := s.jobDAO.ListByUserID(ctx, uid, page, size, st)
	if err != nil {
		return nil, err
	}

	items := make([]do.JobDTO, 0, len(jobs))
	for i := range jobs {
		dto := s.toJobDTO(&jobs[i])
		items = append(items, *dto)
	}

	return &do.JobListDTO{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: size,
	}, nil
}

// AppendLogs records a batch of logs from an agent into the database and broadcasts them to SSE subscribers.
func (s *DefaultJobService) AppendLogs(ctx context.Context, jobID uint64, req *do.AgentLogBatchRequest) error {
	if req == nil || len(req.Logs) == 0 {
		return nil
	}

	logEntities := make([]entity.JobLogEntity, 0, len(req.Logs))
	for _, item := range req.Logs {
		logEntities = append(logEntities, entity.JobLogEntity{
			JobID:    jobID,
			Progress: req.Progress,
			Message:  item.Message,
		})
	}

	if err := s.jobDAO.AppendLogs(ctx, jobID, logEntities); err != nil {
		return err
	}

	// Fan out to active SSE subscribers via LogBroker
	if s.logBroker != nil {
		for _, item := range req.Logs {
			s.logBroker.Publish(jobID, do.LogMessage{
				Progress: req.Progress,
				Message:  item.Message,
			})
		}
	}

	return nil
}

// CompleteJob records the transcription results, notifies SSE subscribers of completion,
// and schedules any waiting pending jobs.
func (s *DefaultJobService) CompleteJob(ctx context.Context, jobID uint64, req *do.AgentCompleteRequest) error {
	if req == nil {
		return errors.New(consts.ErrBindParamsFailed)
	}

	job, err := s.jobDAO.GetByID(ctx, jobID)
	if err != nil {
		return consts.ErrJobNotFound
	}

	status := req.Status
	if status != consts.StatusCompleted && status != consts.StatusFailed {
		status = consts.StatusCompleted
	}

	var resultJSON string
	if req.OpenAIResponse != nil {
		bytes, err := json.Marshal(req.OpenAIResponse)
		if err == nil {
			resultJSON = string(bytes)
		}
	}

	if err := s.jobDAO.UpdateCompletion(ctx, jobID, status, req.DurationSeconds, req.ResultText, resultJSON, req.ErrorMsg); err != nil {
		return err
	}

	// Decrement node session running jobs counter if session exists
	if job.NodeID != nil && s.agentHub != nil {
		if sess, ok := s.agentHub.GetSession(*job.NodeID); ok {
			sess.DecrementRunningJobs()
		}
	}

	// Broadcast finish event
	if s.logBroker != nil {
		s.logBroker.PublishFinish(jobID, do.FinishMessage{
			Status:     status,
			Duration:   req.DurationSeconds,
			ResultText: req.ResultText,
			ErrorMsg:   req.ErrorMsg,
		})
		s.logBroker.CloseJob(jobID)
	}

	// Schedule pending jobs now that capacity is freed
	if s.scheduler != nil {
		schedCtx := context.WithoutCancel(ctx)
		util.Go(func() {
			if err := s.scheduler.SchedulePendingJobs(schedCtx); err != nil {
				logger.ErrorF(schedCtx, "[JobService] scheduler pass after job completion failed: %v", err)
			}
		})
	}

	return nil
}

// CancelJob marks an active or pending job as failed due to cancellation.
func (s *DefaultJobService) CancelJob(ctx context.Context, id uint64) error {
	job, err := s.jobDAO.GetByID(ctx, id)
	if err != nil {
		return consts.ErrJobNotFound
	}

	if job.Status == consts.StatusCompleted || job.Status == consts.StatusFailed {
		return errors.New(consts.ErrInvalidStatus)
	}

	if err := s.jobDAO.UpdateStatus(ctx, id, consts.StatusFailed); err != nil {
		return err
	}

	// Decrement node session running jobs counter if session exists
	if job.NodeID != nil && s.agentHub != nil {
		if sess, ok := s.agentHub.GetSession(*job.NodeID); ok {
			sess.DecrementRunningJobs()
		}
	}

	if s.logBroker != nil {
		s.logBroker.PublishFinish(id, do.FinishMessage{
			Status:   consts.StatusFailed,
			ErrorMsg: "cancelled by user",
		})
		s.logBroker.CloseJob(id)
	}

	return nil
}

func (s *DefaultJobService) toJobDTO(job *entity.JobEntity) *do.JobDTO {
	dto := &do.JobDTO{
		ID:               job.ID,
		UserID:           job.UserID,
		NodeID:           job.NodeID,
		Model:            job.ModelName,
		TaskType:         job.TaskType,
		Status:           job.Status,
		Progress:         job.Progress,
		Duration:         job.DurationSeconds,
		OriginalFileName: job.OriginalFileName,
		ResultText:       job.ResultText,
		ErrorMsg:         job.ErrorMsg,
		CreatedAt:        job.CreatedAt,
		StartedAt:        job.StartedAt,
		CompletedAt:      job.CompletedAt,
	}

	if job.ResultJSON != "" {
		var resp any
		if err := json.Unmarshal([]byte(job.ResultJSON), &resp); err == nil {
			dto.OpenAIResponse = resp
		}
	}

	return dto
}
