// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import "sync"

// SettingSchema defines the configuration schema and metadata for a system or plugin setting.
type SettingSchema struct {
	Key         string `json:"key"`
	Default     any    `json:"default"`
	Description string `json:"description"`
	Type        string `json:"type,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	Public      bool   `json:"public,omitempty"`
	Category    string `json:"category,omitempty"`
	Validation  string `json:"validation,omitempty"`
}

// SettingExtension defines the interface for registering and querying setting configuration schemas.
type SettingExtension interface {
	Register(schema SettingSchema)
	Schemas() []SettingSchema
	Get(key string) (SettingSchema, bool)
	Unregister(key string) bool
}

// SettingRegistry collects and manages setting configuration schemas.
type SettingRegistry struct {
	mu      sync.RWMutex
	schemas []SettingSchema
	lookup  map[string]SettingSchema
}

// NewSettingRegistry creates a new setting schema registry.
func NewSettingRegistry() *SettingRegistry {
	return &SettingRegistry{
		lookup: make(map[string]SettingSchema),
	}
}

// Register registers a SettingSchema into the registry.
// Panics if the schema Key is empty.
func (s *SettingRegistry) Register(schema SettingSchema) {
	if schema.Key == "" {
		panic("core/extpoints: setting schema key cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.lookup[schema.Key]; exists {
		for i, item := range s.schemas {
			if item.Key == schema.Key {
				s.schemas[i] = schema
				break
			}
		}
	} else {
		s.schemas = append(s.schemas, schema)
	}

	s.lookup[schema.Key] = schema
}

// Unregister removes a registered SettingSchema by its key.
func (s *SettingRegistry) Unregister(key string) bool {
	return unregisterEntry(&s.mu, s.lookup, &s.schemas, key, func(item SettingSchema) bool {
		return item.Key == key
	})
}

// Schemas returns a copy of all registered SettingSchemas.
func (s *SettingRegistry) Schemas() []SettingSchema {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]SettingSchema, len(s.schemas))
	copy(res, s.schemas)
	return res
}

// Get retrieves a SettingSchema by its key.
func (s *SettingRegistry) Get(key string) (SettingSchema, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	schema, ok := s.lookup[key]
	return schema, ok
}
