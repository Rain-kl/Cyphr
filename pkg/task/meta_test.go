// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task_test

import (
	"testing"

	"github.com/Rain-kl/Wavelet/pkg/task"
)

func TestDuplicateTaskMeta(t *testing.T) {
	dummyMeta := task.TaskMeta{
		Type:      "test_duplicate_task",
		AsynqTask: "test:duplicate_task",
		Name:      "Test Duplicate Task",
	}

	task.RegisterTaskMeta(dummyMeta)
	task.RegisterTaskMeta(dummyMeta)

	metas := task.GetDispatchableTasks()

	// Check if we have duplicates by checking if a Type appears more than once
	seen := make(map[string]int)
	for _, m := range metas {
		seen[m.Type]++
	}

	for taskType, count := range seen {
		if count > 1 {
			t.Errorf("Task type %q registered %d times, expected at most 1", taskType, count)
		}
	}
}
