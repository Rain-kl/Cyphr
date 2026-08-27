// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func isOIDCLoginEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyOIDCLoginEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func resolveAuthSource(ctx context.Context, sourceName string) (*model.AuthSource, error) {
	name := strings.TrimSpace(strings.ToLower(sourceName))
	if name == "" {
		sources, err := repository.GetActiveAuthSourcesCached(ctx)
		if err != nil {
			return nil, err
		}
		if len(sources) == 0 {
			return nil, errors.New(errNoActiveAuthSource)
		}
		return repository.GetAuthSourceByNameCached(ctx, sources[0].Name)
	}
	return repository.GetAuthSourceByNameCached(ctx, name)
}

func activeLoginSources(ctx context.Context) []AuthSourceView {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyOIDCLoginEnabled)
	if err == nil && !enabled {
		return nil
	}

	dbSources, err := repository.GetActiveAuthSourcesCached(ctx)
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
	sc, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress)
	if err != nil || strings.TrimSpace(sc.Value) == "" {
		return "", errors.New(errServerAddressMissing)
	}
	return strings.TrimRight(sc.Value, "/") + "/login", nil
}

func buildOAuthConfig(ctx context.Context, source *model.AuthSource, redirectURL string) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
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

func uniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}

	existingUsernames, err := repository.ListUsernamesMatchingBase(ctx, base)
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

	return "", errors.New(errUsernameGenerateFailed)
}

func buildOAuthUserInfo(ctx context.Context, source *model.AuthSource, code string, nonce string, redirectURL string) (*model.OAuthUserInfo, error) {
	authConfig, verifier, err := buildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return nil, err
	}

	token, err := authConfig.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	userInfo := &model.OAuthUserInfo{Active: true}
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

func verifyIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier, token *oauth2.Token, nonce string, userInfo *model.OAuthUserInfo) error {
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

func normalizeOAuthUserInfo(userInfo *model.OAuthUserInfo) error {
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

func buildCallbackResult(user *model.User, status string) OAuthCallbackResult {
	result := OAuthCallbackResult{Status: status}
	if user != nil {
		info := BuildBasicUserInfo(user, false)
		result.User = &info
	}
	return result
}
