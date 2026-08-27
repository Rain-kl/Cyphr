// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/repository/logstore"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/plugins/domain/risk_control"
)

const (
	// LogDBSwitchTask 切换日志数据库任务标识。
	LogDBSwitchTask = "logs:db_switch"
	// TaskTypeLogDBSwitch 管理端任务类型。
	TaskTypeLogDBSwitch = "logs_db_switch"

	copyBatchSize    = 1000
	targetPostgres   = "postgres"
	targetSQLite     = "sqlite"
	targetClickHouse = "clickhouse"
)

// LogDBSwitchMeta 描述切换日志数据库任务。
var LogDBSwitchMeta = task.TaskMeta{
	Type:         TaskTypeLogDBSwitch,
	AsynqTask:    LogDBSwitchTask,
	Name:         "切换日志数据库",
	Description:  "复制迁移用户访问日志并在成功后切换日志主库（期间禁止日志写入）",
	SupportsTime: false,
	MaxRetry:     task.DefaultMaxRetry,
	Queue:        task.QueueDefault,
	Retryable:    true,
	Params: []task.TaskParam{
		{Name: "target", Label: "目标日志库", Type: "string", Required: true,
			Placeholder: "postgres|sqlite|clickhouse", Description: "迁移目标：postgres（主库为 PG 时）、sqlite（主库为 SQLite 时）或 clickhouse"},
	},
}

type logDBSwitchPayload struct {
	Target string `json:"target"`
}

// LogDBSwitchHandler 切换日志数据库任务处理器。
type LogDBSwitchHandler struct{}

// ValidatePayload 校验并规范化参数。
func (h *LogDBSwitchHandler) ValidatePayload(payload []byte) ([]byte, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if !validTarget(p.Target) {
		return nil, fmt.Errorf("目标日志库不合法: %s", p.Target)
	}
	out, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeTarget(v string) string {
	switch v {
	case targetPostgres, "postgresql":
		return targetPostgres
	case targetSQLite, "sqlite3":
		return targetSQLite
	case targetClickHouse, "ch":
		return targetClickHouse
	}
	return v
}

func validTarget(v string) bool {
	return v == targetPostgres || v == targetSQLite || v == targetClickHouse
}

// Execute 执行迁移。
func (h *LogDBSwitchHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
	var p logDBSwitchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}
	p.Target = normalizeTarget(p.Target)
	if err := validateSwitch(ctx, p.Target); err != nil {
		return nil, err
	}

	source, err := currentLogDatabase(ctx)
	if err != nil {
		task.AppendLog(ctx, "读取日志主库失败: %v", err)
		return nil, err
	}
	task.AppendLog(ctx, "开始切换日志数据库：%s -> %s", source, p.Target)

	if err := setMigrationFlag(ctx, "migrating"); err != nil {
		return nil, err
	}
	defer func() {
		if err := setMigrationFlag(ctx, ""); err != nil {
			logger.ErrorF(ctx, "清除日志迁移冻结标记失败: %v", err)
		}
	}()

	if err := risk_control.Drain(ctx); err != nil {
		return nil, fmt.Errorf("排空日志写入队列失败: %w", err)
	}

	src, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	dst, err := logstore.BuildForMigration(ctx, p.Target)
	if err != nil {
		return nil, err
	}

	if _, err := dst.UserAccessLogs.DeleteAll(ctx); err != nil {
		return nil, fmt.Errorf("清空目标用户访问日志失败: %w", err)
	}
	from, to, err := src.UserAccessLogs.MigrationRange(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取源库时间范围失败: %w", err)
	}
	if !from.IsZero() && !to.IsZero() {
		if err := dst.UserAccessLogs.EnsurePartitions(ctx, from, to); err != nil {
			return nil, fmt.Errorf("预建目标分区失败: %w", err)
		}
	}

	if err := copyUserAccessLogs(ctx, src, dst); err != nil {
		return nil, err
	}
	if err := flipLogDatabase(ctx, p.Target); err != nil {
		return nil, err
	}
	logstore.InvalidateCache()
	task.AppendLog(ctx, "日志数据库已切换为 %s，写入恢复", p.Target)
	return &task.TaskResult{Message: fmt.Sprintf("日志数据库已从 %s 切换为 %s", source, p.Target)}, nil
}

func validateSwitch(ctx context.Context, target string) error {
	source, err := currentLogDatabase(ctx)
	if err != nil {
		return err
	}
	if source == target {
		return errors.New("目标日志库与当前日志库相同，无需迁移")
	}
	switch target {
	case targetClickHouse:
		if !config.Config.ClickHouse.Enabled {
			return errors.New("ClickHouse 未启用，无法迁移到 ClickHouse")
		}
	case targetPostgres:
		if !config.Config.Database.Enabled {
			return errors.New("PostgreSQL 未启用（当前主库为 SQLite），无法迁移到 PostgreSQL")
		}
	case targetSQLite:
		if config.Config.Database.Enabled {
			return errors.New("当前主库为 PostgreSQL，日志库不能设置为 SQLite")
		}
	}
	return nil
}

func currentLogDatabase(ctx context.Context) (string, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyLogDatabase)
	if err != nil {
		return "", fmt.Errorf("读取日志主库失败: %w", err)
	}
	if cfg.Value == "" {
		return "", errors.New("日志主库配置为空")
	}
	return cfg.Value, nil
}

func setMigrationFlag(ctx context.Context, v string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDBMigration, v)
}

func flipLogDatabase(ctx context.Context, target string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyLogDatabase, target)
}

func copyUserAccessLogs(ctx context.Context, src, dst *logstore.Store) error {
	var afterID uint64
	var copied int
	for {
		rows, err := src.UserAccessLogs.ListForMigration(ctx, afterID, copyBatchSize)
		if err != nil {
			return fmt.Errorf("读取源用户访问日志失败: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		if err := dst.UserAccessLogs.BatchInsert(ctx, rows); err != nil {
			return fmt.Errorf("写入目标用户访问日志失败: %w", err)
		}
		afterID = rows[len(rows)-1].ID
		copied += len(rows)
		task.AppendLog(ctx, "已复制用户访问日志 %d 条", copied)
		if len(rows) < copyBatchSize {
			break
		}
	}
	return nil
}
