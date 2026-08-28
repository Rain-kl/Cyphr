// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/message_gateway/push"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// SendNotificationTask is the asynq task name for push notification.
	SendNotificationTask = "push:send"
	// TaskTypeSendNotification is the admin task manager type identifier.
	TaskTypeSendNotification = "send_notification"
)

// SendNotificationMeta represents the task metadata.
var SendNotificationMeta = contracts.TaskMetaDTO{
	Name:        TaskTypeSendNotification,
	DisplayName: "推送通知",
	Description: "异步执行系统通知的多渠道派发与推送",
	MaxRetry:    3,
	Queue:       "default",
	Params: []contracts.TaskParamDTO{
		{
			Name:        "event_key",
			Type:        "string",
			Description: "事件标识 (如 admin_login)",
			Required:    true,
		},
		{
			Name:        "target",
			Type:        "string",
			Description: "目标接收者",
			Required:    false,
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
func (h *PushHandler) Execute(ctx context.Context, payload []byte) error {
	var req SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.ErrorF(ctx, "[Push] 解析推送参数失败: %v", err)
		return fmt.Errorf("parse payload failed: %w", err)
	}

	logger.InfoF(ctx, "[Push] 开始推送通知: 事件 = %s, 渠道 = %s, 接收目标 = %s", req.EventKey, req.Config.Channel, req.Target)

	pusher, err := push.GetPusher(req.Config.Channel)
	if err != nil {
		errWrap := fmt.Errorf("get pusher failed: %w", err)
		logger.ErrorF(ctx, "[Push] 推送失败: %v", errWrap)
		h.recordHistory(ctx, req, "failed", errWrap.Error())
		return errWrap
	}

	flatBody := req.Body.Flatten()
	upstreamResp, err := pusher.Send(ctx, req.Config, req.Target, flatBody, req.Template, nil)

	title := req.Body.Title
	content := req.Body.Content

	if err != nil {
		logger.ErrorF(ctx, "[Push] 消息推送失败 (标题: %s): %v, 上游返回: %s", title, err, upstreamResp)
		h.recordHistory(ctx, req, "failed", err.Error())
		return fmt.Errorf("pusher.Send failed: %w", err)
	}

	logger.InfoF(ctx, "[Push] 消息推送成功 (标题: %s, 内容摘要: %s), 上游返回: %s", title, content, upstreamResp)
	h.recordHistory(ctx, req, "success", "")

	return nil
}

func (h *PushHandler) recordHistory(ctx context.Context, req SendPayload, status, errMsg string) {
	if dbErr := recordPushHistory(ctx, req, status, errMsg); dbErr != nil {
		logger.ErrorF(ctx, "[Push] 写入推送历史审计记录失败: %v", dbErr)
	}
}
