// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cmd provides CLI command entry points.
//
//nolint:unused
package cmd

import (
	"Wavelet/pkg/buildinfo"
	"Wavelet/pkg/config"
	"fmt"
	"runtime"
	"strings"
)

//nolint:unused // startup banner formatting utilities
type startupState struct {
	mode           string
	listensForHTTP bool
}

func printStartupBanner(state startupState) {
	fmt.Println(formatStartupBanner(state))
}

func formatStartupBanner(state startupState) string {
	lines := []string{
		"",
		"__        __                _      _   ",
		"\\ \\      / /_ ___   _____  | | ___| |_ ",
		" \\ \\ /\\ / / _` \\ \\ / / _ \\ | |/ _ \\ __|",
		"  \\ V  V / (_| |\\ V /  __/ | |  __/ |_ ",
		"   \\_/\\_/ \\__,_| \\_/ \\___|_|\\___|\\__|",
		fmt.Sprintf(" Wavelet %s", buildinfo.Version),
		"",
		fmt.Sprintf(" Environment: %s", config.Config.App.Env),
		fmt.Sprintf(" Runtime:     %s/%s (%s)", runtime.GOOS, runtime.GOARCH, runtime.Version()),
		fmt.Sprintf(" Build time:  %s", buildTime()),
	}
	if state.listensForHTTP {
		lines = append(lines, fmt.Sprintf(" Listening:   http://%s", config.Config.App.Addr))
	}
	lines = append(lines, fmt.Sprintf(" Mode:        %s", state.mode), "")
	return strings.Join(lines, "\n")
}

func buildTime() string {
	if buildinfo.BuildTime == "" {
		return "development build"
	}
	return buildinfo.BuildTime
}
