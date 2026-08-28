// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package errs defines error sentinels and user-facing error message constants
// for the message_gateway plugin.
package errs

import "errors"

// Sentinel errors.
var (
	ErrCodeInvalid          = errors.New("invalid or expired pairing code")
	ErrChannelMismatch      = errors.New("pairing code does not match channel")
	ErrPlatformAlreadyBound = errors.New("this platform account is already bound")
	ErrBindingNotFound      = errors.New("binding not found")
	ErrBindingForbidden     = errors.New("cannot unbind another user's binding")
	ErrChannelIDRequired    = errors.New("channel_id is required")
	ErrChannelDisabled      = errors.New("channel is not enabled")

	// ErrRecordNotFound maps GORM's missing-row sentinel at the repository boundary so
	// upper layers never import gorm. Its text matches gorm.ErrRecordNotFound verbatim.
	ErrRecordNotFound = errors.New("record not found")
)

// User-facing validation and error message constants.
const (
	ErrNameRequired          = "name is required"
	ErrTypeInvalid           = "type must be telegram or qq"
	ErrTelegramTokenRequired = "telegram bot secret is required"   //nolint:gosec // user-facing validation text
	ErrQQCredentialsRequired = "qq app id and secret are required" //nolint:gosec // user-facing validation text
	ErrChannelNotFound       = "channel not found"
	ErrChannelProbeFailed    = "channel probe failed"
	MaskedSecret             = "********"

	ErrLoginRequired    = "login required"
	ErrInvalidBindingID = "invalid binding id"
	ErrInvalidChannelID = "invalid channel id"
	ErrInvalidEventID   = "invalid event id"
	ErrEventNotFound    = "notification event not found"
	ErrValidationFailed = "validation failed"

	ErrMissingTelegramToken = "missing telegram bot token"
	ErrMissingQQCredentials = "missing qq app_id or app_secret" //nolint:gosec // user-facing validation text
	ErrQQTokenFetchFailed   = "qq token fetch failed"           //nolint:gosec // user-facing validation text
	ErrQQEmptyToken         = "qq returned empty access token"  //nolint:gosec // user-facing validation text
	ErrTelegramGetMeFailed  = "telegram getMe failed"
	ErrTelegramNotOK        = "telegram returned ok=false"
	ErrAPIBaseInvalid       = "api_base must start with http:// or https://"

	ErrChannelNameExists      = "channel name already exists"
	ErrChannelNameRequired    = "channel name is required"
	ErrChannelTypeRequired    = "channel type is required"
	ErrEventKeyRequired       = "event_key is required"
	ErrEventAlreadyConfigured = "this notification event is already configured"
	ErrTemplateInvalidJSON    = "custom template is not a valid JSON format"
	ErrEnableWithoutChannels  = "cannot enable event without any push channels configured"
	ErrEventKeyOrTaskType     = "either event_key or task_type must be provided"
	ErrUnsupportedEventKey    = "unsupported built-in event key"
	ErrTaskServiceUnavailable = "task service not available"
	ErrUserNotFound           = "user not found"
	ErrNoAdminUser            = "no admin user found"

	ErrPayloadRequired    = "payload is required"
	ErrInvalidJSONFormat  = "invalid json format"
	ErrParsePayloadFailed = "parse payload failed"
	ErrGetPusherFailed    = "get pusher failed"
	ErrPusherSendFailed   = "pusher.Send failed"
)
