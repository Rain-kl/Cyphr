// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"

	"gorm.io/gorm"
)

// DBService defines the standard contract for relational database access and multi-datasource routing.
type DBService interface {
	// GORM returns the underlying GORM database instance.
	GORM() *gorm.DB

	// DB returns the GORM database instance bound to the given context.
	DB(ctx context.Context) *gorm.DB

	// Named returns a named database connection if multiple data sources or replicas are configured.
	Named(name string) *gorm.DB
}
