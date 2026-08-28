// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"Wavelet/pkg/config"
	"testing"

	"github.com/redis/go-redis/v9/maintnotifications"
)

func TestMaintNotificationsConfig(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		wantMode maintnotifications.Mode
	}{
		{
			name:     "disabled mode when flag is false",
			enabled:  false,
			wantMode: maintnotifications.ModeDisabled,
		},
		{
			name:     "auto mode when flag is true",
			enabled:  true,
			wantMode: maintnotifications.ModeAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := maintNotificationsConfig(tt.enabled)
			if cfg == nil {
				t.Fatalf("maintNotificationsConfig(%v) = nil, want non-nil", tt.enabled)
			}
			if cfg.Mode != tt.wantMode {
				t.Fatalf("maintNotificationsConfig(%v).Mode = %v, want %v", tt.enabled, cfg.Mode, tt.wantMode)
			}
		})
	}
}

func TestPrefixedQueue(t *testing.T) {
	oldPrefix := config.Config.Redis.KeyPrefix
	defer func() {
		config.Config.Redis.KeyPrefix = oldPrefix
	}()

	config.Config.Redis.KeyPrefix = "test:"
	if got := PrefixedQueue("default"); got != "test:default" {
		t.Fatalf("PrefixedQueue() = %q, want %q", got, "test:default")
	}

	config.Config.Redis.KeyPrefix = ""
	if got := PrefixedQueue("default"); got != "default" {
		t.Fatalf("PrefixedQueue() = %q, want %q", got, "default")
	}
}
