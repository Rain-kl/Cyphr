// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDingTalkPusher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	pusher, err := GetPusher("dingtalk")
	if err != nil {
		t.Fatalf("failed to get dingtalk pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{URL: "https://oapi.dingtalk.com/robot/send?access_token=test"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	_, err = pusher.Send(context.Background(), Config{
		URL:    server.URL,
		Secret: "test_secret",
	}, "", map[string]any{
		"title":   "Alert",
		"content": "Server down",
	}, "", nil)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestBarkPusher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer server.Close()

	pusher, err := GetPusher("bark")
	if err != nil {
		t.Fatalf("failed to get bark pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "device_key_123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	_, err = pusher.Send(context.Background(), Config{
		URL: server.URL,
		Key: "device_key_123",
	}, "", map[string]any{
		"title":   "Alert",
		"content": "Bark notification",
	}, "", nil)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestDiscordPusher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	pusher, err := GetPusher("discord")
	if err != nil {
		t.Fatalf("failed to get discord pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{URL: "https://discord.com/api/webhooks/123/abc"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	_, err = pusher.Send(context.Background(), Config{
		URL: server.URL,
	}, "", map[string]any{
		"title":   "Discord Title",
		"content": "Discord Content",
		"level":   "WARN",
	}, "", nil)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestSlackPusher(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	pusher, err := GetPusher("slack")
	if err != nil {
		t.Fatalf("failed to get slack pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{URL: "https://hooks.slack.com/services/123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	_, err = pusher.Send(context.Background(), Config{
		URL: server.URL,
	}, "", map[string]any{
		"title":   "Slack Title",
		"content": "Slack Content",
	}, "", nil)
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
}

func TestPushoverPusher(t *testing.T) {
	pusher, err := GetPusher("pushover")
	if err != nil {
		t.Fatalf("failed to get pushover pusher: %v", err)
	}

	err = pusher.ValidateConfig(Config{Key: "app_token_123"})
	if err != nil {
		t.Errorf("ValidateConfig failed: %v", err)
	}

	err = pusher.ValidateConfig(Config{})
	if err == nil {
		t.Errorf("expected error for empty config, got nil")
	}
}
