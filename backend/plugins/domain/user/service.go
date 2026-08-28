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

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	database "Wavelet/plugins/infra/database"
	"gorm.io/gorm"

	pkgu "Wavelet/pkg/util"
)

const columnUpdatedAt = "updated_at"

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

type userServiceImpl struct {
	events *core.EventBus
}

func newUserService(events ...*core.EventBus) contracts.UserService {
	var bus *core.EventBus
	if len(events) > 0 {
		bus = events[0]
	}
	return &userServiceImpl{events: bus}
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
	updates[columnUpdatedAt] = time.Now()

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
			"password":      user.Password,
			columnUpdatedAt: time.Now(),
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
			columnUpdatedAt: time.Now(),
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

func (s *userServiceImpl) AdminListUsers(ctx context.Context, filter contracts.AdminListUsersFilter) (int64, []*contracts.UserDTO, error) {
	query := database.DB(ctx).Table("w_users")
	if filter.UserID != nil {
		query = query.Where("id = ?", *filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username LIKE ? ESCAPE '\\'", pkgu.EscapeLike(filter.Username)+"%")
	}
	if filter.Email != "" {
		query = query.Where("email LIKE ? ESCAPE '\\'", pkgu.EscapeLike(filter.Email)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var users []*contracts.UserDTO
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, bio, phone, gender, website, location, last_login_at, created_at, updated_at").
		Order("id ASC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

func (s *userServiceImpl) AdminGetUser(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := database.DB(ctx).Table("w_users").
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, bio, phone, gender, website, location, last_login_at, created_at, updated_at").
		Where("id = ?", id).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *userServiceImpl) AdminCreateUser(ctx context.Context, req contracts.AdminCreateUserRequest) (*contracts.UserDTO, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return nil, errors.New("用户名不能为空")
	}
	if req.Email == "" {
		return nil, errors.New("邮箱不能为空")
	}
	const minPasswordLen = 8
	if len(req.Password) < minPasswordLen {
		return nil, errors.New("密码长度至少为 8 位")
	}

	var count int64
	if err := database.DB(ctx).Table("w_users").Where("username = ?", req.Username).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("用户名已被使用")
	}

	var emailCount int64
	if err := database.DB(ctx).Table("w_users").Where("email = ?", req.Email).Count(&emailCount).Error; err != nil {
		return nil, err
	}
	if emailCount > 0 {
		return nil, errors.New("邮箱已被使用")
	}

	hash, err := pkgu.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	if req.Nickname == "" {
		req.Nickname = req.Username
	}

	now := time.Now()
	newUser := contracts.UserDTO{
		ID:        idgen.NextUint64ID(),
		Username:  req.Username,
		Nickname:  req.Nickname,
		Email:     req.Email,
		IsActive:  req.IsActive,
		IsAdmin:   req.IsAdmin,
		CreatedAt: now,
		UpdatedAt: now,
	}

	row := map[string]any{
		"id":            newUser.ID,
		"username":      newUser.Username,
		"password":      hash,
		"nickname":      newUser.Nickname,
		"email":         newUser.Email,
		"is_active":     newUser.IsActive,
		"is_admin":      newUser.IsAdmin,
		"created_at":    now,
		columnUpdatedAt: now,
	}
	if err := database.DB(ctx).Table("w_users").Create(row).Error; err != nil {
		return nil, err
	}

	if s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserCreated, contracts.UserCreatedEvent{
			User:     &newUser,
			Password: req.Password,
		})
	}

	return &newUser, nil
}

func (s *userServiceImpl) AdminUpdateUser(ctx context.Context, currentUserID uint64, req contracts.AdminUpdateUserRequest) error {
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(req.Email)
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" {
		return errors.New("邮箱不能为空")
	}

	var targetUser contracts.UserDTO
	if err := database.DB(ctx).Table("w_users").Where("id = ?", req.ID).First(&targetUser).Error; err != nil {
		return err
	}

	if currentUserID == req.ID && !req.IsAdmin && targetUser.IsAdmin {
		return errors.New("不能取消自己的管理员权限")
	}

	if targetUser.Email != req.Email {
		var count int64
		if err := database.DB(ctx).Table("w_users").Where("email = ? AND id != ?", req.Email, req.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("邮箱已被使用")
		}
	}

	const minPasswordLen = 8
	if req.Password != "" && len(req.Password) < minPasswordLen {
		return errors.New("密码长度至少为 8 位")
	}

	if req.Nickname == "" {
		req.Nickname = targetUser.Username
	}

	updates := map[string]any{
		"nickname":      req.Nickname,
		"email":         req.Email,
		"is_admin":      req.IsAdmin,
		columnUpdatedAt: time.Now(),
	}
	if req.Password != "" {
		hash, err := pkgu.HashPassword(req.Password)
		if err != nil {
			return err
		}
		updates["password"] = hash
	}

	err := database.DB(ctx).Table("w_users").Where("id = ?", req.ID).Updates(updates).Error
	if err == nil && s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserUpdated, &targetUser)
	}
	return err
}

func (s *userServiceImpl) AdminUpdateUserStatus(ctx context.Context, id uint64, active bool) error {
	var flags struct {
		ID      uint64
		IsAdmin bool
	}
	if err := database.DB(ctx).Table("w_users").Select("id, is_admin").Where("id = ?", id).First(&flags).Error; err != nil {
		return err
	}
	if !active && flags.IsAdmin {
		return errors.New("管理员账号无法被禁用")
	}

	err := database.DB(ctx).Table("w_users").Where("id = ?", id).Update("is_active", active).Error
	if err == nil && s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserStatusChanged, contracts.UserStatusChangedEvent{
			UserID:   id,
			IsActive: active,
		})
	}
	return err
}

func (s *userServiceImpl) AdminDeleteUser(ctx context.Context, currentUserID, targetID uint64) error {
	if currentUserID == targetID {
		return errors.New("不能删除当前登录用户")
	}
	var flags struct {
		ID      uint64
		IsAdmin bool
	}
	if err := database.DB(ctx).Table("w_users").Select("id, is_admin").Where("id = ?", targetID).First(&flags).Error; err != nil {
		return err
	}
	if flags.IsAdmin {
		return errors.New("管理员账号无法被删除")
	}

	err := database.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("w_access_tokens").Where("user_id = ?", targetID).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		if err := tx.Table("w_external_accounts").Where("user_id = ?", targetID).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		return tx.Table("w_users").Where("id = ?", targetID).Delete(map[string]any{}).Error
	})
	if err == nil && s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserDeleted, contracts.UserDeletedEvent{
			CurrentUserID: currentUserID,
			TargetUserID:  targetID,
		})
	}
	return err
}
