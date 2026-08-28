// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package risk_control provides the access control, IP rate limiting, and telemetry risk analysis domain plugin for Cordis.
package risk_control

import (
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/domain/risk_control/logstore"
)

//go:embed logstore/migrations/*.sql
var riskControlMigrations embed.FS

// Option configures the risk_control plugin.
type Option func(*Plugin)

// WithMiddleware configures a custom risk control middleware.
func WithMiddleware(mw gin.HandlerFunc) Option {
	return func(p *Plugin) {
		p.middleware = mw
	}
}

// Plugin implements core.Plugin to provide risk control and access logging middleware.
type Plugin struct {
	middleware gin.HandlerFunc
}

// New creates a new risk_control domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier for the risk_control domain plugin.
func (p *Plugin) Name() string {
	return "risk_control"
}

// Inject declares required dependencies for the risk_control domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		reflect.TypeFor[contracts.CacheService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        "risk_control",
		Version:     "1.0.0",
		Description: "Access control, IP rate limiting, and access log telemetry domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers risk control middlewares, settings, and cleanup hooks into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 0. Bind DBService
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		logstore.SetDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			logstore.SetDBService(db)
		})
	}
	ctx.OnDispose(func() error {
		logstore.SetDBService(nil)
		return nil
	})

	// 0. Register user access log table migrations
	ctx.Migrations().Register("risk_control/logstore", riskControlMigrations)

	// 1. Initialize LogWriter if needed
	InitLogWriter(ctx.GoContext())

	// 2. Register router middleware
	mw := p.middleware
	if mw == nil {
		mw = RiskControlMiddleware()
	}
	ctx.Router().Use(mw)

	// 3. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "risk_control.ip_rate_limit_per_minute",
		Default:     60,
		Description: "Maximum requests allowed per IP per minute",
		Type:        "integer",
		Category:    "security",
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "risk_control.enable_access_log",
		Default:     true,
		Description: "Enable structured access log auditing and backpressure queueing",
		Type:        "boolean",
		Category:    "security",
	})

	// 4. Register RiskControlService contract
	core.Provide[contracts.RiskControlService](ctx, &riskControlServiceImpl{})

	// 5. Register lifecycle disposal cleanup
	ctx.OnDispose(func() error {
		return StopLogWriter(context.Background())
	})

	return nil
}

type riskControlServiceImpl struct{}

func (s *riskControlServiceImpl) QueryAccessLogs(ctx context.Context, filter contracts.AccessLogFilterDTO, page, pageSize int) ([]contracts.AccessLogDTO, uint64, error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return nil, 0, err
	}
	f := logstore.AccessLogFilter{
		UserIDs:   filter.UserIDs,
		Path:      filter.Path,
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
	}
	list, total, err := store.UserAccessLogs.List(ctx, f, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]contracts.AccessLogDTO, len(list))
	for i, item := range list {
		items[i] = contracts.AccessLogDTO{
			ID:        item.ID,
			UserID:    item.UserID,
			IP:        item.IP,
			UserAgent: item.UserAgent,
			Method:    item.Method,
			Path:      item.Path,
			Status:    item.Status,
			Latency:   item.Latency,
			CreatedAt: item.CreatedAt,
		}
	}
	return items, total, nil
}

func (s *riskControlServiceImpl) QueryAccessLogStats(ctx context.Context, days int) ([]contracts.AccessLogDailyStatsDTO, error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	trend, err := store.UserAccessLogs.GetDailyTrend(ctx, days)
	if err != nil {
		return nil, err
	}
	res := make([]contracts.AccessLogDailyStatsDTO, len(trend))
	for i, t := range trend {
		res[i] = contracts.AccessLogDailyStatsDTO{
			Date: t.Date,
			PV:   t.Count,
		}
	}
	return res, nil
}

func (s *riskControlServiceImpl) ActiveLogEngine(ctx context.Context) string {
	store, err := logstore.Active(ctx)
	if err != nil {
		return "sqlite"
	}
	active, err := store.Status.ActiveDatabase(ctx)
	if err != nil {
		return "sqlite"
	}
	return active
}

func (s *riskControlServiceImpl) IsLogEngineMigrating(ctx context.Context) bool {
	return logstore.Migrating(ctx)
}

func (s *riskControlServiceImpl) Drain(ctx context.Context) error {
	return Drain(ctx)
}

func (s *riskControlServiceImpl) SwitchLogEngine(ctx context.Context, targetEngine string) error {
	return MigrateAndSwitchEngine(ctx, targetEngine, nil)
}
