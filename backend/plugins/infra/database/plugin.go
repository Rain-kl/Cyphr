// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package database provides the relational database infrastructure plugin for Cordis.
package database

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"

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

// DeclareConfig declares database configuration keys.
func (p *Plugin) DeclareConfig() []core.ConfigBinding {
	return []core.ConfigBinding{
		{Prefix: "database", Target: &Config{}},
		{Prefix: "clickhouse", Target: &ClickHouseConfig{}},
		{Prefix: "app", Target: &appEnvConfig{}},
	}
}

// Apply mounts the database service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	var dbCfg Config
	if err := ctx.Config().Bind("database", &dbCfg); err != nil {
		return err
	}

	var chCfg ClickHouseConfig
	if err := ctx.Config().Bind("clickhouse", &chCfg); err != nil {
		return err
	}

	var appCfg appEnvConfig
	_ = ctx.Config().Bind("app", &appCfg)

	targetDB := p.db
	if targetDB == nil {
		var err error
		targetDB, err = InitDBWithConfig(dbCfg, appCfg.Env == "production" || appCfg.Env == "prod")
		if err != nil {
			return err
		}
	}

	if chCfg.Enabled {
		if err := InitClickHouseWithConfig(chCfg); err != nil {
			return err
		}
	}

	svc := &dbServiceImpl{
		primary:  targetDB,
		namedDBs: p.namedDBs,
	}

	if sqlDB, err := targetDB.DB(); err == nil && sqlDB != nil {
		ctx.OnDispose(func() error {
			return sqlDB.Close()
		})
	}

	core.Provide[contracts.DBService](ctx, svc)
	return nil
}

// NewService wraps a GORM DB instance into a contracts.DBService.
func NewService(primary *gorm.DB) contracts.DBService {
	return &dbServiceImpl{primary: primary}
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
