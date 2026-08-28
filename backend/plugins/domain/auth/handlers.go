// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Wavelet/core/contracts"

	"Wavelet/pkg/idgen"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/pkg/util"
	cachepkg "Wavelet/plugins/infra/cache"
	db "Wavelet/plugins/infra/database"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetLoginSources 获取可用登录源列表
func GetLoginSources(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(activeLoginSources(c.Request.Context())))
}

// GetLoginURL 获取登录授权地址
func GetLoginURL(c *gin.Context) {
	ctx := c.Request.Context()
	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, c.Query("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	session := sessions.Default(c)
	token, isNew := ensureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	userID := GetUserIDFromSession(session)
	sessionHash := hashSessionToken(token)
	if err := reserveOAuthStateSlot(ctx, sessionHash); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	state := uuid.NewString()
	payloadValue, err := encodeOAuthStatePayload(oauthStatePayload{
		SourceName:  source.Name,
		Purpose:     OAuthPurposeLogin,
		UserID:      userID,
		SessionHash: sessionHash,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := cachepkg.Redis.Set(ctx, cachepkg.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, state)), payloadValue, OAuthStateCacheKeyExpiration).Err(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	authorizeURL, err := buildAuthorizeURL(c.Request.Context(), source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

func buildAuthorizeURL(ctx context.Context, source *AuthSource, state string) (string, error) {
	redirectURL, err := getFrontendLoginRedirectURL(ctx)
	if err != nil {
		return "", err
	}
	authConfig, verifier, err := buildOAuthConfig(ctx, source, redirectURL)
	if err != nil {
		return "", err
	}
	if verifier != nil {
		return authConfig.AuthCodeURL(state, oidc.Nonce(state)), nil
	}
	return authConfig.AuthCodeURL(state), nil
}

func reserveOAuthStateSlot(ctx context.Context, sessionHash string) error {
	if cachepkg.Redis == nil || sessionHash == "" {
		return nil
	}
	key := cachepkg.PrefixedKey(fmt.Sprintf(oauthStateLimitKeyFormat, sessionHash))
	n, err := cachepkg.Redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if n == 1 {
		_ = cachepkg.Redis.Expire(ctx, key, OAuthStateCacheKeyExpiration).Err()
	}
	if n > oauthStateLimitMax {
		return errors.New(errOAuthStateRateLimited)
	}
	return nil
}

// Authorize 发起指定认证源授权
func Authorize(c *gin.Context) {
	ctx := c.Request.Context()
	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, c.Param("source"))
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}
	purpose := strings.ToLower(strings.TrimSpace(c.Query("purpose")))
	if purpose != OAuthPurposeBind {
		purpose = OAuthPurposeLogin
	}

	session := sessions.Default(c)
	userID := GetUserIDFromSession(session)
	if purpose == OAuthPurposeBind && userID == 0 {
		response.AbortUnauthorized(c, errUnAuthorized)
		return
	}

	token, isNew := ensureSessionToken(session)
	if isNew {
		if err := session.Save(); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	sessionHash := hashSessionToken(token)
	if err := reserveOAuthStateSlot(ctx, sessionHash); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	state := uuid.NewString()
	payloadValue, err := encodeOAuthStatePayload(oauthStatePayload{
		SourceName:  source.Name,
		Purpose:     purpose,
		UserID:      userID,
		SessionHash: sessionHash,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := cachepkg.Redis.Set(ctx, cachepkg.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, state)), payloadValue, OAuthStateCacheKeyExpiration).Err(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	authorizeURL, err := buildAuthorizeURL(c.Request.Context(), source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

// Callback OAuth 回调处理
func Callback(c *gin.Context) {
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	stateKey := cachepkg.PrefixedKey(fmt.Sprintf(OAuthStateCacheKeyFormat, req.State))
	payloadRaw, err := cachepkg.Redis.Get(ctx, stateKey).Result()
	if err != nil {
		response.AbortBadRequest(c, errInvalidState)
		return
	}
	_ = cachepkg.Redis.Del(ctx, stateKey)

	payload, err := decodeOAuthStatePayload(payloadRaw)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	session := sessions.Default(c)
	currentUserID := GetUserIDFromSession(session)

	if payload.Purpose == OAuthPurposeBind && currentUserID == 0 {
		response.AbortUnauthorized(c, errUnAuthorized)
		return
	}

	token, ok := session.Get(SessionTokenKey).(string)
	if !ok || token == "" {
		response.AbortBadRequest(c, "invalid session context")
		return
	}

	if hashSessionToken(token) != payload.SessionHash {
		response.AbortBadRequest(c, "session mismatch for oauth state")
		return
	}

	if payload.Purpose == OAuthPurposeBind && currentUserID != payload.UserID {
		response.AbortBadRequest(c, "user context mismatch for oauth binding")
		return
	}

	if !isOIDCLoginEnabled(ctx) {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	source, err := resolveAuthSource(ctx, payload.SourceName)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	if !source.IsActive {
		response.AbortBadRequest(c, errAuthSourceDisabled)
		return
	}

	redirectURL, err := getFrontendLoginRedirectURL(ctx)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	userInfo, err := buildOAuthUserInfo(ctx, source, req.Code, req.State, redirectURL)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := normalizeOAuthUserInfo(userInfo); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if userInfo.Sub == "" {
		userInfo.Sub = userInfo.Username
	}

	if payload.Purpose == OAuthPurposeBind {
		handleCallbackBind(ctx, c, source, userInfo)
		return
	}

	handleCallbackLogin(ctx, c, source, userInfo)
}

func handleCallbackBind(ctx context.Context, c *gin.Context, source *AuthSource, userInfo *contracts.OAuthUserInfoDTO) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, errUnAuthorized)
		return
	}
	var user contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").Where("id = ?", userID).First(&user).Error; err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	if err := BindExternalAccount(ctx, &ExternalAccount{
		AuthSourceID:     source.ID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	user.LastLoginAt = time.Now()
	_ = db.DB(ctx).Table("w_users").Where("id = ?", user.ID).Update("last_login_at", user.LastLoginAt).Error
	c.JSON(http.StatusOK, response.OK(buildCallbackResult(&user, "bound")))
}

func handleCallbackLogin(ctx context.Context, c *gin.Context, source *AuthSource, userInfo *contracts.OAuthUserInfoDTO) {
	var user contracts.UserDTO

	account, err := FindExternalAccount(ctx, source.ID, userInfo.Sub)
	switch {
	case err == nil:
		if loadErr := db.DB(ctx).Table("w_users").Where("id = ?", account.UserID).First(&user).Error; loadErr != nil {
			response.AbortInternal(c, loadErr.Error())
			return
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		newUser, ok := handleCallbackRegister(ctx, c, source, userInfo)
		if !ok {
			return
		}
		user = newUser
	default:
		response.AbortInternal(c, err.Error())
		return
	}

	user.LastLoginAt = time.Now()
	_ = db.DB(ctx).Table("w_users").Where("id = ?", user.ID).Update("last_login_at", user.LastLoginAt).Error
	if err := SetLoginSession(ctx, c, &user); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	SetCachedUser(ctx, user.ID, &user)

	c.JSON(http.StatusOK, response.OK(buildCallbackResult(&user, "logged_in")))
}

func uniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}

	var existingUsernames []string
	if err := db.DB(ctx).Table("w_users").
		Where("username = ? OR username LIKE ? ESCAPE '\\'", base, util.EscapeLike(base)+"-%").
		Pluck("username", &existingUsernames).Error; err != nil {
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

func handleCallbackRegister(ctx context.Context, c *gin.Context, source *AuthSource, userInfo *contracts.OAuthUserInfoDTO) (contracts.UserDTO, bool) {
	registrationEnabled := true
	var val string
	if err := db.DB(ctx).Table("w_system_configs").Where("key = ?", "registration_enabled").Pluck("value", &val).Error; err == nil && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			registrationEnabled = b
		}
	}

	if !registrationEnabled {
		c.JSON(http.StatusOK, response.OK(buildCallbackResult(nil, "need_bind")))
		return contracts.UserDTO{}, false
	}

	username, uniqueErr := uniqueUsername(ctx, userInfo.Username)
	if uniqueErr != nil {
		response.AbortInternal(c, uniqueErr.Error())
		return contracts.UserDTO{}, false
	}
	userInfo.Username = username

	now := time.Now()
	user := contracts.UserDTO{
		ID:          idgen.NextUint64ID(),
		Username:    userInfo.Username,
		Nickname:    userInfo.Name,
		Email:       userInfo.Email,
		AvatarURL:   userInfo.AvatarURL,
		IsActive:    userInfo.Active,
		LastLoginAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := db.DB(ctx).Table("w_users").Create(&user).Error; err != nil {
		response.AbortInternal(c, err.Error())
		return contracts.UserDTO{}, false
	}

	if err := BindExternalAccount(ctx, &ExternalAccount{
		AuthSourceID:     source.ID,
		UserID:           user.ID,
		ExternalID:       userInfo.Sub,
		ExternalUsername: userInfo.Username,
		Email:            userInfo.Email,
	}); err != nil {
		response.AbortBadRequest(c, err.Error())
		return contracts.UserDTO{}, false
	}
	logger.InfoF(ctx, "[LoginAudit] successful OAuth registration via source: %s, external ID: %s, user: %s, ID: %d, IP: %s", source.Name, userInfo.Sub, user.Username, user.ID, c.ClientIP())

	return user, true
}

// UserInfo 获取当前登录用户信息
func UserInfo(c *gin.Context) {
	user, _ := util.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	session := sessions.Default(c)
	needChange := session.Get("need_change_password") == true

	c.JSON(
		http.StatusOK,
		response.OK(BuildBasicUserInfo(user, needChange)),
	)
}

// Logout 退出登录
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get(UserIDKey)
	username := session.Get(UserNameKey)
	if userID != nil {
		logger.InfoF(c.Request.Context(), "[LoginAudit] user logged out: %v, ID: %v, IP: %s", username, userID, c.ClientIP())
		if id := ParseUserID(userID); id > 0 {
			InvalidateCachedUser(c.Request.Context(), id)
		}
	}
	session.Options(GetSessionOptions(-1))
	session.Clear()
	if err := session.Save(); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// ListExternalAccounts 获取当前用户的外部帐号绑定列表
func ListExternalAccounts(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	accounts, err := ListExternalAccountsByUserID(c.Request.Context(), userID)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(accounts))
}

// DeleteExternalAccount 解除外部帐号绑定
func DeleteExternalAccount(c *gin.Context) {
	userID := GetUserIDFromContext(c)
	if userID == 0 {
		response.AbortUnauthorized(c, errUnAuthorized)
		return
	}
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, errInvalidExternalAccountBindingID)
		return
	}
	if err := UnbindExternalAccount(c.Request.Context(), id, userID); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
