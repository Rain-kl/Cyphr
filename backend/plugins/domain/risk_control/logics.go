// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"context"
	"sync"
	"time"

	"Wavelet/pkg/batchwriter"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/risk_control/logstore"
)

var (
	logWriterMu sync.RWMutex
	logWriter   *batchwriter.Writer[*logstore.UserAccessLog]
)

// InitLogWriter initializes the access-log batch writer for the active log database.
func InitLogWriter(ctx context.Context) {
	logWriterMu.Lock()
	defer logWriterMu.Unlock()
	if logWriter != nil {
		return
	}

	cfg := batchwriter.DefaultConfig()
	writer, err := batchwriter.New[*logstore.UserAccessLog](cfg, func(ctx context.Context, items []*logstore.UserAccessLog) error {
		rows := make([]logstore.UserAccessLog, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			rows = append(rows, *item)
		}
		store, err := logstore.Active(ctx)
		if err != nil {
			return err
		}
		return store.UserAccessLogs.BatchInsert(ctx, rows)
	},
		batchwriter.WithDropHandler[*logstore.UserAccessLog](func(item *logstore.UserAccessLog) {
			path := ""
			if item != nil {
				path = item.Path
			}
			logger.WarnF(context.Background(), "[RiskControl] Log queue full, dropping log item for path: %s", path)
		}),
		batchwriter.WithFlushErrorHandler[*logstore.UserAccessLog](func(ctx context.Context, items []*logstore.UserAccessLog, err error) {
			logger.ErrorF(ctx, "[RiskControl] flush access-log batch failed (batch=%d): %v", len(items), err)
		}),
	)
	if err != nil {
		logger.ErrorF(ctx, "[RiskControl] init log writer failed: %v", err)
		return
	}

	writer.Start(ctx)
	logWriter = writer
}

// StopLogWriter stops the ClickHouse access-log batch writer and drains pending logs.
func StopLogWriter(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	return writer.Stop(ctx)
}

// IsBufferFull reports whether the access-log queue has no remaining capacity.
func IsBufferFull() bool {
	writer := currentLogWriter()
	if writer == nil {
		return false
	}
	return writer.IsFull()
}

// QueueAccessLog enqueues an access log without blocking.
func QueueAccessLog(logItem *logstore.UserAccessLog) {
	writer := currentLogWriter()
	if writer == nil || logItem == nil {
		return
	}
	writer.TryEnqueue(logItem)
}

// SetLogWriterForTest swaps the access-log writer for unit tests.
func SetLogWriterForTest(writer *batchwriter.Writer[*logstore.UserAccessLog]) func() {
	logWriterMu.Lock()
	previous := logWriter
	logWriter = writer
	logWriterMu.Unlock()
	return func() {
		logWriterMu.Lock()
		logWriter = previous
		logWriterMu.Unlock()
	}
}

func currentLogWriter() *batchwriter.Writer[*logstore.UserAccessLog] {
	logWriterMu.RLock()
	defer logWriterMu.RUnlock()
	return logWriter
}

const drainPollInterval = 50 * time.Millisecond

// Drain waits until the in-memory access-log queue has been empty for one flush interval.
func Drain(ctx context.Context) error {
	writer := currentLogWriter()
	if writer == nil {
		return nil
	}
	quietPeriod := batchwriter.DefaultConfig().FlushInterval
	if quietPeriod <= 0 {
		quietPeriod = time.Second
	}
	ticker := time.NewTicker(drainPollInterval)
	defer ticker.Stop()
	var quietSince time.Time
	for {
		if writer.Len() == 0 {
			if quietSince.IsZero() {
				quietSince = time.Now()
			} else if time.Since(quietSince) >= quietPeriod {
				return nil
			}
		} else {
			quietSince = time.Time{}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// MigrateAndSwitchEngine migrates access logs to target database and switches the active store.
func MigrateAndSwitchEngine(ctx context.Context, targetEngine string, reportProgress func(copied int)) error {
	if err := Drain(ctx); err != nil {
		return err
	}
	src, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	dst, err := logstore.BuildForMigration(ctx, targetEngine)
	if err != nil {
		return err
	}
	if _, err := dst.UserAccessLogs.DeleteAll(ctx); err != nil {
		return err
	}
	from, to, err := src.UserAccessLogs.MigrationRange(ctx)
	if err != nil {
		return err
	}
	if !from.IsZero() && !to.IsZero() {
		if err := dst.UserAccessLogs.EnsurePartitions(ctx, from, to); err != nil {
			return err
		}
	}
	var afterID uint64
	var copied int
	const copyBatchSize = 1000
	for {
		rows, err := src.UserAccessLogs.ListForMigration(ctx, afterID, copyBatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		if err := dst.UserAccessLogs.BatchInsert(ctx, rows); err != nil {
			return err
		}
		afterID = rows[len(rows)-1].ID
		copied += len(rows)
		if reportProgress != nil {
			reportProgress(copied)
		}
		if len(rows) < copyBatchSize {
			break
		}
	}
	logstore.InvalidateCache()
	return nil
}
