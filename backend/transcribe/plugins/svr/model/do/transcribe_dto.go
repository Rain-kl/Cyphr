// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package do provides domain transfer objects for the transcribe plugin.
package do

import "time"

// ModelDTO represents model information.
type ModelDTO struct {
	ID          uint64    `json:"id,string,omitempty"`
	Name        string    `json:"name"`
	TaskType    string    `json:"task_type"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ToggleModelStatusRequest represents model status update payload.
type ToggleModelStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// SystemStatsDTO holds hardware/system utilization stats.
type SystemStatsDTO struct {
	CPUPercent       float64 `json:"cpu_percent"`
	RAMPercent       float64 `json:"ram_percent,omitempty"`
	RAMUsedMB        uint64  `json:"ram_used_mb,omitempty"`
	RAMTotalMB       uint64  `json:"ram_total_mb,omitempty"`
	GPUPercent       float64 `json:"gpu_percent,omitempty"`
	GPUMemoryUsedMB  uint64  `json:"gpu_memory_used_mb,omitempty"`
	GPUMemoryTotalMB uint64  `json:"gpu_memory_total_mb,omitempty"`
}

// NodeDTO represents node information returned in listings and detail views.
type NodeDTO struct {
	ID           uint64          `json:"id,string"`
	Name         string          `json:"name"`
	TokenPrefix  string          `json:"token_prefix"`
	IsActive     bool            `json:"is_active"`
	IsOnline     bool            `json:"is_online,omitempty"`
	LoadedModels []string        `json:"loaded_models,omitempty"`
	RunningJobs  int             `json:"running_jobs,omitempty"`
	System       *SystemStatsDTO `json:"system,omitempty"`
	LastIP       string          `json:"last_ip,omitempty"`
	LastSeenAt   *time.Time      `json:"last_seen_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// NodeCreatedDTO contains node details along with the one-time raw token.
type NodeCreatedDTO struct {
	ID          uint64    `json:"id,string"`
	Name        string    `json:"name"`
	AgentToken  string    `json:"agent_token"`
	TokenPrefix string    `json:"token_prefix"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateNodeRequest represents node creation payload.
type CreateNodeRequest struct {
	Name string `json:"name" binding:"required"`
}

// LoadModelRequest represents manual load/unload model request.
type LoadModelRequest struct {
	ModelName string `json:"model_name" binding:"required"`
}

// JobDTO represents transcription job information.
type JobDTO struct {
	ID               uint64     `json:"id,string"`
	UserID           uint64     `json:"user_id,string,omitempty"`
	NodeID           *uint64    `json:"node_id,string,omitempty"`
	Model            string     `json:"model"`
	TaskType         string     `json:"task_type,omitempty"`
	Status           string     `json:"status"`
	Progress         int        `json:"progress"`
	Duration         float64    `json:"duration"`
	OriginalFileName string     `json:"original_file_name"`
	AudioStoragePath string     `json:"audio_storage_path,omitempty"`
	MediaURL         string     `json:"media_url,omitempty"`
	ResultText       string     `json:"result_text,omitempty"`
	OpenAIResponse   any        `json:"openai_response,omitempty"`
	ErrorMsg         string     `json:"error_msg,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// JobListDTO represents a paginated list of jobs.
type JobListDTO struct {
	Items    []JobDTO `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

// CreateJobRequest represents job creation payload.
type CreateJobRequest struct {
	UserID           uint64  `json:"user_id,string,omitempty"`
	Model            string  `json:"model"`
	TaskType         string  `json:"task_type"`
	AudioStoragePath string  `json:"audio_storage_path"`
	OriginalFileName string  `json:"original_file_name"`
	Language         string  `json:"language,omitempty"`
	Prompt           string  `json:"prompt,omitempty"`
	ResponseFormat   string  `json:"response_format,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
}

// LogMessage represents a SSE log event message.
type LogMessage struct {
	Seq      int    `json:"seq"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

// FinishMessage represents a SSE job finish event.
type FinishMessage struct {
	Status         string  `json:"status"`
	Duration       float64 `json:"duration,omitempty"`
	ResultText     string  `json:"result_text,omitempty"`
	OpenAIResponse any     `json:"openai_response,omitempty"`
	ErrorMsg       string  `json:"error_msg,omitempty"`
}

// AgentLogBatchItem represents a single log line reported by an agent.
type AgentLogBatchItem struct {
	Timestamp string `json:"timestamp,omitempty"`
	Level     string `json:"level,omitempty"`
	Message   string `json:"message"`
}

// AgentLogBatchRequest represents batch logs submitted by an agent.
type AgentLogBatchRequest struct {
	Progress int                 `json:"progress"`
	Logs     []AgentLogBatchItem `json:"logs"`
}

// AgentCompleteRequest represents completion report from an agent.
type AgentCompleteRequest struct {
	Status          string  `json:"status"`
	DurationSeconds float64 `json:"duration_seconds"`
	ResultText      string  `json:"result_text"`
	OpenAIResponse  any     `json:"openai_response,omitempty"`
	ErrorMsg        string  `json:"error_msg,omitempty"`
}

// OpenAITranscriptionResponse represents standard OpenAI JSON format.
type OpenAITranscriptionResponse struct {
	Text string `json:"text"`
}

// OpenAIVerboseTranscriptionResponse represents verbose_json format.
type OpenAIVerboseTranscriptionResponse struct {
	Task     string       `json:"task"`
	Language string       `json:"language"`
	Duration float64      `json:"duration"`
	Text     string       `json:"text"`
	Segments []SegmentDTO `json:"segments,omitempty"`
}

// SegmentDTO represents a transcription segment in verbose_json format.
type SegmentDTO struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

// WSMessage represents a WebSocket signaling envelope between controller and agent.
type WSMessage struct {
	Type    string `json:"type"`
	Action  string `json:"action,omitempty"`
	Seq     int64  `json:"seq,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

// DispatchJobPayload represents payload for dispatch_job command.
type DispatchJobPayload struct {
	JobID     uint64 `json:"job_id,string"`
	ModelName string `json:"model_name"`
	TaskType  string `json:"task_type"`
	Language  string `json:"language,omitempty"`
	MediaPath string `json:"media_path"`
}

// LoadModelPayload represents payload for load_model command.
type LoadModelPayload struct {
	ModelName string `json:"model_name"`
}

// UnloadModelPayload represents payload for unload_model command.
type UnloadModelPayload struct {
	ModelName string `json:"model_name"`
}
