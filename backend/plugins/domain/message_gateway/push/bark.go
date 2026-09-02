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

const (
	defaultBarkServer = "https://api.day.app"
)

func init() {
	Register("bark", &BarkPusher{})
}

// BarkPusher Bark iOS 客户端通知推送实现
type BarkPusher struct{}

type barkPayload struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Group     string `json:"group,omitempty"`
	Sound     string `json:"sound,omitempty"`
	Icon      string `json:"icon,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Send 发送 Bark 通知
func (p *BarkPusher) Send(ctx context.Context, cfg Config, target string, body map[string]any, _ string, _ map[string]any) (string, error) {
	deviceKey := cfg.Key
	if deviceKey == "" {
		deviceKey = cfg.Secret
	}
	if target != "" {
		deviceKey = target
	}
	if deviceKey == "" {
		return "", errors.New("bark: device key is required")
	}

	serverURL := strings.TrimRight(cfg.URL, "/")
	if serverURL == "" {
		serverURL = defaultBarkServer
	}

	title := bodyTitle(body)
	content := bodyContent(body, "%s: %v", "\n")

	payload := barkPayload{
		DeviceKey: deviceKey,
		Title:     title,
		Body:      content,
		Group:     "Wavelet",
	}

	// 提取可选配置 (Ext 字段包含 group, sound, icon 等)
	if cfg.Ext != nil {
		if g, ok := cfg.Ext["group"].(string); ok && g != "" {
			payload.Group = g
		}
		if s, ok := cfg.Ext["sound"].(string); ok && s != "" {
			payload.Sound = s
		}
		if icon, ok := cfg.Ext["icon"].(string); ok && icon != "" {
			payload.Icon = icon
		}
	}

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("bark: marshal payload failed: %w", err)
	}

	pushURL := fmt.Sprintf("%s/push", serverURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, pushURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("bark: create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("bark: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	upstreamResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamResp, fmt.Errorf("bark: http status %s", resp.Status)
	}

	return upstreamResp, nil
}

// ValidateConfig 校验 Bark 配置
func (p *BarkPusher) ValidateConfig(cfg Config) error {
	deviceKey := cfg.Key
	if deviceKey == "" {
		deviceKey = cfg.Secret
	}
	if deviceKey == "" {
		return errors.New("device key is required")
	}
	if cfg.URL != "" && !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("server URL must start with http:// or https://")
	}
	return nil
}
