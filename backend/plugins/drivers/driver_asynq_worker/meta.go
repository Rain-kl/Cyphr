// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"sync"

	"Wavelet/core/extpoints"
)

// TaskParam 任务参数定义
//
//nolint:revive // TaskParam 保留完整名称以避免与通用 Param 混淆
type TaskParam struct {
	Name        string `json:"name"`        // 参数键名
	Label       string `json:"label"`       // 显示名称
	Type        string `json:"type"`        // 类型：string, text, number, boolean
	Required    bool   `json:"required"`    // 是否必填
	Placeholder string `json:"placeholder"` // 占位符
	Description string `json:"description"` // 描述
}

// TaskMeta 任务元数据
//
//nolint:revive // TaskMeta 保留完整名称以避免与通用 Meta 混淆
type TaskMeta struct {
	Type         string      `json:"type"`
	AsynqTask    string      `json:"asynq_task"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	SupportsTime bool        `json:"supports_time"`
	MaxRetry     int         `json:"max_retry"`
	Queue        string      `json:"queue"`
	Retryable    bool        `json:"retryable"` // 是否支持手动重试
	Params       []TaskParam `json:"params,omitempty"`
}

var (
	dispatchableTasksMutex sync.RWMutex
	dispatchableTasks      []TaskMeta
)

// RegisterTaskMeta 注册任务元数据到全局列表
func RegisterTaskMeta(meta TaskMeta) {
	dispatchableTasksMutex.Lock()
	defer dispatchableTasksMutex.Unlock()
	for _, t := range dispatchableTasks {
		if t.Type == meta.Type {
			return
		}
	}
	dispatchableTasks = append(dispatchableTasks, meta)
}

// GetDispatchableTasks 获取所有已注册的元数据列表（返回副本以避免并发读写冲突）
func GetDispatchableTasks() []TaskMeta {
	dispatchableTasksMutex.RLock()
	defer dispatchableTasksMutex.RUnlock()

	metas := make([]TaskMeta, len(dispatchableTasks))
	copy(metas, dispatchableTasks)
	return metas
}

var (
	activeTaskRegMutex sync.RWMutex
	activeTaskReg      extpoints.TaskExtension
)

// SetActiveTaskExtension sets the active task extension registry for task resolution.
func SetActiveTaskExtension(reg extpoints.TaskExtension) {
	activeTaskRegMutex.Lock()
	defer activeTaskRegMutex.Unlock()
	activeTaskReg = reg
}

func getFromActiveTaskExtension(taskType string) *TaskMeta {
	activeTaskRegMutex.RLock()
	defer activeTaskRegMutex.RUnlock()
	if activeTaskReg == nil {
		return nil
	}
	if td, ok := activeTaskReg.Get(taskType); ok {
		return &TaskMeta{
			Type:      td.Pattern,
			Name:      td.Pattern,
			AsynqTask: td.Pattern,
			Queue:     "default",
			Retryable: td.Retry > 0,
			MaxRetry:  td.Retry,
		}
	}
	return nil
}

// GetTaskMeta 根据任务类型获取元数据
func GetTaskMeta(taskType string) *TaskMeta {
	dispatchableTasksMutex.RLock()
	for _, t := range dispatchableTasks {
		if t.Type == taskType {
			copied := t
			dispatchableTasksMutex.RUnlock()
			return &copied
		}
	}
	dispatchableTasksMutex.RUnlock()

	return getFromActiveTaskExtension(taskType)
}

// GetTaskMetaByAsynqTask 根据 Asynq 任务名称获取元数据
func GetTaskMetaByAsynqTask(asynqTask string) *TaskMeta {
	dispatchableTasksMutex.RLock()
	for _, t := range dispatchableTasks {
		if t.AsynqTask == asynqTask {
			copied := t
			dispatchableTasksMutex.RUnlock()
			return &copied
		}
	}
	dispatchableTasksMutex.RUnlock()

	return getFromActiveTaskExtension(asynqTask)
}

// GetRegisteredAsynqTasks 返回所有已注册的 Asynq 任务名称，以便动态注册路由
func GetRegisteredAsynqTasks() []string {
	handlerRegistryMutex.RLock()
	defer handlerRegistryMutex.RUnlock()

	keys := make([]string, 0, len(handlerRegistry))
	for k := range handlerRegistry {
		keys = append(keys, k)
	}
	return keys
}
