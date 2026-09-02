// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"Wavelet/pkg/httppool"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register("dingtalk", &DingTalkPusher{})
}

// DingTalkPusher 钉钉机器人 Webhook 推送实现
type DingTalkPusher struct{}

type dingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type dingTalkMessage struct {
	MsgType  string           `json:"msgtype"`
	Markdown dingTalkMarkdown `json:"markdown"`
}

// Send 发送钉钉通知
func (p *DingTalkPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, _ string, _ map[string]any) (string, error) {
	if cfg.URL == "" {
		return "", errors.New("dingtalk: webhook URL is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "**%s**: %v", "\n\n")

	webhookURL := cfg.URL
	// 如果配置了签名 Secret (Key 或 Secret 字段)，计算时间戳与签名
	secret := cfg.Secret
	if secret == "" {
		secret = cfg.Key
	}
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		stringToSign := timestamp + "\n" + secret
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(stringToSign))
		signature := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

		sep := "?"
		if strings.Contains(webhookURL, "?") {
			sep = "&"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", webhookURL, sep, timestamp, signature)
	}

	markdownText := fmt.Sprintf("### %s\n\n%s", title, content)
	msg := dingTalkMessage{
		MsgType: "markdown",
		Markdown: dingTalkMarkdown{
			Title: title,
			Text:  markdownText,
		},
	}

	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("dingtalk: marshal message failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("dingtalk: create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("dingtalk: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	upstreamResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamResp, fmt.Errorf("dingtalk: http status %s", resp.Status)
	}

	var dingResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &dingResp); err == nil && dingResp.ErrCode != 0 {
		return upstreamResp, fmt.Errorf("dingtalk: api error code %d: %s", dingResp.ErrCode, dingResp.ErrMsg)
	}

	return upstreamResp, nil
}

// ValidateConfig 校验钉钉配置
func (p *DingTalkPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must use https:// protocol")
	}
	return nil
}
