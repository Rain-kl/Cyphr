// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"context"
	"errors"
	"fmt"
)

type inprocTaskService struct {
	taskReg extpoints.TaskExtension
}

func newInprocTaskService(taskReg extpoints.TaskExtension) contracts.TaskService {
	return &inprocTaskService{
		taskReg: taskReg,
	}
}

func (s *inprocTaskService) Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	return DispatchTask(ctx, taskType, payload, triggeredBy)
}

func (s *inprocTaskService) Retry(_ context.Context, id uint64) (string, error) {
	return fmt.Sprintf("inproc_retry_%d", id), nil
}

func (s *inprocTaskService) ListTasks() []contracts.TaskMetaDTO {
	if s.taskReg == nil {
		return nil
	}
	tasks := s.taskReg.Tasks()
	res := make([]contracts.TaskMetaDTO, 0, len(tasks))
	for _, td := range tasks {
		res = append(res, td.ToDTO())
	}
	return res
}

func (s *inprocTaskService) GetTaskMeta(taskType string) (contracts.TaskMetaDTO, bool) {
	if s.taskReg == nil {
		return contracts.TaskMetaDTO{}, false
	}
	for _, td := range s.taskReg.Tasks() {
		if td.Type == taskType || td.Pattern == taskType {
			return td.ToDTO(), true
		}
	}
	return contracts.TaskMetaDTO{}, false
}

func (s *inprocTaskService) ValidatePayload(_ string, payload []byte) ([]byte, error) {
	return payload, nil
}

func (s *inprocTaskService) ReloadScheduler() error {
	return nil
}

func (s *inprocTaskService) AppendLog(_ context.Context, _ string, _ ...any) {
}

func (s *inprocTaskService) ListExecutions(_ context.Context, _, _ string, _, _ int) ([]contracts.TaskExecutionDTO, int64, error) {
	return []contracts.TaskExecutionDTO{}, 0, nil
}

func (s *inprocTaskService) GetExecution(_ context.Context, _ uint64) (*contracts.TaskExecutionDTO, error) {
	return nil, errors.New("driver_inproc_worker: task executions are not tracked")
}
