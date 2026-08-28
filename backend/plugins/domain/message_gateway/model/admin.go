// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

// Field is one admin form field.
type Field struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Definition describes a channel type form.
type Definition struct {
	Type   string  `json:"type"`
	Fields []Field `json:"fields"`
}

// ChannelDTO represents a channel for admin consumption.
type ChannelDTO struct {
	ID          uint64            `json:"id,string"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	OwnerScope  string            `json:"owner_scope"`
	OwnerID     *uint64           `json:"owner_id,string,omitempty"`
	Enabled     bool              `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// CreateChannelRequest is admin create payload.
type CreateChannelRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}

// UpdateChannelRequest is admin update payload.
type UpdateChannelRequest struct {
	Name        string            `json:"name"`
	Enabled     *bool             `json:"enabled"`
	Credentials map[string]string `json:"credentials"`
	Extra       map[string]string `json:"extra"`
}
