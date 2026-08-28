// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/gin-gonic/gin"

	"Wavelet/core/contracts"
	"Wavelet/pkg/util"
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
		if u, ok := util.GetFromContext[*contracts.UserDTO](ginCtx, contracts.AuthUserObjKey); ok && u != nil {
			return u, nil
		}
	}

	if v := ctx.Value(contracts.AuthUserObjKey); v != nil {
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
		if err := getDB(ctx).Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&tokenRow).Error; err != nil {
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
		if err := getDB(ctx).Table("w_users").Where("id = ? AND is_active = ?", tokenRecord.UserID, true).First(&dbUser).Error; err != nil {
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

func (s *authServiceImpl) GetCurrentUserID(ctx context.Context) (uint64, error) {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		return GetUserIDFromContext(ginCtx), nil
	}
	return 0, errors.New("auth: user not found in context")
}

func (s *authServiceImpl) RevokeToken(ctx context.Context, tokenHash string) error {
	InvalidateCachedToken(ctx, tokenHash)
	return nil
}

func (s *authServiceImpl) DisallowTokenAuthMiddleware() any {
	return DisallowTokenAuth()
}

func (s *authServiceImpl) InvalidateCachedUser(ctx context.Context, userID uint64) {
	InvalidateCachedUser(ctx, userID)
}

func (s *authServiceImpl) InvalidateCachedToken(ctx context.Context, tokenHash string) {
	InvalidateCachedToken(ctx, tokenHash)
}

func (s *authServiceImpl) ListAuthSources(ctx context.Context) ([]contracts.AuthSourceViewDTO, error) {
	var sources []AuthSource
	if err := getDB(ctx).Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}

	views := make([]contracts.AuthSourceViewDTO, len(sources))
	for i := range sources {
		views[i] = contracts.AuthSourceViewDTO{
			ID:                     sources[i].ID,
			Name:                   sources[i].Name,
			Type:                   sources[i].Type,
			DisplayName:            sources[i].DisplayName,
			IsActive:               sources[i].IsActive,
			IconURL:                sources[i].IconURL,
			ClientSecretConfigured: sources[i].ClientSecret != "",
		}
	}
	return views, nil
}

func (s *authServiceImpl) CreateAuthSource(ctx context.Context, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	model := AuthSource{
		ID:                 source.ID,
		Name:               source.Name,
		Type:               source.Type,
		DisplayName:        source.DisplayName,
		ClientID:           source.ClientID,
		ClientSecret:       source.ClientSecret,
		OpenIDDiscoveryURL: source.OpenIDDiscoveryURL,
		Scopes:             source.Scopes,
		IconURL:            source.IconURL,
		IsActive:           source.IsActive,
	}

	if err := model.Validate(); err != nil {
		return nil, err
	}

	if err := getDB(ctx).Create(&model).Error; err != nil {
		return nil, err
	}

	model.Sanitize()
	return toAuthSourceDTO(&model), nil
}

func (s *authServiceImpl) UpdateAuthSource(ctx context.Context, id uint64, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	var existing AuthSource
	if err := getDB(ctx).First(&existing, id).Error; err != nil {
		return nil, err
	}

	existing.DisplayName = source.DisplayName
	existing.ClientID = source.ClientID
	if source.ClientSecret != "" {
		existing.ClientSecret = source.ClientSecret
	}
	existing.OpenIDDiscoveryURL = source.OpenIDDiscoveryURL
	existing.Scopes = source.Scopes
	existing.IconURL = source.IconURL

	if err := existing.Validate(); err != nil {
		return nil, err
	}

	if err := getDB(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(&existing), nil
}

func (s *authServiceImpl) DeleteAuthSource(ctx context.Context, id uint64) error {
	var existing AuthSource
	if err := getDB(ctx).First(&existing, id).Error; err != nil {
		return err
	}

	return getDB(ctx).Delete(&existing).Error
}

func (s *authServiceImpl) ToggleAuthSource(ctx context.Context, id uint64) (*contracts.AuthSourceDTO, error) {
	var existing AuthSource
	if err := getDB(ctx).First(&existing, id).Error; err != nil {
		return nil, err
	}

	existing.IsActive = !existing.IsActive
	if err := getDB(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(&existing), nil
}

func toAuthSourceDTO(s *AuthSource) *contracts.AuthSourceDTO {
	if s == nil {
		return nil
	}
	return &contracts.AuthSourceDTO{
		ID:                 s.ID,
		Name:               s.Name,
		Type:               s.Type,
		DisplayName:        s.DisplayName,
		ClientID:           s.ClientID,
		ClientSecret:       s.ClientSecret,
		OpenIDDiscoveryURL: s.OpenIDDiscoveryURL,
		Scopes:             s.Scopes,
		IconURL:            s.IconURL,
		IsActive:           s.IsActive,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
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
