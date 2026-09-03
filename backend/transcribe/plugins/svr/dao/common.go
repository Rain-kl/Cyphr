// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dao provides data access objects for transcribe entities.
package dao

import (
	"Wavelet/pkg/idgen"
	"Wavelet/transcribe/plugins/svr/consts"
	"errors"
	"time"

	"gorm.io/gorm"
)

// mapNotFound maps GORM's ErrRecordNotFound to consts.ErrRecordNotFound.
func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return consts.ErrRecordNotFound
	}
	return err
}

// generateID returns existing id if non-zero, or generates a new snowflake ID.
func generateID(existing uint64) uint64 {
	if existing != 0 {
		return existing
	}
	var id uint64
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Fallback to timestamp-based ID if idgen is not initialized
				id = uint64(time.Now().UnixNano())
			}
		}()
		id = idgen.NextUint64ID()
	}()
	if id == 0 {
		id = uint64(time.Now().UnixNano())
	}
	return id
}
