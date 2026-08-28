// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"sync"

	"github.com/Rain-kl/Wavelet/backend/pkg/logger"
)

// Runner manages lifecycle for long-lived channel adapters (WebSocket, long-polling, etc.).
type Runner struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// GlobalRunner is the default global runner instance.
var GlobalRunner = &Runner{}

// Start starts all background long-lived channel runners.
func Start(ctx context.Context) error {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if GlobalRunner.running {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	GlobalRunner.cancel = cancel
	GlobalRunner.running = true

	logger.InfoF(runCtx, "[MessageGateway] Starting bot channel runners...")
	return nil
}

// Stop stops the channel runner.
func Stop() {
	GlobalRunner.mu.Lock()
	defer GlobalRunner.mu.Unlock()

	if !GlobalRunner.running {
		return
	}

	if GlobalRunner.cancel != nil {
		GlobalRunner.cancel()
	}
	GlobalRunner.running = false
}
