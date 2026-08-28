// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	mail "Wavelet/pkg/mail"
	"Wavelet/pkg/response"
)

const maskedConfigValue = "******"

// CreateSystemConfigRequest 创建系统配置请求
type CreateSystemConfigRequest struct {
	Key         string `json:"key" binding:"required,max=64"`
	Value       string `json:"value" binding:"required"`
	Type        string `json:"type" binding:"required,oneof=system business"`
	Visibility  int    `json:"visibility" binding:"oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// UpdateSystemConfigRequest 更新系统配置请求
type UpdateSystemConfigRequest struct {
	Value       string `json:"value" binding:"required"`
	Visibility  *int   `json:"visibility" binding:"omitempty,oneof=0 1"`
	Description string `json:"description" binding:"max=255"`
}

// GetPublicConfig 获取公共配置
// @Summary 获取公共配置
// @Description 返回系统配置表中 visibility 为 1 的配置键值集合
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} response.Any
// @Router /api/v1/config/public [get]
func GetPublicConfig(c *gin.Context) {
	ctx := c.Request.Context()
	configs, err := ListVisibleSystemConfigs(ctx)
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	resp := make(map[string]string, len(configs))
	for _, config := range configs {
		resp[config.Key] = config.Value
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

// GetRobotsTXT 动态生成 robots.txt
// @Summary 获取 robots.txt
// @Description 根据系统配置决定是否允许搜索引擎检索，并返回相应的 robots.txt 文件内容
// @Tags config
// @Produce text/plain
// @Success 200 {string} string "robots.txt 内容"
// @Router /robots.txt [get]
func GetRobotsTXT(c *gin.Context) {
	ctx := c.Request.Context()
	enabled, err := GetBoolByKey(ctx, ConfigKeySearchEngineIndexingEnabled)
	content := "User-Agent: *\nDisallow: /\n"
	if err == nil && enabled {
		content = "User-Agent: *\nAllow: /\n"
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// CreateSystemConfig 创建系统配置
// @Summary 创建系统配置
// @Description 创建一条新的系统配置项，配置键不可重复，同时将新配置同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body CreateSystemConfigRequest true "创建请求参数"
// @Success 200 {object} response.Any{data=string} "创建成功"
// @Failure 400 {object} response.Any "参数错误或配置键已存在"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [post]
func CreateSystemConfig(c *gin.Context) {
	var req CreateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}
	if isProtectedConfigKey(req.Key) {
		response.AbortBadRequest(c, protectedConfigKeyMessage)
		return
	}

	if err := createSystemConfig(c.Request.Context(), req); err != nil {
		if err.Error() == ConfigKeyExists {
			response.AbortBadRequest(c, ConfigKeyExists)
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

// ListSystemConfigs 获取系统配置列表
// @Summary 获取系统配置列表
// @Description 返回所有系统配置列表，支持按配置类型（system/business）过滤，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param type query string false "配置类型（system/business）"
// @Success 200 {object} response.Any{data=[]SystemConfig} "系统配置列表"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs [get]
func ListSystemConfigs(c *gin.Context) {
	configs, err := listSystemConfigs(c.Request.Context(), c.Query("type"))
	if err != nil {
		response.AbortInternal(c, err.Error())
		return
	}

	for i := range configs {
		configs[i].Value = maskSensitiveConfig(configs[i].Key, configs[i].Value)
	}

	c.JSON(http.StatusOK, response.OK(configs))
}

// GetSystemConfig 获取单个系统配置
// @Summary 获取单个系统配置
// @Description 根据配置键获取对应的系统配置详情，需要管理员权限
// @Tags admin
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Success 200 {object} response.Any{data=SystemConfig} "系统配置详情"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [get]
func GetSystemConfig(c *gin.Context) {
	config, err := getSystemConfig(c.Request.Context(), c.Param("key"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, SystemConfigNotFound)
		} else {
			response.AbortInternal(c, err.Error())
		}
		return
	}

	config.Value = maskSensitiveConfig(config.Key, config.Value)

	c.JSON(http.StatusOK, response.OK(config))
}

// UpdateSystemConfig 更新系统配置
// @Summary 更新系统配置
// @Description 根据配置键更新对应的配置内容，同时将更新同步到 Redis，需要管理员权限
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param key path string true "配置键"
// @Param request body UpdateSystemConfigRequest true "更新请求参数"
// @Success 200 {object} response.Any{data=string} "更新成功"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "未登录"
// @Failure 403 {object} response.Any "无管理员权限"
// @Failure 404 {object} response.Any "配置不存在"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/admin/system-configs/{key} [put]
func UpdateSystemConfig(c *gin.Context) {
	var req UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	key := c.Param("key")
	if isProtectedConfigKey(key) {
		response.AbortBadRequest(c, protectedConfigKeyMessage)
		return
	}
	if err := updateSystemConfig(c.Request.Context(), key, req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.AbortNotFound(c, SystemConfigNotFound)
			return
		}
		if isStorageConfigValidationError(err) {
			response.AbortBadRequest(c, err.Error())
			return
		}
		response.AbortInternal(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, response.OKNil())
}

func isProtectedConfigKey(key string) bool {
	return key == ConfigKeyLogDatabase || key == ConfigKeyLogDBMigration
}

func createSystemConfig(ctx context.Context, req CreateSystemConfigRequest) error {
	if isProtectedConfigKey(req.Key) {
		return errors.New(protectedConfigKeyMessage)
	}
	exists, err := SystemConfigExists(ctx, req.Key)
	if err != nil {
		return err
	}
	if exists {
		return errors.New(ConfigKeyExists)
	}

	config := SystemConfig{
		Key:         req.Key,
		Value:       req.Value,
		Type:        req.Type,
		Visibility:  req.Visibility,
		Description: req.Description,
	}
	if err := CreateSystemConfigRecord(ctx, &config); err != nil {
		return err
	}

	invalidateSystemConfigCaches(ctx, req.Key)
	if err := InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
	return nil
}

func listSystemConfigs(ctx context.Context, configType string) ([]SystemConfig, error) {
	return ListAdminSystemConfigs(ctx, configType)
}

func getSystemConfig(ctx context.Context, key string) (SystemConfig, error) {
	return GetAdminSystemConfigByKey(ctx, key)
}

func updateSystemConfig(ctx context.Context, key string, req UpdateSystemConfigRequest) error {
	if isProtectedConfigKey(key) {
		return errors.New(protectedConfigKeyMessage)
	}
	config, err := GetAdminSystemConfigByKey(ctx, key)
	if err != nil {
		return err
	}

	var originalDriver contracts.StorageDriver
	if key == ConfigKeyStorageConfig {
		var currentCfg contracts.StorageConfigDTO
		if err := json.Unmarshal([]byte(config.Value), &currentCfg); err == nil {
			originalDriver = currentCfg.Driver
		}

		validatedVal, err := validateAndMergeStorageConfig(ctx, req.Value, config.Value)
		if err != nil {
			return err
		}
		req.Value = validatedVal
	}

	gormDB := GetDB(ctx)
	if gormDB == nil {
		return errors.New("database service not available")
	}
	if err := gormDB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"description": req.Description,
		}
		if req.Visibility != nil {
			updates["visibility"] = *req.Visibility
			config.Visibility = *req.Visibility
		}
		if key != ConfigKeySMTPPassword || req.Value != maskedConfigValue {
			updates["value"] = req.Value
			config.Value = req.Value
		}
		if err := tx.Model(&config).Updates(updates).Error; err != nil {
			return err
		}
		resolveStorageMigrationTasksOnDirectDriverUpdate(ctx, tx, key, originalDriver, req.Value)
		return nil
	}); err != nil {
		return err
	}

	invalidateCachesAfterConfigUpdate(ctx, key)
	return nil
}

func resolveStorageMigrationTasksOnDirectDriverUpdate(
	ctx context.Context,
	tx *gorm.DB,
	key string,
	originalDriver contracts.StorageDriver,
	newValue string,
) {
	if key != ConfigKeyStorageConfig || originalDriver == "" {
		return
	}

	var newCfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(newValue), &newCfg); err != nil {
		return
	}
	if newCfg.Driver != originalDriver {
		return
	}

	if err := MarkFailedTaskExecutionsSucceededTx(
		tx,
		"storage:migrate",
		"存储配置直接更新，故障迁移任务自动标记为已解决",
		time.Now(),
	); err != nil {
		logger.ErrorF(ctx, "自动更新迁移任务状态失败: %v", err)
	}
}

func invalidateSystemConfigCaches(ctx context.Context, key string) {
	if err := InvalidateSystemConfigCache(ctx, key); err != nil {
		logger.WarnF(ctx, "清理系统配置缓存失败: %v", err)
	}
	_ = EmitEvent(ctx, contracts.EventTopicConfigChanged, contracts.ConfigChangedEvent{Key: key})
}

func invalidateCachesAfterConfigUpdate(ctx context.Context, key string) {
	invalidateSystemConfigCaches(ctx, key)

	if err := InvalidateVisibleSystemConfigsCache(ctx); err != nil {
		logger.WarnF(ctx, "清理公共配置列表缓存失败: %v", err)
	}
}

// TestSMTPRequest 测试 SMTP 配置请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host" binding:"required,max=255"`
	SMTPPort     int    `json:"smtp_port" binding:"required"`
	SMTPUsername string `json:"smtp_username" binding:"required,max=255"`
	SMTPPassword string `json:"smtp_password" binding:"required,max=255"`
	To           string `json:"to" binding:"required,email"`
}

// TestSMTPResponse 测试 SMTP 配置响应
type TestSMTPResponse struct {
	Success bool   `json:"success"`
	Log     string `json:"log"`
	Error   string `json:"error"`
}

// TestSMTP 测试 SMTP 邮件发送
// @Summary 测试 SMTP 邮件发送
// @Description 使用传入的配置进行 SMTP 邮件发送测试，支持使用 ****** 占位符使用保存的数据库密码
// @Tags admin
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body TestSMTPRequest true "测试请求参数"
// @Success 200 {object} response.Any{data=TestSMTPResponse} "测试执行完毕"
// @Failure 400 {object} response.Any "参数错误"
// @Router /api/v1/admin/system-configs/smtp/test [post]
func TestSMTP(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AbortBadRequest(c, err.Error())
		return
	}

	password := req.SMTPPassword
	if password == maskedConfigValue {
		if sc, err := GetSystemConfigByKey(c.Request.Context(), ConfigKeySMTPPassword); err == nil {
			password = sc.Value
		}
	}

	cfg := mail.Config{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
	}

	subject := "Wavelet SMTP Test Mail"
	body := `<h3>SMTP Mail Connection Test</h3>
<p>If you received this message, your SMTP configuration is correct and mail sending is working properly.</p>
<p>Sent from Wavelet.</p>`

	logs, err := mail.SendMailWithLog(c.Request.Context(), cfg, req.To, subject, body)
	resp := TestSMTPResponse{
		Success: err == nil,
		Log:     logs,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	c.JSON(http.StatusOK, response.OK(resp))
}

func isStorageConfigValidationError(err error) bool {
	msg := err.Error()
	return msg == StorageDriverSwitchRequiresMigration ||
		strings.HasPrefix(msg, "解析") ||
		strings.HasPrefix(msg, "验证") ||
		strings.HasPrefix(msg, "初始化测试") ||
		strings.HasPrefix(msg, "存储连通性") ||
		strings.HasPrefix(msg, "序列化") ||
		strings.HasPrefix(msg, "检查存量文件")
}

func maskSensitiveConfig(key, value string) string {
	if value == "" {
		return value
	}
	switch key {
	case ConfigKeySMTPPassword:
		return maskedConfigValue
	case ConfigKeyStorageConfig:
		var cfg contracts.StorageConfigDTO
		if err := json.Unmarshal([]byte(value), &cfg); err == nil {
			if cfg.S3.SecretAccessKey != "" {
				cfg.S3.SecretAccessKey = maskedConfigValue
			}
			if cfg.R2.SecretAccessKey != "" {
				cfg.R2.SecretAccessKey = maskedConfigValue
			}
			if cfg.MinIO.SecretAccessKey != "" {
				cfg.MinIO.SecretAccessKey = maskedConfigValue
			}
			if cfg.OSS.SecretAccessKey != "" {
				cfg.OSS.SecretAccessKey = maskedConfigValue
			}
			if cfg.WebDAV.Password != "" {
				cfg.WebDAV.Password = maskedConfigValue
			}
			if val, err := json.Marshal(cfg); err == nil {
				return string(val)
			}
		}
	}
	return value
}

// validateAndMergeStorageConfig parses, merges unmasked secrets, validates parameter values,
// and tests connectivity of the new storage configuration.
func validateAndMergeStorageConfig(ctx context.Context, value string, currentConfig string) (string, error) {
	var currentCfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(currentConfig), &currentCfg); err != nil {
		return "", fmt.Errorf("解析当前存储配置失败: %w", err)
	}

	var newCfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(value), &newCfg); err != nil {
		return "", fmt.Errorf("解析目标存储配置失败: %w", err)
	}

	// 合并被掩码屏蔽的敏感信息，获取完整的真实配置
	targetCfg := newCfg
	if targetCfg.S3.SecretAccessKey == maskedConfigValue {
		targetCfg.S3.SecretAccessKey = currentCfg.S3.SecretAccessKey
	}
	if targetCfg.R2.SecretAccessKey == maskedConfigValue {
		targetCfg.R2.SecretAccessKey = currentCfg.R2.SecretAccessKey
	}
	if targetCfg.MinIO.SecretAccessKey == maskedConfigValue {
		targetCfg.MinIO.SecretAccessKey = currentCfg.MinIO.SecretAccessKey
	}
	if targetCfg.OSS.SecretAccessKey == maskedConfigValue {
		targetCfg.OSS.SecretAccessKey = currentCfg.OSS.SecretAccessKey
	}
	if targetCfg.WebDAV.Password == maskedConfigValue {
		targetCfg.WebDAV.Password = currentCfg.WebDAV.Password
	}

	if err := validateMergedStorageConfig(ctx, currentCfg, newCfg, targetCfg); err != nil {
		return "", err
	}

	// 序列化为最终保存的真实明文配置，防止保存屏蔽的 ****** 字符
	unmaskedVal, err := json.Marshal(targetCfg)
	if err != nil {
		return "", fmt.Errorf("序列化存储配置失败: %w", err)
	}

	return string(unmaskedVal), nil
}

func validateMergedStorageConfig(ctx context.Context, currentCfg, newCfg, targetCfg contracts.StorageConfigDTO) error {
	if newCfg.Driver != "" && newCfg.Driver != currentCfg.Driver {
		var uploadCount int64
		gormDB := GetDB(ctx)
		if gormDB != nil {
			if err := gormDB.Table("w_uploads").
				Where("status != ?", "deleted").
				Count(&uploadCount).Error; err != nil {
				return fmt.Errorf("检查存量文件失败: %w", err)
			}
		}
		if uploadCount > 0 {
			return errors.New(StorageDriverSwitchRequiresMigration)
		}
	}

	return nil
}
