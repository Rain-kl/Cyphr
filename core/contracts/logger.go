// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
)

// LoggerService defines the contract for structured logging with trace ID and context correlation.
type LoggerService interface {
	// Debug logs a debug message with optional key-value structured fields.
	Debug(ctx context.Context, msg string, keysAndValues ...any)

	// Info logs an informational message with optional key-value structured fields.
	Info(ctx context.Context, msg string, keysAndValues ...any)

	// Warn logs a warning message with optional key-value structured fields.
	Warn(ctx context.Context, msg string, keysAndValues ...any)

	// Error logs an error message with optional key-value structured fields.
	Error(ctx context.Context, msg string, keysAndValues ...any)

	// Debugf logs a formatted debug message.
	Debugf(ctx context.Context, format string, args ...any)

	// Infof logs a formatted informational message.
	Infof(ctx context.Context, format string, args ...any)

	// Warnf logs a formatted warning message.
	Warnf(ctx context.Context, format string, args ...any)

	// Errorf logs a formatted error message.
	Errorf(ctx context.Context, format string, args ...any)

	// With returns a child logger enriched with additional key-value attributes.
	With(keysAndValues ...any) LoggerService
}
