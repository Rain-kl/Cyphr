// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"strings"
	"sync"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/util"
	"gorm.io/gorm"
)

var (
	dbMu  sync.RWMutex
	dbSvc contracts.DBService
)

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

func getDB(ctx context.Context) *gorm.DB {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s := c.DB(); s != nil {
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

// GetUserByID 通过 ID 获取用户
func GetUserByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := getDB(ctx).First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByUsername 通过用户名获取用户
func GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	if err := getDB(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail 通过邮箱获取用户
func GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := getDB(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser 创建用户
func CreateUser(ctx context.Context, u *User) error {
	return getDB(ctx).Create(u).Error
}

// UpdateUser 更新用户
func UpdateUser(ctx context.Context, u *User) error {
	return getDB(ctx).Save(u).Error
}

// ListUsers 分页查询用户
func ListUsers(ctx context.Context, page, pageSize int, keyword string) ([]*User, int64, error) {
	db := getDB(ctx).Model(&User{})
	if keyword != "" {
		escaped := util.EscapeLike(keyword)
		db = db.Where("username LIKE ? ESCAPE '\\' OR nickname LIKE ? ESCAPE '\\' OR email LIKE ? ESCAPE '\\'", "%"+escaped+"%", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []*User
	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetAccessTokenByHash 通过 Hash 查询访问令牌
func GetAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	var token AccessToken
	if err := getDB(ctx).Where("token_hash = ?", tokenHash).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// AdminUserListFilter 包含后台用户列表过滤条件
type AdminUserListFilter struct {
	Username string
	Keyword  string
	Page     int
	PageSize int
}

// ListAdminUsers 获取后台管理用户列表
func ListAdminUsers(ctx context.Context, filter AdminUserListFilter) (int64, []User, error) {
	query := getDB(ctx).Model(&User{})
	if filter.Username != "" {
		escaped := util.EscapeLike(strings.ToLower(filter.Username))
		query = query.Where("LOWER(username) LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	if filter.Keyword != "" {
		escaped := util.EscapeLike(strings.ToLower(filter.Keyword))
		query = query.Where("LOWER(username) LIKE ? ESCAPE '\\' OR LOWER(nickname) LIKE ? ESCAPE '\\' OR LOWER(email) LIKE ? ESCAPE '\\'",
			"%"+escaped+"%", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var users []User
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

// UpdateUserActive 更新用户激活状态
func UpdateUserActive(ctx context.Context, id uint64, active bool) error {
	return getDB(ctx).Model(&User{}).Where("id = ?", id).Update("is_active", active).Error
}

// GetActiveUserByID 获取处于激活状态的用户
func GetActiveUserByID(ctx context.Context, id uint64) (*User, error) {
	var u User
	if err := getDB(ctx).Where("id = ? AND is_active = ?", id, true).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteUserWithRelations 删除用户及其级联关系
func DeleteUserWithRelations(ctx context.Context, id uint64) error {
	return getDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&AccessToken{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&User{}).Error
	})
}

// GetFirstAdminUser 获取第一个管理员用户
func GetFirstAdminUser(ctx context.Context) (*User, error) {
	var u User
	if err := getDB(ctx).Where("is_admin = ?", true).Order("id ASC").First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsernamesMatchingBase 列出匹配基础用户名的所有用户名
func ListUsernamesMatchingBase(ctx context.Context, base string) ([]string, error) {
	var usernames []string
	escaped := util.EscapeLike(strings.ToLower(base))
	if err := getDB(ctx).Model(&User{}).
		Where("LOWER(username) LIKE ? ESCAPE '\\'", escaped+"%").
		Pluck("username", &usernames).Error; err != nil {
		return nil, err
	}
	return usernames, nil
}
