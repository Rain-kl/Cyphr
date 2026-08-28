// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

func TestPermanentError(t *testing.T) {
	err := PermanentError("自定义永久错误")
	if err == nil {
		t.Fatal("PermanentError returned nil")
	}
	if err.Error() != "自定义永久错误" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "自定义永久错误")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatal("PermanentError does not unwrap to asynq.SkipRetry")
	}
}

func TestPermanentErrorDefaultMessage(t *testing.T) {
	err := PermanentError("   ")
	if err == nil {
		t.Fatal("PermanentError returned nil")
	}
	if err.Error() != defaultPermanentErrorMessage {
		t.Fatalf("Error() = %q, want default %q", err.Error(), defaultPermanentErrorMessage)
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatal("PermanentError does not unwrap to asynq.SkipRetry")
	}
}
