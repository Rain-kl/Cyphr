// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"sync"
	"time"

	pkgpush "Wavelet/plugins/domain/message_gateway/push"
)

// Push channel and payload constants.
const (
	ChannelCustom    = "custom"
	ChannelEmail     = "email"
	ChannelLark      = "lark"
	ChannelTelegram  = "telegram"
	DefaultLevelInfo = "INFO"
	KeyTitle         = "title"
	KeyContent       = "content"
	KeyLevel         = "level"

	// KeyURL represents the URL field key.
	KeyURL = "url"
	// KeyToken represents the Token field key.
	KeyToken = "token"
	// KeyOther represents the Other field key.
	KeyOther = "other"

	// TypeText represents standard text input type.
	TypeText = "text"
	// TypePassword represents password input type.
	TypePassword = "password"
	// TypeTextarea represents textarea input type.
	TypeTextarea = "textarea"
)

// SMTPConfig mirrors the system SMTP settings consumed by the push service.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

// PushField represents a form field configuration for a channel.
type PushField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder"`
	Description string `json:"description"`
}

// PushDefinition represents the metadata and form schema for a notification channel.
type PushDefinition struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Fields      []PushField `json:"fields"`
}

// CreatePushChannelRequest is the create channel request payload.
type CreatePushChannelRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// UpdatePushChannelRequest is the update channel request payload.
type UpdatePushChannelRequest struct {
	Description string `json:"description"`
	Type        string `json:"type" binding:"required"`
	Token       string `json:"token"`
	URL         string `json:"url"`
	Other       string `json:"other"`
	Enabled     bool   `json:"enabled"`
}

// TestPushChannelRequest is the test channel request payload.
type TestPushChannelRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Token  string `json:"token"`
	URL    string `json:"url"`
	Other  string `json:"other"`
	Target string `json:"target"`
}

// CustomPushRequest contains custom webhook parameters.
type CustomPushRequest struct {
	Title       string `json:"title" form:"title"`
	Description string `json:"description" form:"description"`
	Content     string `json:"content" form:"content"`
	URL         string `json:"url" form:"url"`
	To          string `json:"to" form:"to"`
	Token       string `json:"token" form:"token"`
}

// CreatePushEventRequest is the request body for creating a push event.
type CreatePushEventRequest struct {
	EventKey string   `json:"event_key"`
	TaskType string   `json:"task_type"`
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template"`
	Enabled  bool     `json:"enabled"`
}

// UpdatePushEventRequest is the request body for updating a push event.
type UpdatePushEventRequest struct {
	Channels []string `json:"channels"`
	Targets  []string `json:"targets"`
	Template string   `json:"template" binding:"required"`
	Enabled  bool     `json:"enabled"`
}

// TestPushRequest is the request body for testing push config.
type TestPushRequest struct {
	Config pkgpush.Config `json:"config" binding:"required"`
	Target string         `json:"target"`
}

// NotificationMessage represents the structured notification message payload.
type NotificationMessage struct {
	Title   string         `json:"title"`
	Content string         `json:"content"`
	Level   string         `json:"level"`
	Ext     map[string]any `json:"ext,omitempty"`
}

// Flatten converts the structured NotificationMessage back to a flat map (original json structure).
func (m NotificationMessage) Flatten() map[string]any {
	res := map[string]any{
		KeyTitle:   m.Title,
		KeyContent: m.Content,
		KeyLevel:   m.Level,
	}
	for k, v := range m.Ext {
		res[k] = v
	}
	return res
}

// EventMetadata represents the metadata of a push notification event.
type EventMetadata struct {
	Key             string              `json:"key"`
	Name            string              `json:"name"`
	DefaultTemplate NotificationMessage `json:"default_template"`
	Description     string              `json:"description"`
}

// SendPayload is the async push dispatch载荷 consumed by the notification worker.
type SendPayload struct {
	EventKey string              `json:"event_key"`
	Config   pkgpush.Config      `json:"config"`
	Target   string              `json:"target"`
	Body     NotificationMessage `json:"body"`
	Template string              `json:"template"`
}

// PushHistoryListFilter filters push history pagination queries.
type PushHistoryListFilter struct {
	EventKey  string
	Channel   string
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

var (
	pushDefMu       sync.RWMutex
	pushDefinitions = make(map[string]PushDefinition)
)

// RegisterPushChannelDefinition registers a channel definition.
func RegisterPushChannelDefinition(def PushDefinition) {
	pushDefMu.Lock()
	defer pushDefMu.Unlock()
	pushDefinitions[def.Type] = def
}

// ListPushDefinitions returns all registered channel definitions.
func ListPushDefinitions() []PushDefinition {
	pushDefMu.RLock()
	defer pushDefMu.RUnlock()

	order := []string{ChannelCustom, ChannelLark, ChannelTelegram, ChannelEmail}
	res := make([]PushDefinition, 0, len(pushDefinitions))
	for _, t := range order {
		if d, ok := pushDefinitions[t]; ok {
			res = append(res, d)
		}
	}
	for t, d := range pushDefinitions {
		found := false
		for _, o := range order {
			if o == t {
				found = true
				break
			}
		}
		if !found {
			res = append(res, d)
		}
	}
	return res
}

func init() {
	RegisterPushChannelDefinition(PushDefinition{
		Type:        ChannelCustom,
		Name:        "自定义消息通道",
		Description: "使用自定义 HTTP POST 请求向外部 Webhook 发送数据。",
		Fields: []PushField{
			{
				Key:         KeyURL,
				Label:       "请求地址",
				Type:        TypeText,
				Required:    true,
				Placeholder: "在此填写完整的请求地址，必须使用 HTTPS 协议",
				Description: "接口请求的完整 HTTPS URL，例如 https://api.example.com/webhook",
			},
			{
				Key:         KeyOther,
				Label:       "请求体 (JSON)",
				Type:        TypeTextarea,
				Required:    true,
				Placeholder: "在此输入请求体，支持模板变量，必须为合法的 JSON 格式",
				Description: "可使用的变量：$title, $description, $content, $url, $to。例如 {\"text\": \"$content\"}",
			},
		},
	})

	RegisterPushChannelDefinition(PushDefinition{
		Type:        ChannelLark,
		Name:        "飞书群机器人",
		Description: "配置飞书群自定义机器人的 Webhook 接口投递。",
		Fields: []PushField{
			{
				Key:         KeyURL,
				Label:       "Webhook 地址",
				Type:        TypeText,
				Required:    true,
				Placeholder: "https://open.feishu.cn/open-apis/bot/v2/hook/YOUR_TOKEN",
				Description: "从飞书群机器人设置中复制的 Webhook URL",
			},
			{
				Key:         KeyToken,
				Label:       "签名校验密钥 (Secret) (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "可选，若机器人启用了安全设置中的签名校验，请在此输入",
				Description: "飞书群机器人安全设置中的签名校验 Key",
			},
			{
				Key:         KeyOther,
				Label:       "自定义卡片 JSON 模版 (可选)",
				Type:        TypeTextarea,
				Required:    false,
				Placeholder: "可选，留空则默认使用系统内置的精美互动卡片",
				Description: "若填写，必须是合法的飞书卡片 JSON 格式",
			},
		},
	})

	RegisterPushChannelDefinition(PushDefinition{
		Type:        ChannelTelegram,
		Name:        "Telegram 机器人",
		Description: "配置 Telegram 机器人推送消息。",
		Fields: []PushField{
			{
				Key:         KeyURL,
				Label:       "API 基础地址 (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "https://api.telegram.org",
				Description: "接口请求的 HTTPS 基础地址，留空默认为 https://api.telegram.org",
			},
			{
				Key:         KeyToken,
				Label:       "机器人 Token (Bot Token)",
				Type:        TypePassword,
				Required:    true,
				Placeholder: "在此输入 Telegram 机器人的 Bot Token",
				Description: "通过 BotFather 申请到的机器人 Access Token",
			},
			{
				Key:         KeyOther,
				Label:       "默认会话 ID (Chat ID) (可选)",
				Type:        TypeText,
				Required:    false,
				Placeholder: "例如 -100123456789 或 @channel_name",
				Description: "默认的消息接收 Chat ID。如果通知事件中未配置 targets，将推送到此 ID",
			},
		},
	})

	RegisterPushChannelDefinition(PushDefinition{
		Type:        ChannelEmail,
		Name:        "邮件推送通道",
		Description: "邮件推送通道直接使用系统全局 SMTP 设置进行发送，无需在此填写服务器配置。",
		Fields:      []PushField{},
	})
}
