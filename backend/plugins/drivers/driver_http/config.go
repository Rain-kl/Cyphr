// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

type httpAppConfig struct {
	Addr                    string `config:"addr" env:"APP_ADDR" default:":8000"`
	AppName                 string `config:"app_name" env:"APP_NAME" default:"Wavelet"`
	APIPrefix               string `config:"api_prefix" env:"APP_API_PREFIX" default:"/api/v1"`
	Env                     string `config:"env" env:"APP_ENV" default:"development"`
	GracefulShutdownTimeout int    `config:"graceful_shutdown_timeout" env:"APP_GRACEFUL_SHUTDOWN_TIMEOUT" default:"30"`
	SessionCookieName       string `config:"session_cookie_name" env:"APP_SESSION_COOKIE_NAME" default:"wavelet_session"`
	SessionSecret           string `config:"session_secret" env:"APP_SESSION_SECRET" secret:"true"`
	SessionDomain           string `config:"session_domain" env:"APP_SESSION_DOMAIN"`
	SessionAge              int    `config:"session_age" env:"APP_SESSION_AGE" default:"86400"`
	SessionHTTPOnly         bool   `config:"session_http_only" env:"APP_SESSION_HTTP_ONLY" default:"true"`
	SessionSecure           bool   `config:"session_secure" env:"APP_SESSION_SECURE"`
	RedirectTrailingSlash   *bool  `config:"redirect_trailing_slash" env:"APP_REDIRECT_TRAILING_SLASH"`
}

type httpRedisConfig struct {
	Enabled     bool     `config:"enabled" env:"REDIS_ENABLED" default:"false"`
	Addrs       []string `config:"addrs" env:"REDIS_ADDR"`
	Username    string   `config:"username" env:"REDIS_USERNAME"`
	Password    string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB          int      `config:"db" env:"REDIS_DB"`
	KeyPrefix   string   `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	MinIdleConn int      `config:"min_idle_conn" env:"REDIS_MIN_IDLE_CONN"`
}
