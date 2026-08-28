// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package qq

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/plugins/domain/message_gateway"
)

func TestHandleEvent_DropsNonC2C(t *testing.T) {
	var got int
	a := &Adapter{onInbound: func(ctx context.Context, msg message_gateway.InboundMessage) error {
		got++
		return nil
	}}
	a.handleEvent(context.Background(), qqEvent{Kind: "group", UserID: "u1", Text: "hi"})
	if got != 0 {
		t.Fatal("non-C2C must be ignored")
	}
}

func TestHandleEvent_C2CText(t *testing.T) {
	var got message_gateway.InboundMessage
	a := &Adapter{cfg: message_gateway.ChannelConfig{ID: 3}, onInbound: func(ctx context.Context, msg message_gateway.InboundMessage) error {
		got = msg
		return nil
	}}
	a.handleEvent(context.Background(), qqEvent{Kind: "c2c", UserID: "openid-1", Text: "hello", MessageID: "m1"})
	if got.Text != "hello" || got.PlatformUserID != "openid-1" || got.ChannelID != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestNew_RequiresCreds(t *testing.T) {
	_, err := New(message_gateway.ChannelConfig{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
