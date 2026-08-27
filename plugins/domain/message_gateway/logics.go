// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	pkgmg "github.com/Rain-kl/Wavelet/pkg/message_gateway"
	"gorm.io/gorm"
)

// BindRequest is the user bind body.
type BindRequest struct {
	ChannelID string `json:"channel_id"`
	Code      string `json:"code"`
}

// BindingDTO is a user-facing binding row.
type BindingDTO struct {
	ID             uint64    `json:"id,string"`
	UserID         uint64    `json:"user_id,string"`
	ChannelID      uint64    `json:"channel_id,string"`
	ChannelName    string    `json:"channel_name"`
	ChannelType    string    `json:"channel_type"`
	PlatformUserID string    `json:"platform_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func bindChannel(ctx context.Context, userID uint64, req BindRequest) (BindingDTO, error) {
	channelID, err := strconv.ParseUint(strings.TrimSpace(req.ChannelID), 10, 64)
	if err != nil || channelID == 0 {
		return BindingDTO{}, errChannelIDRequired
	}
	code := pkgmg.NormalizeCode(req.Code)
	if code == "" {
		return BindingDTO{}, errCodeInvalid
	}
	pairing, err := repository.GetPairingCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BindingDTO{}, errCodeInvalid
		}
		return BindingDTO{}, err
	}
	if !pairing.ExpiresAt.After(time.Now()) {
		return BindingDTO{}, errCodeInvalid
	}
	if pairing.ChannelID != channelID {
		return BindingDTO{}, errChannelMismatch
	}
	ch, err := repository.GetMessageChannel(ctx, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return BindingDTO{}, errCodeInvalid
		}
		return BindingDTO{}, err
	}
	if !ch.Enabled {
		return BindingDTO{}, errChannelDisabled
	}

	existing, err := repository.GetBindingByChannelPlatform(ctx, channelID, pairing.PlatformUserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return BindingDTO{}, err
	}
	if err == nil && existing != nil {
		if existing.UserID != userID {
			return BindingDTO{}, errPlatformAlreadyBound
		}
		_ = repository.DeletePairingCode(ctx, pairing.Code)
		return toBindingDTO(existing, ch), nil
	}

	row := &model.MessageBinding{
		UserID:         userID,
		ChannelID:      channelID,
		PlatformUserID: pairing.PlatformUserID,
	}
	if err := repository.CreateMessageBinding(ctx, row); err != nil {
		return BindingDTO{}, err
	}
	if err := repository.DeletePairingCode(ctx, pairing.Code); err != nil {
		return BindingDTO{}, err
	}
	return toBindingDTO(row, ch), nil
}

// PublicChannelDTO is an enabled channel a user can bind to.
type PublicChannelDTO struct {
	ID   uint64 `json:"id,string"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func listEnabledPublicChannels(ctx context.Context) ([]PublicChannelDTO, error) {
	rows, err := repository.ListEnabledMessageChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicChannelDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, PublicChannelDTO{ID: row.ID, Name: row.Name, Type: row.Type})
	}
	return out, nil
}

func listUserBindings(ctx context.Context, userID uint64) ([]BindingDTO, error) {
	rows, err := repository.ListBindingsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]BindingDTO, 0, len(rows))
	for i := range rows {
		ch, err := repository.GetMessageChannel(ctx, rows[i].ChannelID)
		if err != nil {
			out = append(out, toBindingDTO(&rows[i], nil))
			continue
		}
		out = append(out, toBindingDTO(&rows[i], ch))
	}
	return out, nil
}

func unbindChannel(ctx context.Context, userID, bindingID uint64) error {
	row, err := repository.GetMessageBinding(ctx, bindingID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errBindingNotFound
		}
		return err
	}
	if row.UserID != userID {
		return errBindingForbidden
	}
	return repository.DeleteMessageBinding(ctx, bindingID)
}

func toBindingDTO(row *model.MessageBinding, ch *model.MessageChannel) BindingDTO {
	dto := BindingDTO{
		ID:             row.ID,
		UserID:         row.UserID,
		ChannelID:      row.ChannelID,
		PlatformUserID: row.PlatformUserID,
		CreatedAt:      row.CreatedAt,
	}
	if ch != nil {
		dto.ChannelName = ch.Name
		dto.ChannelType = ch.Type
	}
	return dto
}
