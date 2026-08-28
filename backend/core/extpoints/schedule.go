// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import "sync"

// ScheduleDefinition holds the configuration for a scheduled/cron task.
type ScheduleDefinition struct {
	Spec     string
	TaskType string
	Payload  any
	Options  map[string]any
}

// ScheduleOption configures a ScheduleDefinition.
type ScheduleOption func(*ScheduleDefinition)

// WithScheduleOption adds a custom option to the schedule definition.
func WithScheduleOption(key string, val any) ScheduleOption {
	return func(sd *ScheduleDefinition) {
		if sd.Options == nil {
			sd.Options = make(map[string]any)
		}
		sd.Options[key] = val
	}
}

// ScheduleExtension defines the interface for registering and querying cron/scheduled tasks.
type ScheduleExtension interface {
	Register(spec, taskType string, payload any, opts ...ScheduleOption)
	RegisterCron(spec, taskType string, payload any, opts ...ScheduleOption)
	Schedules() []ScheduleDefinition
	Get(taskType string) (ScheduleDefinition, bool)
	Unregister(taskType string) bool
}

// ScheduleRegistry collects and manages schedule registrations.
type ScheduleRegistry struct {
	mu        sync.RWMutex
	schedules []ScheduleDefinition
	lookup    map[string]ScheduleDefinition
}

// NewScheduleRegistry creates a new schedule registry.
func NewScheduleRegistry() *ScheduleRegistry {
	return &ScheduleRegistry{
		lookup: make(map[string]ScheduleDefinition),
	}
}

// Register adds a schedule definition.
func (s *ScheduleRegistry) Register(spec, taskType string, payload any, opts ...ScheduleOption) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd := ScheduleDefinition{
		Spec:     spec,
		TaskType: taskType,
		Payload:  payload,
		Options:  make(map[string]any),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&sd)
		}
	}

	if _, exists := s.lookup[taskType]; exists {
		for i, item := range s.schedules {
			if item.TaskType == taskType {
				s.schedules[i] = sd
				break
			}
		}
	} else {
		s.schedules = append(s.schedules, sd)
	}

	s.lookup[taskType] = sd
}

// RegisterCron is an alias for Register.
func (s *ScheduleRegistry) RegisterCron(spec, taskType string, payload any, opts ...ScheduleOption) {
	s.Register(spec, taskType, payload, opts...)
}

// Unregister removes a registered schedule definition by its task type.
func (s *ScheduleRegistry) Unregister(taskType string) bool {
	return unregisterEntry(&s.mu, s.lookup, &s.schedules, taskType, func(item ScheduleDefinition) bool {
		return item.TaskType == taskType
	})
}

// Schedules returns a copy of all registered ScheduleDefinitions.
func (s *ScheduleRegistry) Schedules() []ScheduleDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]ScheduleDefinition, len(s.schedules))
	copy(res, s.schedules)
	return res
}

// Get retrieves a schedule definition by its task type.
func (s *ScheduleRegistry) Get(taskType string) (ScheduleDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sd, ok := s.lookup[taskType]
	return sd, ok
}
