// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultCapChallengeCount      = 1
	defaultCapChallengeSize       = 32
	defaultCapChallengeDifficulty = 4
	defaultCapChallengeTTL        = 10 * time.Minute
	defaultCapTokenTTL            = 20 * time.Minute
)

// CapRuntimeSettings is the parsed CAPTCHA runtime configuration loaded from system_configs.
type CapRuntimeSettings struct {
	LoginEnabled        bool
	ChallengeCount      int
	ChallengeSize       int
	ChallengeDifficulty int
	ChallengeTTL        time.Duration
	TokenTTL            time.Duration
}

// CAP 动态配置键常量
const (
	ConfigKeyCapLoginEnabled        = "cap_login_enabled"
	ConfigKeyCapChallengeCount      = "cap_challenge_count"
	ConfigKeyCapChallengeSize       = "cap_challenge_size"
	ConfigKeyCapChallengeDifficulty = "cap_challenge_difficulty"
	ConfigKeyCapChallengeTTL        = "cap_challenge_ttl"
	// ConfigKeyCapTokenTTL 验证码 Token 过期时间键
	// #nosec G101
	ConfigKeyCapTokenTTL = "cap_token_ttl"
)

var capRuntimeConfigKeys = []string{
	ConfigKeyCapLoginEnabled,
	ConfigKeyCapChallengeCount,
	ConfigKeyCapChallengeSize,
	ConfigKeyCapChallengeDifficulty,
	ConfigKeyCapChallengeTTL,
	ConfigKeyCapTokenTTL,
}

var capRuntimeConfigKeySet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(capRuntimeConfigKeys))
	for _, key := range capRuntimeConfigKeys {
		set[key] = struct{}{}
	}
	return set
}()

type capRuntimeSettingsStore struct {
	snapshot  atomic.Pointer[CapRuntimeSettings]
	loadGroup singleflight.Group
}

var capSettingsStore = &capRuntimeSettingsStore{}

// IsCapRuntimeConfigKey reports whether a system config key affects CAPTCHA runtime settings.
func IsCapRuntimeConfigKey(key string) bool {
	_, ok := capRuntimeConfigKeySet[key]
	return ok
}

// CurrentCapSettings returns the cached CAPTCHA runtime settings snapshot.
func CurrentCapSettings(ctx context.Context) (CapRuntimeSettings, error) {
	return capSettingsStore.current(ctx)
}

// CapProtectionEnabled reports whether CAPTCHA verification is required for protected routes.
func CapProtectionEnabled(ctx context.Context) bool {
	settings, err := CurrentCapSettings(ctx)
	if err != nil {
		return false
	}
	return settings.LoginEnabled
}

// InvalidateCapRuntimeSettings drops the in-process CAPTCHA settings snapshot.
func InvalidateCapRuntimeSettings() {
	capSettingsStore.snapshot.Store(nil)
}

// ResetCapRuntimeSettingsForTest clears the CAPTCHA runtime snapshot.
func ResetCapRuntimeSettingsForTest() {
	InvalidateCapRuntimeSettings()
}

// InstallCapTestRuntimeSettings installs a fixed snapshot for unit tests.
func InstallCapTestRuntimeSettings(settings CapRuntimeSettings) func() {
	snapshot := settings
	capSettingsStore.snapshot.Store(&snapshot)
	return InvalidateCapRuntimeSettings
}

func (s *capRuntimeSettingsStore) current(ctx context.Context) (CapRuntimeSettings, error) {
	if snapshot := s.snapshot.Load(); snapshot != nil {
		return *snapshot, nil
	}

	loaded, err, _ := s.loadGroup.Do("cap-runtime-settings", func() (any, error) {
		if snapshot := s.snapshot.Load(); snapshot != nil {
			return *snapshot, nil
		}

		settings, loadErr := loadCapRuntimeSettings(ctx)
		if loadErr != nil {
			return CapRuntimeSettings{}, loadErr
		}

		s.snapshot.Store(&settings)
		return settings, nil
	})
	if err != nil {
		return CapRuntimeSettings{}, err
	}

	settings, ok := loaded.(CapRuntimeSettings)
	if !ok {
		return CapRuntimeSettings{}, errors.New("cap runtime settings loader returned unexpected type")
	}
	return settings, nil
}

func loadCapRuntimeSettings(ctx context.Context) (CapRuntimeSettings, error) {
	var records []capConfigRecord
	db := getDB(ctx)
	if db == nil {
		return parseCapRuntimeSettings(nil), nil
	}
	if err := db.Table("w_system_configs").Where("key IN ?", capRuntimeConfigKeys).Find(&records).Error; err != nil {
		return CapRuntimeSettings{}, err
	}
	configs := make(map[string]string, len(records))
	for _, r := range records {
		configs[r.Key] = r.Value
	}
	return parseCapRuntimeSettings(configs), nil
}

func parseCapRuntimeSettings(configs map[string]string) CapRuntimeSettings {
	settings := CapRuntimeSettings{
		ChallengeCount:      defaultCapChallengeCount,
		ChallengeSize:       defaultCapChallengeSize,
		ChallengeDifficulty: defaultCapChallengeDifficulty,
		ChallengeTTL:        defaultCapChallengeTTL,
		TokenTTL:            defaultCapTokenTTL,
	}

	if len(configs) == 0 {
		return settings
	}

	if val, ok := configs[ConfigKeyCapLoginEnabled]; ok {
		if enabled, err := strconv.ParseBool(val); err == nil {
			settings.LoginEnabled = enabled
		}
	}
	if val, ok := configs[ConfigKeyCapChallengeCount]; ok {
		if count, err := strconv.Atoi(val); err == nil && count > 0 {
			settings.ChallengeCount = count
		}
	}
	if val, ok := configs[ConfigKeyCapChallengeSize]; ok {
		if size, err := strconv.Atoi(val); err == nil && size > 0 {
			settings.ChallengeSize = size
		}
	}
	if val, ok := configs[ConfigKeyCapChallengeDifficulty]; ok {
		if diff, err := strconv.Atoi(val); err == nil && diff > 0 {
			settings.ChallengeDifficulty = diff
		}
	}
	if val, ok := configs[ConfigKeyCapChallengeTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(val); err == nil && ttlSeconds > 0 {
			settings.ChallengeTTL = time.Duration(ttlSeconds) * time.Second
		}
	}
	if val, ok := configs[ConfigKeyCapTokenTTL]; ok {
		if ttlSeconds, err := strconv.Atoi(val); err == nil && ttlSeconds > 0 {
			settings.TokenTTL = time.Duration(ttlSeconds) * time.Second
		}
	}

	return settings
}
