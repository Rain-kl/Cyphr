// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/pkg/idgen"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	activePushChannelCacheTTL = 24 * time.Hour
	activePushEventCacheTTL   = 24 * time.Hour
)

// CreateMessageChannel inserts a channel row.
func CreateMessageChannel(ctx context.Context, ch *MessageChannel) error {
	if ch.ID == 0 {
		ch.ID = idgen.NextUint64ID()
	}
	return getDB(ctx).Create(ch).Error
}

// UpdateMessageChannel saves a channel row.
func UpdateMessageChannel(ctx context.Context, ch *MessageChannel) error {
	return getDB(ctx).Save(ch).Error
}

// GetMessageChannel loads a channel by id.
func GetMessageChannel(ctx context.Context, id uint64) (*MessageChannel, error) {
	var ch MessageChannel
	if err := getDB(ctx).Where("id = ?", id).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// ListMessageChannels returns all channels newest first.
func ListMessageChannels(ctx context.Context) ([]MessageChannel, error) {
	var rows []MessageChannel
	if err := getDB(ctx).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteMessageChannel removes pairings, bindings, then the channel.
func DeleteMessageChannel(ctx context.Context, id uint64) error {
	return getDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&MessagePairingCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("channel_id = ?", id).Delete(&MessageBinding{}).Error; err != nil {
			return err
		}
		return tx.Delete(&MessageChannel{}, id).Error
	})
}

// CreateMessageBinding inserts a binding.
func CreateMessageBinding(ctx context.Context, b *MessageBinding) error {
	if b.ID == 0 {
		b.ID = idgen.NextUint64ID()
	}
	return getDB(ctx).Create(b).Error
}

// GetBindingByChannelPlatform finds a binding for a platform user on a channel.
func GetBindingByChannelPlatform(ctx context.Context, channelID uint64, platformUserID string) (*MessageBinding, error) {
	var b MessageBinding
	err := getDB(ctx).Where("channel_id = ? AND platform_user_id = ?", channelID, platformUserID).First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListBindingsByUser lists bindings for a Wavelet user.
func ListBindingsByUser(ctx context.Context, userID uint64) ([]MessageBinding, error) {
	var rows []MessageBinding
	if err := getDB(ctx).Where("user_id = ?", userID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetMessageBinding loads a binding by id.
func GetMessageBinding(ctx context.Context, id uint64) (*MessageBinding, error) {
	var b MessageBinding
	if err := getDB(ctx).Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteMessageBinding deletes a binding by id.
func DeleteMessageBinding(ctx context.Context, id uint64) error {
	return getDB(ctx).Delete(&MessageBinding{}, id).Error
}

// UpsertPairingCode reuses an unexpired code for the same channel+platform user.
func UpsertPairingCode(ctx context.Context, channelID uint64, platformUserID, code string, expiresAt time.Time) (*MessagePairingCode, error) {
	var existing MessagePairingCode
	err := getDB(ctx).
		Where("channel_id = ? AND platform_user_id = ? AND expires_at > ?", channelID, platformUserID, time.Now()).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := &MessagePairingCode{
		Code:           code,
		ChannelID:      channelID,
		PlatformUserID: platformUserID,
		ExpiresAt:      expiresAt,
	}
	if err := getDB(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// GetPairingCode loads a pairing code by normalized code string.
func GetPairingCode(ctx context.Context, code string) (*MessagePairingCode, error) {
	var row MessagePairingCode
	if err := getDB(ctx).Where("code = ?", code).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// DeletePairingCode removes a pairing code.
func DeletePairingCode(ctx context.Context, code string) error {
	return getDB(ctx).Where("code = ?", code).Delete(&MessagePairingCode{}).Error
}

// DeleteExpiredPairingCodes removes expired pairing rows.
func DeleteExpiredPairingCodes(ctx context.Context) error {
	return getDB(ctx).Where("expires_at <= ?", time.Now()).Delete(&MessagePairingCode{}).Error
}

// ListEnabledMessageChannels returns enabled channels.
func ListEnabledMessageChannels(ctx context.Context) ([]MessageChannel, error) {
	var rows []MessageChannel
	if err := getDB(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListPushChannelsRecord returns all push channels ordered by creation time descending.
func ListPushChannelsRecord(ctx context.Context) ([]PushChannel, error) {
	var channels []PushChannel
	if err := getDB(ctx).Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// GetPushChannelByIDRecord loads a push channel by primary key.
func GetPushChannelByIDRecord(ctx context.Context, id uint64) (PushChannel, error) {
	var channel PushChannel
	if err := getDB(ctx).Where("id = ?", id).First(&channel).Error; err != nil {
		return PushChannel{}, err
	}
	return channel, nil
}

// GetPushChannelByNameRecord 根据名称获取消息通道。
func GetPushChannelByNameRecord(ctx context.Context, name string) (*PushChannel, error) {
	var channel PushChannel
	if err := getDB(ctx).Where("name = ?", name).First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// CountPushChannelsByNameRecord returns how many channels share the given name.
func CountPushChannelsByNameRecord(ctx context.Context, name string) (int64, error) {
	var count int64
	if err := getDB(ctx).Model(&PushChannel{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushChannelRecord persists a new channel and invalidates cache.
func CreatePushChannelRecord(ctx context.Context, channel *PushChannel) error {
	if err := getDB(ctx).Create(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// SavePushChannelRecord updates a channel and invalidates cache.
func SavePushChannelRecord(ctx context.Context, channel *PushChannel) error {
	if err := getDB(ctx).Save(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// DeletePushChannelRecord removes a channel and invalidates cache.
func DeletePushChannelRecord(ctx context.Context, channel *PushChannel) error {
	if err := getDB(ctx).Delete(channel).Error; err != nil {
		return err
	}
	DeleteActivePushChannelCache(ctx, channel.Name)
	return nil
}

// GetActivePushChannelByName 根据名称获取启用的消息通道 (优先从 Redis 缓存获取)。
func GetActivePushChannelByName(ctx context.Context, name string) (*PushChannel, error) {
	cacheKey := "push:channel:active:" + name
	var channel PushChannel
	if cache := getCache(ctx); cache != nil {
		if err := cache.Get(ctx, cacheKey, &channel); err == nil {
			return &channel, nil
		}
	}

	if err := getDB(ctx).Where("name = ? AND enabled = ?", name, true).First(&channel).Error; err != nil {
		return nil, err
	}

	if cache := getCache(ctx); cache != nil {
		_ = cache.Set(ctx, cacheKey, channel, activePushChannelCacheTTL)
	}

	return &channel, nil
}

// DeleteActivePushChannelCache 清理启用消息通道的缓存。
func DeleteActivePushChannelCache(ctx context.Context, name string) {
	if cache := getCache(ctx); cache != nil {
		_ = cache.Delete(ctx, "push:channel:active:"+name)
	}
}

// ListPushEventsRecord returns all push events ordered by creation time descending.
func ListPushEventsRecord(ctx context.Context) ([]PushEvent, error) {
	var events []PushEvent
	if err := getDB(ctx).Order("created_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetPushEventByIDRecord loads a push event by primary key.
func GetPushEventByIDRecord(ctx context.Context, id uint64) (PushEvent, error) {
	var event PushEvent
	if err := getDB(ctx).First(&event, id).Error; err != nil {
		return PushEvent{}, err
	}
	return event, nil
}

// GetPushEventByKeyRecord loads a push event by event key.
func GetPushEventByKeyRecord(ctx context.Context, key string) (PushEvent, error) {
	var event PushEvent
	if err := getDB(ctx).Where("event_key = ?", key).First(&event).Error; err != nil {
		return PushEvent{}, err
	}
	return event, nil
}

// CountPushEventsByKeyRecord returns how many events use the given event key.
func CountPushEventsByKeyRecord(ctx context.Context, key string) (int64, error) {
	var count int64
	if err := getDB(ctx).Model(&PushEvent{}).Where("event_key = ?", key).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePushEventRecord persists a new push event and invalidates cache.
func CreatePushEventRecord(ctx context.Context, event *PushEvent) error {
	if err := getDB(ctx).Create(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// SavePushEventRecord updates a push event and invalidates cache.
func SavePushEventRecord(ctx context.Context, event *PushEvent) error {
	if err := getDB(ctx).Save(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// UpdatePushEventEnabledRecord toggles the enabled flag for a push event.
func UpdatePushEventEnabledRecord(ctx context.Context, event *PushEvent, enabled bool) error {
	event.Enabled = enabled
	if err := getDB(ctx).Model(event).Update("enabled", enabled).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// DeletePushEventRecord removes a push event and invalidates cache.
func DeletePushEventRecord(ctx context.Context, event *PushEvent) error {
	if err := getDB(ctx).Delete(event).Error; err != nil {
		return err
	}
	DeleteActivePushEventCache(ctx, event.EventKey)
	return nil
}

// ListActivePushEventsByTaskTypeRecord returns enabled events bound to a task type.
func ListActivePushEventsByTaskTypeRecord(ctx context.Context, taskType string) ([]PushEvent, error) {
	var events []PushEvent
	if err := getDB(ctx).Where("task_type = ? AND enabled = ?", taskType, true).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetActivePushEventByKey 获取启用的通知事件 (优先从 Redis 缓存获取)。
func GetActivePushEventByKey(ctx context.Context, key string) (*PushEvent, error) {
	cacheKey := "push:event:active:" + key
	var event PushEvent
	if cache := getCache(ctx); cache != nil {
		if err := cache.Get(ctx, cacheKey, &event); err == nil {
			return &event, nil
		}
	}

	if err := getDB(ctx).Where("event_key = ? AND enabled = ?", key, true).First(&event).Error; err != nil {
		return nil, err
	}

	if cache := getCache(ctx); cache != nil {
		_ = cache.Set(ctx, cacheKey, event, activePushEventCacheTTL)
	}

	return &event, nil
}

// DeleteActivePushEventCache 清理启用通知事件的缓存。
func DeleteActivePushEventCache(ctx context.Context, key string) {
	if cache := getCache(ctx); cache != nil {
		_ = cache.Delete(ctx, "push:event:active:"+key)
	}
}

// ListPushHistoriesRecord returns paginated push history records.
func ListPushHistoriesRecord(ctx context.Context, filter PushHistoryListFilter) (int64, []PushHistory, error) {
	query := getDB(ctx).Model(&PushHistory{}).Order("created_at DESC")
	if filter.EventKey != "" {
		query = query.Where("event_key = ?", filter.EventKey)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var results []PushHistory
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&results).Error; err != nil {
		return 0, nil, err
	}

	return total, results, nil
}

// CreatePushHistoryRecord persists a push history audit record.
func CreatePushHistoryRecord(ctx context.Context, history *PushHistory) error {
	return getDB(ctx).Create(history).Error
}

// PushHistoryQuery returns a scoped query builder for push histories.
func PushHistoryQuery(ctx context.Context) *gorm.DB {
	return getDB(ctx).Model(&PushHistory{})
}
