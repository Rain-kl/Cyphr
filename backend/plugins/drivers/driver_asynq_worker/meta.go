// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"sync"
)

// TaskParam 任务参数定义
//
// TaskParam 保留完整名称以避免与通用 Param 混淆
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
// TaskMeta 保留完整名称以避免与通用 Meta 混淆
type TaskMeta struct {
	Type         string      `json:"type"`
	AsynqTask    string      `json:"asynq_task"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Category     string      `json:"category,omitempty"`
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

// GetDispatchableTasks 获取所有已注册的元数据列表（优先结合 activeTaskReg 和 dispatchableTasks）
func GetDispatchableTasks() []TaskMeta {
	activeTaskRegMutex.RLock()
	reg := activeTaskReg
	activeTaskRegMutex.RUnlock()

	var metas []TaskMeta
	seen := make(map[string]bool)

	if reg != nil {
		for _, td := range reg.Tasks() {
			dto := td.ToDTO()
			m := toInternalTaskMeta(dto)
			metas = append(metas, m)
			seen[m.Type] = true
			seen[m.AsynqTask] = true
		}
	}

	dispatchableTasksMutex.RLock()
	for _, t := range dispatchableTasks {
		if !seen[t.Type] && !seen[t.AsynqTask] {
			metas = append(metas, t)
			seen[t.Type] = true
			seen[t.AsynqTask] = true
		}
	}
	dispatchableTasksMutex.RUnlock()

	return metas
}

func toInternalTaskMeta(dto contracts.TaskMetaDTO) TaskMeta {
	params := make([]TaskParam, 0, len(dto.Params))
	for _, p := range dto.Params {
		params = append(params, TaskParam{
			Name:        p.Name,
			Label:       p.Label,
			Type:        p.Type,
			Required:    p.Required,
			Placeholder: p.Placeholder,
			Description: p.Description,
		})
	}
	taskType := dto.Type
	if taskType == "" {
		taskType = dto.AsynqTask
	}
	if taskType == "" {
		taskType = dto.Name
	}
	asynqTask := dto.AsynqTask
	if asynqTask == "" {
		asynqTask = taskType
	}
	name := dto.Name
	if name == "" {
		name = dto.DisplayName
	}
	if name == "" {
		name = taskType
	}
	return TaskMeta{
		Type:         taskType,
		AsynqTask:    asynqTask,
		Name:         name,
		Description:  dto.Description,
		Category:     dto.Category,
		SupportsTime: dto.SupportsTime,
		MaxRetry:     dto.MaxRetry,
		Queue:        dto.Queue,
		Retryable:    dto.Retryable,
		Params:       params,
	}
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
	for _, td := range activeTaskReg.Tasks() {
		if td.Type == taskType || td.Pattern == taskType {
			m := toInternalTaskMeta(td.ToDTO())
			return &m
		}
	}
	return nil
}

// GetTaskMeta 根据任务类型获取元数据
func GetTaskMeta(taskType string) *TaskMeta {
	if m := getFromActiveTaskExtension(taskType); m != nil {
		return m
	}

	dispatchableTasksMutex.RLock()
	for _, t := range dispatchableTasks {
		if t.Type == taskType || t.AsynqTask == taskType {
			copied := t
			dispatchableTasksMutex.RUnlock()
			return &copied
		}
	}
	dispatchableTasksMutex.RUnlock()

	return nil
}

// GetTaskMetaByAsynqTask 根据 Asynq 任务名称获取元数据
func GetTaskMetaByAsynqTask(asynqTask string) *TaskMeta {
	return GetTaskMeta(asynqTask)
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
