// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/pkg/response"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	pkgpush "Wavelet/plugins/domain/message_gateway/push"
)

const (
	// KeyURL represents the URL field key
	KeyURL = "url"
	// KeyToken represents the Token field key
	KeyToken = "token"
	// KeyOther represents the Other field key
	KeyOther = "other"

	// TypeText represents standard text input type
	TypeText = "text"
	// TypePassword represents password input type
	TypePassword = "password"
	// TypeTextarea represents textarea input type
	TypeTextarea = "textarea"
)

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

	order := []string{channelCustom, channelLark, channelTelegram, channelEmail}
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
		Type:        channelCustom,
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
		Type:        channelLark,
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
		Type:        channelTelegram,
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
		Type:        channelEmail,
		Name:        "邮件推送通道",
		Description: "邮件推送通道直接使用系统全局 SMTP 设置进行发送，无需在此填写服务器配置。",
		Fields:      []PushField{},
	})
}

// ListPushChannelDefinitions returns channel definitions.
func ListPushChannelDefinitions(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(ListPushDefinitions()))
}

// ListPushChannels lists configured push channels.
func ListPushChannels(c *gin.Context) {
	channels, err := listPushChannels(c.Request.Context())
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(channels))
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

func parsePushChannelID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.AbortBadRequest(c, "invalid channel id")
		return 0, false
	}
	return id, true
}

func handlePushChannelNotFoundError(c *gin.Context, err error, fallback func(c *gin.Context, msg string)) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, "channel not found")
		return
	}
	fallback(c, err.Error())
}

// CreatePushChannel creates a push channel.
func CreatePushChannel(c *gin.Context) {
	handleJSONRequest(c, createPushChannel)
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

// UpdatePushChannel updates a push channel.
func UpdatePushChannel(c *gin.Context) {
	handleEntityUpdate(c, parsePushChannelID, updatePushChannel, func(c *gin.Context, err error) {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
	})
}

// DeletePushChannel deletes a push channel.
func DeletePushChannel(c *gin.Context) {
	id, ok := parsePushChannelID(c)
	if !ok {
		return
	}

	if err := deletePushChannel(c.Request.Context(), id); err != nil {
		handlePushChannelNotFoundError(c, err, response.AbortInternal)
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
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

// TestPushChannel tests connectivity of a push channel.
func TestPushChannel(c *gin.Context) {
	var req TestPushChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	url, token, other, channelType, err := loadChannelForTest(ctx, req)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if channelType == channelEmail {
		url, token, other = resolveSMTPConfig(ctx, url, token, other)
	}

	tempChannel := PushChannel{
		Name:    "test_temp",
		URL:     url,
		Token:   token,
		Other:   other,
		Type:    channelType,
		Enabled: true,
	}
	if err := tempChannel.Validate(); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	url = tempChannel.URL

	var config pkgpush.Config
	var renderedJSON string
	switch channelType {
	case channelLark:
		config = pkgpush.Config{Channel: channelLark, URL: url, Secret: token}
		renderedJSON = other
	case channelEmail:
		config = pkgpush.Config{Channel: channelEmail, URL: url, Key: token, Secret: other}
	case channelTelegram:
		config = pkgpush.Config{Channel: channelTelegram, URL: url, Secret: token, Key: other}
	default:
		config = pkgpush.Config{Channel: channelCustom, URL: url}
		customPushReq := CustomPushRequest{
			Title:       "通道测试通知",
			Content:     "这是一条来自系统的消息通道连通性测试消息。",
			Description: "系统通道测试",
			URL:         "https://example.com",
			To:          req.Target,
		}
		renderedJSON = renderCustomPayload(other, customPushReq)
	}

	payload := SendPayload{
		EventKey: "test_channel",
		Config:   config,
		Target:   req.Target,
		Body: NotificationMessage{
			Title:   "通道测试通知",
			Content: "这是一条来自系统的消息通道连通性测试消息。",
			Level:   defaultLevelInfo,
		},
		Template: renderedJSON,
	}
	if err := enqueuePushTask(ctx, payload); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
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

func escapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	const minJSONLen = 2
	if len(b) >= minJSONLen {
		return string(b[1 : len(b)-1])
	}
	return s
}

func renderCustomPayload(template string, req CustomPushRequest) string {
	result := template
	result = strings.ReplaceAll(result, "$title", escapeJSONString(req.Title))
	result = strings.ReplaceAll(result, "$description", escapeJSONString(req.Description))
	result = strings.ReplaceAll(result, "$content", escapeJSONString(req.Content))
	result = strings.ReplaceAll(result, "$url", escapeJSONString(req.URL))
	result = strings.ReplaceAll(result, "$to", escapeJSONString(req.To))
	return result
}
