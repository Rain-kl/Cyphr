// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import "context"

// Handler processes one inbound message.
type Handler func(ctx context.Context, msg InboundMessage) error

// Factory constructs a Channel from decrypted config.
type Factory func(cfg ChannelConfig, onInbound Handler) (Channel, error)

// Channel is one connected messaging adapter.
type Channel interface {
	Type() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Send(ctx context.Context, to Recipient, msg OutboundMessage) error
	Capabilities() Capability
}
