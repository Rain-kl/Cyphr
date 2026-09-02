// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"Wavelet/pkg/httppool"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	pushoverAPIEndpoint = "https://api.pushover.net/1/messages.json"
)

func init() {
	Register("pushover", &PushoverPusher{})
}

// PushoverPusher Pushover 移动端推送实现
type PushoverPusher struct{}

// Send 发送 Pushover 通知
func (p *PushoverPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	appToken := cfg.Key
	if appToken == "" {
		appToken = cfg.Secret
	}
	if appToken == "" {
		return "", errors.New("pushover: app token is required")
	}

	userKey := cfg.URL
	if userKey == "" && cfg.Ext != nil {
		if k, ok := cfg.Ext["user_key"].(string); ok {
			userKey = k
		}
	}
	if target != "" {
		userKey = target
	}
	if userKey == "" {
		return "", errors.New("pushover: user key is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "%s: %v", "\n")

	formData := url.Values{}
	formData.Set("token", appToken)
	formData.Set("user", userKey)
	formData.Set("title", title)
	formData.Set("message", content)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, pushoverAPIEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("pushover: create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("pushover: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	upstreamResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamResp, fmt.Errorf("pushover: http status %s", resp.Status)
	}

	return upstreamResp, nil
}

// ValidateConfig 校验 Pushover 配置
func (p *PushoverPusher) ValidateConfig(cfg Config) error {
	appToken := cfg.Key
	if appToken == "" {
		appToken = cfg.Secret
	}
	if appToken == "" {
		return errors.New("app token is required")
	}
	return nil
}
