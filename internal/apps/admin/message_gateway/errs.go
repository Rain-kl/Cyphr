// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

const (
	errNameRequired          = "name is required"
	errTypeInvalid           = "type must be telegram or qq"
	errTelegramTokenRequired = "telegram bot secret is required"   //nolint:gosec // user-facing validation text
	errQQCredentialsRequired = "qq app id and secret are required" //nolint:gosec // user-facing validation text
	errChannelNotFound       = "channel not found"
	errChannelProbeFailed    = "channel probe failed"
	maskedSecret             = "********"
)
