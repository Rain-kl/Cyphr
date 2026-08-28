//go:build !embed_frontend

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import "io/fs"

// frontendAssets returns nil: this binary was built without the embedded frontend bundle,
// so unmatched routes keep Gin's default 404 and the frontend is served separately.
func frontendAssets() fs.FS { return nil }
