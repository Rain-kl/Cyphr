// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"

	"Wavelet/core/extpoints"
)

// ConfigGet reads one resolved configuration value with its declared type. It is the
// generic counterpart of the fallback accessors on ConfigView, used when a caller must
// distinguish "unset" from "set to the zero value".
func ConfigGet[T any](view extpoints.ConfigView, key string) (T, error) {
	var zero T
	if view == nil {
		return zero, extpoints.ErrConfigNotResolved
	}

	raw, ok := view.Value(key)
	if !ok {
		return zero, fmt.Errorf("%w: %s", extpoints.ErrConfigUnknownKey, key)
	}

	value, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%w: key %q holds %T, want %T", extpoints.ErrConfigType, key, raw, zero)
	}
	return value, nil
}
