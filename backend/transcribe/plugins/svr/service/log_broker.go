// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/transcribe/plugins/svr/model/do"
	"sync"
	"sync/atomic"
)

const defaultLogBufferSize = 100

// LogBroker defines in-memory pub/sub for SSE log streams with bounded buffers and non-blocking drops.
type LogBroker interface {
	Subscribe(jobID uint64) (<-chan do.LogMessage, func())
	Publish(jobID uint64, msg do.LogMessage)
	SubscribeFinish(jobID uint64) (<-chan do.FinishMessage, func())
	PublishFinish(jobID uint64, msg do.FinishMessage)
	CloseJob(jobID uint64)
}

// subBroker is a generic pub/sub channel router for a specific message type.
type subBroker[T any] struct {
	mu      sync.RWMutex
	subs    map[uint64]map[uint64]chan T
	bufSize int
}

func newSubBroker[T any](bufSize int) *subBroker[T] {
	return &subBroker[T]{
		subs:    make(map[uint64]map[uint64]chan T),
		bufSize: bufSize,
	}
}

func (sb *subBroker[T]) subscribe(jobID, subID uint64) (<-chan T, func()) {
	ch := make(chan T, sb.bufSize)

	sb.mu.Lock()
	if sb.subs[jobID] == nil {
		sb.subs[jobID] = make(map[uint64]chan T)
	}
	sb.subs[jobID][subID] = ch
	sb.mu.Unlock()

	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			sb.mu.Lock()
			if m, ok := sb.subs[jobID]; ok {
				delete(m, subID)
				if len(m) == 0 {
					delete(sb.subs, jobID)
				}
			}
			sb.mu.Unlock()
		})
	}
	return ch, cancel
}

func (sb *subBroker[T]) publish(jobID uint64, msg T) {
	sb.mu.RLock()
	m := sb.subs[jobID]
	if len(m) == 0 {
		sb.mu.RUnlock()
		return
	}
	channels := make([]chan T, 0, len(m))
	for _, ch := range m {
		channels = append(channels, ch)
	}
	sb.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- msg:
		default:
			// Dropped for slow subscriber
		}
	}
}

func (sb *subBroker[T]) closeJob(jobID uint64) {
	sb.mu.Lock()
	delete(sb.subs, jobID)
	sb.mu.Unlock()
}

// DefaultLogBroker implements LogBroker with thread-safe fan-out to subscribers.
type DefaultLogBroker struct {
	logs      *subBroker[do.LogMessage]
	finishes  *subBroker[do.FinishMessage]
	nextSubID atomic.Uint64
}

var _ LogBroker = (*DefaultLogBroker)(nil)

// NewLogBroker creates a new DefaultLogBroker instance.
func NewLogBroker(bufSize ...int) *DefaultLogBroker {
	size := defaultLogBufferSize
	if len(bufSize) > 0 && bufSize[0] > 0 {
		size = bufSize[0]
	}
	return &DefaultLogBroker{
		logs:     newSubBroker[do.LogMessage](size),
		finishes: newSubBroker[do.FinishMessage](size),
	}
}

// Subscribe registers a new subscriber for log messages belonging to a job.
func (b *DefaultLogBroker) Subscribe(jobID uint64) (<-chan do.LogMessage, func()) {
	return b.logs.subscribe(jobID, b.nextSubID.Add(1))
}

// Publish broadcasts a log message to all active subscribers for the given job.
func (b *DefaultLogBroker) Publish(jobID uint64, msg do.LogMessage) {
	b.logs.publish(jobID, msg)
}

// SubscribeFinish registers a new subscriber for the finish event of a job.
func (b *DefaultLogBroker) SubscribeFinish(jobID uint64) (<-chan do.FinishMessage, func()) {
	return b.finishes.subscribe(jobID, b.nextSubID.Add(1))
}

// PublishFinish broadcasts a finish event to all finish subscribers for the given job.
func (b *DefaultLogBroker) PublishFinish(jobID uint64, msg do.FinishMessage) {
	b.finishes.publish(jobID, msg)
}

// CloseJob removes all subscribers and cleans up memory state for a completed or cancelled job.
func (b *DefaultLogBroker) CloseJob(jobID uint64) {
	b.logs.closeJob(jobID)
	b.finishes.closeJob(jobID)
}
