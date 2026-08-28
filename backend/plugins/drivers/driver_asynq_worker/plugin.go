// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package driver_asynq_worker provides the Asynq worker driver plugin for Cordis.
package driver_asynq_worker

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/config"
)

const (
	defaultConcurrency     = 10
	defaultShutdownTimeout = 10 * time.Second
)

//go:embed migrations/*/*.sql
var workerMigrations embed.FS

// Option configures the Asynq worker driver plugin.
type Option func(*Plugin)

// WithRedisOpt sets the Redis connection options for Asynq.
func WithRedisOpt(opt asynq.RedisConnOpt) Option {
	return func(p *Plugin) {
		p.redisOpt = opt
	}
}

// WithConcurrency sets the worker concurrency limit.
func WithConcurrency(concurrency int) Option {
	return func(p *Plugin) {
		p.concurrency = concurrency
	}
}

// WithQueues sets the queue priorities mapping.
func WithQueues(queues map[string]int) Option {
	return func(p *Plugin) {
		p.queues = queues
	}
}

// WithStrictPriority sets whether to process queues strictly in priority order.
func WithStrictPriority(strict bool) Option {
	return func(p *Plugin) {
		p.strictPriority = strict
	}
}

// WithShutdownTimeout sets the timeout for graceful worker shutdown.
func WithShutdownTimeout(d time.Duration) Option {
	return func(p *Plugin) {
		p.shutdownTimeout = d
	}
}

// WithServer injects an existing Asynq server instance.
func WithServer(srv *asynq.Server) Option {
	return func(p *Plugin) {
		p.server = srv
	}
}

// Plugin implements core.Plugin and core.Driver for Asynq background worker server.
type Plugin struct {
	mu              sync.RWMutex
	redisOpt        asynq.RedisConnOpt
	concurrency     int
	queues          map[string]int
	strictPriority  bool
	shutdownTimeout time.Duration
	server          *asynq.Server
	mux             *asynq.ServeMux
	running         bool
	coreCtx         *core.Context
	taskSvc         contracts.TaskService
}

// New creates a new Asynq Worker driver plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		redisOpt:        RedisOpt,
		concurrency:     defaultConcurrency,
		shutdownTimeout: defaultShutdownTimeout,
		queues:          map[string]int{"default": 1},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}

	return p
}

// Name returns the unique plugin identifier.
func (p *Plugin) Name() string {
	return "driver_asynq_worker"
}

// Apply mounts the Asynq Worker driver into the micro-kernel Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	p.mu.Lock()
	p.coreCtx = ctx
	p.mu.Unlock()

	// 0. Bind DBService
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		setDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			setDBService(db)
		})
	}
	ctx.OnDispose(func() error {
		setDBService(nil)
		return nil
	})

	// 1. Provide contracts.TaskService
	p.taskSvc = &taskServiceImpl{}
	core.Provide[contracts.TaskService](ctx, p.taskSvc)
	SetActiveTaskExtension(ctx.Tasks())

	// 2. Register migrations for w_task_executions table
	ctx.Migrations().Register("driver_asynq_worker", workerMigrations)

	ctx.OnDispose(func() error {
		SetActiveTaskExtension(nil)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
		defer cancel()
		return p.Stop(shutdownCtx)
	})

	return ctx.RegisterDriver(p)
}

// Type returns DriverTypeWorker.
func (p *Plugin) Type() core.DriverType {
	return core.DriverTypeWorker
}

// Start boots the Asynq worker server and starts processing background tasks.
func (p *Plugin) Start(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	mux := asynq.NewServeMux()

	if p.coreCtx != nil && p.coreCtx.Tasks() != nil {
		for _, td := range p.coreCtx.Tasks().Tasks() {
			handler, err := toAsynqHandler(td.Handler)
			if err != nil {
				return fmt.Errorf("driver_asynq_worker: invalid handler for task pattern %q: %w", td.Pattern, err)
			}
			mux.Handle(td.Pattern, handler)
		}
	}

	for _, taskName := range GetRegisteredAsynqTasks() {
		mux.HandleFunc(taskName, ProcessTask)
	}

	if p.server == nil {
		opt := p.redisOpt
		if opt == nil {
			if RedisOpt != nil {
				opt = RedisOpt
			} else {
				redisCfg := config.Config.Redis
				addr := "127.0.0.1:6379"
				if len(redisCfg.Addrs) > 0 && redisCfg.Addrs[0] != "" {
					addr = redisCfg.Addrs[0]
				}
				opt = asynq.RedisClientOpt{
					Addr:     addr,
					Username: redisCfg.Username,
					Password: redisCfg.Password,
					DB:       redisCfg.DB,
				}
			}
		}
		p.server = asynq.NewServer(
			opt,
			asynq.Config{
				Concurrency:     p.concurrency,
				Queues:          p.queues,
				StrictPriority:  p.strictPriority,
				ShutdownTimeout: p.shutdownTimeout,
			},
		)
	}

	if err := p.server.Start(mux); err != nil {
		return fmt.Errorf("driver_asynq_worker: start server failed: %w", err)
	}

	p.mux = mux
	p.running = true
	return nil
}

// Stop gracefully shuts down the Asynq worker server.
func (p *Plugin) Stop(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.running = false

	if p.server != nil {
		p.server.Stop()
		p.server.Shutdown()
		p.server = nil
	}

	return nil
}

// IsRunning returns whether the worker server is running.
func (p *Plugin) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Server returns the underlying Asynq server instance.
func (p *Plugin) Server() *asynq.Server {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.server
}

// Mux returns the underlying Asynq serve mux.
func (p *Plugin) Mux() *asynq.ServeMux {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mux
}

func toAsynqHandler(h any) (asynq.Handler, error) {
	if h == nil {
		return nil, errors.New("nil handler")
	}

	switch fn := h.(type) {
	case asynq.HandlerFunc:
		return fn, nil
	case asynq.Handler:
		return fn, nil
	case TaskHandler:
		return asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
			RegisterHandler(t.Type(), fn)
			return ProcessTask(c, t)
		}), nil
	case func(context.Context, *asynq.Task) error:
		return asynq.HandlerFunc(fn), nil
	case func(context.Context, []byte) error:
		return asynq.HandlerFunc(func(c context.Context, t *asynq.Task) error {
			return fn(c, t.Payload())
		}), nil
	case func(context.Context) error:
		return asynq.HandlerFunc(func(c context.Context, _ *asynq.Task) error {
			return fn(c)
		}), nil
	case func([]byte) error:
		return asynq.HandlerFunc(func(_ context.Context, t *asynq.Task) error {
			return fn(t.Payload())
		}), nil
	case func() error:
		return asynq.HandlerFunc(func(_ context.Context, _ *asynq.Task) error {
			return fn()
		}), nil
	default:
		return nil, fmt.Errorf("unsupported task handler type: %T", h)
	}
}

type taskServiceImpl struct{}

func (s *taskServiceImpl) Dispatch(ctx context.Context, taskType string, payload []byte, triggeredBy string) (string, error) {
	return DispatchTask(ctx, taskType, payload, triggeredBy)
}

func (s *taskServiceImpl) ListTasks() []contracts.TaskMetaDTO {
	all := GetDispatchableTasks()
	res := make([]contracts.TaskMetaDTO, 0, len(all))
	for _, m := range all {
		params := make([]contracts.TaskParamDTO, 0, len(m.Params))
		for _, param := range m.Params {
			params = append(params, contracts.TaskParamDTO{
				Name:        param.Name,
				Type:        param.Type,
				Description: param.Description,
				Required:    param.Required,
			})
		}
		res = append(res, contracts.TaskMetaDTO{
			Name:        m.Type,
			DisplayName: m.Name,
			Description: m.Description,
			Params:      params,
			MaxRetry:    m.MaxRetry,
			Queue:       m.Queue,
		})
	}
	return res
}

func (s *taskServiceImpl) GetTaskMeta(taskType string) (contracts.TaskMetaDTO, bool) {
	m := GetTaskMeta(taskType)
	if m == nil {
		return contracts.TaskMetaDTO{}, false
	}
	params := make([]contracts.TaskParamDTO, 0, len(m.Params))
	for _, param := range m.Params {
		params = append(params, contracts.TaskParamDTO{
			Name:        param.Name,
			Type:        param.Type,
			Description: param.Description,
			Required:    param.Required,
		})
	}
	return contracts.TaskMetaDTO{
		Name:        m.Type,
		DisplayName: m.Name,
		Description: m.Description,
		Params:      params,
		MaxRetry:    m.MaxRetry,
		Queue:       m.Queue,
	}, true
}

func (s *taskServiceImpl) ListExecutions(ctx context.Context, taskType string, status string, page, pageSize int) ([]contracts.TaskExecutionDTO, int64, error) {
	db := getDB(ctx)
	if db == nil {
		return nil, 0, errors.New("db not initialized")
	}
	query := db.Model(&TaskExecution{})
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []TaskExecution
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	res := make([]contracts.TaskExecutionDTO, 0, len(rows))
	for _, r := range rows {
		res = append(res, contracts.TaskExecutionDTO{
			ID:           r.ID,
			TaskID:       r.TaskID,
			TaskType:     r.TaskType,
			TaskName:     r.TaskName,
			Status:       string(r.Status),
			Retryable:    r.Retryable,
			MaxRetry:     r.MaxRetry,
			RetryCount:   r.RetryCount,
			Log:          r.Log,
			ErrorMessage: r.ErrorMessage,
			Result:       r.Result,
			StartedAt:    r.StartedAt,
			FinishedAt:   r.FinishedAt,
			Duration:     r.Duration,
			Payload:      r.Payload,
			TriggeredBy:  r.TriggeredBy,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
		})
	}
	return res, total, nil
}

func (s *taskServiceImpl) Retry(ctx context.Context, id uint64) (string, error) {
	return RetryTask(ctx, id)
}

func (s *taskServiceImpl) ValidatePayload(taskType string, payload []byte) ([]byte, error) {
	meta := GetTaskMeta(taskType)
	if meta == nil {
		return payload, nil
	}
	return ValidateAndNormalizePayload(meta.AsynqTask, payload)
}

func (s *taskServiceImpl) ReloadScheduler() error {
	return nil
}

func (s *taskServiceImpl) AppendLog(ctx context.Context, format string, args ...any) {
	AppendLog(ctx, format, args...)
}

func (s *taskServiceImpl) GetExecution(ctx context.Context, id uint64) (*contracts.TaskExecutionDTO, error) {
	exec, err := GetTaskExecutionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &contracts.TaskExecutionDTO{
		ID:           exec.ID,
		TaskID:       exec.TaskID,
		TaskType:     exec.TaskType,
		TaskName:     exec.TaskName,
		Status:       string(exec.Status),
		Retryable:    exec.Retryable,
		MaxRetry:     exec.MaxRetry,
		RetryCount:   exec.RetryCount,
		Log:          exec.Log,
		ErrorMessage: exec.ErrorMessage,
		Result:       exec.Result,
		StartedAt:    exec.StartedAt,
		FinishedAt:   exec.FinishedAt,
		Duration:     exec.Duration,
		Payload:      exec.Payload,
		TriggeredBy:  exec.TriggeredBy,
		CreatedAt:    exec.CreatedAt,
		UpdatedAt:    exec.UpdatedAt,
	}, nil
}
