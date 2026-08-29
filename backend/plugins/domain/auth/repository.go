// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/util"
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	dbMu     sync.RWMutex
	dbSvc    contracts.DBService
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
)

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

func setCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

func getDB(ctx context.Context) *gorm.DB {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.DBService](c); err == nil && s != nil {
			return s.DB(ctx)
		}
	}
	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}

func getCache(ctx context.Context) contracts.CacheService {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.CacheService](c); err == nil && s != nil {
			return s
		}
	}
	cacheMu.RLock()
	s := cacheSvc
	cacheMu.RUnlock()
	return s
}

// GetAccessTokenByHash 按令牌哈希读取访问令牌记录（仅取鉴权所需字段）
func GetAccessTokenByHash(ctx context.Context, tokenHash string) (*CachedToken, error) {
	var row struct {
		ID      uint64
		UserID  uint64
		IsAdmin bool
	}
	if err := getDB(ctx).Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&row).Error; err != nil {
		return nil, err
	}
	return &CachedToken{
		ID:      row.ID,
		UserID:  row.UserID,
		IsAdmin: row.IsAdmin,
	}, nil
}

// GetActiveUserByID 读取仍处于启用状态的用户
func GetActiveUserByID(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := getDB(ctx).Table("w_users").Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID 按 ID 读取用户（不限制启用状态）
func GetUserByID(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := getDB(ctx).Table("w_users").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// InsertUser 新建用户记录
func InsertUser(ctx context.Context, user *contracts.UserDTO) error {
	return getDB(ctx).Table("w_users").Create(user).Error
}

// TouchUserLastLogin 刷新用户最后登录时间
func TouchUserLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	return getDB(ctx).Table("w_users").Where("id = ?", userID).Update("last_login_at", at).Error
}

// ListSimilarUsernames 查询与基础用户名相同或带 `-序号` 后缀的用户名（用于用户名去重）
func ListSimilarUsernames(ctx context.Context, base string) ([]string, error) {
	var existingUsernames []string
	if err := getDB(ctx).Table("w_users").
		Where("username = ? OR username LIKE ? ESCAPE '\\'", base, util.EscapeLike(base)+"-%").
		Pluck("username", &existingUsernames).Error; err != nil {
		return nil, err
	}
	return existingUsernames, nil
}

// GetSystemConfigValue 读取系统配置项原始值
func GetSystemConfigValue(ctx context.Context, key string) (string, error) {
	var val string
	if err := getDB(ctx).Table("w_system_configs").Where("key = ?", key).Pluck("value", &val).Error; err != nil {
		return "", err
	}
	return val, nil
}

// ListAllAuthSources 获取全部认证源（含未启用），按 ID 升序
func ListAllAuthSources(ctx context.Context) ([]AuthSource, error) {
	var sources []AuthSource
	if err := getDB(ctx).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// GetAuthSourceByID 根据 ID 获取认证源
func GetAuthSourceByID(ctx context.Context, id uint64) (*AuthSource, error) {
	var src AuthSource
	if err := getDB(ctx).First(&src, id).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// GetAuthSourceByName 根据名称获取认证源
func GetAuthSourceByName(ctx context.Context, name string) (*AuthSource, error) {
	var src AuthSource
	if err := getDB(ctx).Where("name = ?", name).First(&src).Error; err != nil {
		return nil, err
	}
	return &src, nil
}

// ListActiveAuthSources 获取所有启用的认证源
func ListActiveAuthSources(ctx context.Context) ([]AuthSource, error) {
	var sources []AuthSource
	if err := getDB(ctx).Where("is_active = ?", true).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	return sources, nil
}

// CreateAuthSourceRecord 新建认证源记录
func CreateAuthSourceRecord(ctx context.Context, source *AuthSource) error {
	return getDB(ctx).Create(source).Error
}

// SaveAuthSourceRecord 全量保存认证源记录
func SaveAuthSourceRecord(ctx context.Context, source *AuthSource) error {
	return getDB(ctx).Save(source).Error
}

// DeleteAuthSourceRecord 删除认证源记录
func DeleteAuthSourceRecord(ctx context.Context, source *AuthSource) error {
	return getDB(ctx).Delete(source).Error
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
	if err := getDB(ctx).Where("auth_source_id = ? AND external_id = ?", authSourceID, externalID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// BindExternalAccount 绑定外部账号
func BindExternalAccount(ctx context.Context, account *ExternalAccount) error {
	return getDB(ctx).Create(account).Error
}

// ListExternalAccountsByUserID 获取用户绑定的所有外部账号
func ListExternalAccountsByUserID(ctx context.Context, userID uint64) ([]ExternalAccount, error) {
	var accounts []ExternalAccount
	if err := getDB(ctx).Where("user_id = ?", userID).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// UnbindExternalAccount 解绑外部账号
func UnbindExternalAccount(ctx context.Context, id, userID uint64) error {
	return getDB(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&ExternalAccount{}).Error
}
