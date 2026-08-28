// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import "time"

// AccessLogFilter scopes user access log queries.
type AccessLogFilter struct {
	// UserIDs filters by user IDs. nil means no user filter; an empty slice means no matches.
	UserIDs []uint64
	Path    string
	// StartTime filters created_at >= StartTime when non-nil.
	StartTime *time.Time
	// EndTime filters created_at <= EndTime when non-nil.
	EndTime *time.Time
}

// DailyTrend is a single day's access count.
type DailyTrend struct {
	Date  string
	Count uint64
}

// BrowserShare is a browser group's share of access logs.
type BrowserShare struct {
	Browser string
	Count   uint64
}

// TopUser is an active user ranked by access count.
type TopUser struct {
	UserID uint64
	Count  uint64
}
