// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config manages configuration file reading, writing, and environment overrides for the transcribe CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultControllerURL is the default fallback URL for the transcribe controller.
	DefaultControllerURL = "http://localhost:8080"
	// DefaultModel is the default transcription model.
	DefaultModel = "mock-whisper-base"

	// EnvTranscribeURL overrides controller_url from environment.
	EnvTranscribeURL = "TRANSCRIBE_URL"
	// EnvTranscribeToken overrides access_token from environment.
	//nolint:gosec // G101: Environment variable name, not a hardcoded credential
	EnvTranscribeToken = "TRANSCRIBE_TOKEN"
	// EnvTranscribeModel overrides default_model from environment.
	EnvTranscribeModel = "TRANSCRIBE_MODEL"
	// EnvTranscribeConfig specifies custom config file path.
	EnvTranscribeConfig = "TRANSCRIBE_CONFIG"

	dirPerm  = 0o700
	filePerm = 0o600
)

// Config holds client configurations.
type Config struct {
	ControllerURL string `yaml:"controller_url"`
	AccessToken   string `yaml:"access_token"`
	DefaultModel  string `yaml:"default_model"`
}

// DefaultConfigPath returns the standard path ~/.transcribe/config.yaml.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".transcribe", "config.yaml"), nil
}

// ResolvePath determines the config path given an optional input, environment, or default path.
func ResolvePath(paths ...string) (string, error) {
	if len(paths) > 0 && strings.TrimSpace(paths[0]) != "" {
		return filepath.Clean(paths[0]), nil
	}
	if envPath := strings.TrimSpace(os.Getenv(EnvTranscribeConfig)); envPath != "" {
		return filepath.Clean(envPath), nil
	}
	defaultPath, err := DefaultConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Clean(defaultPath), nil
}

// Load loads the configuration from file and applies environment variable overrides.
func Load(paths ...string) (*Config, error) {
	targetPath, err := ResolvePath(paths...)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		ControllerURL: DefaultControllerURL,
		DefaultModel:  DefaultModel,
	}

	//nolint:gosec // G304: Configuration path supplied by user flag or standard home directory
	data, err := os.ReadFile(targetPath)
	if err == nil {
		if unmarshalErr := yaml.Unmarshal(data, cfg); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", targetPath, unmarshalErr)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read config file %s: %w", targetPath, err)
	}

	// Apply environment variable overrides
	if envURL := strings.TrimSpace(os.Getenv(EnvTranscribeURL)); envURL != "" {
		cfg.ControllerURL = envURL
	}
	if envToken := strings.TrimSpace(os.Getenv(EnvTranscribeToken)); envToken != "" {
		cfg.AccessToken = envToken
	}
	if envModel := strings.TrimSpace(os.Getenv(EnvTranscribeModel)); envModel != "" {
		cfg.DefaultModel = envModel
	}

	// Normalize fields
	cfg.ControllerURL = strings.TrimRight(strings.TrimSpace(cfg.ControllerURL), "/")
	cfg.AccessToken = strings.TrimSpace(cfg.AccessToken)
	cfg.DefaultModel = strings.TrimSpace(cfg.DefaultModel)
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = DefaultModel
	}

	return cfg, nil
}

// Save writes the configuration to file, ensuring parent directory exists.
func Save(cfg *Config, paths ...string) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}

	targetPath, err := ResolvePath(paths...)
	if err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	//nolint:gosec // G117: Config intentionally stores user token
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(targetPath, data, filePerm); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", targetPath, err)
	}

	return nil
}
