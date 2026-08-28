// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"Wavelet/plugins/domain/message_gateway/push"
	"Wavelet/plugins/drivers/driver_asynq_worker"
)

const (
	// SendNotificationTask is the asynq task name for push notification.
	SendNotificationTask = "push:send"
	// TaskTypeSendNotification is the admin task manager type identifier.
	TaskTypeSendNotification = "send_notification"
)

// SendNotificationMeta represents the task metadata.
var SendNotificationMeta = driver_asynq_worker.TaskMeta{
	Type:         TaskTypeSendNotification,
	AsynqTask:    SendNotificationTask,
	Name:         "推送通知",
	Description:  "异步执行系统通知的多渠道派发与推送",
	SupportsTime: false,
	MaxRetry:     driver_asynq_worker.DefaultMaxRetry,
	Queue:        driver_asynq_worker.QueueDefault,
	Retryable:    true,
	Params: []driver_asynq_worker.TaskParam{
		{
			Name:        "event_key",
			Label:       "事件标识",
			Type:        "string",
			Required:    true,
			Placeholder: "admin_login",
		},
		{
			Name:     "target",
			Label:    "目标接收者",
			Type:     "string",
			Required: false,
		},
	},
}

// PushHandler handles asynchronous notification sending.
type PushHandler struct{}

// ValidatePayload validates and normalizes push parameters.
func (h *PushHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New("payload is required")
	}

	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid json format: %w", err)
	}

	if req.Config.Channel == "" {
		return nil, errors.New("channel type is required")
	}

	return json.Marshal(req)
}

// Execute performs the push send and logs delivery history audit.
func (h *PushHandler) Execute(ctx context.Context, payload []byte) (*driver_asynq_worker.TaskResult, error) {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		driver_asynq_worker.AppendLog(ctx, "解析推送参数失败: %v", err)
		return nil, fmt.Errorf("parse payload failed: %w", err)
	}

	driver_asynq_worker.AppendLog(ctx, "开始推送通知: 事件 = %s, 渠道 = %s, 接收目标 = %s", req.EventKey, req.Config.Channel, req.Target)

	pusher, err := push.GetPusher(req.Config.Channel)
	if err != nil {
		errWrap := fmt.Errorf("get pusher failed: %w", err)
		driver_asynq_worker.AppendLog(ctx, "推送失败: %v", errWrap)
		if driver_asynq_worker.IsFinalAttempt(ctx) {
			h.recordHistory(ctx, req, "failed", errWrap.Error())
		}
		return nil, errWrap
	}

	flatBody := req.Body.Flatten()
	upstreamResp, err := pusher.Send(ctx, req.Config, req.Target, flatBody, req.Template, nil)

	title := req.Body.Title
	content := req.Body.Content

	if err != nil {
		driver_asynq_worker.AppendLog(ctx, "消息推送失败 (标题: %s): %v", title, err)
		if upstreamResp != "" {
			driver_asynq_worker.AppendLog(ctx, "上游返回: %s", upstreamResp)
		}
		if driver_asynq_worker.IsFinalAttempt(ctx) {
			h.recordHistory(ctx, req, "failed", err.Error())
		}
		return nil, fmt.Errorf("pusher.Send failed: %w", err)
	}

	driver_asynq_worker.AppendLog(ctx, "消息推送成功 (标题: %s, 内容摘要: %s)", title, content)
	if upstreamResp != "" {
		driver_asynq_worker.AppendLog(ctx, "上游返回: %s", upstreamResp)
	}
	h.recordHistory(ctx, req, "success", "")

	return &driver_asynq_worker.TaskResult{
		Message: fmt.Sprintf("推送成功: [%s] -> %s", req.Config.Channel, req.Target),
	}, nil
}

func (h *PushHandler) recordHistory(ctx context.Context, req SendPayload, status string, errMsg string) {
	if dbErr := recordPushHistory(ctx, req, status, errMsg); dbErr != nil {
		driver_asynq_worker.AppendLog(ctx, "写入推送历史审计记录失败: %v", dbErr)
	}
}
