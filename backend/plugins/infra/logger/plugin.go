// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logger provides the structured logging infrastructure plugin for Cordis.
package logger

import (
	"context"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/pkg/logger"
)

// Plugin implements core.Plugin to provide contracts.LoggerService.
type Plugin struct{}

// New creates a new logger infrastructure plugin.
func New() *Plugin {
	return &Plugin{}
}

// Name returns the unique identifier of the logger plugin.
func (p *Plugin) Name() string {
	return "logger"
}

// Apply mounts the logger service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	svc := &loggerServiceImpl{}
	core.Provide[contracts.LoggerService](ctx, svc)
	return nil
}

type loggerServiceImpl struct {
	extraFields []any
}

func (s *loggerServiceImpl) formatMsg(msg string, keysAndValues ...any) string {
	allFields := make([]any, 0, len(s.extraFields)+len(keysAndValues))
	allFields = append(allFields, s.extraFields...)
	allFields = append(allFields, keysAndValues...)

	if len(allFields) == 0 {
		return msg
	}

	var sb strings.Builder
	sb.WriteString(msg)
	sb.WriteString(" [")
	for i := 0; i < len(allFields); i += 2 {
		if i > 0 {
			sb.WriteString(" ")
		}
		if i+1 < len(allFields) {
			fmt.Fprintf(&sb, "%v=%v", allFields[i], allFields[i+1])
		} else {
			fmt.Fprintf(&sb, "%v", allFields[i])
		}
	}
	sb.WriteString("]")
	return sb.String()
}

func (s *loggerServiceImpl) Debug(ctx context.Context, msg string, keysAndValues ...any) {
	logger.DebugF(ctx, "%s", s.formatMsg(msg, keysAndValues...))
}

func (s *loggerServiceImpl) Info(ctx context.Context, msg string, keysAndValues ...any) {
	logger.InfoF(ctx, "%s", s.formatMsg(msg, keysAndValues...))
}

func (s *loggerServiceImpl) Warn(ctx context.Context, msg string, keysAndValues ...any) {
	logger.WarnF(ctx, "%s", s.formatMsg(msg, keysAndValues...))
}

func (s *loggerServiceImpl) Error(ctx context.Context, msg string, keysAndValues ...any) {
	logger.ErrorF(ctx, "%s", s.formatMsg(msg, keysAndValues...))
}

func (s *loggerServiceImpl) Debugf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.DebugF(ctx, "%s", s.formatMsg(msg))
}

func (s *loggerServiceImpl) Infof(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.InfoF(ctx, "%s", s.formatMsg(msg))
}

func (s *loggerServiceImpl) Warnf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.WarnF(ctx, "%s", s.formatMsg(msg))
}

func (s *loggerServiceImpl) Errorf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.ErrorF(ctx, "%s", s.formatMsg(msg))
}

func (s *loggerServiceImpl) With(keysAndValues ...any) contracts.LoggerService {
	merged := make([]any, 0, len(s.extraFields)+len(keysAndValues))
	merged = append(merged, s.extraFields...)
	merged = append(merged, keysAndValues...)
	return &loggerServiceImpl{
		extraFields: merged,
	}
}
