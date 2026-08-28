// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"sync"
	"testing"
)

func resetDispatchableTasks() {
	dispatchableTasksMutex.Lock()
	defer dispatchableTasksMutex.Unlock()
	dispatchableTasks = nil
}

func TestRegisterTaskMeta_ThreadSafe(t *testing.T) {
	resetDispatchableTasks()
	defer resetDispatchableTasks()

	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				RegisterTaskMeta(TaskMeta{
					Type:      "task_type_a",
					AsynqTask: "asynq_task_a",
					Name:      "Task A",
				})
				RegisterTaskMeta(TaskMeta{
					Type:      "task_type_b",
					AsynqTask: "asynq_task_b",
					Name:      "Task B",
				})
			}
		}(i)
	}

	wg.Wait()

	tasks := GetDispatchableTasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 unique tasks, got %d", len(tasks))
	}
}

func TestGetTaskMeta_Isolation(t *testing.T) {
	resetDispatchableTasks()
	defer resetDispatchableTasks()

	RegisterTaskMeta(TaskMeta{
		Type:      "task_isolated",
		AsynqTask: "asynq_isolated",
		Name:      "Original Name",
	})

	meta := GetTaskMeta("task_isolated")
	if meta == nil {
		t.Fatal("expected task to be found")
	}

	// Modify returned struct
	meta.Name = "Modified Name"

	// Fetch again and verify original data is unaffected
	meta2 := GetTaskMeta("task_isolated")
	if meta2.Name != "Original Name" {
		t.Fatalf("expected name to be 'Original Name', got %s", meta2.Name)
	}
}

func TestGetTaskMetaByAsynqTask_Isolation(t *testing.T) {
	resetDispatchableTasks()
	defer resetDispatchableTasks()

	RegisterTaskMeta(TaskMeta{
		Type:      "task_isolated",
		AsynqTask: "asynq_isolated",
		Name:      "Original Name",
	})

	meta := GetTaskMetaByAsynqTask("asynq_isolated")
	if meta == nil {
		t.Fatal("expected task to be found")
	}

	meta.Name = "Modified Name"

	meta2 := GetTaskMetaByAsynqTask("asynq_isolated")
	if meta2.Name != "Original Name" {
		t.Fatalf("expected name to be 'Original Name', got %s", meta2.Name)
	}
}
