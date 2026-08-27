// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"time"
)

// UserDTO represents a unified user data transfer object across plugins.
type UserDTO struct {
	ID                 uint64    `json:"id,string"`
	Username           string    `json:"username"`
	Nickname           string    `json:"nickname"`
	Email              string    `json:"email"`
	AvatarURL          string    `json:"avatar_url"`
	IsActive           bool      `json:"is_active"`
	IsAdmin            bool      `json:"is_admin"`
	NeedChangePassword bool      `json:"need_change_password,omitempty"`
	Bio                string    `json:"bio,omitempty"`
	Phone              string    `json:"phone,omitempty"`
	Gender             string    `json:"gender,omitempty"`
	Website            string    `json:"website,omitempty"`
	Location           string    `json:"location,omitempty"`
	LastLoginAt        time.Time `json:"last_login_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// OAuthUserInfoDTO contains user identity claims obtained from an OAuth provider.
type OAuthUserInfoDTO struct {
	ID                uint64 `json:"id"`
	Sub               string `json:"sub"`
	Username          string `json:"username"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Active            bool   `json:"active"`
	AvatarURL         string `json:"avatar_url"`
}

// OAuthProvider defines the pluggable OAuth provider contract.
type OAuthProvider interface {
	Name() string
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*OAuthUserInfoDTO, error)
}

// AuthService defines the contract for authentication, session verification, and token management.
type AuthService interface {
	// RequireAuthMiddleware returns a middleware handler (compatible with gin.HandlerFunc or standard middleware).
	RequireAuthMiddleware() any

	// RequireAdminMiddleware returns an admin authorization middleware.
	RequireAdminMiddleware() any

	// GetCurrentUser retrieves the authenticated UserDTO from context.
	GetCurrentUser(ctx context.Context) (*UserDTO, error)

	// VerifyToken validates an access token and returns the associated user DTO.
	VerifyToken(ctx context.Context, token string) (*UserDTO, error)

	// CreateSession establishes an authenticated session for the given user ID.
	CreateSession(ctx context.Context, userID uint64, extras map[string]any) (string, error)

	// RevokeUserSessions revokes all active sessions and cached tokens for a user.
	RevokeUserSessions(ctx context.Context, userID uint64) error
}

// AuthRegistry allows downstream and domain plugins to register custom authentication providers.
type AuthRegistry interface {
	RegisterOAuthProvider(name string, provider OAuthProvider)
	GetOAuthProvider(name string) (OAuthProvider, bool)
	ListOAuthProviders() []string
}
