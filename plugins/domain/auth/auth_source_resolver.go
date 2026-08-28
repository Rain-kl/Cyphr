// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/core/contracts"

	db "github.com/Rain-kl/Wavelet/pkg/persistence"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func isOIDCLoginEnabled(ctx context.Context) bool {
	var val string
	if err := db.DB(ctx).Table("w_system_configs").Where("key = ?", "oidc_login_enabled").Pluck("value", &val).Error; err != nil || val == "" {
		return true
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return true
	}
	return b
}

func resolveAuthSource(ctx context.Context, sourceName string) (*AuthSource, error) {
	name := strings.TrimSpace(strings.ToLower(sourceName))
	if name == "" {
		sources, err := GetActiveAuthSourcesCached(ctx)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return nil, errors.New(errNoActiveAuthSource)
		}
		src, err := GetAuthSourceByNameCached(ctx, sources[0].Name)
		if err != nil {
			return nil, err
		}
		return src, nil
	}
	src, err := GetAuthSourceByNameCached(ctx, name)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func activeLoginSources(ctx context.Context) []AuthSourceView {
	if !isOIDCLoginEnabled(ctx) {
		return nil
	}

	dbSources, err := GetActiveAuthSourcesCached(ctx)
	if err != nil {
		return nil
	}
	sources := make([]AuthSourceView, 0, len(dbSources))
	for _, source := range dbSources {
		sources = append(sources, AuthSourceView{
			ID:                     source.ID,
			Name:                   source.Name,
			Type:                   source.Type,
			DisplayName:            source.DisplayName,
			IsActive:               source.IsActive,
			IconURL:                source.IconURL,
			ClientSecretConfigured: source.ClientSecretConfigured,
		})
	}
	return sources
}

func getFrontendLoginRedirectURL(ctx context.Context) (string, error) {
	var val string
	if err := db.DB(ctx).Table("w_system_configs").Where("key = ?", "server_address").Pluck("value", &val).Error; err != nil || strings.TrimSpace(val) == "" {
		return "", errors.New(errServerAddressMissing)
	}
	return strings.TrimRight(val, "/") + "/login", nil
}

func buildOAuthConfig(ctx context.Context, source *AuthSource, redirectURL string) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if source == nil {
		return nil, nil, errors.New(errAuthSourceRequired)
	}

	if source.OpenIDDiscoveryURL == "" {
		return nil, nil, errors.New(errDiscoveryURLRequired)
	}

	// Clean the issuer URL
	issuer := strings.TrimSuffix(strings.TrimSpace(source.OpenIDDiscoveryURL), "/")
	issuer = strings.TrimSuffix(issuer, "/.well-known/openid-configuration")
	issuer = strings.TrimSuffix(issuer, "/.well-known/oauth-authorization-server")

	provider, err := globalOIDCProviderCache.get(ctx, issuer)
	if err != nil {
		return nil, nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: source.ClientID})
	scopes := strings.Fields(source.Scopes)
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if !containsScope(scopes, oidc.ScopeOpenID) {
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}

	return &oauth2.Config{
		ClientID:     source.ClientID,
		ClientSecret: source.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     provider.Endpoint(),
	}, verifier, nil
}

func containsScope(scopes []string, scope string) bool {
	for _, item := range scopes {
		if item == scope {
			return true
		}
	}
	return false
}

func buildOAuthUserInfo(ctx context.Context, source *AuthSource, code string, nonce string, redirectURL string) (*contracts.OAuthUserInfoDTO, error) {
	authConfig, verifier, err := buildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return nil, err
	}

	token, err := authConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	userInfo := &contracts.OAuthUserInfoDTO{Active: true}
	if verifier != nil {
		if verifyErr := verifyIDToken(ctx, verifier, token, nonce, userInfo); verifyErr != nil {
			return nil, verifyErr
		}
	}

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}

	return userInfo, nil
}

func verifyIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier, token *oauth2.Token, nonce string, userInfo *contracts.OAuthUserInfoDTO) error {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil
	}
	idToken, verifyErr := verifier.Verify(ctx, rawIDToken)
	if verifyErr != nil {
		return fmt.Errorf(errIDTokenVerifyFailedFormat, errIDTokenVerifyFailed, verifyErr)
	}
	if nonce != "" && idToken.Nonce != nonce {
		return errors.New(errNonceMismatch)
	}
	if claimsErr := idToken.Claims(userInfo); claimsErr != nil {
		return claimsErr
	}
	return nil
}

func normalizeOAuthUserInfo(userInfo *contracts.OAuthUserInfoDTO) error {
	userInfo.Username = strings.TrimSpace(userInfo.Username)
	userInfo.PreferredUsername = strings.TrimSpace(userInfo.PreferredUsername)
	userInfo.Email = strings.TrimSpace(userInfo.Email)
	userInfo.Name = strings.TrimSpace(userInfo.Name)
	userInfo.AvatarURL = strings.TrimSpace(userInfo.AvatarURL)

	if userInfo.Username == "" && userInfo.PreferredUsername != "" {
		userInfo.Username = userInfo.PreferredUsername
	}
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = strings.Split(userInfo.Email, "@")[0]
	}
	if userInfo.Username == "" && userInfo.Sub != "" {
		userInfo.Username = userInfo.Sub
	}
	if userInfo.Username == "" {
		return errors.New(errUsernameFromSourceFailed)
	}
	if userInfo.Name == "" {
		userInfo.Name = userInfo.Username
	}
	if !userInfo.Active {
		userInfo.Active = true
	}
	return nil
}

func buildCallbackResult(user *contracts.UserDTO, status string) OAuthCallbackResult {
	result := OAuthCallbackResult{Status: status}
	if user != nil {
		info := BuildBasicUserInfo(user, false)
		result.User = &info
	}
	return result
}
