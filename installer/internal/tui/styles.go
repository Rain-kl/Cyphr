package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Palette
	colorPrimary = lipgloss.Color("#06B6D4") // Cyan
	colorSuccess = lipgloss.Color("#10B981") // Green
	colorWarning = lipgloss.Color("#F59E0B") // Yellow
	colorDanger  = lipgloss.Color("#EF4444") // Red
	colorMuted   = lipgloss.Color("#64748B") // Slate
	colorBorder  = lipgloss.Color("#334155")
	colorBgDark  = lipgloss.Color("#0F172A")
	colorWhite   = lipgloss.Color("#F8FAFC")

	// Base styles
	StyleApp = lipgloss.NewStyle().
			Padding(1, 2)

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1).
			MarginBottom(1)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginBottom(1)

	StyleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			MarginBottom(1)

	StyleCardTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	StyleBadgeSuccess = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSuccess)

	StyleBadgeDanger = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorDanger)

	StyleBadgeWarning = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWarning)

	StyleBadgeMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleMenuItem = lipgloss.NewStyle().
			PaddingLeft(2)

	StyleMenuItemSelected = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary).
				PaddingLeft(1).
				SetString("❯ ")

	StyleKeyHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	StyleLogBox = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1).
			Foreground(lipgloss.Color("#94A3B8"))
)
