// Package cmd provides CLI command entry points.
//
//nolint:unused
package cmd

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
)

//nolint:unused // startup banner formatting utilities
type startupState struct {
	mode           string
	relationalDB   migrator.Report
	clickHouseDB   migrator.Report
	listensForHTTP bool
}

func printStartupBanner(state startupState) {
	log.Print(formatStartupBanner(state))
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
		fmt.Sprintf(" Database:    %s", formatMigration(state.relationalDB)),
		fmt.Sprintf(" Analytics:   %s", formatMigration(state.clickHouseDB)),
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

func formatMigration(report migrator.Report) string {
	if !report.Enabled {
		return "disabled"
	}
	state := "up to date"
	if report.Applied {
		state = "upgraded"
	}
	return fmt.Sprintf("%s (version %d, %s)", report.Backend, report.Version, state)
}
