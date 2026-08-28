// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"Wavelet/core/contracts"
)

var authSourceNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

// AuthSource 认证源实体
//
//nolint:revive // auth.AuthSource is standard domain entity name
type AuthSource struct {
	ID                     uint64    `json:"id" gorm:"primaryKey"`
	Name                   string    `json:"name" gorm:"uniqueIndex;size:80;not null"`
	Type                   string    `json:"type" gorm:"size:20;not null"`
	DisplayName            string    `json:"display_name" gorm:"size:100"`
	IsActive               bool      `json:"is_active" gorm:"index;not null;default:false"`
	ClientID               string    `json:"client_id" gorm:"size:255"`
	ClientSecret           string    `json:"-" gorm:"size:1024"`
	OpenIDDiscoveryURL     string    `json:"openid_discovery_url" gorm:"column:openid_discovery_url;size:1024"`
	Scopes                 string    `json:"scopes" gorm:"size:255"`
	IconURL                string    `json:"icon_url" gorm:"size:1024"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	ClientSecretConfigured bool      `json:"client_secret_configured" gorm:"-"`
}

// TableName 表名
func (AuthSource) TableName() string {
	return "w_auth_sources"
}

// Normalize 对认证源字段进行标准化处理
func (source *AuthSource) Normalize() {
	source.Type = strings.ToLower(strings.TrimSpace(source.Type))
	source.Name = strings.TrimSpace(source.Name)
	source.DisplayName = strings.TrimSpace(source.DisplayName)
	source.ClientID = strings.TrimSpace(source.ClientID)
	source.ClientSecret = strings.TrimSpace(source.ClientSecret)
	source.OpenIDDiscoveryURL = strings.TrimSpace(source.OpenIDDiscoveryURL)
	source.Scopes = strings.TrimSpace(source.Scopes)
	source.IconURL = strings.TrimSpace(source.IconURL)
	if source.DisplayName == "" {
		source.DisplayName = source.Name
	}
	if source.Type == AuthSourceTypeOIDC && source.Scopes == "" {
		source.Scopes = "openid profile email"
	}
}

// Validate 校验认证源字段合法性
func (source *AuthSource) Validate() error {
	source.Normalize()
	if source.Name == "" {
		return errors.New(errAuthSourceNameRequired)
	}
	if !authSourceNamePattern.MatchString(source.Name) {
		return errors.New(errAuthSourceNameInvalid)
	}
	if source.Type != AuthSourceTypeOIDC {
		return errors.New(errAuthSourceTypeUnsupported)
	}
	if source.OpenIDDiscoveryURL == "" {
		//nolint:staticcheck // descriptive error constant
		return errors.New(errAuthSourceDiscoveryURLRequired)
	}
	if source.IsActive && (source.ClientID == "" || source.ClientSecret == "") {
		return errors.New(errAuthSourceClientCredentialsRequired)
	}
	return nil
}

// Sanitize 脱敏处理，将 ClientSecret 清空并设置 ClientSecretConfigured 标志
func (source *AuthSource) Sanitize() {
	source.ClientSecretConfigured = source.ClientSecret != ""
	source.ClientSecret = ""
}

// ExternalAccount 外部账号绑定实体
type ExternalAccount struct {
	ID               uint64    `json:"id" gorm:"primaryKey"`
	AuthSourceID     uint64    `json:"auth_source_id" gorm:"uniqueIndex:idx_external_accounts_source_external,priority:1;index"`
	UserID           uint64    `json:"user_id" gorm:"index;not null"`
	ExternalID       string    `json:"external_id" gorm:"uniqueIndex:idx_external_accounts_source_external,priority:2;size:255;not null"`
	ExternalUsername string    `json:"external_username" gorm:"size:255"`
	Email            string    `json:"email" gorm:"size:255"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 表名
func (ExternalAccount) TableName() string {
	return "w_external_accounts"
}

// ExternalAccountView 外部帐号绑定视图（脱敏展示用）
type ExternalAccountView struct {
	ID               uint64    `json:"id"`
	AuthSourceID     uint64    `json:"auth_source_id"`
	AuthSourceName   string    `json:"auth_source_name"`
	AuthSourceType   string    `json:"auth_source_type"`
	AuthSourceLabel  string    `json:"auth_source_label"`
	ExternalUsername string    `json:"external_username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
}

// AuthSourceView 登录源展示信息
//
//nolint:revive // auth.AuthSourceView is standard domain presentation struct
type AuthSourceView struct {
	ID                     uint64 `json:"id"`
	Name                   string `json:"name"`
	Type                   string `json:"type"`
	DisplayName            string `json:"display_name"`
	IsActive               bool   `json:"is_active"`
	IconURL                string `json:"icon_url"`
	ClientSecretConfigured bool   `json:"client_secret_configured"`
}

// OAuthAuthorizeResponse 授权 URL 响应
type OAuthAuthorizeResponse struct {
	AuthorizeURL string `json:"authorize_url"`
}

// OAuthCallbackResult 回调处理结果
type OAuthCallbackResult struct {
	Status string         `json:"status"`
	User   *BasicUserInfo `json:"user,omitempty"`
}

// CallbackRequest OAuth 回调请求参数
type CallbackRequest struct {
	State string `json:"state" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// BasicUserInfo 用户基本信息结构体
type BasicUserInfo struct {
	ID                 uint64 `json:"id"`
	Username           string `json:"username"`
	Nickname           string `json:"nickname"`
	Email              string `json:"email"`
	AvatarURL          string `json:"avatar_url"`
	IsAdmin            bool   `json:"is_admin"`
	NeedChangePassword bool   `json:"need_change_password"`
	Bio                string `json:"bio"`
	Phone              string `json:"phone"`
	Gender             string `json:"gender"`
	Website            string `json:"website"`
	Location           string `json:"location"`
}

// BuildBasicUserInfo 将 UserDTO 转换为 BasicUserInfo
func BuildBasicUserInfo(user *contracts.UserDTO, needChange bool) BasicUserInfo {
	if user == nil {
		return BasicUserInfo{}
	}
	return BasicUserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		Nickname:           user.Nickname,
		Email:              user.Email,
		AvatarURL:          user.AvatarURL,
		IsAdmin:            user.IsAdmin,
		NeedChangePassword: needChange,
		Bio:                user.Bio,
		Phone:              user.Phone,
		Gender:             user.Gender,
		Website:            user.Website,
		Location:           user.Location,
	}
}

type oauthStatePayload struct {
	SourceName  string `json:"source_name"`
	Purpose     string `json:"purpose"`
	UserID      uint64 `json:"user_id,omitempty"`
	SessionHash string `json:"session_hash"`
}

func encodeOAuthStatePayload(payload oauthStatePayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeOAuthStatePayload(value string) (oauthStatePayload, error) {
	var payload oauthStatePayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return oauthStatePayload{}, err
	}
	return payload, nil
}

type loginRequiredAuditLog struct {
	UserID     uint64 `json:"user_id"`
	Username   string `json:"username"`
	ClientIP   string `json:"client_ip"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	RequestURI string `json:"request_uri"`
	UserAgent  string `json:"user_agent"`
	Referer    string `json:"referer"`
}

// ParseUserID parses a string or float64 user ID representation.
func ParseUserID(v any) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		if val > 0 {
			return uint64(val)
		}
	case int:
		if val > 0 {
			return uint64(val)
		}
	case float64:
		if val > 0 {
			return uint64(val)
		}
	case string:
		if id, err := strconv.ParseUint(val, 10, 64); err == nil {
			return id
		}
	}
	return 0
}
