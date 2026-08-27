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

	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
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

func toUserResponse(u model.User) userResponse {
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

	total, modelUsers, err := listUsers(c.Request.Context(), req)
	if err != nil {
		logger.ErrorF(c.Request.Context(), "List admin users failed: %v", err)
		response.AbortInternal(c, "获取用户列表失败")
		return
	}

	users := make([]userResponse, 0, len(modelUsers))
	for _, modelUser := range modelUsers {
		users = append(users, toUserResponse(modelUser))
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

	currUser, _ := auth.GetFromContext[*model.User](c, auth.UserObjKey)
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

	currUser, _ := auth.GetFromContext[*model.User](c, auth.UserObjKey)
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

func listUsers(ctx context.Context, req listUsersRequest) (int64, []model.User, error) {
	return repository.ListAdminUsers(ctx, repository.AdminUserListFilter{
		UserID:   req.UserID,
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

func getUserDetail(ctx context.Context, id uint64) (model.User, error) {
	return repository.GetAdminUserDetail(ctx, id)
}

func updateUserStatus(ctx context.Context, id uint64, active bool) error {
	flags, err := repository.GetUserAdminFlags(ctx, id)
	if err != nil {
		return err
	}
	if !active && flags.IsAdmin {
		return errors.New(cannotDisable)
	}

	var tokens []model.AccessToken
	if !active {
		tokens, _ = repository.ListAccessTokensByUserID(ctx, id)
	}

	err = repository.UpdateUserActive(ctx, id, active)
	if err == nil {
		auth.InvalidateCachedUser(ctx, id)
		if !active {
			for _, token := range tokens {
				auth.InvalidateCachedToken(ctx, token.TokenHash)
			}
		}
	}
	return err
}

func deleteUser(ctx context.Context, currentUserID, targetID uint64) error {
	if currentUserID == targetID {
		return errors.New(cannotDeleteSelf)
	}
	flags, err := repository.GetUserAdminFlags(ctx, targetID)
	if err != nil {
		return err
	}
	if flags.IsAdmin {
		return errors.New(cannotDelete)
	}

	tokens, _ := repository.ListAccessTokensByUserID(ctx, targetID)

	err = repository.DeleteUserWithRelations(ctx, targetID)
	if err == nil {
		auth.InvalidateCachedUser(ctx, targetID)
		for _, token := range tokens {
			auth.InvalidateCachedToken(ctx, token.TokenHash)
		}
	}
	return err
}

func createUser(ctx context.Context, req createUserRequest) (model.User, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)

	if req.Username == "" {
		return model.User{}, errors.New(usernameRequired)
	}
	if req.Email == "" {
		return model.User{}, errors.New(emailRequired)
	}
	if len(req.Password) < minPasswordLength {
		return model.User{}, errors.New(passwordTooShort)
	}

	count, err := repository.CountUsersByUsername(ctx, req.Username)
	if err != nil {
		return model.User{}, err
	}
	if count > 0 {
		return model.User{}, errors.New(usernameExists)
	}

	emailCount, err := repository.CountUsersByEmail(ctx, req.Email)
	if err != nil {
		return model.User{}, err
	}
	if emailCount > 0 {
		return model.User{}, errors.New(emailExists)
	}

	newUser := model.User{
		ID:          idgen.NextUint64ID(),
		Username:    req.Username,
		Nickname:    req.Nickname,
		Email:       req.Email,
		IsActive:    req.IsActive,
		IsAdmin:     req.IsAdmin,
		LastLoginAt: time.Time{},
	}
	if newUser.Nickname == "" {
		newUser.Nickname = req.Username
	}
	if err := newUser.SetEncryptedPassword(req.Password); err != nil {
		return model.User{}, err
	}
	if err := repository.CreateUser(ctx, &newUser); err != nil {
		return model.User{}, err
	}
	return newUser, nil
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

	targetUser, err := repository.GetAdminUserDetail(ctx, param.ID)
	if err != nil {
		return err
	}

	// 不能撤销当前登录用户的管理员权限
	if currentUserID == param.ID && !param.IsAdmin && targetUser.IsAdmin {
		return errors.New(cannotRevokeSelfAdmin)
	}

	// 如果修改了邮箱，检查邮箱是否被其他用户占用
	if targetUser.Email != param.Email {
		count, err := repository.CountUsersByEmail(ctx, param.Email)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New(emailExists)
		}
	}

	// 密码强度校验（如果输入了新密码）
	if param.Password != "" && len(param.Password) < minPasswordLength {
		return errors.New(passwordTooShort)
	}

	needRevokeTokens := (param.Password != "") || (targetUser.IsAdmin && !param.IsAdmin)
	var tokens []model.AccessToken
	if needRevokeTokens {
		tokens, _ = repository.ListAccessTokensByUserID(ctx, param.ID)
	}

	targetUser.Nickname = param.Nickname
	if targetUser.Nickname == "" {
		targetUser.Nickname = targetUser.Username
	}
	targetUser.Email = param.Email
	targetUser.IsAdmin = param.IsAdmin

	if param.Password != "" {
		if err := targetUser.SetEncryptedPassword(param.Password); err != nil {
			return err
		}
	}

	err = repository.UpdateUser(ctx, &targetUser)
	if err == nil {
		auth.InvalidateCachedUser(ctx, param.ID)
		if needRevokeTokens {
			for _, token := range tokens {
				auth.InvalidateCachedToken(ctx, token.TokenHash)
			}
		}
	}
	return err
}
