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

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
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
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// CreateChannelRequest is the admin create body.
type CreateChannelRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    *bool  `json:"enabled"`
	BotToken   string `json:"bot_token"`
	AppID      string `json:"app_id"`
	AppSecret  string `json:"app_secret"`
	BaseURL    string `json:"base_url"`
	PortalHost string `json:"portal_host"`
	Sandbox    string `json:"sandbox"`
}

// UpdateChannelRequest is the admin patch body.
type UpdateChannelRequest struct {
	Name       *string `json:"name"`
	Enabled    *bool   `json:"enabled"`
	BotToken   string  `json:"bot_token"`
	AppID      string  `json:"app_id"`
	AppSecret  string  `json:"app_secret"`
	BaseURL    *string `json:"base_url"`
	PortalHost *string `json:"portal_host"`
	Sandbox    *string `json:"sandbox"`
}

// ChannelDTO is a list/detail view with secrets masked.
type ChannelDTO struct {
	ID         uint64    `json:"id,string"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	OwnerScope string    `json:"owner_scope"`
	Enabled    bool      `json:"enabled"`
	BotToken   string    `json:"bot_token,omitempty"`
	AppID      string    `json:"app_id,omitempty"`
	AppSecret  string    `json:"app_secret,omitempty"`
	BaseURL    string    `json:"base_url,omitempty"`
	PortalHost string    `json:"portal_host,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func channelDefinitions() []Definition {
	return []Definition{
		{
			Type: model.MessageChannelTypeTelegram,
			Name: "Telegram",
			Fields: []Field{
				{Key: "bot_token", Type: "password", Required: true},
				{Key: "base_url", Type: "text"},
			},
		},
		{
			Type: model.MessageChannelTypeQQ,
			Name: "QQ",
			Fields: []Field{
				{Key: "app_id", Required: true},
				{Key: "app_secret", Type: "password", Required: true},
				{Key: "portal_host", Type: "text"},
			},
		},
	}
}

func createChannel(ctx context.Context, req CreateChannelRequest) (ChannelDTO, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ChannelDTO{}, errors.New(errNameRequired)
	}
	typ := strings.TrimSpace(req.Type)
	creds, extra, err := credentialsFromCreate(req)
	if err != nil {
		return ChannelDTO{}, err
	}
	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return ChannelDTO{}, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := &model.MessageChannel{
		Name:        name,
		Type:        typ,
		OwnerScope:  model.MessageOwnerScopeSystem,
		Enabled:     enabled,
		Credentials: cipher,
		Extra:       EncodeExtra(extra),
	}
	if err := repository.CreateMessageChannel(ctx, row); err != nil {
		return ChannelDTO{}, err
	}
	return toDTO(row, creds, extra), nil
}

func updateChannel(ctx context.Context, id uint64, req UpdateChannelRequest) (ChannelDTO, error) {
	row, err := repository.GetMessageChannel(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChannelDTO{}, errors.New(errChannelNotFound)
		}
		return ChannelDTO{}, err
	}
	creds, err := DecryptCredentials(row.Credentials)
	if err != nil {
		creds = map[string]string{}
	}
	extra := ParseExtra(row.Extra)
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return ChannelDTO{}, errors.New(errNameRequired)
		}
		row.Name = name
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if token := strings.TrimSpace(req.BotToken); token != "" {
		creds["bot_token"] = token
	}
	if appID := strings.TrimSpace(req.AppID); appID != "" {
		creds["app_id"] = appID
	}
	if secret := strings.TrimSpace(req.AppSecret); secret != "" {
		creds["app_secret"] = secret
	}
	if req.BaseURL != nil {
		extra["base_url"] = strings.TrimSpace(*req.BaseURL)
	}
	if req.PortalHost != nil {
		extra["portal_host"] = strings.TrimSpace(*req.PortalHost)
	}
	if req.Sandbox != nil {
		extra["sandbox"] = strings.TrimSpace(*req.Sandbox)
	}
	if err := validateCredentials(row.Type, creds); err != nil {
		return ChannelDTO{}, err
	}
	cipher, err := EncryptCredentials(creds)
	if err != nil {
		return ChannelDTO{}, err
	}
	row.Credentials = cipher
	row.Extra = EncodeExtra(extra)
	if err := repository.UpdateMessageChannel(ctx, row); err != nil {
		return ChannelDTO{}, err
	}
	return toDTO(row, creds, extra), nil
}

func listChannels(ctx context.Context) ([]ChannelDTO, error) {
	rows, err := repository.ListMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelDTO, 0, len(rows))
	for i := range rows {
		creds, err := DecryptCredentials(rows[i].Credentials)
		if err != nil {
			creds = map[string]string{}
		}
		out = append(out, toDTO(&rows[i], creds, ParseExtra(rows[i].Extra)))
	}
	return out, nil
}

func deleteChannel(ctx context.Context, id uint64) error {
	if _, err := repository.GetMessageChannel(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(errChannelNotFound)
		}
		return err
	}
	return repository.DeleteMessageChannel(ctx, id)
}

func probeChannel(ctx context.Context, id uint64) error {
	row, err := repository.GetMessageChannel(ctx, id)
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
	extra := ParseExtra(row.Extra)
	if err := probeCredentials(ctx, row.Type, creds, extra); err != nil {
		return fmt.Errorf("%s: %w", errChannelProbeFailed, err)
	}
	return nil
}

func credentialsFromCreate(req CreateChannelRequest) (map[string]string, map[string]string, error) {
	typ := strings.TrimSpace(req.Type)
	creds := map[string]string{}
	extra := map[string]string{}
	switch typ {
	case model.MessageChannelTypeTelegram:
		creds["bot_token"] = strings.TrimSpace(req.BotToken)
		if base := strings.TrimSpace(req.BaseURL); base != "" {
			extra["base_url"] = base
		}
	case model.MessageChannelTypeQQ:
		creds["app_id"] = strings.TrimSpace(req.AppID)
		creds["app_secret"] = strings.TrimSpace(req.AppSecret)
		if host := strings.TrimSpace(req.PortalHost); host != "" {
			extra["portal_host"] = host
		} else {
			extra["portal_host"] = "q.qq.com"
		}
		if sandbox := strings.TrimSpace(req.Sandbox); sandbox != "" {
			extra["sandbox"] = sandbox
		}
	default:
		return nil, nil, errors.New(errTypeInvalid)
	}
	if err := validateCredentials(typ, creds); err != nil {
		return nil, nil, err
	}
	return creds, extra, nil
}

func validateCredentials(typ string, creds map[string]string) error {
	switch typ {
	case model.MessageChannelTypeTelegram:
		if strings.TrimSpace(creds["bot_token"]) == "" {
			return errors.New(errTelegramTokenRequired)
		}
	case model.MessageChannelTypeQQ:
		if strings.TrimSpace(creds["app_id"]) == "" || strings.TrimSpace(creds["app_secret"]) == "" {
			return errors.New(errQQCredentialsRequired)
		}
	default:
		return errors.New(errTypeInvalid)
	}
	return nil
}

func toDTO(row *model.MessageChannel, creds, extra map[string]string) ChannelDTO {
	dto := ChannelDTO{
		ID:         row.ID,
		Name:       row.Name,
		Type:       row.Type,
		OwnerScope: row.OwnerScope,
		Enabled:    row.Enabled,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if strings.TrimSpace(creds["bot_token"]) != "" {
		dto.BotToken = maskedSecret
	}
	if id := strings.TrimSpace(creds["app_id"]); id != "" {
		dto.AppID = id
	}
	if strings.TrimSpace(creds["app_secret"]) != "" {
		dto.AppSecret = maskedSecret
	}
	dto.BaseURL = extra["base_url"]
	dto.PortalHost = extra["portal_host"]
	return dto
}

func probeCredentials(ctx context.Context, typ string, creds, extra map[string]string) error {
	switch typ {
	case model.MessageChannelTypeTelegram:
		base := strings.TrimSpace(extra["base_url"])
		if base == "" {
			base = defaultTelegramAPI
		}
		url := strings.TrimRight(base, "/") + "/bot" + creds["bot_token"] + "/getMe"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		const probeBodyLimit = 4096
		body, _ := io.ReadAll(io.LimitReader(resp.Body, probeBodyLimit))
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("telegram getMe status %d", resp.StatusCode)
		}
		var parsed struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return err
		}
		if !parsed.OK {
			return errors.New("telegram getMe returned ok=false")
		}
		return nil
	case model.MessageChannelTypeQQ:
		src := token.NewQQBotTokenSource(&token.QQBotCredentials{
			AppID:     creds["app_id"],
			AppSecret: creds["app_secret"],
		})
		_, err := src.Token()
		return err
	default:
		return errors.New(errTypeInvalid)
	}
}
