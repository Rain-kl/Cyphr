// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import "errors"

var (
	errCodeInvalid          = errors.New("invalid or expired pairing code")
	errChannelMismatch      = errors.New("pairing code does not match channel")
	errPlatformAlreadyBound = errors.New("this platform account is already bound")
	errBindingNotFound      = errors.New("binding not found")
	errBindingForbidden     = errors.New("cannot unbind another user's binding")
	errChannelIDRequired    = errors.New("channel_id is required")
	errChannelDisabled      = errors.New("channel is not enabled")
)

const (
	errNameRequired          = "name is required"
	errTypeInvalid           = "type must be telegram or qq"
	errTelegramTokenRequired = "telegram bot secret is required"   //nolint:gosec // user-facing validation text
	errQQCredentialsRequired = "qq app id and secret are required" //nolint:gosec // user-facing validation text
	errChannelNotFound       = "channel not found"
	errChannelProbeFailed    = "channel probe failed"
	maskedSecret             = "********"
)
