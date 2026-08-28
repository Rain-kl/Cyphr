// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package batchwriter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEvent struct {
	ID   int
	Data string
}

func testConfig() Config {
	return Config{
		Name:          "test-writer",
		QueueSize:     100,
		MaxBatchSize:  5,
		FlushInterval: 20 * time.Millisecond,
	}
}

func TestWriter_BatchSizeFlush(t *testing.T) {
	var (
		mu      sync.Mutex
		batches [][]testEvent
		flushWg sync.WaitGroup
	)
	flushWg.Add(1)

	cfg := testConfig()
	cfg.FlushInterval = time.Hour

	w, err := New(cfg, func(_ context.Context, items []testEvent) error {
		mu.Lock()
		defer mu.Unlock()
		batches = append(batches, items)
		if len(items) == 5 {
			flushWg.Done()
		}
		return nil
	})
	require.NoError(t, err)

	w.Start(context.Background())
	defer func() { _ = w.Stop(context.Background()) }()

	for i := 1; i <= 5; i++ {
		ok := w.TryEnqueue(testEvent{ID: i, Data: "payload"})
		assert.True(t, ok)
	}

	done := make(chan struct{})
	go func() {
		flushWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for batch flush")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, batches, 1)
	assert.Len(t, batches[0], 5)
	for i, item := range batches[0] {
		assert.Equal(t, i+1, item.ID)
	}
}

func TestWriter_IntervalFlush(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed []testEvent
		done    = make(chan struct{})
	)

	cfg := testConfig()
	cfg.MaxBatchSize = 100
	cfg.FlushInterval = 30 * time.Millisecond

	w, err := New(cfg, func(_ context.Context, items []testEvent) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, items...)
		if len(flushed) == 2 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
		return nil
	})
	require.NoError(t, err)

	w.Start(context.Background())
	defer func() { _ = w.Stop(context.Background()) }()

	assert.True(t, w.TryEnqueue(testEvent{ID: 1}))
	assert.True(t, w.TryEnqueue(testEvent{ID: 2}))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for interval flush")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 2)
}

func TestWriter_MinBatchSizeThreshold(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed []testEvent
		done    = make(chan struct{})
	)

	cfg := testConfig()
	cfg.MaxBatchSize = 100
	cfg.MinBatchSize = 3
	cfg.FlushInterval = 20 * time.Millisecond
	cfg.MaxFlushWait = 60 * time.Millisecond

	w, err := New(cfg, func(_ context.Context, items []testEvent) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, items...)
		if len(flushed) == 2 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
		return nil
	})
	require.NoError(t, err)

	w.Start(context.Background())
	defer func() { _ = w.Stop(context.Background()) }()

	assert.True(t, w.TryEnqueue(testEvent{ID: 1}))
	assert.True(t, w.TryEnqueue(testEvent{ID: 2}))

	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	assert.Empty(t, flushed, "items should wait until MinBatchSize or MaxFlushWait")
	mu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forced max wait flush")
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 2)
}

func TestWriter_StopDrainsRemaining(t *testing.T) {
	var (
		mu      sync.Mutex
		flushed []testEvent
	)

	cfg := testConfig()
	cfg.FlushInterval = time.Hour

	w, err := New(cfg, func(_ context.Context, items []testEvent) error {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, items...)
		return nil
	})
	require.NoError(t, err)

	w.Start(context.Background())
	for i := 1; i <= 3; i++ {
		assert.True(t, w.TryEnqueue(testEvent{ID: i}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, w.Stop(ctx))

	assert.False(t, w.Running())
	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, flushed, 3)
}

func TestWriter_DropWhenFull(t *testing.T) {
	var (
		dropped atomic.Int64
		blockCh = make(chan struct{})
	)

	cfg := Config{
		QueueSize:     2,
		MaxBatchSize:  10,
		FlushInterval: time.Hour,
	}

	w, err := New(cfg, func(_ context.Context, _ []testEvent) error {
		<-blockCh
		return nil
	}, WithDropHandler(func(_ testEvent) {
		dropped.Add(1)
	}))
	require.NoError(t, err)

	w.Start(context.Background())
	defer func() {
		close(blockCh)
		_ = w.Stop(context.Background())
	}()

	time.Sleep(10 * time.Millisecond)

	w.ch <- testEvent{ID: 1}
	w.ch <- testEvent{ID: 2}

	assert.False(t, w.TryEnqueue(testEvent{ID: 3}))
	assert.Equal(t, int64(1), dropped.Load())
	assert.Equal(t, int64(1), w.Stats().Drops)
}

func TestWriter_FlushErrorCallback(t *testing.T) {
	var (
		called   atomic.Bool
		flushErr = errors.New("clickhouse write timeout")
		done     = make(chan struct{})
	)

	cfg := testConfig()
	cfg.MaxBatchSize = 1
	cfg.FlushInterval = time.Hour

	w, err := New(cfg, func(_ context.Context, _ []testEvent) error {
		return flushErr
	}, WithFlushErrorHandler(func(_ context.Context, items []testEvent, err error) {
		called.Store(true)
		assert.Equal(t, flushErr, err)
		assert.Len(t, items, 1)
		close(done)
	}))
	require.NoError(t, err)

	w.Start(context.Background())
	defer func() { _ = w.Stop(context.Background()) }()

	assert.True(t, w.TryEnqueue(testEvent{ID: 1}))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error callback")
	}

	assert.True(t, called.Load())
	assert.Equal(t, int64(1), w.Stats().FlushErrors)
}

func TestWriter_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", DefaultConfig(), false},
		{"zero queue", Config{QueueSize: 0, MaxBatchSize: 10, FlushInterval: time.Second}, true},
		{"zero max batch", Config{QueueSize: 10, MaxBatchSize: 0, FlushInterval: time.Second}, true},
		{"negative min batch", Config{QueueSize: 10, MaxBatchSize: 10, MinBatchSize: -1, FlushInterval: time.Second}, true},
		{"zero flush interval", Config{QueueSize: 10, MaxBatchSize: 10, FlushInterval: 0}, true},
		{"negative max flush wait", Config{QueueSize: 10, MaxBatchSize: 10, FlushInterval: time.Second, MaxFlushWait: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg, func(_ context.Context, _ []testEvent) error { return nil })
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriter_NilFlushFunc(t *testing.T) {
	_, err := New[testEvent](DefaultConfig(), nil)
	assert.ErrorIs(t, err, errNilFlushFunc)
}
