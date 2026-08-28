// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/core/contracts"
	db "github.com/Rain-kl/Wavelet/pkg/persistence"
	pkgpush "github.com/Rain-kl/Wavelet/pkg/push"
	"github.com/Rain-kl/Wavelet/pkg/task"
	"gorm.io/gorm"
)

type smtpConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

func loadSMTPConfig(ctx context.Context) smtpConfig {
	var cfg smtpConfig
	var host, port, user, pass string
	_ = db.DB(ctx).Table("w_system_configs").Where("key = ?", "smtp_host").Pluck("value", &host).Error
	_ = db.DB(ctx).Table("w_system_configs").Where("key = ?", "smtp_port").Pluck("value", &port).Error
	_ = db.DB(ctx).Table("w_system_configs").Where("key = ?", "smtp_username").Pluck("value", &user).Error
	_ = db.DB(ctx).Table("w_system_configs").Where("key = ?", "smtp_password").Pluck("value", &pass).Error
	cfg.Host = host
	cfg.Port = port
	cfg.Username = user
	cfg.Password = pass
	return cfg
}

func syncBuiltInEvents(ctx context.Context) error {
	for _, meta := range GetBuiltInEvents() {
		_, err := GetPushEventByKeyRecord(ctx, meta.Key)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var defaultTemplateStr string
			if defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate); err == nil {
				defaultTemplateStr = string(defaultTemplateBytes)
			}
			event := PushEvent{
				EventKey: meta.Key,
				Name:     meta.Name,
				Channels: []string{},
				Targets:  []string{},
				Template: defaultTemplateStr,
				Enabled:  false,
			}
			if err := CreatePushEventRecord(ctx, &event); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func listPushEvents(ctx context.Context) ([]PushEvent, error) {
	return ListPushEventsRecord(ctx)
}

func createPushEvent(ctx context.Context, req CreatePushEventRequest) (PushEvent, error) {
	eventKey, eventName, defaultTemplateBytes, err := getEventInfo(req)
	if err != nil {
		return PushEvent{}, err
	}

	count, err := CountPushEventsByKeyRecord(ctx, eventKey)
	if err != nil {
		return PushEvent{}, err
	}
	if count > 0 {
		return PushEvent{}, errors.New("this notification event is already configured")
	}

	templateStr := strings.TrimSpace(req.Template)
	if templateStr == "" {
		templateStr = string(defaultTemplateBytes)
	} else {
		var tempMap map[string]any
		if err := json.Unmarshal([]byte(templateStr), &tempMap); err != nil {
			return PushEvent{}, errors.New("custom template is not a valid JSON format")
		}
	}

	channels := req.Channels
	if channels == nil {
		channels = []string{}
	}
	targets := req.Targets
	if targets == nil {
		targets = []string{}
	}

	event := PushEvent{
		EventKey: eventKey,
		Name:     eventName,
		TaskType: req.TaskType,
		Channels: channels,
		Targets:  targets,
		Template: templateStr,
		Enabled:  req.Enabled,
	}
	if err := event.Validate(); err != nil {
		return PushEvent{}, err
	}
	if err := CreatePushEventRecord(ctx, &event); err != nil {
		return PushEvent{}, err
	}
	return event, nil
}

func deletePushEvent(ctx context.Context, id uint64) error {
	event, err := GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return DeletePushEventRecord(ctx, &event)
}

func updatePushEvent(ctx context.Context, id uint64, req UpdatePushEventRequest) error {
	event, err := GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return err
	}

	event.Channels = req.Channels
	event.Targets = req.Targets
	event.Template = req.Template
	event.Enabled = req.Enabled
	if err := event.Validate(); err != nil {
		return err
	}
	return SavePushEventRecord(ctx, &event)
}

func togglePushEvent(ctx context.Context, id uint64) (bool, error) {
	event, err := GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return false, err
	}

	enabled := !event.Enabled
	if enabled && len(event.Channels) == 0 {
		return false, errors.New("cannot enable event without any push channels configured")
	}
	if err := UpdatePushEventEnabledRecord(ctx, &event, enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func listPushHistories(ctx context.Context, filter PushHistoryListFilter) (int64, []PushHistory, error) {
	return ListPushHistoriesRecord(ctx, filter)
}

func applySMTPFallbackToPushConfig(ctx context.Context, cfg *pkgpush.Config) {
	if cfg.Channel != channelEmail || (cfg.URL != "" && cfg.Key != "") {
		return
	}
	smtp := loadSMTPConfig(ctx)
	if smtp.Host == "" || smtp.Username == "" {
		return
	}
	port := smtp.Port
	if port == "" {
		port = "587"
	}
	cfg.URL = smtp.Host + ":" + port
	cfg.Key = smtp.Username
	cfg.Secret = smtp.Password
}

func listPushChannels(ctx context.Context) ([]PushChannel, error) {
	return ListPushChannelsRecord(ctx)
}

func createPushChannel(ctx context.Context, req CreatePushChannelRequest) (PushChannel, error) {
	count, err := CountPushChannelsByNameRecord(ctx, req.Name)
	if err != nil {
		return PushChannel{}, err
	}
	if count > 0 {
		return PushChannel{}, errors.New("channel name already exists")
	}

	channel := PushChannel{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Token:       req.Token,
		URL:         req.URL,
		Other:       req.Other,
		Enabled:     req.Enabled,
	}
	if err := channel.Validate(); err != nil {
		return PushChannel{}, err
	}
	if err := CreatePushChannelRecord(ctx, &channel); err != nil {
		return PushChannel{}, err
	}
	return channel, nil
}

func updatePushChannel(ctx context.Context, id uint64, req UpdatePushChannelRequest) (PushChannel, error) {
	channel, err := GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return PushChannel{}, err
	}

	channel.Description = req.Description
	channel.Type = req.Type
	channel.Token = req.Token
	channel.URL = req.URL
	channel.Other = req.Other
	channel.Enabled = req.Enabled
	if err := channel.Validate(); err != nil {
		return PushChannel{}, err
	}
	if err := SavePushChannelRecord(ctx, &channel); err != nil {
		return PushChannel{}, err
	}
	return channel, nil
}

func deletePushChannel(ctx context.Context, id uint64) error {
	channel, err := GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return DeletePushChannelRecord(ctx, &channel)
}

func loadChannelForTest(ctx context.Context, req TestPushChannelRequest) (string, string, string, string, error) {
	if req.Name != "" {
		channel, err := GetPushChannelByNameRecord(ctx, req.Name)
		if err != nil {
			return "", "", "", "", errors.New("channel not found")
		}
		return channel.URL, channel.Token, channel.Other, channel.Type, nil
	}
	return req.URL, req.Token, req.Other, req.Type, nil
}

func listActivePushEventsByTaskType(ctx context.Context, taskType string) ([]PushEvent, error) {
	return ListActivePushEventsByTaskTypeRecord(ctx, taskType)
}

func loadUserFromPayload(ctx context.Context, data map[string]any) any {
	if u, exists := data["user"]; exists && u != nil {
		return u
	}

	if userID, ok := extractUserID(data); ok && userID > 0 {
		var user contracts.UserDTO
		if err := db.DB(ctx).Table("w_users").Where("id = ?", userID).First(&user).Error; err == nil {
			return &user
		}
	}

	if username := extractUsername(data); username != "" {
		var user contracts.UserDTO
		if err := db.DB(ctx).Table("w_users").Where("username = ?", username).First(&user).Error; err == nil {
			return &user
		}
	}
	return nil
}

func recordPushHistory(ctx context.Context, req SendPayload, status, errMsg string) error {
	title := req.Body.Title
	content := req.Body.Content
	level := req.Body.Level
	if title == "" {
		title = "系统通知"
	}
	if level == "" {
		level = defaultLevelInfo
	}

	target := req.Target
	if target == "" {
		if req.Config.URL != "" {
			target = req.Config.URL
			const maxTargetLen = 50
			const truncatedLen = 47
			if len(target) > maxTargetLen {
				target = target[:truncatedLen] + "..."
			}
		} else {
			target = "default"
		}
	}

	history := PushHistory{
		EventKey: req.EventKey,
		Channel:  req.Config.Channel,
		Target:   target,
		Title:    title,
		Content:  content,
		Level:    level,
		Status:   status,
		ErrorMsg: errMsg,
	}
	return CreatePushHistoryRecord(ctx, &history)
}

func resolveTarget(ctx context.Context, target string, flatBody map[string]any, channel string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	resolved := resolveDynamicKeyword(target, flatBody)
	if strings.Contains(resolved, "@") {
		return resolved
	}
	if val, matched := resolveSystemTarget(ctx, resolved, channel); matched {
		return val
	}

	user, found := resolveTargetUser(ctx, resolved, channel)
	if !found {
		return resolved
	}
	if channel == channelEmail && user.Email != "" {
		return user.Email
	}
	if channel != channelEmail && user.Username != "" {
		return user.Username
	}
	return resolved
}

func resolveDynamicKeyword(target string, flatBody map[string]any) string {
	switch target {
	case "user.id", "id":
		if val, ok := flatBody["user.id"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["id"]; ok {
			return fmt.Sprintf("%v", val)
		}
	case "user.username", "username":
		if val, ok := flatBody["user.username"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["username"]; ok {
			return fmt.Sprintf("%v", val)
		}
	case "user.email", channelEmail:
		if val, ok := flatBody["user.email"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["email"]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return target
}

func resolveTargetUser(ctx context.Context, resolved string, _ string) (contracts.UserDTO, bool) {
	var user contracts.UserDTO
	if id, err := strconv.ParseUint(resolved, 10, 64); err == nil {
		if err := db.DB(ctx).Table("w_users").Where("id = ?", id).First(&user).Error; err == nil {
			return user, true
		}
	}
	if err := db.DB(ctx).Table("w_users").Where("username = ?", resolved).First(&user).Error; err == nil {
		return user, true
	}
	return user, false
}

func resolveSystemTarget(ctx context.Context, resolved string, channel string) (string, bool) {
	if resolved != "系统" && resolved != "system" && resolved != "0" {
		return "", false
	}
	var adminUser contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").Where("is_admin = ?", true).Order("id ASC").First(&adminUser).Error; err != nil {
		return resolved, true
	}
	if channel == channelEmail && adminUser.Email != "" {
		return adminUser.Email, true
	}
	if channel != channelEmail && adminUser.Username != "" {
		return adminUser.Username, true
	}
	return resolved, true
}

func resolveSMTPConfig(ctx context.Context, url, token, other string) (string, string, string) {
	if url != "" && token != "" {
		return url, token, other
	}
	smtp := loadSMTPConfig(ctx)
	if smtp.Host == "" || smtp.Username == "" {
		return url, token, other
	}
	port := smtp.Port
	if port == "" {
		port = "587"
	}
	if url == "" {
		url = smtp.Host + ":" + port
	}
	if token == "" {
		token = smtp.Username
	}
	if other == "" {
		other = smtp.Password
	}
	return url, token, other
}

func getSystemUser(ctx context.Context) *contracts.UserDTO {
	var user contracts.UserDTO
	if err := db.DB(ctx).Table("w_users").Where("is_admin = ?", true).Order("id ASC").First(&user).Error; err == nil {
		return &user
	}
	return &contracts.UserDTO{
		Username: "system",
		Nickname: "系统管理员",
	}
}

func findBuiltInEvent(key string) (EventMetadata, bool) {
	for _, meta := range GetBuiltInEvents() {
		if meta.Key == key {
			return meta, true
		}
	}
	return EventMetadata{}, false
}

func getEventInfo(req CreatePushEventRequest) (string, string, []byte, error) {
	if req.TaskType != "" {
		meta := task.GetTaskMetaByAsynqTask(req.TaskType)
		if meta == nil {
			return "", "", nil, errors.New("unsupported task type")
		}
		eventKey := "task_completed:" + req.TaskType
		eventName := "任务完成: " + meta.Name
		defaultTemplate := NotificationMessage{
			Title:   "任务完成: " + meta.Name,
			Content: "异步任务 {{task_name}} (ID: {{task_id}}) 已完成。状态: {{task_status}}，耗时: {{task_duration}} ms。",
			Level:   defaultLevelInfo,
		}
		defaultTemplateBytes, err := json.Marshal(defaultTemplate)
		if err != nil {
			return "", "", nil, err
		}
		return eventKey, eventName, defaultTemplateBytes, nil
	}

	if req.EventKey == "" {
		return "", "", nil, errors.New("either event_key or task_type must be provided")
	}

	meta, found := findBuiltInEvent(req.EventKey)
	if !found {
		return "", "", nil, errors.New("unsupported built-in event key")
	}

	defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate)
	if err != nil {
		return "", "", nil, err
	}
	return req.EventKey, meta.Name, defaultTemplateBytes, nil
}

func enqueuePushTask(ctx context.Context, payload SendPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = task.DispatchTask(ctx, "send_notification", payloadBytes, "system")
	return err
}

func getFlatBody(body map[string]any) map[string]any {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return body
	}
	var jsonMap map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return body
	}

	flatResult := make(map[string]any)
	flattenMap("", jsonMap, flatResult)
	return flatResult
}

func flattenMap(prefix string, m map[string]any, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nestedMap, ok := v.(map[string]any); ok {
			flattenMap(key, nestedMap, result)
		} else {
			result[key] = v
		}
	}
}
