package extpoints

import (
	"sync"
	"time"
)

// TaskDefinition holds the definition and runtime options for an asynchronous background task.
type TaskDefinition struct {
	Pattern     string
	Handler     any
	Concurrency int
	Retry       int
	Timeout     time.Duration
	Metadata    map[string]any
}

// TaskOption configures a TaskDefinition.
type TaskOption func(*TaskDefinition)

// WithTaskConcurrency sets the concurrency limit for the task.
func WithTaskConcurrency(concurrency int) TaskOption {
	return func(td *TaskDefinition) {
		td.Concurrency = concurrency
	}
}

// WithTaskRetry sets the maximum retry count for the task.
func WithTaskRetry(retry int) TaskOption {
	return func(td *TaskDefinition) {
		td.Retry = retry
	}
}

// WithTaskTimeout sets the execution timeout for the task.
func WithTaskTimeout(timeout time.Duration) TaskOption {
	return func(td *TaskDefinition) {
		td.Timeout = timeout
	}
}

// WithTaskMetadata adds a key-value pair to the task metadata.
func WithTaskMetadata(key string, val any) TaskOption {
	return func(td *TaskDefinition) {
		if td.Metadata == nil {
			td.Metadata = make(map[string]any)
		}
		td.Metadata[key] = val
	}
}

// TaskExtension defines the interface for registering and querying background task handlers.
type TaskExtension interface {
	Register(pattern string, handler any, opts ...TaskOption)
	Tasks() []TaskDefinition
	Get(pattern string) (TaskDefinition, bool)
}

// TaskRegistry collects and manages task registrations.
type TaskRegistry struct {
	mu     sync.RWMutex
	tasks  []TaskDefinition
	lookup map[string]TaskDefinition
}

// NewTaskRegistry creates a new task registry.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		lookup: make(map[string]TaskDefinition),
	}
}

// Register registers a task pattern and its handler with optional configuration.
func (t *TaskRegistry) Register(pattern string, handler any, opts ...TaskOption) {
	t.mu.Lock()
	defer t.mu.Unlock()

	td := TaskDefinition{
		Pattern:  pattern,
		Handler:  handler,
		Metadata: make(map[string]any),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&td)
		}
	}

	if _, exists := t.lookup[pattern]; exists {
		for i, item := range t.tasks {
			if item.Pattern == pattern {
				t.tasks[i] = td
				break
			}
		}
	} else {
		t.tasks = append(t.tasks, td)
	}

	t.lookup[pattern] = td
}

// Tasks returns a copy of all registered TaskDefinitions.
func (t *TaskRegistry) Tasks() []TaskDefinition {
	t.mu.RLock()
	defer t.mu.RUnlock()
	res := make([]TaskDefinition, len(t.tasks))
	copy(res, t.tasks)
	return res
}

// Get retrieves a task definition by its pattern.
func (t *TaskRegistry) Get(pattern string) (TaskDefinition, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	td, ok := t.lookup[pattern]
	return td, ok
}
