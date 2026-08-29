// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

// SessionConfig defines the session configuration declared by the auth plugin.
type SessionConfig struct {
	SessionCookieName string `config:"session_cookie_name" env:"APP_SESSION_COOKIE_NAME" default:"wavelet_session"`
	SessionSecret     string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
	SessionDomain     string `config:"session_domain" env:"APP_SESSION_DOMAIN"`
	SessionAge        int    `config:"session_age" env:"APP_SESSION_AGE" default:"86400"`
	SessionHTTPOnly   bool   `config:"session_http_only" env:"APP_SESSION_HTTP_ONLY" default:"true"`
	SessionSecure     bool   `config:"session_secure" env:"APP_SESSION_SECURE"`
}
