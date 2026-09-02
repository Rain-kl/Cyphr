// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package msg_gateway

import (
	"context"
	"testing"
)

type stubChannel struct{}

func (stubChannel) Type() string { return "stub" }
func (stubChannel) Connect(context.Context) error {
	return nil
}
func (stubChannel) Disconnect(context.Context) error { return nil }
func (stubChannel) Send(context.Context, Recipient, OutboundMessage) error {
	return nil
}
func (stubChannel) Capabilities() Capability { return Capability{Text: true} }

func TestRegisterLookup(t *testing.T) {
	Register("stub", func(ChannelConfig, Handler) (Channel, error) {
		return stubChannel{}, nil
	})
	fn, ok := Lookup("stub")
	if !ok {
		t.Fatal("expected factory")
	}
	ch, err := fn(ChannelConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Type() != "stub" {
		t.Fatalf("type=%s", ch.Type())
	}
}
