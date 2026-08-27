// Package auth provides authentication, OAuth, session management, and access token domain services.
package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/internal/apps/admin"
	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
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
	return oauth.LoginRequired()
}

func (s *authServiceImpl) RequireAdminMiddleware() any {
	return admin.LoginAdminRequired()
}

func (s *authServiceImpl) GetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if ginCtx, ok := ctx.(*gin.Context); ok {
		if u, ok := oauth.GetFromContext[*model.User](ginCtx, oauth.UserObjKey); ok && u != nil {
			return toUserDTO(u), nil
		}
	}

	if v := ctx.Value(oauth.UserObjKey); v != nil {
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
	tokenRecord, err := oauth.GetCachedToken(ctx, tokenHash)
	if err != nil {
		dbToken, err := repository.GetAccessTokenByHash(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		tokenRecord = &dbToken
	}

	user, err := oauth.GetCachedUser(ctx, tokenRecord.UserID)
	if err != nil || !user.IsActive {
		dbUser, err := repository.GetActiveUserByID(ctx, tokenRecord.UserID)
		if err != nil {
			return nil, err
		}
		user = &dbUser
	}

	if user.Username == "system" {
		return nil, errors.New("auth: system user token not allowed")
	}

	return toUserDTO(user), nil
}

func (s *authServiceImpl) CreateSession(_ context.Context, _ uint64, _ map[string]any) (string, error) {
	// Session creation helper
	return "", nil
}

func (s *authServiceImpl) RevokeUserSessions(ctx context.Context, userID uint64) error {
	oauth.InvalidateCachedUser(ctx, userID)
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
