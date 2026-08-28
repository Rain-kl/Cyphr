// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"errors"
)

// ToUserResponse projects the user contract DTO onto the console response shape.
func ToUserResponse(u *contracts.UserDTO) model.UserResponse {
	if u == nil {
		return model.UserResponse{}
	}
	return model.UserResponse{
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

// AdminListUsers pages users through the user contract service.
func AdminListUsers(
	ctx context.Context,
	filter contracts.AdminListUsersFilter,
) (int64, []*contracts.UserDTO, error) {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return 0, nil, err
	}

	total, dtos, err := userSvc.AdminListUsers(ctx, filter)
	if err != nil {
		logger.ErrorF(ctx, "List admin users failed: %v", err)
		return 0, nil, errors.New(errs.ListAdminUsersFailed)
	}
	return total, dtos, nil
}

// AdminGetUser loads a single user profile.
func AdminGetUser(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return nil, err
	}

	targetUser, err := userSvc.AdminGetUser(ctx, id)
	if err != nil {
		return nil, translateNotFound(err, errs.ErrUserNotFound)
	}
	return targetUser, nil
}

// AdminUpdateUserStatus enables or disables a user account.
func AdminUpdateUserStatus(ctx context.Context, id uint64, isActive bool) error {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return err
	}

	err = userSvc.AdminUpdateUserStatus(ctx, id, isActive)
	return translateNotFound(err, errs.ErrUserNotFound)
}

// AdminDeleteUser removes a user on behalf of the acting administrator.
func AdminDeleteUser(ctx context.Context, operatorID, id uint64) error {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return err
	}

	err = userSvc.AdminDeleteUser(ctx, operatorID, id)
	return translateNotFound(err, errs.ErrUserNotFound)
}

// AdminCreateUser registers a local-password user.
func AdminCreateUser(ctx context.Context, req contracts.AdminCreateUserRequest) (*contracts.UserDTO, error) {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return nil, err
	}

	newUser, err := userSvc.AdminCreateUser(ctx, req)
	if err != nil {
		return nil, translateNotFound(err, errs.ErrUserNotFound)
	}
	return newUser, nil
}

// AdminUpdateUser rewrites a user profile and optionally resets its password.
func AdminUpdateUser(
	ctx context.Context,
	operatorID uint64,
	req contracts.AdminUpdateUserRequest,
) error {
	userSvc, err := requireUserService(ctx)
	if err != nil {
		return err
	}

	err = userSvc.AdminUpdateUser(ctx, operatorID, req)
	return translateNotFound(err, errs.ErrUserNotFound)
}
