// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"time"

	"github.com/Rain-kl/Wavelet/core/contracts"
)

// AdminLogin is the metadata definition for the admin login event.
var AdminLogin = EventMetadata{
	Key:  "admin_login",
	Name: "管理员登录",
	DefaultTemplate: NotificationMessage{
		Title:   "管理员登录提醒",
		Content: "管理员 {{user.username}} 于 {{time}} 从 IP {{ip}} 登录系统。",
		Level:   "INFO",
	},
	Description: "当管理员成功登录系统时触发此通知",
}

// HandleAdminLoggedIn 处理管理员登录事件并触发通知
func HandleAdminLoggedIn(ctx context.Context, event contracts.AdminLoggedIn) {
	if event.User == nil {
		return
	}

	body := map[string]any{
		"user": event.User,
		"ip":   event.IP,
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	DefaultTrigger.Trigger(ctx, AdminLogin, body)
}

// RegisterCustomEvents registers default domain push notification events.
func RegisterCustomEvents() {
	RegisterBuiltInEvent(AdminLogin)
}
