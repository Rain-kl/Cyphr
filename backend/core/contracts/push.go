// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import "context"

type PushNotificationTemplate struct {
	Title   string
	Content string
	Level   string
	Ext     map[string]any
}

type PushEventMeta struct {
	Key             string
	Name            string
	Description     string
	DefaultTemplate PushNotificationTemplate
}

type PushRegistry interface {
	RegisterBuiltInEvent(meta PushEventMeta)
	SyncEvents(ctx context.Context) error
}
