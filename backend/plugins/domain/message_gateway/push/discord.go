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
	discordColorInfo = 3447003  // Blue
	discordColorWarn = 15105570 // Orange
	discordColorErr  = 15158332 // Red
)

func init() {
	Register("discord", &DiscordPusher{})
}

// DiscordPusher Discord Webhook 机器人推送实现
type DiscordPusher struct{}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
}

type discordPayload struct {
	Username string         `json:"username,omitempty"`
	Embeds   []discordEmbed `json:"embeds"`
}

// Send 发送 Discord 通知
func (p *DiscordPusher) Send(ctx context.Context, cfg Config, _ string, body map[string]any, _ string, _ map[string]any) (string, error) {
	if cfg.URL == "" {
		return "", errors.New("discord: webhook URL is required")
	}

	title := bodyTitle(body)
	content := bodyContent(body, "**%s**: %v", "\n")
	level := bodyLevel(body)

	color := discordColorInfo
	switch strings.ToUpper(level) {
	case "WARN", "WARNING":
		color = discordColorWarn
	case "ERROR", "FATAL":
		color = discordColorErr
	}

	payload := discordPayload{
		Username: "Wavelet System",
		Embeds: []discordEmbed{
			{
				Title:       title,
				Description: content,
				Color:       color,
			},
		},
	}

	reqBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("discord: marshal payload failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("discord: create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := httppool.NewClient(defaultHTTPClientTimeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("discord: http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	upstreamResp := strings.TrimSpace(string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamResp, fmt.Errorf("discord: http status %s", resp.Status)
	}

	return upstreamResp, nil
}

// ValidateConfig 校验 Discord 配置
func (p *DiscordPusher) ValidateConfig(cfg Config) error {
	if cfg.URL == "" {
		return errors.New("webhook URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") {
		return errors.New("webhook URL must start with https://")
	}
	return nil
}
