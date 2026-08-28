//go:build embed_frontend

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var frontendFS embed.FS

// frontendAssets returns the exported Next.js bundle shipped inside this binary.
func frontendAssets() fs.FS {
	sub, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		panic("driver_http: embedded frontend bundle unavailable: " + err.Error())
	}

	return sub
}
