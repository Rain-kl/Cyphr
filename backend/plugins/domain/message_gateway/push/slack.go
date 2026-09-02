// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"Wavelet/pkg/httppool"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func init() {
	Register("slack", &SlackPusher{})
}

// SlackPusher Slack Webhook 推送实现
type SlackPusher struct{}

type slackPayload struct {
	Text string `json:"text"`
}

// Send 发送 Slack 通知
func (p *SlackPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, _ string, _ map[string]any) (string, error) {
	if cfg.URL == "" {
		return "", errors.New("slack: webhook URL is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "*%s*: %v", "\n")

	text := fmt.Sprintf("*%s*\n%s", title, content)
	payload := slackPayload{
		Text: text,
	}

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("slack: marshal payload failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("slack: create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("slack: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	upstreamResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamResp, fmt.Errorf("slack: http status %s", resp.Status)
	}

	return upstreamResp, nil
}

// ValidateConfig 校验 Slack 配置
func (p *SlackPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must start with https://")
	}
	return nil
}
