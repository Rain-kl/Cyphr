// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user provides user profiles, credentials, role management, and access token domain services.
package user

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgu "Wavelet/pkg/util"
)

const columnUpdatedAt = "updated_at"

func toUserDTO(u *User) *contracts.UserDTO {
	if u == nil {
		return nil
	}
	return &contracts.UserDTO{
		ID:                 u.ID,
		Username:           u.Username,
		Nickname:           u.Nickname,
		Email:              u.Email,
		AvatarURL:          u.AvatarURL,
		IsActive:           u.IsActive,
		IsAdmin:            u.IsAdmin,
		NeedChangePassword: u.NeedChangePassword || u.IsPlaintextPassword(),
		Bio:                u.Bio,
		Phone:              u.Phone,
		Gender:             u.Gender,
		Website:            u.Website,
		Location:           u.Location,
		LastLoginAt:        u.LastLoginAt,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
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

func (s *userServiceImpl) GetUsersByIDs(ctx context.Context, ids []uint64) ([]*contracts.UserDTO, error) {
	users, err := GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	dtos := make([]*contracts.UserDTO, 0, len(users))
	for i := range users {
		dtos = append(dtos, toUserDTO(&users[i]))
	}
	return dtos, nil
}

func (s *userServiceImpl) GetUserByUsername(ctx context.Context, username string) (*contracts.UserDTO, error) {
	u, err := GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

func (s *userServiceImpl) GetUserByEmail(ctx context.Context, email string) (*contracts.UserDTO, error) {
	u, err := GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return toUserDTO(u), nil
}

func (s *userServiceImpl) CreateUser(ctx context.Context, req contracts.CreateUserRequest) (*contracts.UserDTO, error) {
	if req.Username == "" {
		return nil, errors.New(errServiceUsernameEmpty)
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

	if err := updateUserColumns(ctx, id, updates); err != nil {
		return nil, err
	}

	return s.GetUserByID(ctx, id)
}

func (s *userServiceImpl) UpdatePassword(ctx context.Context, id uint64, oldPassword, newPassword string) error {
	user, err := GetUserByID(ctx, id)
	if err != nil {
		return err
	}

	if !user.CheckPassword(oldPassword) {
		return errors.New(errServiceOldPasswordIncorrect)
	}

	if err := user.SetEncryptedPassword(newPassword); err != nil {
		return err
	}

	return updateUserColumns(ctx, id, map[string]any{
		"password":      user.Password,
		columnUpdatedAt: time.Now(),
	})
}

func (s *userServiceImpl) VerifyPassword(ctx context.Context, id uint64, password string) bool {
	user, err := GetUserByID(ctx, id)
	if err != nil {
		pkgu.DummyCheckPassword(password)
		return false
	}
	return user.CheckPassword(password)
}

func (s *userServiceImpl) UpdateLastLogin(ctx context.Context, id uint64, _ string) error {
	return updateUserColumns(ctx, id, map[string]any{
		"last_login_at": time.Now(),
		columnUpdatedAt: time.Now(),
	})
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
	return setUserAdminFlag(ctx, id, admin)
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
	return countAllUsers(ctx)
}

func (s *userServiceImpl) CountActiveUsers(ctx context.Context) (int64, error) {
	return countActiveUsers(ctx)
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

	return "", errors.New(errUniqueUsernameFailed)
}

func (s *userServiceImpl) AdminListUsers(ctx context.Context, filter contracts.AdminListUsersFilter) (int64, []*contracts.UserDTO, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	return adminListUserRows(ctx, filter)
}

func (s *userServiceImpl) AdminGetUser(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	return adminGetUserRow(ctx, id)
}

func (s *userServiceImpl) AdminCreateUser(ctx context.Context, req contracts.AdminCreateUserRequest) (*contracts.UserDTO, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return nil, errors.New(errUsernameEmpty)
	}
	if req.Email == "" {
		return nil, errors.New(errEmailEmpty)
	}
	const minPasswordLen = 8
	if len(req.Password) < minPasswordLen {
		return nil, errors.New(errServicePasswordTooShort)
	}

	count, err := countUsersByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New(errUsernameTaken)
	}

	emailCount, err := countUsersByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if emailCount > 0 {
		return nil, errors.New(errEmailTaken)
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
	if err := insertUserRow(ctx, row); err != nil {
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
		return errors.New(errEmailEmpty)
	}

	targetUser, err := getUserRow(ctx, req.ID)
	if err != nil {
		return err
	}

	if currentUserID == req.ID && !req.IsAdmin && targetUser.IsAdmin {
		return errors.New(errCannotRevokeSelf)
	}

	if targetUser.Email != req.Email {
		count, err := countOtherUsersByEmail(ctx, req.Email, req.ID)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New(errEmailTaken)
		}
	}

	const minPasswordLen = 8
	if req.Password != "" && len(req.Password) < minPasswordLen {
		return errors.New(errServicePasswordTooShort)
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

	err = updateUserColumns(ctx, req.ID, updates)
	if err == nil && s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserUpdated, targetUser)
	}
	return err
}

func (s *userServiceImpl) AdminUpdateUserStatus(ctx context.Context, id uint64, active bool) error {
	flags, err := getUserAdminFlags(ctx, id)
	if err != nil {
		return err
	}
	if !active && flags.IsAdmin {
		return errors.New(errAdminCannotDisable)
	}

	err = setUserActiveColumn(ctx, id, active)
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
		return errors.New(errCannotDeleteSelf)
	}
	flags, err := getUserAdminFlags(ctx, targetID)
	if err != nil {
		return err
	}
	if flags.IsAdmin {
		return errors.New(errAdminCannotDelete)
	}

	if err := deleteUserCascadeAdmin(ctx, targetID); err != nil {
		return err
	}
	if s.events != nil {
		_ = s.events.Emit(ctx, contracts.EventTopicUserDeleted, contracts.UserDeletedEvent{
			CurrentUserID: currentUserID,
			TargetUserID:  targetID,
		})
	}
	return nil
}
