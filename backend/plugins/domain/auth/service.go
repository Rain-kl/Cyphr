// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core/contracts"
	"context"
	"errors"
	"sync"
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

// GetCurrentUser 从 context 中读取登录用户。
//
// 中间件通过 gin 的 c.Set(contracts.AuthUserObjKey, user) 写入登录态；
// *gin.Context 自身实现了 context.Context，且其 Value(key) 对 string 类型 key
// 等价于 c.Get(key)（未命中时再回落到 Request.Context().Value），
// 因此这里无需感知 gin 即可读取同一份登录态。
func (s *authServiceImpl) GetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if v := ctx.Value(contracts.AuthUserObjKey); v != nil {
		if u, ok := v.(*contracts.UserDTO); ok && u != nil {
			return u, nil
		}
	}

	return nil, errors.New(errUserNotInContext)
}

func (s *authServiceImpl) VerifyToken(ctx context.Context, token string) (*contracts.UserDTO, error) {
	if token == "" {
		return nil, errors.New(errEmptyToken)
	}

	tokenHash := hashToken(token)
	tokenRecord, err := GetCachedToken(ctx, tokenHash)
	if err != nil {
		tokenRecord, err = GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		SetCachedToken(ctx, tokenHash, tokenRecord)
	}

	user, err := GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || user == nil || !user.IsActive {
		user, err = GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, err
		}
		SetCachedUser(ctx, tokenRecord.UserID, user)
	}

	if user.Username == SystemUsername {
		return nil, errors.New(errSystemUserTokenNotAllowed)
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

// GetCurrentUserID 从请求登录态中读取用户 ID。
//
// Session 读取依赖 gin，属于接入层职责，因此这里通过接入层桥接函数
// currentUserIDFromRequestContext（见 middleware.go）取值，Service 层本身不感知 gin。
func (s *authServiceImpl) GetCurrentUserID(ctx context.Context) (uint64, error) {
	userID, ok := currentUserIDFromRequestContext(ctx)
	if !ok {
		return 0, errors.New(errUserNotInContext)
	}
	return userID, nil
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
	sources, err := ListAllAuthSources(ctx)
	if err != nil {
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

	if err := CreateAuthSourceRecord(ctx, &model); err != nil {
		return nil, err
	}

	model.Sanitize()
	return toAuthSourceDTO(&model), nil
}

func (s *authServiceImpl) UpdateAuthSource(ctx context.Context, id uint64, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	existing, err := GetAuthSourceByID(ctx, id)
	if err != nil {
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

	if err := SaveAuthSourceRecord(ctx, existing); err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(existing), nil
}

func (s *authServiceImpl) DeleteAuthSource(ctx context.Context, id uint64) error {
	existing, err := GetAuthSourceByID(ctx, id)
	if err != nil {
		return err
	}

	return DeleteAuthSourceRecord(ctx, existing)
}

func (s *authServiceImpl) ToggleAuthSource(ctx context.Context, id uint64) (*contracts.AuthSourceDTO, error) {
	existing, err := GetAuthSourceByID(ctx, id)
	if err != nil {
		return nil, err
	}

	existing.IsActive = !existing.IsActive
	if err := SaveAuthSourceRecord(ctx, existing); err != nil {
		return nil, err
	}

	existing.Sanitize()
	return toAuthSourceDTO(existing), nil
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
