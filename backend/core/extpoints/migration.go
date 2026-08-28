// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package extpoints defines extension points for router, migrations, tasks, schedules, and settings.
package extpoints

import (
	"io/fs"
	"sync"
)

// MigrationEntry contains the migration filesystem and configuration for a plugin.
type MigrationEntry struct {
	PluginID string
	FS       fs.FS
	Dir      string
}

// MigrationExtension defines the interface for registering and querying plugin migrations.
type MigrationExtension interface {
	Register(pluginID string, fsys fs.FS, dir ...string)
	Unregister(pluginID string) bool
	Entries() []MigrationEntry
	Get(pluginID string) (MigrationEntry, bool)
}

// MigrationRegistry implements MigrationExtension.
type MigrationRegistry struct {
	mu      sync.RWMutex
	entries []MigrationEntry
	lookup  map[string]MigrationEntry
}

// NewMigrationRegistry creates a new MigrationRegistry.
func NewMigrationRegistry() *MigrationRegistry {
	return &MigrationRegistry{
		lookup: make(map[string]MigrationEntry),
	}
}

// Register registers an embedded migration filesystem for a plugin.
func (m *MigrationRegistry) Register(pluginID string, fsys fs.FS, dir ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	migrationDir := "migrations"
	if len(dir) > 0 && dir[0] != "" {
		migrationDir = dir[0]
	}

	entry := MigrationEntry{
		PluginID: pluginID,
		FS:       fsys,
		Dir:      migrationDir,
	}

	// If entry already exists, update in-place; otherwise append
	if _, exists := m.lookup[pluginID]; exists {
		for i, e := range m.entries {
			if e.PluginID == pluginID {
				m.entries[i] = entry
				break
			}
		}
	} else {
		m.entries = append(m.entries, entry)
	}

	m.lookup[pluginID] = entry
}

// Unregister removes a registered migration entry by plugin ID.
func (m *MigrationRegistry) Unregister(pluginID string) bool {
	return unregisterEntry(&m.mu, m.lookup, &m.entries, pluginID, func(e MigrationEntry) bool {
		return e.PluginID == pluginID
	})
}

// Entries returns a copy of all registered migration entries in registration order.
func (m *MigrationRegistry) Entries() []MigrationEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]MigrationEntry, len(m.entries))
	copy(res, m.entries)
	return res
}

// Get retrieves the migration entry for a specific plugin ID.
func (m *MigrationRegistry) Get(pluginID string) (MigrationEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.lookup[pluginID]
	return e, ok
}
