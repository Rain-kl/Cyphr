// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants, sentinel errors, and user-facing error messages
// for the msg_gateway plugin.
package consts

import "errors"

// Channel type and scope constants.
const (
	ChannelTypeTelegram        = "telegram"
	ChannelTypeQQ              = "qq"
	MessageChannelTypeTelegram = "telegram"
	MessageChannelTypeQQ       = "qq"
	MessageOwnerScopeSystem    = "system"

	TypeCustom      = "custom"
	TypeEmail       = "email"
	TypeTelegram    = "telegram"
	ChannelCustom   = "custom"
	ChannelEmail    = "email"
	ChannelLark     = "lark"
	ChannelDingTalk = "dingtalk"
	ChannelTelegram = "telegram"
	ChannelBark     = "bark"
	ChannelDiscord  = "discord"
	ChannelSlack    = "slack"
	ChannelPushover = "pushover"

	DefaultLevelInfo = "INFO"
	KeyTitle         = "title"
	KeyContent       = "content"
	KeyLevel         = "level"

	// KeyURL represents the URL field key.
	KeyURL = "url"
	// KeyToken represents the Token field key.
	KeyToken = "token"
	// KeyOther represents the Other field key.
	KeyOther = "other"

	// TypeText represents standard text input type.
	TypeText = "text"
	// TypePassword represents password input type.
	TypePassword = "password"
	// TypeTextarea represents textarea input type.
	TypeTextarea = "textarea"
)

// Task and Schedule identifier constants.
const (
	TaskPushNotification     = "msg_gateway:push_notification"
	TaskCleanupPairingCodes  = "msg_gateway:cleanup_pairing_codes"
	TaskDispatchBotMsg       = "msg_gateway:dispatch_bot_msg"
	TaskTypeDispatchBotMsg   = "dispatch_bot_msg"
	SendNotificationTask     = "push:send"
	TaskTypeSendNotification = "send_notification"
)

// Pairing code constants.
const (
	CodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	CodeLength   = 8
)

// Sentinel errors.
var (
	ErrCodeInvalid          = errors.New("invalid or expired pairing code")
	ErrChannelMismatch      = errors.New("pairing code does not match channel")
	ErrPlatformAlreadyBound = errors.New("this platform account is already bound")
	ErrBindingNotFound      = errors.New("binding not found")
	ErrBindingForbidden     = errors.New("cannot unbind another user's binding")
	ErrChannelIDRequired    = errors.New("channel_id is required")
	ErrChannelDisabled      = errors.New("channel is not enabled")

	// ErrRecordNotFound maps GORM's missing-row sentinel at the DAO boundary so
	// upper layers never import gorm. Its text matches gorm.ErrRecordNotFound verbatim.
	ErrRecordNotFound = errors.New("record not found")

	// ErrUnsupportedUserLookupField rejects a column name that the DAO is not
	// allowed to interpolate into a WHERE clause.
	ErrUnsupportedUserLookupField = errors.New("unsupported user lookup field")
)

// User-facing validation and error message constants.
const (
	ErrNameRequired            = "name is required"
	ErrTypeInvalid             = "type must be telegram or qq"
	ErrTelegramTokenRequired   = "telegram bot secret is required"   //nolint:gosec // user-facing validation text
	ErrQQCredentialsRequired   = "qq app id and secret are required" //nolint:gosec // user-facing validation text
	ErrChannelNotFound         = "channel not found"
	ErrChannelProbeFailed      = "channel probe failed"
	ErrBotDispatchTextRequired = "message text is required"
	ErrBotChannelNotRegistered = "channel adapter is not registered"
	MaskedSecret               = "********"

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
