// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import "context"

// PublicConfigProvider supplies the payload for GET /api/v1/config/public
// when a downstream plugin replaces Wavelet's default {configs, app} JSON.
type PublicConfigProvider interface {
	PublicConfig(ctx context.Context) (any, error)
}
