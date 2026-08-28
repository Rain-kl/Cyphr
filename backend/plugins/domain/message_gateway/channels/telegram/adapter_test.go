// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package telegram

import (
	"context"
	"testing"

	"Wavelet/plugins/domain/message_gateway"
	tele "gopkg.in/telebot.v4"
)

func TestHandleUpdate_DropsGroups(t *testing.T) {
	var got int
	a := &Adapter{onInbound: func(ctx context.Context, msg message_gateway.InboundMessage) error {
		got++
		return nil
	}}
	a.handleTeleMessage(context.Background(), &tele.Message{
		ID:     1,
		Text:   "hi",
		Chat:   &tele.Chat{ID: -100, Type: tele.ChatGroup},
		Sender: &tele.User{ID: 1},
	})
	if got != 0 {
		t.Fatalf("group must be ignored")
	}
}

func TestHandleUpdate_PrivateText(t *testing.T) {
	var got message_gateway.InboundMessage
	a := &Adapter{
		cfg: message_gateway.ChannelConfig{ID: 7, Type: "telegram"},
		onInbound: func(ctx context.Context, msg message_gateway.InboundMessage) error {
			got = msg
			return nil
		},
	}
	a.handleTeleMessage(context.Background(), &tele.Message{
		ID:     9,
		Text:   "hi",
		Chat:   &tele.Chat{ID: 42, Type: tele.ChatPrivate},
		Sender: &tele.User{ID: 42},
	})
	if got.Text != "hi" || got.PlatformUserID != "42" || got.ChannelID != 7 {
		t.Fatalf("%+v", got)
	}
}

func TestNew_RequiresToken(t *testing.T) {
	_, err := New(message_gateway.ChannelConfig{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
