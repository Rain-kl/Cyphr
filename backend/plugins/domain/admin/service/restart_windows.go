//go:build windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/admin/errs"
	"errors"
)

// ReplaceAndRestart is blocked on Windows.
func ReplaceAndRestart(_, _ string) error {
	return errors.New(errs.ErrAutomaticUpgradeBlocked)
}
