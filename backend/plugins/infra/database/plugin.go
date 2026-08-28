// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package database provides the relational database infrastructure plugin for Cordis.
package database

import (
	"context"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"gorm.io/gorm"
)

// Option configures the database plugin.
type Option func(*Plugin)

// WithDB configures an explicit *gorm.DB instance for the plugin.
func WithDB(d *gorm.DB) Option {
	return func(p *Plugin) {
		p.db = d
	}
}

// WithNamedDB registers a named secondary database connection.
func WithNamedDB(name string, d *gorm.DB) Option {
	return func(p *Plugin) {
		if p.namedDBs == nil {
			p.namedDBs = make(map[string]*gorm.DB)
		}
		p.namedDBs[name] = d
	}
}

// Plugin implements core.Plugin to provide contracts.DBService into the Cordis micro-kernel.
type Plugin struct {
	db       *gorm.DB
	namedDBs map[string]*gorm.DB
}

// New creates a new database infrastructure plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		namedDBs: make(map[string]*gorm.DB),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the database plugin.
func (p *Plugin) Name() string {
	return "database"
}

// Apply mounts the database service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	targetDB := p.db
	if targetDB == nil {
		targetDB = DB(context.Background())
	}

	svc := &dbServiceImpl{
		primary:  targetDB,
		namedDBs: p.namedDBs,
	}

	core.Provide[contracts.DBService](ctx, svc)
	return nil
}

type dbServiceImpl struct {
	primary  *gorm.DB
	namedDBs map[string]*gorm.DB
}

func (s *dbServiceImpl) GORM() *gorm.DB {
	if s.primary != nil {
		return s.primary
	}
	return DB(context.Background())
}

func (s *dbServiceImpl) DB(ctx context.Context) *gorm.DB {
	if s.primary != nil {
		return s.primary.WithContext(ctx)
	}
	return DB(ctx)
}

func (s *dbServiceImpl) Named(name string) *gorm.DB {
	if s.namedDBs != nil {
		if d, ok := s.namedDBs[name]; ok && d != nil {
			return d
		}
	}
	return s.GORM()
}
