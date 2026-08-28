// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tencent-connect/botgo/token"
	"gorm.io/gorm"
)

const defaultTelegramAPI = "https://api.telegram.org"

// Field is one admin form field.
type Field struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Definition describes a channel type form.
type Definition struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields"`
}

// ChannelDTO represents a channel for admin consumption.
type ChannelDTO struct {
	ID          uint64            `json:"id,string"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	OwnerScope  string            `json:"owner_scope"`
	OwnerID     *uint64           `json:"owner_id,string,omitempty"`
	Enabled     bool              `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// CreateChannelRequest is admin create payload.
type CreateChannelRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// UpdateChannelRequest is admin update payload.
type UpdateChannelRequest struct {
	Name        string            `json:"name"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

func listDefinitions() []Definition {
	return []Definition{
		{
			Type: MessageChannelTypeTelegram,
			Fields: []Field{
				{Key: "token", Type: "password", Required: true},
				{Key: "api_base", Type: "text", Required: false},
			},
		},
		{
			Type: MessageChannelTypeQQ,
			Fields: []Field{
				{Key: "app_id", Type: "text", Required: true},
				{Key: "client_secret", Type: "password", Required: true},
			},
		},
	}
}

func createChannel(ctx context.Context, req CreateChannelRequest) (ChannelDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ChannelDTO{}, errors.New(errNameRequired)
	}
	channelType := strings.TrimSpace(req.Type)
	if channelType != MessageChannelTypeTelegram && channelType != MessageChannelTypeQQ {
		return ChannelDTO{}, errors.New(errTypeInvalid)
	}
	creds := req.Credentials
	if creds == nil {
		creds = map[string]string{}
	}
	if err := validateCredentials(channelType, creds, false); err != nil {
		return ChannelDTO{}, err
	}
	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return ChannelDTO{}, err
	}
	extra := req.Extra
	if extra == nil {
		extra = map[string]string{}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := &MessageChannel{
		Name:        name,
		Type:        channelType,
		OwnerScope:  MessageOwnerScopeSystem,
		Enabled:     enabled,
		Credentials: cipher,
		Extra:       EncodeExtra(extra),
	}
	if err := CreateMessageChannel(ctx, row); err != nil {
		return ChannelDTO{}, err
	}
	return toDTO(row, creds, extra), nil
}

func updateChannel(ctx context.Context, id uint64, req UpdateChannelRequest) (ChannelDTO, error) {
	row, err := GetMessageChannel(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChannelDTO{}, errors.New(errChannelNotFound)
		}
		return ChannelDTO{}, err
	}
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		return ChannelDTO{}, err
	}
	extra := ParseExtra(row.Extra)

	if name := strings.TrimSpace(req.Name); name != "" {
		row.Name = name
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if req.Extra != nil {
		extra = req.Extra
	}
	if len(req.Credentials) > 0 {
		merged := make(map[string]string, len(creds))
		for k, v := range creds {
			merged[k] = v
		}
		for k, v := range req.Credentials {
			if strings.TrimSpace(v) == "" {
				continue
			}
			merged[k] = v
		}
		if err := validateCredentials(row.Type, merged, true); err != nil {
			return ChannelDTO{}, err
		}
		creds = merged
	}

	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return ChannelDTO{}, err
	}
	row.Credentials = cipher
	row.Extra = EncodeExtra(extra)
	if err := UpdateMessageChannel(ctx, row); err != nil {
		return ChannelDTO{}, err
	}
	return toDTO(row, creds, extra), nil
}

func listChannels(ctx context.Context) ([]ChannelDTO, error) {
	rows, err := ListMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelDTO, 0, len(rows))
	for i := range rows {
		creds, _ := DecryptCredentials(rows[i].Credentials)
		extra := ParseExtra(rows[i].Extra)
		out = append(out, toDTO(&rows[i], creds, extra))
	}
	return out, nil
}

func deleteChannel(ctx context.Context, id uint64) error {
	if _, err := GetMessageChannel(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errChannelNotFound)
		}
		return err
	}
	return DeleteMessageChannel(ctx, id)
}

func probeChannel(ctx context.Context, id uint64) error {
	row, err := GetMessageChannel(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errChannelNotFound)
		}
		return err
	}
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		return err
	}
	switch row.Type {
	case MessageChannelTypeTelegram:
		return probeTelegram(ctx, creds)
	case MessageChannelTypeQQ:
		return probeQQ(ctx, creds)
	default:
		return errors.New(errTypeInvalid)
	}
}

func probeTelegram(ctx context.Context, creds map[string]string) error {
	tok := creds["token"]
	if strings.TrimSpace(tok) == "" {
		return errors.New("missing telegram bot token")
	}
	base := creds["api_base"]
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = defaultTelegramAPI
	}
	url := fmt.Sprintf("%s/bot%s/getMe", base, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram getMe failed (%d): %s", resp.StatusCode, string(body))
	}
	var res struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	if !res.OK {
		return fmt.Errorf("telegram returned ok=false: %s", string(body))
	}
	return nil
}

func probeQQ(_ context.Context, creds map[string]string) error {
	appID := strings.TrimSpace(creds["app_id"])
	secret := strings.TrimSpace(creds["app_secret"])
	if appID == "" || secret == "" {
		return errors.New("missing qq app_id or app_secret")
	}
	credentials := &token.QQBotCredentials{
		AppID:     appID,
		AppSecret: secret,
	}
	tokSrc := token.NewQQBotTokenSource(credentials)
	tok, err := tokSrc.Token()
	if err != nil {
		return fmt.Errorf("qq token fetch failed: %w", err)
	}
	if tok == nil || tok.AccessToken == "" {
		return errors.New("qq returned empty access token")
	}
	return nil
}

func validateCredentials(t string, creds map[string]string, isUpdate bool) error {
	switch t {
	case MessageChannelTypeTelegram:
		tok := creds["token"]
		if strings.TrimSpace(tok) == "" && !isUpdate {
			return errors.New(errTelegramTokenRequired)
		}
		if base, ok := creds["api_base"]; ok && strings.TrimSpace(base) != "" {
			if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
				return errors.New("api_base must start with http:// or https://")
			}
		}
	case MessageChannelTypeQQ:
		appID := creds["app_id"]
		secret := creds["client_secret"]
		if (strings.TrimSpace(appID) == "" || strings.TrimSpace(secret) == "") && !isUpdate {
			return errors.New(errQQCredentialsRequired)
		}
	default:
		return errors.New(errTypeInvalid)
	}
	return nil
}

func toDTO(row *MessageChannel, creds, extra map[string]string) ChannelDTO {
	return ChannelDTO{
		ID:          row.ID,
		Name:        row.Name,
		Type:        row.Type,
		OwnerScope:  row.OwnerScope,
		OwnerID:     row.OwnerID,
		Enabled:     row.Enabled,
		Credentials: maskCredentials(row.Type, creds),
		Extra:       extra,
	}
}

func maskCredentials(_ string, in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "token" || k == "client_secret" {
			out[k] = maskSecret(v)
		} else {
			out[k] = v
		}
	}
	return out
}

const minMaskSecretLength = 8

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= minMaskSecretLength {
		return "******"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
