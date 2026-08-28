// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

// EventTopicAdminLoggedIn 管理员登录事件主题
const EventTopicAdminLoggedIn = "admin:logged_in"

// AdminLoggedIn 管理员登录领域事件载荷
type AdminLoggedIn struct {
	User *UserDTO `json:"user"`
	IP   string   `json:"ip"`
}
