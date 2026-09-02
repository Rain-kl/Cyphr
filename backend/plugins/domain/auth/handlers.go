// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GetLoginSources 获取可用登录源列表
// @Summary 获取可用登录源
// @Description 返回当前系统已启用的所有 OAuth 登录源，前端展示登录按钮列表时调用
// @Tags oauth
// @Produce json
// @Success 200 {object} response.Any{data=[]auth.AuthSourceView} "登录源列表"
// @Router /api/v1/oauth/sources [get]
func GetLoginSources(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(activeLoginSources(c.Request.Context())))
}

// GetLoginURL 获取登录授权地址
// @Summary 获取登录授权地址
// @Description 根据指定认证源生成 OAuth 授权 URL，前端跳转到该 URL 完成 OAuth 登录授权。source 参数为空时使用第一个启用的认证源。
// @Tags oauth
// @Produce json
// @Param source query string false "认证源名称，为空使用第一个启用的认证源"
// @Success 200 {object} response.Any{data=auth.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未配置"
// @Failure 500 {object} response.Any "构造 URL 失败"
// @Router /api/v1/oauth/login [get]
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
	stateKey := fmt.Sprintf(OAuthStateCacheKeyFormat, state)
	if cache := getCache(ctx); cache != nil {
		if err := cache.Set(ctx, stateKey, payloadValue, OAuthStateCacheKeyExpiration); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
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
	if sessionHash == "" {
		return nil
	}
	if limiter := getLimiter(ctx); limiter != nil {
		key := fmt.Sprintf(oauthStateLimitKeyFormat, sessionHash)
		res, err := limiter.Allow(ctx, key, contracts.Rate{
			Limit:  oauthStateLimitMax,
			Period: OAuthStateCacheKeyExpiration,
		})
		if err != nil {
			return err
		}
		if !res.Allowed {
			return errors.New(errOAuthStateRateLimited)
		}
		return nil
	}

	cache := getCache(ctx)
	if cache == nil {
		return nil
	}
	key := fmt.Sprintf(oauthStateLimitKeyFormat, sessionHash)
	var count int
	_ = cache.Get(ctx, key, &count)
	count++
	_ = cache.Set(ctx, key, count, OAuthStateCacheKeyExpiration)
	if count > oauthStateLimitMax {
		return errors.New(errOAuthStateRateLimited)
	}
	return nil
}

// Authorize 发起指定认证源授权
// @Summary 发起指定认证源授权
// @Description 根据指定认证源名称发起 OAuth 授权，支持 purpose 参数用于区分登录和账号绑定场景。认证源必须已启用。
// @Tags oauth
// @Produce json
// @Param source path string true "认证源名称"
// @Param purpose query string false "授权目的：login（登录）或 bind（绑定账号），默认 login"
// @Success 200 {object} response.Any{data=auth.OAuthAuthorizeResponse} "授权 URL"
// @Failure 400 {object} response.Any "认证源不存在或未启用"
// @Failure 500 {object} response.Any "构造 URL 失败"
// @Router /api/v1/oauth/{source}/authorize [get]
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
	stateKey := fmt.Sprintf(OAuthStateCacheKeyFormat, state)
	if cache := getCache(ctx); cache != nil {
		if err := cache.Set(ctx, stateKey, payloadValue, OAuthStateCacheKeyExpiration); err != nil {
			response.AbortInternal(c, err.Error())
			return
		}
	}

	authorizeURL, err := buildAuthorizeURL(c.Request.Context(), source, state)
	if err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, response.OK(OAuthAuthorizeResponse{AuthorizeURL: authorizeURL}))
}

// Callback OAuth 回调处理
// @Summary OAuth 回调处理
// @Description 接收前端传回的 state 和 code，完成 OAuth/OIDC 认证并建立会话。支持登录（login）和账号绑定（bind）两种场景。
// @Tags oauth
// @Accept json
// @Produce json
// @Param request body auth.CallbackRequest true "回调请求参数"
// @Success 200 {object} response.Any{data=auth.OAuthCallbackResult} "登录或绑定成功"
// @Failure 400 {object} response.Any "state 无效、参数错误或认证源错误"
// @Failure 401 {object} response.Any "绑定场景未登录"
// @Failure 500 {object} response.Any "OAuth 认证失败或内部错误"
// @Router /api/v1/oauth/callback [post]
func Callback(c *gin.Context) {
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	stateKey := fmt.Sprintf(OAuthStateCacheKeyFormat, req.State)
	var payloadRaw string
	cache := getCache(ctx)
	if cache == nil {
		response.AbortBadRequest(c, errInvalidState)
		return
	}
	if err := cache.Get(ctx, stateKey, &payloadRaw); err != nil {
		response.AbortBadRequest(c, errInvalidState)
		return
	}
	_ = cache.Delete(ctx, stateKey)

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
		response.AbortBadRequest(c, errInvalidSessionContext)
		return
	}

	if hashSessionToken(token) != payload.SessionHash {
		response.AbortBadRequest(c, errSessionMismatchForOAuth)
		return
	}

	if payload.Purpose == OAuthPurposeBind && currentUserID != payload.UserID {
		response.AbortBadRequest(c, errUserContextMismatch)
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
	user, err := GetUserByID(ctx, userID)
	if err != nil {
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
	_ = TouchUserLastLogin(ctx, user.ID, user.LastLoginAt)
	c.JSON(http.StatusOK, response.OK(buildCallbackResult(user, "bound")))
}

func handleCallbackLogin(ctx context.Context, c *gin.Context, source *AuthSource, userInfo *contracts.OAuthUserInfoDTO) {
	var user *contracts.UserDTO

	account, err := FindExternalAccount(ctx, source.ID, userInfo.Sub)
	switch {
	case err == nil:
		loaded, loadErr := GetUserByID(ctx, account.UserID)
		if loadErr != nil {
			response.AbortInternal(c, loadErr.Error())
			return
		}
		user = loaded
	case errors.Is(err, gorm.ErrRecordNotFound):
		newUser, ok := handleCallbackRegister(ctx, c, source, userInfo)
		if !ok {
			return
		}
		user = &newUser
	default:
		response.AbortInternal(c, err.Error())
		return
	}

	user.LastLoginAt = time.Now()
	_ = TouchUserLastLogin(ctx, user.ID, user.LastLoginAt)
	if err := SetLoginSession(ctx, c, user); err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	SetCachedUser(ctx, user.ID, user)

	c.JSON(http.StatusOK, response.OK(buildCallbackResult(user, "logged_in")))
}

func uniqueUsername(ctx context.Context, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "user"
	}

	existingUsernames, err := ListSimilarUsernames(ctx, base)
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

func handleCallbackRegister(ctx context.Context, c *gin.Context, source *AuthSource, userInfo *contracts.OAuthUserInfoDTO) (contracts.UserDTO, bool) {
	registrationEnabled := true
	val, cfgErr := GetSystemConfigValue(ctx, "registration_enabled")
	if cfgErr == nil && val != "" {
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

	if err := InsertUser(ctx, &user); err != nil {
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
// @Summary 获取当前登录用户信息
// @Description 返回当前登录用户的基本信息，需要登录。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=auth.BasicUserInfo} "用户信息"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/oauth/user-info [get]
// @Router /api/v1/user-info [get]
func UserInfo(c *gin.Context) {
	user, _ := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	session := sessions.Default(c)
	needChange := session.Get("need_change_password") == true || (user != nil && user.NeedChangePassword)

	c.JSON(
		http.StatusOK,
		response.OK(BuildBasicUserInfo(user, needChange)),
	)
}

// Logout 退出登录
// @Summary 退出登录
// @Description 清除当前用户的登录会话，完成退出。清除 Cookie 中的 Session 数据。
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any{data=string} "退出成功"
// @Failure 500 {object} response.Any "Session 清除失败"
// @Router /api/v1/oauth/logout [get]
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
// @Summary 获取外部帐号列表
// @Description 返回当前登录用户已绑定的所有外部 OAuth 帐号信息，需要登录
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} response.Any "外部帐号列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/oauth/external-accounts [get]
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
// @Summary 解除外部帐号绑定
// @Description 解除当前登录用户与指定外部帐号的绑定关系，需要登录
// @Tags oauth
// @Produce json
// @Security SessionCookie
// @Param id path uint64 true "外部帐号绑定记录 ID"
// @Success 200 {object} response.Any{data=string} "解除绑定成功"
// @Failure 400 {object} response.Any "ID 无效或解除失败"
// @Failure 401 {object} response.Any "未登录"
// @Router /api/v1/oauth/external-accounts/{id}/delete [post]
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
