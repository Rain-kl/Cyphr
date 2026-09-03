// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"time"
)

// AccessLogFilterDTO defines filter criteria for querying user access logs.
type AccessLogFilterDTO struct {
	UserIDs   []uint64
	Path      string
	StartTime *time.Time
	EndTime   *time.Time
}

// AccessLogDTO represents a single access log entry.
type AccessLogDTO struct {
	ID        uint64    `json:"id,string"`
	UserID    uint64    `json:"user_id,string"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int32     `json:"status"`
	Latency   int64     `json:"latency"`
	CreatedAt time.Time `json:"created_at"`
}

// AccessLogDailyStatsDTO represents aggregate access statistics for a single day.
type AccessLogDailyStatsDTO struct {
	Date         string `json:"date"`
	PV           uint64 `json:"pv"`
	UV           uint64 `json:"uv"`
	IPCount      uint64 `json:"ip_count"`
	ErrorCount   uint64 `json:"error_count"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
	SlowReqCount uint64 `json:"slow_req_count"`
	MaxLatencyMs int64  `json:"max_latency_ms"`
	P95LatencyMs int64  `json:"p95_latency_ms"`
	P99LatencyMs int64  `json:"p99_latency_ms"`
}

// RiskControlService defines the contract for accessing security risk control and audit logstore.
type RiskControlService interface {
	// QueryAccessLogs retrieves paginated access logs matching the filter.
	QueryAccessLogs(ctx context.Context, filter AccessLogFilterDTO, page, pageSize int) ([]AccessLogDTO, uint64, error)

	// QueryAccessLogStats returns aggregate daily statistics for the last N days.
	QueryAccessLogStats(ctx context.Context, days int) ([]AccessLogDailyStatsDTO, error)

	// ActiveLogEngine returns the current active logstore engine name.
	ActiveLogEngine(ctx context.Context) string

	// IsLogEngineMigrating reports whether a log engine migration is in progress.
	IsLogEngineMigrating(ctx context.Context) bool

	// Drain flushes pending in-flight log buffers.
	Drain(ctx context.Context) error

	// SwitchLogEngine migrates and switches the active log storage engine.
	SwitchLogEngine(ctx context.Context, targetEngine string) error
}
