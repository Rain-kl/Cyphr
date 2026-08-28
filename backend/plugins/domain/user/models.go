// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"errors"

	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/backend/pkg/util"
)

// AccessToken 个人访问令牌实体
type AccessToken struct {
	ID          uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID      uint64    `json:"user_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:128;not null"`
	TokenHash   string    `json:"-" gorm:"size:64;uniqueIndex;not null"`
	MaskedToken string    `json:"masked_token" gorm:"size:64;not null"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (AccessToken) TableName() string {
	return "w_access_tokens"
}

// User 用户表实体
type User struct {
	ID          uint64    `json:"id,string" gorm:"primaryKey;not null"`
	Username    string    `json:"username" gorm:"size:64;uniqueIndex"`
	Password    string    `json:"password,omitempty" gorm:"size:255"`
	Nickname    string    `json:"nickname" gorm:"size:255"`
	Email       string    `json:"email" gorm:"size:255;index"`
	AvatarURL   string    `json:"avatar_url" gorm:"size:255"`
	IsActive    bool      `json:"is_active" gorm:"default:true;index"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	Bio         string    `json:"bio" gorm:"size:500"`
	Phone       string    `json:"phone" gorm:"size:32"`
	Gender      string    `json:"gender" gorm:"size:16"`
	Website     string    `json:"website" gorm:"size:255"`
	Location    string    `json:"location" gorm:"size:255"`
	LastLoginAt time.Time `json:"last_login_at" gorm:"index"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime;index"`
}

// TableName 表名
func (User) TableName() string {
	return "w_users"
}

// SetEncryptedPassword 设置加密密码
func (u *User) SetEncryptedPassword(password string) error {
	trimmed := strings.TrimSpace(password)
	if trimmed == "" {
		return errors.New("password cannot be empty")
	}
	hash, err := util.HashPassword(trimmed)
	if err != nil {
		return err
	}
	u.Password = hash
	return nil
}

// CheckPassword 校验密码
func (u *User) CheckPassword(password string) bool {
	if u.Password == "" {
		util.DummyCheckPassword(password)
		return false
	}
	return util.CheckPasswordHash(u.Password, password)
}
