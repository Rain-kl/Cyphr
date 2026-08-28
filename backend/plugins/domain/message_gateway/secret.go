// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Rain-kl/Wavelet/backend/pkg/config"
	"github.com/Rain-kl/Wavelet/backend/pkg/util"
)

// CredentialKey is AES-256 hex derived from the session secret.
func CredentialKey() string {
	secret := ""
	if config.Config != nil {
		secret = config.Config.App.SessionSecret
	}
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// EncryptCredentials encrypts a credential map as JSON.
func EncryptCredentials(creds map[string]string) (string, error) {
	if creds == nil {
		creds = map[string]string{}
	}
	raw, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}
	return util.Encrypt(CredentialKey(), string(raw))
}

// DecryptCredentials decrypts a credential map.
func DecryptCredentials(ciphertext string) (map[string]string, error) {
	if ciphertext == "" {
		return map[string]string{}, nil
	}
	plain, err := util.Decrypt(CredentialKey(), ciphertext)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(plain), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

// ParseExtra decodes optional extra JSON into a string map.
func ParseExtra(raw string) map[string]string {
	if raw == "" {
		return map[string]string{}
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]string{}
	}
	return out
}

// EncodeExtra encodes extra fields as JSON.
func EncodeExtra(extra map[string]string) string {
	if extra == nil {
		return ""
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return ""
	}
	return string(raw)
}
