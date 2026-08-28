// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"

	db "github.com/Rain-kl/Wavelet/pkg/persistence"
)

// GetAuthSourceByID 根据 ID 获取认证源
func GetAuthSourceByID(ctx context.Context, id uint64) (*AuthSource, error) {
	var src AuthSource
	if err := db.DB(ctx).First(&src, id).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// GetAuthSourceByName 根据名称获取认证源
func GetAuthSourceByName(ctx context.Context, name string) (*AuthSource, error) {
	var src AuthSource
	if err := db.DB(ctx).Where("name = ?", name).First(&src).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// ListActiveAuthSources 获取所有启用的认证源
func ListActiveAuthSources(ctx context.Context) ([]AuthSource, error) {
	var sources []AuthSource
	if err := db.DB(ctx).Where("is_active = ?", true).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveAuthSourcesCached 获取所有启用的认证源（带缓存或直接查询）
func GetActiveAuthSourcesCached(ctx context.Context) ([]AuthSource, error) {
	return ListActiveAuthSources(ctx)
}

// GetAuthSourceByNameCached 根据名称获取认证源（带缓存或直接查询）
func GetAuthSourceByNameCached(ctx context.Context, name string) (*AuthSource, error) {
	return GetAuthSourceByName(ctx, name)
}

// FindExternalAccount 查询指定认证源的外部账号绑定
func FindExternalAccount(ctx context.Context, authSourceID uint64, externalID string) (*ExternalAccount, error) {
	var account ExternalAccount
	if err := db.DB(ctx).Where("auth_source_id = ? AND external_id = ?", authSourceID, externalID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// BindExternalAccount 绑定外部账号
func BindExternalAccount(ctx context.Context, account *ExternalAccount) error {
	return db.DB(ctx).Create(account).Error
}

// ListExternalAccountsByUserID 获取用户绑定的所有外部账号
func ListExternalAccountsByUserID(ctx context.Context, userID uint64) ([]ExternalAccount, error) {
	var accounts []ExternalAccount
	if err := db.DB(ctx).Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// UnbindExternalAccount 解绑外部账号
func UnbindExternalAccount(ctx context.Context, id uint64, userID uint64) error {
	return db.DB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&ExternalAccount{}).Error
}
