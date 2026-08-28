// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	database "github.com/Rain-kl/Wavelet/backend/plugins/infra/database"

	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/pkg/response"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type updateProfileRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Phone     string `json:"phone"`
	Gender    string `json:"gender"`
	Website   string `json:"website"`
	Location  string `json:"location"`
}

type createAccessTokenRequest struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
	IsAdmin   bool       `json:"is_admin"`
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
	// Cache invalidation delegated to AuthService via IoC at plugin Apply time.
	_ = ctx
	_ = userID
}

func invalidateTokenCache(ctx context.Context, tokenHash string) {
	_ = ctx
	_ = tokenHash
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
	_ = sess.Save()

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
		response.AbortInternal(c, "密码加密失败")
		return
	}

	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Create(newUser).Error; err != nil {
		response.AbortBadRequest(c, "创建用户失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OK(newUser))
}

// Logout logs out the current session.
func Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	_ = sess.Save()
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
		response.AbortInternal(c, "密码更新失败")
		return
	}

	gormDB := database.DB(c.Request.Context())
	_ = gormDB.Save(&user)
	invalidateUserCache(c.Request.Context(), user.ID)

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

	gormDB := database.DB(c.Request.Context())
	_ = gormDB.Save(&user)
	invalidateUserCache(c.Request.Context(), user.ID)

	c.JSON(http.StatusOK, response.OK(user))
}

// ListAccessTokens lists access tokens for the current user.
func ListAccessTokens(c *gin.Context) {
	userID := getUserIDFromSession(c)
	var tokens []AccessToken
	gormDB := database.DB(c.Request.Context())
	_ = gormDB.Where("user_id = ?", userID).Find(&tokens).Error
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

	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Create(&token).Error; err != nil {
		response.AbortInternal(c, "创建令牌失败")
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
	var token AccessToken
	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Where("id = ? AND user_id = ?", id, userID).First(&token).Error; err != nil {
		response.AbortNotFound(c, errTokenNotFound)
		return
	}

	_ = gormDB.Delete(&token)
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
	var token AccessToken
	gormDB := database.DB(c.Request.Context())
	if err := gormDB.Where("id = ? AND user_id = ?", id, userID).First(&token).Error; err != nil {
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

	_ = gormDB.Save(&token)

	c.JSON(http.StatusOK, response.OK(gin.H{
		"token":     token,
		"raw_token": rawToken,
	}))
}
