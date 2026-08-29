// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"Wavelet/plugins/domain/upload/shared"

	"context"
	"strings"
	"testing"
)

// TestMigrationHandlerRejectsCorruptActiveConfig pins the failure path the task
// package used to swallow. Its local loader discarded the read error and the
// JSON parse error and returned a zero config with a nil error, so a migration
// could proceed believing the active storage driver was simply unknown.
func TestMigrationHandlerRejectsCorruptActiveConfig(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()

	if err := dbConn.Table("w_system_configs").
		Where("key = ?", "storage_config").
		Update("value", "{ this is not json").Error; err != nil {
		t.Fatalf("corrupt stored config: %v", err)
	}

	payload := []byte(`{"target":{"driver":"s3","s3":{"region":"us-east-1","bucket":"dst"}}}`)

	result, err := (&MigrationHandler{}).Execute(context.Background(), payload)
	if err == nil {
		t.Fatalf("Execute() = (%v, nil), want an error for an unparsable active config", result)
	}
	if !strings.Contains(err.Error(), "load active storage config") {
		t.Errorf("Execute() error = %q, want it to name the failed active-config read", err)
	}
}
