// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"time"
)

// TaskParamDTO describes a parameter accepted by a background task.
type TaskParamDTO struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// TaskMetaDTO describes the metadata and configuration of a registered background task.
type TaskMetaDTO struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Params      []TaskParamDTO `json:"params,omitempty"`
	MaxRetry    int            `json:"max_retry"`
	Timeout     time.Duration  `json:"timeout"`
	Queue       string         `json:"queue"`
	Schedule    string         `json:"schedule,omitempty"`
}

// TaskResultDTO represents the outcome of a background task execution.
type TaskResultDTO struct {
	Message string `json:"message"`
	Detail  any    `json:"detail,omitempty"`
}

// TaskExecutionDTO represents a single task execution record.
type TaskExecutionDTO struct {
	ID           uint64     `json:"id,string"`
	TaskID       string     `json:"task_id"`
	TaskType     string     `json:"task_type"`
	TaskName     string     `json:"task_name"`
	Status       string     `json:"status"`
	Retryable    bool       `json:"retryable"`
	MaxRetry     int        `json:"max_retry"`
	RetryCount   int        `json:"retry_count"`
	Log          string     `json:"log"`
	ErrorMessage string     `json:"error_message"`
	Result       string     `json:"result"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Duration     int64      `json:"duration"`
	Payload      string     `json:"payload"`
	TriggeredBy  string     `json:"triggered_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TaskService defines the unified contract for dispatching and tracking background tasks.
type TaskService interface {
	Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error)
	Retry(ctx context.Context, id uint64) (string, error)
	ListTasks() []TaskMetaDTO
	GetTaskMeta(taskType string) (TaskMetaDTO, bool)
	ValidatePayload(taskType string, payload []byte) ([]byte, error)
	ReloadScheduler() error
	AppendLog(ctx context.Context, format string, args ...any)
	ListExecutions(ctx context.Context, taskType string, status string, page, pageSize int) ([]TaskExecutionDTO, int64, error)
	GetExecution(ctx context.Context, id uint64) (*TaskExecutionDTO, error)
}
