// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"log/slog"
	"runtime"
	"runtime/debug"
)

// Go runs fn in a new goroutine and recovers panics, so a background task
// cannot crash the whole process. The panic is logged together with the
// util.Go call site. Use it for every fire-and-forget / long-lived
// background goroutine; HTTP handlers are already covered by gin.Recovery.
func Go(fn func()) {
	pc, file, line, _ := runtime.Caller(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in background goroutine",
					"caller", runtime.FuncForPC(pc).Name(),
					"file", file,
					"line", line,
					"panic", r,
					"stack", string(debug.Stack()))
			}
		}()
		fn()
	}()
}
