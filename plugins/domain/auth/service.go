// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/gin-gonic/gin"
)

func toUserDTO(u *model.User) *contracts.UserDTO {
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

type authServiceImpl struct{}

func newAuthService() contracts.AuthService {
	return &authServiceImpl{}
}

func (s *authServiceImpl) RequireAuthMiddleware() any {
	return LoginRequired()
}

func (s *authServiceImpl) RequireAdminMiddleware() any {
	return AdminRequired()
}

func (s *authServiceImpl) GetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		if u, ok := GetFromContext[*model.User](ginCtx, UserObjKey); ok && u != nil {
			return toUserDTO(u), nil
		}
	}

	if v := ctx.Value(UserObjKey); v != nil {
		if u, ok := v.(*model.User); ok && u != nil {
			return toUserDTO(u), nil
		}
		if u, ok := v.(*contracts.UserDTO); ok && u != nil {
			return u, nil
		}
	}

	return nil, errors.New("auth: user not found in context")
}

func (s *authServiceImpl) VerifyToken(ctx context.Context, token string) (*contracts.UserDTO, error) {
	if token == "" {
		return nil, errors.New("auth: empty token")
	}

	tokenHash := model.HashToken(token)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err != nil {
		dbToken, err := repository.GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		tokenRecord = &dbToken
		SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || !user.IsActive {
		dbUser, err := repository.GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, err
		}
		user = &dbUser
		SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	if user.Username == SystemUsername {
		return nil, errors.New("auth: system user token not allowed")
	}

	return toUserDTO(user), nil
}

func (s *authServiceImpl) CreateSession(_ context.Context, _ uint64, _ map[string]any) (string, error) {
	return "", nil
}

func (s *authServiceImpl) RevokeUserSessions(ctx context.Context, userID uint64) error {
	InvalidateCachedUser(ctx, userID)
	return nil
}

type authRegistryImpl struct {
	mu        sync.RWMutex
	providers map[string]contracts.OAuthProvider
}

func newAuthRegistry() contracts.AuthRegistry {
	return &authRegistryImpl{
		providers: make(map[string]contracts.OAuthProvider),
	}
}

func (r *authRegistryImpl) RegisterOAuthProvider(name string, provider contracts.OAuthProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

func (r *authRegistryImpl) GetOAuthProvider(name string) (contracts.OAuthProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

func (r *authRegistryImpl) ListOAuthProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, 0, len(r.providers))
	for name := range r.providers {
		res = append(res, name)
	}
	return res
}
