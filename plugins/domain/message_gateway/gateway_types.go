// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package message_gateway defines channel adapters, pairing codes, and inbound types.
package message_gateway

// ChannelTypeTelegram is the Telegram private-chat adapter type.
const ChannelTypeTelegram = "telegram"

// ChannelTypeQQ is the official QQ Bot C2C adapter type.
const ChannelTypeQQ = "qq"

// Capability describes what an adapter can send and receive.
type Capability struct {
	Text  bool
	Image bool
	File  bool
	Reply bool
	Group bool
}

// ChannelConfig is the decrypted runtime config passed to a factory.
type ChannelConfig struct {
	ID          uint64
	Type        string
	Name        string
	Credentials map[string]string
	Extra       map[string]string
}

// Recipient is the outbound destination on a platform.
type Recipient struct {
	ChatID         string
	PlatformUserID string
}

// Attachment is a downloaded inbound file sitting on local disk.
type Attachment struct {
	Path     string
	FileName string
	MIME     string
	Error    string
}

// InboundMessage is a normalized private-chat message.
type InboundMessage struct {
	ChannelID      uint64
	ChannelType    string
	PlatformUserID string
	ChatID         string
	MessageID      string
	Text           string
	Attachments    []Attachment
	BindingUserID  *uint64
}

// OutboundMessage is a reply or probe send.
type OutboundMessage struct {
	Text        string
	ReplyToID   string
	Attachments []Attachment
}
