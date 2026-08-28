// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"context"
	"encoding/json"
	"strconv"
	"time"
)

func handleTaskCompleted(ctx context.Context, e contracts.TaskCompletedEvent) {
	events, err := listActivePushEventsByTaskType(ctx, e.TaskType)
	if err != nil {
		logger.ErrorF(ctx, "push_task_completed_listener: failed to query push events for task type %s: %v", e.TaskType, err)
		return
	}
	if len(events) == 0 {
		return
	}

	body := map[string]any{
		"task_id":       e.TaskID,
		"task_name":     e.TaskName,
		"task_type":     e.TaskType,
		"task_status":   e.Status,
		"task_duration": e.Duration,
		"time":          time.Now().Format("2006-01-02 15:04:05"),
		"task_error":    e.ErrorMsg,
		"task_result":   e.ResultMsg,
	}

	var payloadMap map[string]any
	if e.Payload != "" {
		if err := json.Unmarshal([]byte(e.Payload), &payloadMap); err == nil {
			body["payload"] = payloadMap
			extractUserFromMap(ctx, payloadMap, body)
		}
	}
	if e.Detail != "" {
		var detailMap map[string]any
		if err := json.Unmarshal([]byte(e.Detail), &detailMap); err == nil {
			body["detail"] = detailMap
			extractUserFromMap(ctx, detailMap, body)
		}
	}

	for _, event := range events {
		meta := EventMetadata{
			Key:         event.EventKey,
			Name:        event.Name,
			Description: "异步任务执行完毕触发的自动通知",
		}
		DefaultTrigger.Trigger(ctx, meta, body)
	}
}

func extractUserFromMap(ctx context.Context, data, body map[string]any) {
	if u, exists := body["user"]; exists && u != nil {
		return
	}
	if user := loadUserFromPayload(ctx, data); user != nil {
		body["user"] = user
	}
}

func extractUserID(data map[string]any) (uint64, bool) {
	for _, k := range []string{"user_id", "userId", "uid"} {
		val, ok := data[k]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case float64:
			if v >= 0 {
				return uint64(v), true
			}
		case int:
			if v >= 0 {
				return uint64(v), true
			}
		case int64:
			if v >= 0 {
				return uint64(v), true
			}
		case uint64:
			return v, true
		case string:
			if id, err := strconv.ParseUint(v, 10, 64); err == nil {
				return id, true
			}
		}
	}
	return 0, false
}

func extractUsername(data map[string]any) string {
	for _, k := range []string{"username", "user_name"} {
		if val, ok := data[k]; ok && val != nil {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
