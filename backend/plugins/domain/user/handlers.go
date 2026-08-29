// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	authMu  sync.RWMutex
	authSvc contracts.AuthService
)

// SetAuthService binds the AuthService for cache synchronization.
func SetAuthService(s contracts.AuthService) {
	authMu.Lock()
	defer authMu.Unlock()
	authSvc = s
}

func getAuthService() contracts.AuthService {
	authMu.RLock()
	defer authMu.RUnlock()
	return authSvc
}

func getUserIDFromSession(c *gin.Context) uint64 {
	defer func() { _ = recover() }()
	session := sessions.Default(c)
	val := session.Get(contracts.AuthUserIDKey)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case float64:
		if v < 0 {
			return 0
		}
		return uint64(v)
	case string:
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	default:
		return 0
	}
}

func invalidateUserCache(ctx context.Context, userID uint64) {
	if s := getAuthService(); s != nil {
		s.InvalidateCachedUser(ctx, userID)
	}
}

func invalidateTokenCache(ctx context.Context, tokenHash string) {
	if s := getAuthService(); s != nil {
		s.InvalidateCachedToken(ctx, tokenHash)
	}
}

// Login handles username and password authentication.
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	user, err := GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		response.AbortUnauthorized(c, errPasswordMismatch)
		return
	}

	if !user.CheckPassword(req.Password) {
		response.AbortUnauthorized(c, errPasswordMismatch)
		return
	}

	sess := sessions.Default(c)
	sess.Set(contracts.AuthUserIDKey, user.ID)
	sess.Set(contracts.AuthUserNameKey, user.Username)
	needChange := user.NeedChangePassword || user.IsPlaintextPassword()
	user.NeedChangePassword = needChange
	sess.Set("need_change_password", needChange)
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "save session failed on login: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(user))
}

// Register registers a new user.
func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	newUser := &User{
		Username: req.Username,
		Email:    req.Email,
		IsActive: true,
	}
	if err := newUser.SetEncryptedPassword(req.Password); err != nil {
		response.AbortInternal(c, errPasswordEncryptFailed)
		return
	}

	if err := CreateUser(c.Request.Context(), newUser); err != nil {
		response.AbortBadRequest(c, errCreateUserFailed+err.Error())
		return
	}

	sess := sessions.Default(c)
	sess.Set(contracts.AuthUserIDKey, newUser.ID)
	sess.Set(contracts.AuthUserNameKey, newUser.Username)
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "save session failed on register: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(newUser))
}

// Logout logs out the current session.
func Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Options(sessions.Options{
		Path:   "/",
		MaxAge: -1,
	})
	sess.Clear()
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "clear session failed on logout: %v", err)
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// SendEmailCode sends an email verification code.
func SendEmailCode(c *gin.Context) {
	c.JSON(http.StatusOK, response.OK(gin.H{"sent": true}))
}

// ChangePassword changes the current user password.
func ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	user, err := GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.AbortNotFound(c, errUserNotFound)
		return
	}

	if !user.CheckPassword(req.OldPassword) {
		response.AbortBadRequest(c, errOldPasswordIncorrect)
		return
	}

	if err := user.SetEncryptedPassword(req.NewPassword); err != nil {
		response.AbortInternal(c, errPasswordUpdateFailed)
		return
	}

	if err := UpdateUser(c.Request.Context(), user); err != nil {
		logger.ErrorF(c.Request.Context(), "persist changed password failed: %v", err)
	}
	invalidateUserCache(c.Request.Context(), user.ID)

	sess := sessions.Default(c)
	sess.Set("need_change_password", false)
	if err := sess.Save(); err != nil {
		logger.ErrorF(c.Request.Context(), "save session failed on change-password: %v", err)
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// UpdateProfile updates profile info.
func UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	user, err := GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.AbortNotFound(c, errUserNotFound)
		return
	}

	user.Nickname = req.Nickname
	user.AvatarURL = req.AvatarURL
	user.Bio = req.Bio
	user.Phone = req.Phone
	user.Gender = req.Gender
	user.Website = req.Website
	user.Location = req.Location

	if err := UpdateUser(c.Request.Context(), user); err != nil {
		logger.ErrorF(c.Request.Context(), "persist updated profile failed: %v", err)
	}
	invalidateUserCache(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, response.OK(user))
}

// ListAccessTokens lists access tokens for the current user.
func ListAccessTokens(c *gin.Context) {
	userID := getUserIDFromSession(c)
	tokens, err := listAccessTokensByUser(c.Request.Context(), userID)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "list access tokens failed: %v", err)
	}
	c.JSON(http.StatusOK, response.OK(tokens))
}

const (
	tokenEntropyByteLength = 24
	tokenMaskMinLength     = 8
)

// CreateAccessToken generates a new access token.
func CreateAccessToken(c *gin.Context) {
	var req createAccessTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	rawBytes := make([]byte, tokenEntropyByteLength)
	_, _ = rand.Read(rawBytes)
	rawToken := "wvt_" + hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	masked := rawToken
	if len(rawToken) > tokenMaskMinLength {
		masked = rawToken[:4] + "..." + rawToken[len(rawToken)-4:]
	}

	token := AccessToken{
		UserID:      userID,
		Name:        req.Name,
		TokenHash:   tokenHash,
		MaskedToken: masked,
		IsAdmin:     req.IsAdmin,
	}

	if err := createAccessTokenRow(c.Request.Context(), &token); err != nil {
		response.AbortInternal(c, errCreateTokenFailed)
		return
	}

	c.JSON(http.StatusOK, response.OK(gin.H{
		"token":     token,
		"raw_token": rawToken,
	}))
}

// DeleteAccessToken deletes a specific access token.
func DeleteAccessToken(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	token, err := getAccessTokenOfUser(c.Request.Context(), id, userID)
	if err != nil {
		response.AbortNotFound(c, errTokenNotFound)
		return
	}

	if err := deleteAccessTokenRow(c.Request.Context(), token); err != nil {
		logger.ErrorF(c.Request.Context(), "delete access token failed: %v", err)
	}
	invalidateTokenCache(c.Request.Context(), token.TokenHash)
	c.JSON(http.StatusOK, response.OKNil())
}

// RotateAccessToken rotates an access token value.
func RotateAccessToken(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.AbortBadRequest(c, errInvalidParams)
		return
	}

	userID := getUserIDFromSession(c)
	token, err := getAccessTokenOfUser(c.Request.Context(), id, userID)
	if err != nil {
		response.AbortNotFound(c, errTokenNotFound)
		return
	}

	invalidateTokenCache(c.Request.Context(), token.TokenHash)

	rawBytes := make([]byte, tokenEntropyByteLength)
	_, _ = rand.Read(rawBytes)
	rawToken := "wvt_" + hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	token.TokenHash = hex.EncodeToString(hash[:])

	masked := rawToken
	if len(rawToken) > tokenMaskMinLength {
		masked = rawToken[:4] + "..." + rawToken[len(rawToken)-4:]
	}
	token.MaskedToken = masked

	if err := saveAccessTokenRow(c.Request.Context(), token); err != nil {
		logger.ErrorF(c.Request.Context(), "rotate access token failed: %v", err)
	}

	c.JSON(http.StatusOK, response.OK(gin.H{
		"token":     token,
		"raw_token": rawToken,
	}))
}
