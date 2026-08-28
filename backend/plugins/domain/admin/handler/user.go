// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/ginutil"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/response"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func parseUserID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.AbortBadRequest(c, errs.UserNotFound)
		return 0, false
	}
	return id, true
}

// abortUserLogicError maps the user service outcome onto the unified response envelope.
func abortUserLogicError(c *gin.Context, err error, notFoundMsg string, forbiddenMsgs, badRequestMsgs []string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errs.ErrUserServiceUnavailable) {
		response.AbortInternal(c, err.Error())
		return true
	}
	if errors.Is(err, errs.ErrUserNotFound) {
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
	response.AbortInternal(c, errs.InternalServerError)
	return true
}

// ListUsers 获取用户列表
// @Summary 获取用户列表
// @Description 分页返回用户列表，支持按用户 ID 和用户名筛选，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param request query model.ListUsersRequest true "查询参数"
// @Success 200 {object} response.Any{data=model.ListUsersResponse} "用户列表"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users [get]
func ListUsers(c *gin.Context) {
	var req model.ListUsersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	total, dtos, err := service.AdminListUsers(c.Request.Context(), contracts.AdminListUsersFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
		UserID:   req.UserID,
		Username: req.Username,
		Email:    req.Email,
	})
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	users := make([]model.UserResponse, 0, len(dtos))
	for _, dto := range dtos {
		users = append(users, service.ToUserResponse(dto))
	}

	c.JSON(http.StatusOK, response.OK(model.ListUsersResponse{
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
// @Success 200 {object} response.Any{data=model.UserResponse} "用户详情"
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

	targetUser, err := service.AdminGetUser(c.Request.Context(), id)
	if abortUserLogicError(c, err, errs.UserNotFound, nil, nil) {
		return
	}

	c.JSON(http.StatusOK, response.OK(service.ToUserResponse(targetUser)))
}

// UpdateUserStatus 更新用户状态（启用/禁用）
// @Summary 更新用户状态
// @Description 启用或禁用指定用户，管理员账号无法被禁用，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Param request body model.UpdateUserStatusRequest true "状态参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限或尝试禁用管理员"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id}/status [put]
func UpdateUserStatus(c *gin.Context) {
	var req model.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := service.AdminUpdateUserStatus(c.Request.Context(), id, req.IsActive); err != nil {
		if abortUserLogicError(c, err, errs.UserNotFound, []string{errs.CannotDisable}, nil) {
			return
		}
		response.AbortInternal(c, errs.UpdateUserFailed)
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

	currUser, _ := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	if currUser == nil {
		response.AbortUnauthorized(c, errs.AdminRequired)
		return
	}

	if err := service.AdminDeleteUser(c.Request.Context(), currUser.ID, id); err != nil {
		if abortUserLogicError(c, err, errs.UserNotFound, []string{errs.CannotDelete, errs.CannotDeleteSelf}, nil) {
			return
		}
		response.AbortInternal(c, errs.DeleteUserFailed)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// CreateUser 创建用户
// @Summary 创建用户
// @Description 创建一个本地密码登录的新用户，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body model.CreateUserRequest true "创建用户参数"
// @Success 200 {object} response.Any{data=model.UserResponse} "创建成功"
// @Failure 400 {object} response.Any "参数错误或用户名已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users [post]
func CreateUser(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	newUser, err := service.AdminCreateUser(c.Request.Context(), contracts.AdminCreateUserRequest{
		Username: req.Username,
		Password: req.Password,
		Nickname: req.Nickname,
		Email:    req.Email,
		IsActive: req.IsActive,
		IsAdmin:  req.IsAdmin,
	})
	if abortUserLogicError(c, err, "", nil, []string{errs.UsernameRequired, errs.EmailRequired, errs.PasswordTooShort, errs.UsernameExists, errs.EmailExists}) {
		return
	}

	c.JSON(http.StatusOK, response.OK(service.ToUserResponse(newUser)))
}

// UpdateUser 更新用户信息
// @Summary 更新用户信息
// @Description 更新指定用户的昵称、邮箱、管理员权限，并可选重置密码，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "用户 ID"
// @Param request body model.UpdateUserRequest true "更新参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限或尝试修改自身权限"
// @Failure 404 {object} response.Any "用户不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/users/{id} [put]
func UpdateUser(c *gin.Context) {
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	currUser, _ := ginutil.GetFromContext[*contracts.UserDTO](c, contracts.AuthUserObjKey)
	if currUser == nil {
		response.AbortUnauthorized(c, errs.AdminRequired)
		return
	}

	err := service.AdminUpdateUser(c.Request.Context(), currUser.ID, contracts.AdminUpdateUserRequest{
		ID:       id,
		Nickname: req.Nickname,
		Email:    req.Email,
		IsAdmin:  req.IsAdmin,
		Password: req.Password,
	})
	if err != nil {
		if abortUserLogicError(c, err, errs.UserNotFound, []string{errs.CannotRevokeSelfAdmin}, []string{errs.EmailRequired, errs.EmailExists, errs.PasswordTooShort}) {
			return
		}
		response.AbortInternal(c, errs.UpdateUserInfoFailed)
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}
