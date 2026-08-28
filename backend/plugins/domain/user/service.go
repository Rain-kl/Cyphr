// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user provides user profiles, credentials, role management, and access token domain services.
package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	database "Wavelet/plugins/infra/database"

	pkgu "Wavelet/pkg/util"
)

func toUserDTO(u *User) *contracts.UserDTO {
	if u == nil {
		return nil
	}
	return &contracts.UserDTO{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		Email:       u.Email,
		AvatarURL:   u.AvatarURL,
		IsActive:    u.IsActive,
		IsAdmin:     u.IsAdmin,
		Bio:         u.Bio,
		Phone:       u.Phone,
		Gender:      u.Gender,
		Website:     u.Website,
		Location:    u.Location,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

type userServiceImpl struct{}

func newUserService() contracts.UserService {
	return &userServiceImpl{}
}

func (s *userServiceImpl) GetUserByID(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	u, err := GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

func (s *userServiceImpl) GetUserByUsername(ctx context.Context, username string) (*contracts.UserDTO, error) {
	u, err := GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

func (s *userServiceImpl) GetUserByEmail(ctx context.Context, email string) (*contracts.UserDTO, error) {
	var u User
	if err := database.DB(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return toUserDTO(&u), nil
}

func (s *userServiceImpl) CreateUser(ctx context.Context, req contracts.CreateUserRequest) (*contracts.UserDTO, error) {
	if req.Username == "" {
		return nil, errors.New("user: username cannot be empty")
	}

	user := User{
		ID:          idgen.NextUint64ID(),
		Username:    req.Username,
		Nickname:    req.Nickname,
		Email:       req.Email,
		IsActive:    true,
		IsAdmin:     req.IsAdmin,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		LastLoginAt: time.Now(),
	}

	if user.Nickname == "" {
		user.Nickname = req.Username
	}

	if req.Password != "" {
		if err := user.SetEncryptedPassword(req.Password); err != nil {
			return nil, err
		}
	}

	if err := CreateUser(ctx, &user); err != nil {
		return nil, err
	}

	return toUserDTO(&user), nil
}

func (s *userServiceImpl) UpdateProfile(ctx context.Context, id uint64, req contracts.UpdateUserProfileRequest) (*contracts.UserDTO, error) {
	updates := make(map[string]any)
	if req.Nickname != nil {
		updates["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.Website != nil {
		updates["website"] = *req.Website
	}
	if req.Location != nil {
		updates["location"] = *req.Location
	}
	updates["updated_at"] = time.Now()

	if err := database.DB(ctx).Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetUserByID(ctx, id)
}

func (s *userServiceImpl) UpdatePassword(ctx context.Context, id uint64, oldPassword, newPassword string) error {
	var user User
	if err := database.DB(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return err
	}

	if !user.CheckPassword(oldPassword) {
		return errors.New("user: incorrect old password")
	}

	if err := user.SetEncryptedPassword(newPassword); err != nil {
		return err
	}

	return database.DB(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{
			"password":   user.Password,
			"updated_at": time.Now(),
		}).Error
}

func (s *userServiceImpl) VerifyPassword(ctx context.Context, id uint64, password string) bool {
	var user User
	if err := database.DB(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		pkgu.DummyCheckPassword(password)
		return false
	}
	return user.CheckPassword(password)
}

func (s *userServiceImpl) UpdateLastLogin(ctx context.Context, id uint64, _ string) error {
	return database.DB(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{
			"last_login_at": time.Now(),
			"updated_at":    time.Now(),
		}).Error
}

func (s *userServiceImpl) ListUsers(ctx context.Context, page, pageSize int, keyword string) ([]*contracts.UserDTO, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	filter := AdminUserListFilter{
		Username: keyword,
		Page:     page,
		PageSize: pageSize,
	}

	total, users, err := ListAdminUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]*contracts.UserDTO, 0, len(users))
	for i := range users {
		dtos = append(dtos, toUserDTO(&users[i]))
	}

	return dtos, total, nil
}

func (s *userServiceImpl) SetUserActive(ctx context.Context, id uint64, active bool) error {
	return UpdateUserActive(ctx, id, active)
}

func (s *userServiceImpl) SetUserAdmin(ctx context.Context, id uint64, admin bool) error {
	return database.DB(ctx).Model(&User{}).Where("id = ?", id).Update("is_admin", admin).Error
}

func (s *userServiceImpl) VerifyAccessToken(ctx context.Context, tokenHash string) (*contracts.UserDTO, bool, error) {
	tokenRecord, err := GetAccessTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, false, err
	}

	user, err := GetActiveUserByID(ctx, tokenRecord.UserID)
	if err != nil {
		return nil, false, err
	}

	return toUserDTO(user), tokenRecord.IsAdmin, nil
}

func (s *userServiceImpl) DeleteUser(ctx context.Context, id uint64) error {
	return DeleteUserWithRelations(ctx, id)
}

func (s *userServiceImpl) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := database.DB(ctx).Model(&User{}).Count(&count).Error
	return count, err
}

func (s *userServiceImpl) CountActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := database.DB(ctx).Model(&User{}).Where("is_active = ?", true).Count(&count).Error
	return count, err
}

func (s *userServiceImpl) GetFirstAdminUser(ctx context.Context) (*contracts.UserDTO, error) {
	u, err := GetFirstAdminUser(ctx)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

func (s *userServiceImpl) UniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = PluginName
	}

	existingUsernames, err := ListUsernamesMatchingBase(ctx, base)
	if err != nil {
		return "", err
	}

	exists := make(map[string]bool, len(existingUsernames))
	for _, u := range existingUsernames {
		exists[strings.ToLower(u)] = true
	}

	if !exists[strings.ToLower(base)] {
		return base, nil
	}

	for i := 1; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !exists[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}

	return "", errors.New("failed to generate unique username")
}
