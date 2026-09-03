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
	DefaultModelName = "mock-whisper-base"
)

// Token constants.
const (
	AgentTokenPrefix = "agt_"
)

// Response format constants.
const (
	ResponseFormatJSON        = "json"
	ResponseFormatVerboseJSON = "verbose_json"
	ResponseFormatText        = "text"
	ResponseFormatSRT         = "srt"
	ResponseFormatVTT         = "vtt"
)
