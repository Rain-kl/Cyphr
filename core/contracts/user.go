// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
)

// CreateUserRequest contains fields to register or create a new user.
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	IsAdmin  bool   `json:"is_admin"`
}

// UpdateUserProfileRequest contains fields for updating a user's profile.
type UpdateUserProfileRequest struct {
	Nickname  *string `json:"nickname,omitempty"`
	Email     *string `json:"email,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Bio       *string `json:"bio,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Gender    *string `json:"gender,omitempty"`
	Website   *string `json:"website,omitempty"`
	Location  *string `json:"location,omitempty"`
}

// UserService defines the contract for user account management and profile queries.
type UserService interface {
	// GetUserByID retrieves a user by ID.
	GetUserByID(ctx context.Context, id uint64) (*UserDTO, error)

	// GetUserByUsername retrieves a user by username.
	GetUserByUsername(ctx context.Context, username string) (*UserDTO, error)

	// GetUserByEmail retrieves a user by email.
	GetUserByEmail(ctx context.Context, email string) (*UserDTO, error)

	// CreateUser registers or creates a new user account.
	CreateUser(ctx context.Context, req CreateUserRequest) (*UserDTO, error)

	// UpdateProfile updates the profile of the specified user.
	UpdateProfile(ctx context.Context, id uint64, req UpdateUserProfileRequest) (*UserDTO, error)

	// UpdatePassword updates the password for the specified user after verifying the old password.
	UpdatePassword(ctx context.Context, id uint64, oldPassword, newPassword string) error

	// VerifyPassword verifies if the given password matches the user's password.
	VerifyPassword(ctx context.Context, id uint64, password string) bool

	// UpdateLastLogin updates the user's last login timestamp.
	UpdateLastLogin(ctx context.Context, id uint64, ip string) error

	// ListUsers returns a paginated list of users with optional keyword search.
	ListUsers(ctx context.Context, page, pageSize int, keyword string) ([]*UserDTO, int64, error)

	// SetUserActive sets the active/banned status for a user.
	SetUserActive(ctx context.Context, id uint64, active bool) error

	// SetUserAdmin sets the admin role status for a user.
	SetUserAdmin(ctx context.Context, id uint64, admin bool) error

	// VerifyAccessToken verifies an access token hash and returns the user DTO and isAdmin flag.
	VerifyAccessToken(ctx context.Context, tokenHash string) (*UserDTO, bool, error)

	// DeleteUser removes a user and related access tokens.
	DeleteUser(ctx context.Context, id uint64) error

	// CountUsers returns total user count.
	CountUsers(ctx context.Context) (int64, error)

	// CountActiveUsers returns active user count.
	CountActiveUsers(ctx context.Context) (int64, error)

	// GetFirstAdminUser returns the earliest admin user.
	GetFirstAdminUser(ctx context.Context) (*UserDTO, error)

	// UniqueUsername generates a unique username candidate based on base.
	UniqueUsername(ctx context.Context, base string) (string, error)
}
