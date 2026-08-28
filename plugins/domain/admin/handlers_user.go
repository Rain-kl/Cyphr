// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/pkg/idgen"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/response"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	db "github.com/Rain-kl/Wavelet/plugins/infra/database"
)

const minPasswordLength = 8

// listUsersRequest 用户列表查询请求
type listUsersRequest struct {
	Page     int     `form:"page" binding:"min=1"`
	PageSize int     `form:"page_size" binding:"min=1,max=100"`
	UserID   *uint64 `form:"user_id" binding:"omitempty,gt=0"`
	Username string  `form:"username"`
	Email    string  `form:"email"`
}

type userResponse struct {
	ID          uint64    `json:"id,string"`
	Username    string    `json:"username"`
	Nickname    string    `json:"nickname"`
	Email       string    `json:"email"`
	AvatarURL   string    `json:"avatar_url"`
	IsActive    bool      `json:"is_active"`
	IsAdmin     bool      `json:"is_admin"`
	Bio         string    `json:"bio"`
	Phone       string    `json:"phone"`
	Gender      string    `json:"gender"`
	Website     string    `json:"website"`
	Location    string    `json:"location"`
	LastLoginAt time.Time `json:"last_login_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// listUsersResponse 用户列表响应
type listUsersResponse struct {
	Users []userResponse `json:"users"`
	Total int64          `json:"total"`
}

func parseUserID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, userNotFound)
		return 0, false
	}
	return id, true
}

func toUserResponse(u *contracts.UserDTO) userResponse {
	if u == nil {
		return userResponse{}
	}
	return userResponse{
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

func abortUserLogicError(c *gin.Context, err error, notFoundMsg string, forbiddenMsgs, badRequestMsgs []string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.AbortNotFound(c, notFoundMsg)
		return true
	}
	msg := err.Error()
	for _, m := range badRequestMsgs {
		if msg == m {
			response.AbortBadRequest(c, msg)
			return true
		}
	}
	for _, m := range forbiddenMsgs {
		if msg == m {
			response.AbortForbidden(c, msg)
			return true
		}
	}
	logger.ErrorF(c.Request.Context(), "Admin user error: %v", err)
	response.AbortInternal(c, "内部服务器错误")
	return true
}

// ListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 分页返回用户列表，支持按用户 ID 和用户名筛选，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param request query listUsersRequest true "查询参数"
// @Success 200 {object} response.Any{data=listUsersResponse} "用户列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users [get]
func ListUsers(c *gin.Context) {
	var req listUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	total, dtos, err := listUsers(c.Request.Context(), req)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "List admin users failed: %v", err)
		response.AbortInternal(c, "获取用户列表失败")
		return
	}

	users := make([]userResponse, 0, len(dtos))
	for _, dto := range dtos {
		users = append(users, toUserResponse(dto))
	}

	c.JSON(http.StatusOK, response.OK(listUsersResponse{
		Users: users,
		Total: total,
	}))
}

// GetUser 获取用户详情
// @Summary 获取用户详情
// @Description 返回指定用户的完整个人资料和系统状态，需要管理员权限，不返回密码等敏感字段
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Any{data=userResponse} "用户详情"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id} [get]
func GetUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	targetUser, err := getUserDetail(c.Request.Context(), id)
	if abortUserLogicError(c, err, userNotFound, nil, nil) {
		return
	}

	c.JSON(http.StatusOK, response.OK(toUserResponse(targetUser)))
}

// updateUserStatusRequest 更新用户状态请求
type updateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateUserStatus 更新用户状态（启用/禁用）
// @Summary 更新用户状态
// @Description 启用或禁用指定用户，管理员账号无法被禁用，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Param request body updateUserStatusRequest true "状态参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限或尝试禁用管理员"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id}/status [put]
func UpdateUserStatus(c *gin.Context) {
	var req updateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := updateUserStatus(c.Request.Context(), id, req.IsActive); err != nil {
		if abortUserLogicError(c, err, userNotFound, []string{cannotDisable}, nil) {
			return
		}
		response.AbortInternal(c, updateUserFailed)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 删除指定非管理员用户，需要管理员权限，不能删除当前登录用户
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Success 200 {object} response.Any{data=string} "删除成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限、尝试删除管理员或当前用户"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	currUser, _ := util.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	if currUser == nil {
		response.AbortUnauthorized(c, AdminRequired)
		return
	}
	if err := deleteUser(c.Request.Context(), currUser.ID, id); err != nil {
		if abortUserLogicError(c, err, userNotFound, []string{cannotDelete, cannotDeleteSelf}, nil) {
			return
		}
		response.AbortInternal(c, deleteUserFailed)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// createUserRequest 创建用户请求
type createUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=64"`
	Nickname string `json:"nickname" binding:"omitempty,max=64"`
	Email    string `json:"email" binding:"required,email,max=255"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
}

// CreateUser 创建用户
// @Summary 创建用户
// @Description 创建一个本地密码登录的新用户，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body createUserRequest true "创建用户参数"
// @Success 200 {object} response.Any{data=userResponse} "创建成功"
// @Failure 400 {object} response.Any "参数错误或用户名已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users [post]
func CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	newUser, err := createUser(c.Request.Context(), req)
	if abortUserLogicError(c, err, "", nil, []string{usernameRequired, emailRequired, passwordTooShort, usernameExists, emailExists}) {
		return
	}

	c.JSON(http.StatusOK, response.OK(toUserResponse(newUser)))
}

// updateUserRequest 更新用户信息请求
type updateUserRequest struct {
	Nickname string `json:"nickname" binding:"max=64"`
	Email    string `json:"email" binding:"required,email,max=255"`
	IsAdmin  bool   `json:"is_admin"`
	Password string `json:"password" binding:"omitempty,min=8,max=64"`
}

// UpdateUser 更新用户信息
// @Summary 更新用户信息
// @Description 更新指定用户的昵称、邮箱、管理员权限，并可选重置密码，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Param request body updateUserRequest true "更新参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限或尝试修改自身权限"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id} [put]
func UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	currUser, _ := util.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	if currUser == nil {
		response.AbortUnauthorized(c, AdminRequired)
		return
	}
	err := updateUser(c.Request.Context(), currUser.ID, updateUserParam{
		ID:       id,
		Nickname: req.Nickname,
		Email:    req.Email,
		IsAdmin:  req.IsAdmin,
		Password: req.Password,
	})

	if err != nil {
		if abortUserLogicError(c, err, userNotFound, []string{cannotRevokeSelfAdmin}, []string{emailRequired, emailExists, passwordTooShort}) {
			return
		}
		response.AbortInternal(c, updateUserInfoFailed)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

func listUsers(ctx context.Context, req listUsersRequest) (int64, []*contracts.UserDTO, error) {
	query := db.DB(ctx).Table("w_users")
	if req.UserID != nil {
		query = query.Where("id = ?", *req.UserID)
	}
	if req.Username != "" {
		query = query.Where("username LIKE ? ESCAPE '\\'", util.EscapeLike(req.Username)+"%")
	}
	if req.Email != "" {
		query = query.Where("email LIKE ? ESCAPE '\\'", util.EscapeLike(req.Email)+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var users []*contracts.UserDTO
	offset := (req.Page - 1) * req.PageSize
	if err := query.
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, last_login_at, created_at, updated_at").
		Order("id ASC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&users).Error; err != nil {
		return 0, nil, err
	}
	return total, users, nil
}

func getUserDetail(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	var user contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").
		Select("id, username, nickname, email, avatar_url, is_active, is_admin, bio, phone, gender, website, location, last_login_at, created_at, updated_at").
		Where("id = ?", id).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func updateUserStatus(ctx context.Context, id uint64, active bool) error {
	var flags struct {
		ID      uint64
		IsAdmin bool
	}
	if err := db.DB(ctx).Table("w_users").Select("id, is_admin").Where("id = ?", id).First(&flags).Error; err != nil {
		return err
	}
	if !active && flags.IsAdmin {
		return errors.New(cannotDisable)
	}

	var tokenHashes []string
	if !active {
		_ = db.DB(ctx).Table("w_access_tokens").Where("user_id = ?", id).Pluck("token_hash", &tokenHashes).Error
	}

	err := db.DB(ctx).Table("w_users").Where("id = ?", id).Update("is_active", active).Error
	if err == nil {
		auth.InvalidateCachedUser(ctx, id)
		if !active {
			for _, hash := range tokenHashes {
				auth.InvalidateCachedToken(ctx, hash)
			}
		}
	}
	return err
}

func deleteUser(ctx context.Context, currentUserID, targetID uint64) error {
	if currentUserID == targetID {
		return errors.New(cannotDeleteSelf)
	}
	var flags struct {
		ID      uint64
		IsAdmin bool
	}
	if err := db.DB(ctx).Table("w_users").Select("id, is_admin").Where("id = ?", targetID).First(&flags).Error; err != nil {
		return err
	}
	if flags.IsAdmin {
		return errors.New(cannotDelete)
	}

	var tokenHashes []string
	_ = db.DB(ctx).Table("w_access_tokens").Where("user_id = ?", targetID).Pluck("token_hash", &tokenHashes).Error

	err := db.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("w_access_tokens").Where("user_id = ?", targetID).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		if err := tx.Table("w_external_accounts").Where("user_id = ?", targetID).Delete(map[string]any{}).Error; err != nil {
			return err
		}
		return tx.Table("w_users").Where("id = ?", targetID).Delete(map[string]any{}).Error
	})
	if err == nil {
		auth.InvalidateCachedUser(ctx, targetID)
		for _, hash := range tokenHashes {
			auth.InvalidateCachedToken(ctx, hash)
		}
	}
	return err
}

func createUser(ctx context.Context, req createUserRequest) (*contracts.UserDTO, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return nil, errors.New(usernameRequired)
	}
	if req.Email == "" {
		return nil, errors.New(emailRequired)
	}
	if len(req.Password) < minPasswordLength {
		return nil, errors.New(passwordTooShort)
	}

	var count int64
	if err := db.DB(ctx).Table("w_users").Where("username = ?", req.Username).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New(usernameExists)
	}

	var emailCount int64
	if err := db.DB(ctx).Table("w_users").Where("email = ?", req.Email).Count(&emailCount).Error; err != nil {
		return nil, err
	}
	if emailCount > 0 {
		return nil, errors.New(emailExists)
	}

	hash, err := util.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	if req.Nickname == "" {
		req.Nickname = req.Username
	}

	now := time.Now()
	newUser := contracts.UserDTO{
		ID:        idgen.NextUint64ID(),
		Username:  req.Username,
		Nickname:  req.Nickname,
		Email:     req.Email,
		IsActive:  req.IsActive,
		IsAdmin:   req.IsAdmin,
		CreatedAt: now,
		UpdatedAt: now,
	}

	row := map[string]any{
		"id":         newUser.ID,
		"username":   newUser.Username,
		"password":   hash,
		"nickname":   newUser.Nickname,
		"email":      newUser.Email,
		"is_active":  newUser.IsActive,
		"is_admin":   newUser.IsAdmin,
		"created_at": now,
		"updated_at": now,
	}
	if err := db.DB(ctx).Table("w_users").Create(row).Error; err != nil {
		return nil, err
	}
	return &newUser, nil
}

type updateUserParam struct {
	ID       uint64
	Nickname string
	Email    string
	IsAdmin  bool
	Password string
}

func updateUser(ctx context.Context, currentUserID uint64, param updateUserParam) error {
	param.Nickname = strings.TrimSpace(param.Nickname)
	param.Email = strings.TrimSpace(param.Email)
	param.Password = strings.TrimSpace(param.Password)

	if param.Email == "" {
		return errors.New(emailRequired)
	}

	var targetUser contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").Where("id = ?", param.ID).First(&targetUser).Error; err != nil {
		return err
	}

	if currentUserID == param.ID && !param.IsAdmin && targetUser.IsAdmin {
		return errors.New(cannotRevokeSelfAdmin)
	}

	if targetUser.Email != param.Email {
		var count int64
		if err := db.DB(ctx).Table("w_users").Where("email = ? AND id != ?", param.Email, param.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New(emailExists)
		}
	}

	if param.Password != "" && len(param.Password) < minPasswordLength {
		return errors.New(passwordTooShort)
	}

	needRevokeTokens := (param.Password != "") || (targetUser.IsAdmin && !param.IsAdmin)
	var tokenHashes []string
	if needRevokeTokens {
		_ = db.DB(ctx).Table("w_access_tokens").Where("user_id = ?", param.ID).Pluck("token_hash", &tokenHashes).Error
	}

	if param.Nickname == "" {
		param.Nickname = targetUser.Username
	}

	updates := map[string]any{
		"nickname":   param.Nickname,
		"email":      param.Email,
		"is_admin":   param.IsAdmin,
		"updated_at": time.Now(),
	}
	if param.Password != "" {
		hash, err := util.HashPassword(param.Password)
		if err != nil {
			return err
		}
		updates["password"] = hash
	}

	err := db.DB(ctx).Table("w_users").Where("id = ?", param.ID).Updates(updates).Error
	if err == nil {
		auth.InvalidateCachedUser(ctx, param.ID)
		if needRevokeTokens {
			for _, hash := range tokenHashes {
				auth.InvalidateCachedToken(ctx, hash)
			}
		}
	}
	return err
}
