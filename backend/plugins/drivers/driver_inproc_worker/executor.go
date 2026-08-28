// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_worker

import (
	"Wavelet/core/extpoints"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/util"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRetryBackoff = 500 * time.Millisecond

// TaskMessage represents an in-process task item in the queue.
type TaskMessage struct {
	ID        string
	TaskType  string
	Payload   []byte
	Source    string
	CreatedAt time.Time
	RetryLeft int
}

// InprocQueue manages in-memory task queuing and worker pool execution.
type InprocQueue struct {
	concurrency int
	queue       chan TaskMessage
	taskReg     extpoints.TaskExtension
	running     atomic.Bool
	stopCh      chan struct{}
	wg          sync.WaitGroup

	// baseCtx is the app-lifetime context captured at Start; task handlers
	// derive their timeouts from it so shutdown cancellation propagates.
	baseCtx context.Context
}

// NewInprocQueue creates a new InprocQueue with a given concurrency and queue capacity.
func NewInprocQueue(concurrency, queueCap int, taskReg extpoints.TaskExtension) *InprocQueue {
	if concurrency <= 0 {
		concurrency = 10
	}
	if queueCap <= 0 {
		queueCap = 1000
	}

	return &InprocQueue{
		concurrency: concurrency,
		queue:       make(chan TaskMessage, queueCap),
		taskReg:     taskReg,
		stopCh:      make(chan struct{}),
	}
}

// Enqueue puts a new task into the in-process queue.
func (q *InprocQueue) Enqueue(taskType string, payload []byte, source string) (string, error) {
	if !q.running.Load() {
		return "", errors.New("driver_inproc_worker: queue is not running")
	}

	taskID := fmt.Sprintf("inproc_%d", idgen.NextUint64ID())
	msg := TaskMessage{
		ID:        taskID,
		TaskType:  taskType,
		Payload:   payload,
		Source:    source,
		CreatedAt: time.Now(),
	}

	if q.taskReg != nil {
		if td, ok := q.taskReg.Get(taskType); ok {
			msg.RetryLeft = td.Retry
		}
	}

	select {
	case q.queue <- msg:
		return taskID, nil
	default:
		return "", errors.New("driver_inproc_worker: queue is full")
	}
}

// Start begins processing tasks with the worker pool. ctx is the app-lifetime
// context used as the parent for per-task execution contexts.
func (q *InprocQueue) Start(ctx context.Context) {
	if !q.running.CompareAndSwap(false, true) {
		return
	}

	if q.baseCtx == nil {
		q.baseCtx = ctx
	}
	for i := 0; i < q.concurrency; i++ {
		q.wg.Add(1)
		util.Go(func() {
			defer q.wg.Done()
			q.workerLoop(ctx)
		})
	}
}

// Stop gracefully waits for in-flight tasks and shuts down workers.
func (q *InprocQueue) Stop(ctx context.Context) error {
	if !q.running.CompareAndSwap(true, false) {
		return nil
	}

	close(q.stopCh)

	done := make(chan struct{})
	util.Go(func() {
		q.wg.Wait()
		close(done)
	})

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *InprocQueue) workerLoop(ctx context.Context) {
	for {
		select {
		case <-q.stopCh:
			return
		case <-ctx.Done():
			return
		case msg, ok := <-q.queue:
			if !ok {
				return
			}
			q.executeTask(ctx, msg)
		}
	}
}

func (q *InprocQueue) executeTask(ctx context.Context, msg TaskMessage) {
	if q.taskReg == nil {
		return
	}

	td, ok := q.taskReg.Get(msg.TaskType)
	if !ok {
		return
	}

	timeout := td.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := invokeHandler(taskCtx, td.Handler, msg.Payload)
	if err != nil && msg.RetryLeft > 0 {
		msg.RetryLeft--
		// Retry with backoff
		util.Go(func() {
			select {
			case <-time.After(defaultRetryBackoff):
			case <-q.stopCh:
				return
			case <-ctx.Done():
				return
			}
			if q.running.Load() {
				select {
				case q.queue <- msg:
				default:
				}
			}
		})
	}
}

func invokeHandler(ctx context.Context, handler any, payload []byte) error {
	if handler == nil {
		return errors.New("nil task handler")
	}

	switch fn := handler.(type) {
	case func(context.Context, []byte) error:
		return fn(ctx, payload)
	case func(context.Context) error:
		return fn(ctx)
	case func([]byte) error:
		return fn(payload)
	case func() error:
		return fn()
	default:
		return fmt.Errorf("unsupported handler type: %T", handler)
	}
}
