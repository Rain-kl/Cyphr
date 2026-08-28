// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/pkg/config"
	db "github.com/Rain-kl/Wavelet/plugins/infra/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gsessions "github.com/gorilla/sessions"
)

// GetSessionOptions 根据配置构建 Session 选项
func GetSessionOptions(maxAge int) sessions.Options {
	return sessions.Options{
		Path:     "/",
		Domain:   config.Config.App.SessionDomain,
		MaxAge:   maxAge,
		HttpOnly: config.Config.App.SessionHTTPOnly,
		Secure:   config.Config.App.SessionSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// StripCookieMaxAgeAndExpires 从 Set-Cookie 响应头中移除 Max-Age 和 Expires，从而使其成为浏览器会话 Cookie
func StripCookieMaxAgeAndExpires(header http.Header, cookieName string) {
	headers := header["Set-Cookie"]
	if len(headers) == 0 {
		return
	}

	newHeaders := make([]string, 0, len(headers))
	for _, h := range headers {
		if strings.HasPrefix(h, cookieName+"=") {
			parts := strings.Split(h, ";")
			newParts := make([]string, 0, len(parts))
			for _, p := range parts {
				trimmed := strings.TrimSpace(p)
				lower := strings.ToLower(trimmed)
				if strings.HasPrefix(lower, "max-age=") || strings.HasPrefix(lower, "expires=") {
					continue
				}
				newParts = append(newParts, p)
			}
			newHeaders = append(newHeaders, strings.Join(newParts, ";"))
		} else {
			newHeaders = append(newHeaders, h)
		}
	}
	header["Set-Cookie"] = newHeaders
}

// GetUserIDFromSession 从 Session 中提取用户 ID
func GetUserIDFromSession(s sessions.Session) uint64 {
	val := s.Get(UserIDKey)
	return ParseUserID(val)
}

// GetUserIDFromContext 从 Gin Context 的 Session 中提取用户 ID
func GetUserIDFromContext(c *gin.Context) (uid uint64) {
	defer func() {
		_ = recover()
	}()
	session := sessions.Default(c)
	return GetUserIDFromSession(session)
}

func ensureSessionToken(s sessions.Session) (string, bool) {
	token, ok := s.Get(SessionTokenKey).(string)
	if !ok || token == "" {
		token = uuid.NewString()
		s.Set(SessionTokenKey, token)
		return token, true
	}
	return token, false
}

func hashSessionToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func rotateSessionID(s sessions.Session) {
	if inner, ok := s.(interface{ Session() *gsessions.Session }); ok {
		if sess := inner.Session(); sess != nil {
			sess.ID = ""
		}
	}
}

// SetLoginSession writes the authenticated user into a freshly rotated session.
func SetLoginSession(ctx context.Context, c *gin.Context, user *contracts.UserDTO, extras ...map[string]any) error {
	session := sessions.Default(c)
	session.Clear()
	rotateSessionID(session)

	session.Set(UserIDKey, user.ID)
	session.Set(UserNameKey, user.Username)
	if len(extras) > 0 {
		for key, value := range extras[0] {
			session.Set(key, value)
		}
	}

	// 根据系统配置动态设置 Session 过期时间
	maxAge := config.Config.App.SessionAge
	isSessionCookie := false

	var val string
	if err := db.DB(ctx).Table("w_system_configs").Where("key = ?", "login_session_ttl_hours").Pluck("value", &val).Error; err == nil && val != "" {
		if ttlHours, err := strconv.Atoi(val); err == nil {
			switch {
			case ttlHours == -1:
				// 永不过期，设置为 10 年
				maxAge = 10 * 365 * 24 * 3600
			case ttlHours > 0:
				maxAge = ttlHours * 3600
			case ttlHours == 0:
				isSessionCookie = true
			}
		}
	}
	session.Options(GetSessionOptions(maxAge))

	if err := session.Save(); err != nil {
		return err
	}

	if isSessionCookie {
		StripCookieMaxAgeAndExpires(c.Writer.Header(), config.Config.App.SessionCookieName)
	}

	return nil
}
