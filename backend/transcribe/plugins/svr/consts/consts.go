// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants and error codes for transcribe svr plugin.
package consts

// Job status constants.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Task type constants.
const (
	TaskTypeASR = "asr"
)

// Model constants.
const (
	DefaultModelName = "qwen3-asr-0.6b"
	ModelQwen3ASR06B = "qwen3-asr-0.6b"
	ModelQwen3ASR17B = "qwen3-asr-1.7b"
)

// Token constants.
const (
	AgentTokenPrefix = "agt_"
)

// Work mode constants.
const (
	WorkModeCPU = "cpu"
	WorkModeGPU = "gpu"
)

// Response format constants.
const (
	ResponseFormatJSON        = "json"
	ResponseFormatVerboseJSON = "verbose_json"
	ResponseFormatText        = "text"
	ResponseFormatSRT         = "srt"
	ResponseFormatVTT         = "vtt"
)
