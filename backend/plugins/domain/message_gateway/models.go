// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"errors"
	"strings"
	"time"
)

// Message channel and push channel constants.
const (
	MessageChannelTypeTelegram = "telegram"
	MessageChannelTypeQQ       = "qq"
	MessageOwnerScopeSystem    = "system"

	TypeCustom   = "custom"
	TypeEmail    = "email"
	TypeTelegram = "telegram"
)

// MessageChannel is an admin-configured messaging adapter.
type MessageChannel struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type        string    `json:"type" gorm:"size:32;not null"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	OwnerScope  string    `json:"owner_scope" gorm:"size:32;not null;default:'system'"`
	OwnerID     *uint64   `json:"owner_id,omitempty"`
	Credentials string    `json:"credentials" gorm:"type:text;not null"`
	Extra       string    `json:"extra" gorm:"type:text"`
	Enabled     bool      `json:"enabled" gorm:"default:false;not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (MessageChannel) TableName() string {
	return "w_message_channels"
}

// MessageBinding maps a platform user to a Wavelet user on one channel.
type MessageBinding struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelID      uint64    `json:"channel_id" gorm:"not null;index"`
	PlatformUserID string    `json:"platform_user_id" gorm:"size:128;not null;index"`
	UserID         uint64    `json:"user_id" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (MessageBinding) TableName() string {
	return "w_message_bindings"
}

// MessagePairingCode is a one-time bind code.
type MessagePairingCode struct {
	ID             uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Code           string    `json:"code" gorm:"size:32;uniqueIndex;not null"`
	ChannelID      uint64    `json:"channel_id" gorm:"not null;index"`
	PlatformUserID string    `json:"platform_user_id" gorm:"size:128;not null;index"`
	UserID         uint64    `json:"user_id" gorm:"not null;index"`
	ExpiresAt      time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 表名
func (MessagePairingCode) TableName() string {
	return "w_message_pairing_codes"
}

// PushChannel 消息通道模型
type PushChannel struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:100;not null"`
	Description string    `json:"description" gorm:"size:255"`
	Type        string    `json:"type" gorm:"size:50;not null;index"`
	URL         string    `json:"url" gorm:"type:text"`
	Token       string    `json:"token" gorm:"type:text"`
	Other       string    `json:"other" gorm:"type:text"`
	Enabled     bool      `json:"enabled" gorm:"index;not null;default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 指定 GORM 表名
func (PushChannel) TableName() string {
	return "w_push_channels"
}

// Validate 验证与标准化字段
func (c *PushChannel) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("channel name is required")
	}
	c.Type = strings.TrimSpace(c.Type)
	if c.Type == "" {
		return errors.New("channel type is required")
	}
	return nil
}

// PushEvent 系统通知事件模型
type PushEvent struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"uniqueIndex;size:80;not null"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	TaskType  string    `json:"task_type" gorm:"size:100;index;not null;default:''"`
	Channels  []string  `json:"channels" gorm:"type:text;serializer:json;not null"`
	Targets   []string  `json:"targets" gorm:"type:text;serializer:json;not null"`
	Template  string    `json:"template" gorm:"type:text;not null"`
	Enabled   bool      `json:"enabled" gorm:"index;not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 指定 GORM 表名
func (PushEvent) TableName() string {
	return "w_push_events"
}

// Validate 验证 PushEvent 实体字段
func (e *PushEvent) Validate() error {
	e.EventKey = strings.TrimSpace(e.EventKey)
	if e.EventKey == "" {
		return errors.New("event_key is required")
	}
	e.Name = strings.TrimSpace(e.Name)
	if e.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// PushHistory 推送日志/历史实体
type PushHistory struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	EventKey  string    `json:"event_key" gorm:"size:80;not null;index"`
	Channel   string    `json:"channel" gorm:"size:50;not null;index"`
	Target    string    `json:"target" gorm:"size:255;not null"`
	Title     string    `json:"title" gorm:"size:255;not null"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Level     string    `json:"level" gorm:"size:20;not null;default:'INFO'"`
	Status    string    `json:"status" gorm:"size:20;not null;index"`
	ErrorMsg  string    `json:"error_msg" gorm:"type:text"`
	Payload   string    `json:"payload" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName 指定 GORM 表名
func (PushHistory) TableName() string {
	return "w_push_histories"
}

// PushHistoryListFilter filters push history pagination queries.
type PushHistoryListFilter struct {
	EventKey  string
	Channel   string
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}
