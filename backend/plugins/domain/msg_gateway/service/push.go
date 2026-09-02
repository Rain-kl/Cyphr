// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/msg_gateway/consts"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	pkgpush "Wavelet/plugins/domain/msg_gateway/push"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	builtInEventsMu sync.RWMutex
	// BuiltInEvents lists all built-in events defined in custom_events.
	BuiltInEvents []do.EventMetadata
)

// RegisterBuiltInEvent registers a built-in event definition.
func RegisterBuiltInEvent(meta do.EventMetadata) {
	builtInEventsMu.Lock()
	defer builtInEventsMu.Unlock()
	for i, e := range BuiltInEvents {
		if e.Key == meta.Key {
			BuiltInEvents[i] = meta
			return
		}
	}
	BuiltInEvents = append(BuiltInEvents, meta)
}

// GetBuiltInEvents returns a copy of registered built-in events.
func GetBuiltInEvents() []do.EventMetadata {
	builtInEventsMu.RLock()
	defer builtInEventsMu.RUnlock()
	out := make([]do.EventMetadata, len(BuiltInEvents))
	copy(out, BuiltInEvents)
	return out
}

// PushRegistryAdapter adapts contracts.PushRegistry onto the built-in event store.
type PushRegistryAdapter struct{}

// RegisterBuiltInEvent records a built-in push event definition.
func (PushRegistryAdapter) RegisterBuiltInEvent(meta contracts.PushEventMeta) {
	RegisterBuiltInEvent(eventMetadataFromContract(meta))
}

// SyncEvents persists registered built-in events into the database.
func (PushRegistryAdapter) SyncEvents(ctx context.Context) error {
	return SyncEvents(ctx)
}

func eventMetadataFromContract(meta contracts.PushEventMeta) do.EventMetadata {
	return do.EventMetadata{
		Key:         meta.Key,
		Name:        meta.Name,
		Description: meta.Description,
		DefaultTemplate: do.NotificationMessage{
			Title:   meta.DefaultTemplate.Title,
			Content: meta.DefaultTemplate.Content,
			Level:   meta.DefaultTemplate.Level,
			Ext:     meta.DefaultTemplate.Ext,
		},
	}
}

// SyncBuiltInEvents seeds a database row for every registered built-in event.
func SyncBuiltInEvents(ctx context.Context) error {
	for _, meta := range GetBuiltInEvents() {
		_, err := dao.GetPushEventByKeyRecord(ctx, meta.Key)
		if errors.Is(err, consts.ErrRecordNotFound) {
			var defaultTemplateStr string
			if defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate); err == nil {
				defaultTemplateStr = string(defaultTemplateBytes)
			}
			event := entity.PushEvent{
				EventKey: meta.Key,
				Name:     meta.Name,
				Channels: []string{},
				Targets:  []string{},
				Template: defaultTemplateStr,
				Enabled:  false,
			}
			if err := dao.CreatePushEventRecord(ctx, &event); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// ListPushEvents lists all configured push events.
func ListPushEvents(ctx context.Context) ([]entity.PushEvent, error) {
	return dao.ListPushEventsRecord(ctx)
}

// CreatePushEvent stores a push event configuration for a built-in event or task type.
func CreatePushEvent(ctx context.Context, req do.CreatePushEventRequest) (entity.PushEvent, error) {
	eventKey, eventName, defaultTemplateBytes, err := GetEventInfo(ctx, req)
	if err != nil {
		return entity.PushEvent{}, err
	}

	count, err := dao.CountPushEventsByKeyRecord(ctx, eventKey)
	if err != nil {
		return entity.PushEvent{}, err
	}
	if count > 0 {
		return entity.PushEvent{}, errors.New(consts.ErrEventAlreadyConfigured)
	}

	templateStr := strings.TrimSpace(req.Template)
	if templateStr == "" {
		templateStr = string(defaultTemplateBytes)
	} else {
		var tempMap map[string]any
		if err := json.Unmarshal([]byte(templateStr), &tempMap); err != nil {
			return entity.PushEvent{}, errors.New(consts.ErrTemplateInvalidJSON)
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

	event := entity.PushEvent{
		EventKey: eventKey,
		Name:     eventName,
		TaskType: req.TaskType,
		Channels: channels,
		Targets:  targets,
		Template: templateStr,
		Enabled:  req.Enabled,
	}
	if err := event.Validate(); err != nil {
		return entity.PushEvent{}, err
	}
	if err := dao.CreatePushEventRecord(ctx, &event); err != nil {
		return entity.PushEvent{}, err
	}
	return event, nil
}

// DeletePushEvent deletes a push event configuration by id.
func DeletePushEvent(ctx context.Context, id uint64) error {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return dao.DeletePushEventRecord(ctx, &event)
}

// UpdatePushEvent replaces mutable push event fields.
func UpdatePushEvent(ctx context.Context, id uint64, req do.UpdatePushEventRequest) error {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
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
	return dao.SavePushEventRecord(ctx, &event)
}

// TogglePushEvent flips the enabled flag of a push event.
func TogglePushEvent(ctx context.Context, id uint64) (bool, error) {
	event, err := dao.GetPushEventByIDRecord(ctx, id)
	if err != nil {
		return false, err
	}

	enabled := !event.Enabled
	if enabled && len(event.Channels) == 0 {
		return false, errors.New(consts.ErrEnableWithoutChannels)
	}
	if err := dao.UpdatePushEventEnabledRecord(ctx, &event, enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

// ListPushHistories returns a paginated push delivery audit page.
func ListPushHistories(ctx context.Context, filter do.PushHistoryListFilter) (int64, []entity.PushHistory, error) {
	return dao.ListPushHistoriesRecord(ctx, filter)
}

// ApplySMTPFallbackToPushConfig fills an email config from the system SMTP settings.
func ApplySMTPFallbackToPushConfig(ctx context.Context, cfg *pkgpush.Config) {
	if cfg.Channel != consts.ChannelEmail || (cfg.URL != "" && cfg.Key != "") {
		return
	}
	smtp, err := dao.LoadSMTPConfigRecord(ctx)
	if err != nil {
		logger.ErrorF(ctx, "[Push] 读取 SMTP 系统配置失败: %v", err)
		return
	}
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

// RunPushTest validates an ad-hoc channel config and sends a connectivity probe.
func RunPushTest(ctx context.Context, cfg pkgpush.Config, target string) error {
	pusher, err := pkgpush.GetPusher(cfg.Channel)
	if err != nil {
		return err
	}
	if err := pusher.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("%s: %w", consts.ErrValidationFailed, err)
	}

	ApplySMTPFallbackToPushConfig(ctx, &cfg)

	testBody := map[string]any{
		consts.KeyTitle:   "测试通道推送",
		consts.KeyContent: "当您收到这条消息，说明当前渠道连通性测试通过。",
		consts.KeyLevel:   consts.DefaultLevelInfo,
	}
	if _, err := pusher.Send(ctx, cfg, target, testBody, "", nil); err != nil {
		return err
	}
	return nil
}

// ListPushChannels returns every configured push channel.
func ListPushChannels(ctx context.Context) ([]entity.PushChannel, error) {
	return dao.ListPushChannelsRecord(ctx)
}

// CreatePushChannel validates uniqueness and persists a new push channel.
func CreatePushChannel(ctx context.Context, req do.CreatePushChannelRequest) (entity.PushChannel, error) {
	count, err := dao.CountPushChannelsByNameRecord(ctx, req.Name)
	if err != nil {
		return entity.PushChannel{}, err
	}
	if count > 0 {
		return entity.PushChannel{}, errors.New(consts.ErrChannelNameExists)
	}

	channel := entity.PushChannel{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Token:       req.Token,
		URL:         req.URL,
		Other:       req.Other,
		Enabled:     req.Enabled,
	}
	if err := channel.Validate(); err != nil {
		return entity.PushChannel{}, err
	}
	if err := dao.CreatePushChannelRecord(ctx, &channel); err != nil {
		return entity.PushChannel{}, err
	}
	return channel, nil
}

// UpdatePushChannel replaces the mutable fields of an existing push channel.
func UpdatePushChannel(ctx context.Context, id uint64, req do.UpdatePushChannelRequest) (entity.PushChannel, error) {
	channel, err := dao.GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return entity.PushChannel{}, err
	}

	channel.Description = req.Description
	channel.Type = req.Type
	channel.Token = req.Token
	channel.URL = req.URL
	channel.Other = req.Other
	channel.Enabled = req.Enabled
	if err := channel.Validate(); err != nil {
		return entity.PushChannel{}, err
	}
	if err := dao.SavePushChannelRecord(ctx, &channel); err != nil {
		return entity.PushChannel{}, err
	}
	return channel, nil
}

// DeletePushChannel removes a push channel by id.
func DeletePushChannel(ctx context.Context, id uint64) error {
	channel, err := dao.GetPushChannelByIDRecord(ctx, id)
	if err != nil {
		return err
	}
	return dao.DeletePushChannelRecord(ctx, &channel)
}

// LoadChannelForTest resolves the credentials under test, either from a stored
// channel name or from the ad-hoc values sent by the caller.
func LoadChannelForTest(ctx context.Context, req do.TestPushChannelRequest) (string, string, string, string, error) {
	if req.Name != "" {
		channel, err := dao.GetPushChannelByNameRecord(ctx, req.Name)
		if err != nil {
			return "", "", "", "", errors.New(consts.ErrChannelNotFound)
		}
		return channel.URL, channel.Token, channel.Other, channel.Type, nil
	}
	return req.URL, req.Token, req.Other, req.Type, nil
}

// PreparePushChannelTest builds the connectivity probe payload for a channel.
func PreparePushChannelTest(ctx context.Context, req do.TestPushChannelRequest) (do.SendPayload, error) {
	url, token, other, channelType, err := LoadChannelForTest(ctx, req)
	if err != nil {
		return do.SendPayload{}, err
	}

	if channelType == consts.ChannelEmail {
		url, token, other = ResolveSMTPConfig(ctx, url, token, other)
	}

	tempChannel := entity.PushChannel{
		Name:    "test_temp",
		URL:     url,
		Token:   token,
		Other:   other,
		Type:    channelType,
		Enabled: true,
	}
	if err := tempChannel.Validate(); err != nil {
		return do.SendPayload{}, err
	}
	url = tempChannel.URL

	var config pkgpush.Config
	var renderedJSON string
	switch channelType {
	case consts.ChannelLark:
		config = pkgpush.Config{Channel: consts.ChannelLark, URL: url, Secret: token}
		renderedJSON = other
	case consts.ChannelEmail:
		config = pkgpush.Config{Channel: consts.ChannelEmail, URL: url, Key: token, Secret: other}
	case consts.ChannelTelegram:
		config = pkgpush.Config{Channel: consts.ChannelTelegram, URL: url, Secret: token, Key: other}
	default:
		config = pkgpush.Config{Channel: consts.ChannelCustom, URL: url}
		customPushReq := do.CustomPushRequest{
			Title:       "通道测试通知",
			Content:     "这是一条来自系统的消息通道连通性测试消息。",
			Description: "系统通道测试",
			URL:         "https://example.com",
			To:          req.Target,
		}
		renderedJSON = RenderCustomPayload(other, customPushReq)
	}

	return do.SendPayload{
		EventKey: "test_channel",
		Config:   config,
		Target:   req.Target,
		Body: do.NotificationMessage{
			Title:   "通道测试通知",
			Content: "这是一条来自系统的消息通道连通性测试消息。",
			Level:   consts.DefaultLevelInfo,
		},
		Template: renderedJSON,
	}, nil
}

// RenderCustomPayload substitutes the supported template variables of a custom
// webhook body, JSON-escaping every injected value.
func RenderCustomPayload(template string, req do.CustomPushRequest) string {
	result := template
	result = strings.ReplaceAll(result, "$title", EscapeJSONString(req.Title))
	result = strings.ReplaceAll(result, "$description", EscapeJSONString(req.Description))
	result = strings.ReplaceAll(result, "$content", EscapeJSONString(req.Content))
	result = strings.ReplaceAll(result, "$url", EscapeJSONString(req.URL))
	result = strings.ReplaceAll(result, "$to", EscapeJSONString(req.To))
	return result
}

// EscapeJSONString renders s as a JSON string body without the surrounding quotes.
func EscapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	const minJSONLen = 2
	if len(b) >= minJSONLen {
		return string(b[1 : len(b)-1])
	}
	return s
}

// ListActivePushEventsByTaskType returns enabled push events for a given task type.
func ListActivePushEventsByTaskType(ctx context.Context, taskType string) ([]entity.PushEvent, error) {
	return dao.ListActivePushEventsByTaskTypeRecord(ctx, taskType)
}

// QueryUser resolves a user through the UserService contract, falling back to the
// DAO read path while the contract is not wired yet.
func QueryUser(ctx context.Context, fromService func(contracts.UserService) (*contracts.UserDTO, error), dbField string, dbVal any) (*contracts.UserDTO, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		return fromService(userSvc)
	}
	if user, err := dao.FindUserByFieldRecord(ctx, dbField, dbVal); err == nil && user != nil {
		return user, nil
	}
	return nil, errors.New(consts.ErrUserNotFound)
}

// FindUserByID resolves a user by primary key.
func FindUserByID(ctx context.Context, id uint64) (*contracts.UserDTO, error) {
	return QueryUser(ctx, func(s contracts.UserService) (*contracts.UserDTO, error) {
		return s.GetUserByID(ctx, id)
	}, "id", id)
}

// FindUserByUsername resolves a user by login name.
func FindUserByUsername(ctx context.Context, username string) (*contracts.UserDTO, error) {
	return QueryUser(ctx, func(s contracts.UserService) (*contracts.UserDTO, error) {
		return s.GetUserByUsername(ctx, username)
	}, "username", username)
}

// LoadUserFromPayload extracts user info from data.
func LoadUserFromPayload(ctx context.Context, data map[string]any) any {
	if u, exists := data["user"]; exists && u != nil {
		return u
	}

	if userID, ok := ExtractUserID(data); ok && userID > 0 {
		if user, err := FindUserByID(ctx, userID); err == nil && user != nil {
			return user
		}
	}

	if username := ExtractUsername(data); username != "" {
		if user, err := FindUserByUsername(ctx, username); err == nil && user != nil {
			return user
		}
	}
	return nil
}

// RecordPushHistory creates a push history audit record.
func RecordPushHistory(ctx context.Context, req do.SendPayload, status, errMsg string) error {
	title := req.Body.Title
	content := req.Body.Content
	level := req.Body.Level
	if title == "" {
		title = "系统通知"
	}
	if level == "" {
		level = consts.DefaultLevelInfo
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

	history := entity.PushHistory{
		EventKey: req.EventKey,
		Channel:  req.Config.Channel,
		Target:   target,
		Title:    title,
		Content:  content,
		Level:    level,
		Status:   status,
		ErrorMsg: errMsg,
	}
	return dao.CreatePushHistoryRecord(ctx, &history)
}

// ResolveTarget parses dynamic placeholders into concrete receiver targets.
func ResolveTarget(ctx context.Context, target string, flatBody map[string]any, channel string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}

	resolved := ResolveDynamicKeyword(target, flatBody)
	if strings.Contains(resolved, "@") {
		return resolved
	}
	if val, matched := ResolveSystemTarget(ctx, resolved, channel); matched {
		return val
	}

	user, found := ResolveTargetUser(ctx, resolved, channel)
	if !found {
		return resolved
	}
	if channel == consts.ChannelEmail && user.Email != "" {
		return user.Email
	}
	if channel != consts.ChannelEmail && user.Username != "" {
		return user.Username
	}
	return resolved
}

// ResolveDynamicKeyword resolves user.id, username, email keywords.
func ResolveDynamicKeyword(target string, flatBody map[string]any) string {
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
	case "user.email", consts.ChannelEmail:
		if val, ok := flatBody["user.email"]; ok {
			return fmt.Sprintf("%v", val)
		}
		if val, ok := flatBody["email"]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return target
}

// ResolveTargetUser resolves user by numeric ID or username string.
func ResolveTargetUser(ctx context.Context, resolved, _ string) (contracts.UserDTO, bool) {
	if id, err := strconv.ParseUint(resolved, 10, 64); err == nil {
		if u, err := FindUserByID(ctx, id); err == nil && u != nil {
			return *u, true
		}
	}
	if u, err := FindUserByUsername(ctx, resolved); err == nil && u != nil {
		return *u, true
	}
	return contracts.UserDTO{}, false
}

// GetFirstAdminUser resolves the first administrator through the UserService
// contract, falling back to the DAO read path when it is unavailable.
func GetFirstAdminUser(ctx context.Context) (*contracts.UserDTO, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		return userSvc.GetFirstAdminUser(ctx)
	}
	if adminUser, err := dao.FindFirstAdminUserRecord(ctx); err == nil && adminUser != nil {
		return adminUser, nil
	}
	return nil, errors.New(consts.ErrNoAdminUser)
}

// ResolveSystemTarget maps system receiver aliases to administrator contact info.
func ResolveSystemTarget(ctx context.Context, resolved, channel string) (string, bool) {
	if resolved != "系统" && resolved != "system" && resolved != "0" {
		return "", false
	}
	adminUser, err := GetFirstAdminUser(ctx)
	if err != nil || adminUser == nil {
		return resolved, true
	}
	if channel == consts.ChannelEmail && adminUser.Email != "" {
		return adminUser.Email, true
	}
	if channel != consts.ChannelEmail && adminUser.Username != "" {
		return adminUser.Username, true
	}
	return resolved, true
}

// ResolveSMTPConfig fills missing email endpoint fields from the system SMTP settings.
func ResolveSMTPConfig(ctx context.Context, url, token, other string) (string, string, string) {
	if url != "" && token != "" {
		return url, token, other
	}
	smtp, err := dao.LoadSMTPConfigRecord(ctx)
	if err != nil {
		logger.ErrorF(ctx, "[Push] 读取 SMTP 系统配置失败: %v", err)
		return url, token, other
	}
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

// GetSystemUser gets a system user DTO.
func GetSystemUser(ctx context.Context) *contracts.UserDTO {
	if adminUser, err := GetFirstAdminUser(ctx); err == nil && adminUser != nil {
		return adminUser
	}
	return &contracts.UserDTO{
		Username: "system",
		Nickname: "系统管理员",
	}
}

// FindBuiltInEvent finds a registered built-in event by key.
func FindBuiltInEvent(key string) (do.EventMetadata, bool) {
	for _, meta := range GetBuiltInEvents() {
		if meta.Key == key {
			return meta, true
		}
	}
	return do.EventMetadata{}, false
}

// GetEventInfo derives the event key, display name and default template for a
// task-completion based event or a registered built-in event key.
func GetEventInfo(ctx context.Context, req do.CreatePushEventRequest) (string, string, []byte, error) {
	if req.TaskType != "" {
		taskName := req.TaskType
		if taskSvc := GetTaskService(ctx); taskSvc != nil {
			if meta, ok := taskSvc.GetTaskMeta(req.TaskType); ok {
				taskName = meta.DisplayName
			}
		}
		eventKey := "task_completed:" + req.TaskType
		eventName := "任务完成: " + taskName
		defaultTemplate := do.NotificationMessage{
			Title:   "任务完成: " + taskName,
			Content: "异步任务 {{task_name}} (ID: {{task_id}}) 已完成。状态: {{task_status}}，耗时: {{task_duration}} ms。",
			Level:   consts.DefaultLevelInfo,
		}
		defaultTemplateBytes, err := json.Marshal(defaultTemplate)
		if err != nil {
			return "", "", nil, err
		}
		return eventKey, eventName, defaultTemplateBytes, nil
	}

	if req.EventKey == "" {
		return "", "", nil, errors.New(consts.ErrEventKeyOrTaskType)
	}

	meta, found := FindBuiltInEvent(req.EventKey)
	if !found {
		return "", "", nil, errors.New(consts.ErrUnsupportedEventKey)
	}

	defaultTemplateBytes, err := json.Marshal(meta.DefaultTemplate)
	if err != nil {
		return "", "", nil, err
	}
	return req.EventKey, meta.Name, defaultTemplateBytes, nil
}

// EnqueuePushTask dispatches a notification payload to the async push worker.
func EnqueuePushTask(ctx context.Context, payload do.SendPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if taskSvc := GetTaskService(ctx); taskSvc != nil {
		_, err = taskSvc.Dispatch(ctx, "send_notification", payloadBytes, contracts.TaskTriggerSystem)
		return err
	}
	return errors.New(consts.ErrTaskServiceUnavailable)
}

// GetFlatBody flattens nested body map.
func GetFlatBody(body map[string]any) map[string]any {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return body
	}
	var jsonMap map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return body
	}

	flatResult := make(map[string]any)
	FlattenMap("", jsonMap, flatResult)
	return flatResult
}

// FlattenMap recursively flattens map key-values.
func FlattenMap(prefix string, m, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nestedMap, ok := v.(map[string]any); ok {
			FlattenMap(key, nestedMap, result)
		} else {
			result[key] = v
		}
	}
}

const (
	// SendNotificationTask is the asynq task name for push notification.
	SendNotificationTask = consts.SendNotificationTask
	// TaskTypeSendNotification is the admin task manager type identifier.
	TaskTypeSendNotification = consts.TaskTypeSendNotification
)

// SendNotificationMeta represents the task metadata.
var SendNotificationMeta = contracts.TaskMetaDTO{
	Type:         TaskTypeSendNotification,
	AsynqTask:    SendNotificationTask,
	Name:         "推送通知",
	DisplayName:  "推送通知",
	Description:  "异步执行系统通知的多渠道派发与推送",
	Category:     "push",
	SupportsTime: false,
	MaxRetry:     3,
	Queue:        taskQueueDefault,
	Retryable:    true,
	Params: []contracts.TaskParamDTO{
		{
			Name:        "event_key",
			Label:       "事件标识",
			Type:        taskParamTypeString,
			Required:    true,
			Placeholder: "admin_login",
			Description: "事件标识 (如 admin_login)",
		},
		{
			Name:        "target",
			Label:       "目标接收者",
			Type:        taskParamTypeString,
			Required:    false,
			Description: "目标接收者",
		},
	},
}

// PushHandler handles asynchronous notification sending.
type PushHandler struct{}

// ValidatePayload validates and normalizes push parameters.
func (h *PushHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New(consts.ErrPayloadRequired)
	}

	var req do.SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("%s: %w", consts.ErrInvalidJSONFormat, err)
	}

	if req.Config.Channel == "" {
		return nil, errors.New(consts.ErrChannelTypeRequired)
	}

	return json.Marshal(req)
}

// Execute performs the push send and logs delivery history audit.
func (h *PushHandler) Execute(ctx context.Context, payload []byte) error {
	var req do.SendPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.ErrorF(ctx, "[Push] 解析推送参数失败: %v", err)
		return fmt.Errorf("%s: %w", consts.ErrParsePayloadFailed, err)
	}

	logger.InfoF(ctx, "[Push] 开始推送通知: 事件 = %s, 渠道 = %s, 接收目标 = %s", req.EventKey, req.Config.Channel, req.Target)

	pusher, err := pkgpush.GetPusher(req.Config.Channel)
	if err != nil {
		errWrap := fmt.Errorf("%s: %w", consts.ErrGetPusherFailed, err)
		logger.ErrorF(ctx, "[Push] 推送失败: %v", errWrap)
		h.recordHistory(ctx, req, "failed", errWrap.Error())
		return errWrap
	}

	flatBody := req.Body.Flatten()
	upstreamResp, err := pusher.Send(ctx, req.Config, req.Target, flatBody, req.Template, nil)

	title := req.Body.Title
	content := req.Body.Content

	if err != nil {
		logger.ErrorF(ctx, "[Push] 消息推送失败 (标题: %s): %v, 上游返回: %s", title, err, upstreamResp)
		h.recordHistory(ctx, req, "failed", err.Error())
		return fmt.Errorf("pusher.Send failed: %w", err)
	}

	logger.InfoF(ctx, "[Push] 消息推送成功 (标题: %s, 内容摘要: %s), 上游返回: %s", title, content, upstreamResp)
	h.recordHistory(ctx, req, "success", "")

	return nil
}

func (h *PushHandler) recordHistory(ctx context.Context, req do.SendPayload, status, errMsg string) {
	if dbErr := RecordPushHistory(ctx, req, status, errMsg); dbErr != nil {
		logger.ErrorF(ctx, "[Push] 写入推送历史审计记录失败: %v", dbErr)
	}
}

// HandleTaskCompleted handles task completion notifications.
func HandleTaskCompleted(ctx context.Context, e contracts.TaskCompletedEvent) {
	events, err := ListActivePushEventsByTaskType(ctx, e.TaskType)
	if err != nil {
		logger.ErrorF(ctx, "push_task_completed_listener: failed to query push events for task type %s: %v", e.TaskType, err)
		return
	}
	if len(events) == 0 {
		return
	}

	body := map[string]any{
		"task_id":       e.TaskID,
		"task_name":     e.TaskName,
		"task_type":     e.TaskType,
		"task_status":   e.Status,
		"task_duration": e.Duration,
		"time":          time.Now().Format("2006-01-02 15:04:05"),
		"task_error":    e.ErrorMsg,
		"task_result":   e.ResultMsg,
	}

	var payloadMap map[string]any
	if e.Payload != "" {
		if err := json.Unmarshal([]byte(e.Payload), &payloadMap); err == nil {
			body["payload"] = payloadMap
			ExtractUserFromMap(ctx, payloadMap, body)
		}
	}
	if e.Detail != "" {
		var detailMap map[string]any
		if err := json.Unmarshal([]byte(e.Detail), &detailMap); err == nil {
			body["detail"] = detailMap
			ExtractUserFromMap(ctx, detailMap, body)
		}
	}

	for _, event := range events {
		meta := do.EventMetadata{
			Key:         event.EventKey,
			Name:        event.Name,
			Description: "异步任务执行完毕触发的自动通知",
		}
		DefaultTrigger.Trigger(ctx, meta, body)
	}
}

// ExtractUserFromMap extracts user info from payload/detail into body map.
func ExtractUserFromMap(ctx context.Context, data, body map[string]any) {
	if u, exists := body["user"]; exists && u != nil {
		return
	}
	if user := LoadUserFromPayload(ctx, data); user != nil {
		body["user"] = user
	}
}

// ExtractUserID extracts a user ID from map keys.
func ExtractUserID(data map[string]any) (uint64, bool) {
	for _, k := range []string{"user_id", "userId", "uid"} {
		val, ok := data[k]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case float64:
			if v >= 0 {
				return uint64(v), true
			}
		case int:
			if v >= 0 {
				return uint64(v), true
			}
		case int64:
			if v >= 0 {
				return uint64(v), true
			}
		case uint64:
			return v, true
		case string:
			if id, err := strconv.ParseUint(v, 10, 64); err == nil {
				return id, true
			}
		}
	}
	return 0, false
}

// ExtractUsername extracts a username string from map keys.
func ExtractUsername(data map[string]any) string {
	for _, k := range []string{"username", "user_name"} {
		if val, ok := data[k]; ok && val != nil {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// EventTrigger represents the unified event trigger class.
type EventTrigger struct{}

// DefaultTrigger is the singleton instance of EventTrigger.
var DefaultTrigger = &EventTrigger{}

// Trigger receives event metadata and processes the event notification dispatch asynchronously.
func (t *EventTrigger) Trigger(ctx context.Context, meta do.EventMetadata, body map[string]any) {
	asyncCtx := context.WithoutCancel(ctx)
	util.Go(func() {
		if body == nil {
			body = make(map[string]any)
		}
		if _, hasUser := body["user"]; !hasUser || body["user"] == nil {
			body["user"] = GetSystemUser(asyncCtx)
		}

		eventPtr, err := dao.GetActivePushEventByKey(asyncCtx, meta.Key)
		if err != nil {
			if errors.Is(err, consts.ErrRecordNotFound) {
				return
			}
			logger.ErrorF(asyncCtx, "push_event_trigger: failed to get active event %s: %v", meta.Key, err)
			return
		}
		event := *eventPtr
		if len(event.Channels) == 0 {
			return
		}

		flatBody := GetFlatBody(body)
		msg, _ := t.buildMessage(&event, meta, flatBody, body)
		t.enqueuePushTasks(asyncCtx, meta, &event, msg, flatBody)
	})
}

func (t *EventTrigger) buildMessage(event *entity.PushEvent, meta do.EventMetadata, flatBody, body map[string]any) (do.NotificationMessage, string) {
	var msg do.NotificationMessage
	renderedTemplate := ""

	templateSource := event.Template
	if templateSource != "" {
		var err error
		msg, renderedTemplate, err = t.parseCustomTemplate(event, templateSource, flatBody)
		if err != nil {
			msg.Title = event.Name
			msg.Content = renderedTemplate
			msg.Level = consts.DefaultLevelInfo
		}
	} else {
		msg = t.parseDefaultTemplate(meta, flatBody)
	}

	if msg.Ext == nil {
		msg.Ext = make(map[string]any)
	}
	for k, v := range body {
		if k == consts.KeyTitle || k == consts.KeyContent || k == consts.KeyLevel {
			continue
		}
		if _, exists := msg.Ext[k]; !exists {
			msg.Ext[k] = v
		}
	}

	return msg, renderedTemplate
}

func (t *EventTrigger) parseCustomTemplate(event *entity.PushEvent, templateSource string, flatBody map[string]any) (do.NotificationMessage, string, error) {
	var msg do.NotificationMessage
	renderedTemplate := pkgpush.ParseTemplate(templateSource, flatBody)

	var tMap map[string]any
	if err := json.Unmarshal([]byte(renderedTemplate), &tMap); err != nil {
		return msg, renderedTemplate, err
	}

	if title, ok := tMap[consts.KeyTitle].(string); ok && title != "" {
		msg.Title = title
	} else {
		msg.Title = event.Name
	}
	delete(tMap, consts.KeyTitle)

	if content, ok := tMap[consts.KeyContent].(string); ok && content != "" {
		msg.Content = content
	} else {
		msg.Content = renderedTemplate
	}
	delete(tMap, consts.KeyContent)

	if level, ok := tMap[consts.KeyLevel].(string); ok && level != "" {
		msg.Level = level
	} else {
		msg.Level = consts.DefaultLevelInfo
	}
	delete(tMap, consts.KeyLevel)

	msg.Ext = tMap
	return msg, renderedTemplate, nil
}

func (t *EventTrigger) parseDefaultTemplate(meta do.EventMetadata, flatBody map[string]any) do.NotificationMessage {
	var msg do.NotificationMessage
	msg.Title = pkgpush.ParseTemplate(meta.DefaultTemplate.Title, flatBody)
	msg.Content = pkgpush.ParseTemplate(meta.DefaultTemplate.Content, flatBody)
	msg.Level = pkgpush.ParseTemplate(meta.DefaultTemplate.Level, flatBody)

	if meta.DefaultTemplate.Ext != nil {
		msg.Ext = make(map[string]any)
		for k, v := range meta.DefaultTemplate.Ext {
			if strVal, ok := v.(string); ok {
				msg.Ext[k] = pkgpush.ParseTemplate(strVal, flatBody)
			} else {
				msg.Ext[k] = v
			}
		}
	}
	return msg
}

func (t *EventTrigger) enqueuePushTasks(ctx context.Context, meta do.EventMetadata, event *entity.PushEvent, msg do.NotificationMessage, flatBody map[string]any) {
	for _, channelName := range event.Channels {
		customChannel, err := dao.GetActivePushChannelByName(ctx, channelName)
		if err == nil {
			t.enqueueCustomPushChannelTasks(ctx, meta, event, customChannel, msg, flatBody)
			continue
		}
		logger.WarnF(ctx, "push_event_trigger: channel %q not found in DB or disabled: %v", channelName, err)
	}
}

func (t *EventTrigger) enqueueCustomPushChannelTasks(ctx context.Context, meta do.EventMetadata, event *entity.PushEvent, channel *entity.PushChannel, msg do.NotificationMessage, flatBody map[string]any) {
	if len(event.Targets) == 0 {
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, "", msg)
		return
	}

	for _, target := range event.Targets {
		resolvedTarget := ResolveTarget(ctx, target, flatBody, channel.Name)
		t.enqueueSingleCustomPushChannelTask(ctx, meta, channel, resolvedTarget, msg)
	}
}

func (t *EventTrigger) enqueueSingleCustomPushChannelTask(ctx context.Context, meta do.EventMetadata, channel *entity.PushChannel, target string, msg do.NotificationMessage) {
	var config pkgpush.Config
	var renderedTemplate string

	switch channel.Type {
	case consts.ChannelLark:
		config = pkgpush.Config{Channel: consts.ChannelLark, URL: channel.URL, Secret: channel.Token}
		renderedTemplate = channel.Other
	case consts.ChannelEmail:
		url, token, other := ResolveSMTPConfig(ctx, channel.URL, channel.Token, channel.Other)
		config = pkgpush.Config{Channel: consts.ChannelEmail, URL: url, Key: token, Secret: other}
	case consts.ChannelTelegram:
		config = pkgpush.Config{Channel: consts.ChannelTelegram, URL: channel.URL, Secret: channel.Token, Key: channel.Other}
	default:
		config = pkgpush.Config{Channel: consts.ChannelCustom, URL: channel.URL}
		customPushReq := do.CustomPushRequest{
			Title:       msg.Title,
			Content:     msg.Content,
			Description: meta.Description,
			To:          target,
		}
		if urlVal, ok := msg.Ext["url"].(string); ok {
			customPushReq.URL = urlVal
		}
		renderedTemplate = RenderCustomPayload(channel.Other, customPushReq)
	}

	payload := do.SendPayload{
		EventKey: meta.Key,
		Config:   config,
		Target:   target,
		Body:     msg,
		Template: renderedTemplate,
	}
	if err := EnqueuePushTask(ctx, payload); err != nil {
		logger.ErrorF(ctx, "push_event_trigger: enqueuePushTask failed for %s channel %s -> %s: %v", channel.Type, channel.Name, target, err)
	}
}

// SyncEvents automatically registers/updates built-in events in the database.
func SyncEvents(ctx context.Context) error {
	return SyncBuiltInEvents(ctx)
}

// AdminLogin is the metadata definition for the admin login event.
var AdminLogin = do.EventMetadata{
	Key:  "admin_login",
	Name: "管理员登录",
	DefaultTemplate: do.NotificationMessage{
		Title:   "管理员登录提醒",
		Content: "管理员 {{user.username}} 于 {{time}} 从 IP {{ip}} 登录系统。",
		Level:   "INFO",
	},
	Description: "当管理员成功登录系统时触发此通知",
}

// HandleAdminLoggedIn 处理管理员登录事件并触发通知
func HandleAdminLoggedIn(ctx context.Context, event contracts.AdminLoggedIn) {
	if event.User == nil {
		return
	}

	body := map[string]any{
		"user": event.User,
		"ip":   event.IP,
		"time": time.Now().Format("2006-01-02 15:04:05"),
	}
	DefaultTrigger.Trigger(ctx, AdminLogin, body)
}

// RegisterCustomEvents registers default domain push notification events.
func RegisterCustomEvents() {
	RegisterBuiltInEvent(AdminLogin)
}
