// Package tui provides interactive terminal user interface views.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Common key and source constants used across views.
const (
	KeyEsc           = "esc"
	KeyEnter         = "enter"
	KeyDown          = "down"
	KeyQ             = "q"
	SourceModelScope = "modelscope"
	SourceHFMirror   = "hf-mirror"
	SourceHF         = "hf"
)

const (
	padV    = 1
	padH    = 2
	padLeft = 2
)

var (
	// Palette
	colorPrimary = lipgloss.Color("#06B6D4") // Cyan
	colorSuccess = lipgloss.Color("#10B981") // Green
	colorWarning = lipgloss.Color("#F59E0B") // Yellow
	colorDanger  = lipgloss.Color("#EF4444") // Red
	colorMuted   = lipgloss.Color("#64748B") // Slate
	colorBorder  = lipgloss.Color("#334155")
	colorWhite   = lipgloss.Color("#F8FAFC")

	// StyleApp defines the outer application padding.
	StyleApp = lipgloss.NewStyle().
			Padding(padV, padH)

	// StyleHeader defines the top banner style.
	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1).
			MarginBottom(1)

	// StyleSubtitle defines muted secondary text style.
	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginBottom(1)

	// StyleCard defines border and padding for structured cards.
	StyleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(padV, padH).
			MarginBottom(1)

	// StyleCardTitle defines card header titles.
	StyleCardTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	// StyleBadgeSuccess defines green status indicator.
	StyleBadgeSuccess = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSuccess)

	// StyleBadgeDanger defines red error/failure indicator.
	StyleBadgeDanger = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorDanger)

	// StyleBadgeWarning defines yellow warning/in-progress indicator.
	StyleBadgeWarning = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning)

	// StyleBadgeMuted defines gray inactive indicator.
	StyleBadgeMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	// StyleMenuItem defines regular menu item style.
	StyleMenuItem = lipgloss.NewStyle().
			PaddingLeft(padLeft)

	// StyleMenuItemSelected defines active highlighted menu item style.
	StyleMenuItemSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				PaddingLeft(1).
				SetString("❯ ")

	// StyleKeyHelp defines keyboard navigation help text.
	StyleKeyHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	// StyleLogBox defines bordered container for log viewing.
	StyleLogBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			Foreground(lipgloss.Color("#94A3B8"))
)

type actionFooterOptions struct {
	Active     bool
	ActiveMsg  string
	Spinner    string
	WaitTip    string
	Err        error
	ErrHelp    string
	Done       bool
	DoneMsg    string
	DoneHelp   string
	DefaultTip string
	DefHelp    string
}

func renderActionFooter(b *strings.Builder, opts actionFooterOptions) {
	switch {
	case opts.Active:
		b.WriteString(StyleBadgeWarning.Render(fmt.Sprintf("%s %s", opts.Spinner, opts.ActiveMsg)) + "\n\n")
		b.WriteString(StyleSubtitle.Render(opts.WaitTip) + "\n\n")
	case opts.Err != nil:
		b.WriteString(StyleBadgeDanger.Render(fmt.Sprintf("✗ 操作失败: %v", opts.Err)) + "\n\n")
		b.WriteString(StyleKeyHelp.Render(opts.ErrHelp))
	case opts.Done:
		b.WriteString(StyleBadgeSuccess.Render(opts.DoneMsg) + "\n\n")
		b.WriteString(StyleKeyHelp.Render(opts.DoneHelp))
	default:
		b.WriteString(StyleSubtitle.Render(opts.DefaultTip) + "\n\n")
		b.WriteString(StyleKeyHelp.Render(opts.DefHelp))
	}
}
