// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/Rain-kl/Wavelet/core/contracts"
	db "github.com/Rain-kl/Wavelet/plugins/infra/database"
	"github.com/gin-gonic/gin"
)

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
		if u, ok := GetFromContext[*contracts.UserDTO](ginCtx, UserObjKey); ok && u != nil {
			return u, nil
		}
	}

	if v := ctx.Value(UserObjKey); v != nil {
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

	tokenHash := hashToken(token)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err != nil {
		var tokenRow struct {
			ID      uint64
			UserID  uint64
			IsAdmin bool
		}
		if err := db.DB(ctx).Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&tokenRow).Error; err != nil {
			return nil, err
		}
		tokenRecord = &CachedToken{
			ID:      tokenRow.ID,
			UserID:  tokenRow.UserID,
			IsAdmin: tokenRow.IsAdmin,
		}
		SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || user == nil || !user.IsActive {
		var dbUser contracts.UserDTO
		if err := db.DB(ctx).Table("w_users").Where("id = ? AND is_active = ?", tokenRecord.UserID, true).First(&dbUser).Error; err != nil {
			return nil, err
		}
		user = &dbUser
		SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	if user.Username == SystemUsername {
		return nil, errors.New("auth: system user token not allowed")
	}

	return user, nil
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
