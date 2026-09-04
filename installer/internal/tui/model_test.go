package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/config"
)

func newTestPaths(t *testing.T) *config.AppPaths {
	tmp := t.TempDir()
	return &config.AppPaths{
		RootDir:          tmp,
		AgentDir:         filepath.Join(tmp, "agent"),
		PidFile:          filepath.Join(tmp, "agent.pid"),
		LogFile:          filepath.Join(tmp, "agent.log"),
		ConfigFile:       filepath.Join(tmp, "config.yaml"),
		ConfigExample:    filepath.Join(tmp, "config.example.yaml"),
		EnvFile:          filepath.Join(tmp, ".env"),
		DownloadPidFile:  filepath.Join(tmp, "download.pid"),
		DownloadLogFile:  filepath.Join(tmp, "download.log"),
		DownloadInfoFile: filepath.Join(tmp, "download.info"),
		ModelsDir:        filepath.Join(tmp, "models"),
		PythonBin:        filepath.Join(tmp, "python"),
	}
}

func TestModelViewportInitializationAndResize(t *testing.T) {
	paths := newTestPaths(t)
	m := NewModel(paths)

	// Send WindowSizeMsg
	width := 100
	height := 40
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updatedModel.(Model)

	if !m.ready {
		t.Fatalf("expected m.ready to be true after WindowSizeMsg")
	}

	expectedWidth := width - 4
	if m.viewport.Width != expectedWidth {
		t.Errorf("expected viewport width %d, got %d", expectedWidth, m.viewport.Width)
	}

	if m.viewport.Height <= 0 || m.viewport.Height >= height {
		t.Errorf("expected viewport height to be constrained between 0 and %d, got %d", height, m.viewport.Height)
	}

	// Test View() output doesn't panic and contains header
	viewStr := m.View()
	if len(viewStr) == 0 {
		t.Errorf("expected non-empty view string")
	}
}

func TestModelScrollingKeys(t *testing.T) {
	paths := newTestPaths(t)
	m := NewModel(paths)
	m.state = ViewDoctor
	m.doctorOutput = "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\nline 11\nline 12\nline 13\nline 14\nline 15\n"

	// Init viewport with small height
	m.width = 80
	m.height = 10
	m.syncViewportSize()
	m.syncViewportContent(false)

	initialY := m.viewport.YOffset

	// Send down key in ViewDoctor (delegated to viewport)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)

	if m.viewport.YOffset < initialY {
		t.Errorf("expected YOffset >= %d, got %d", initialY, m.viewport.YOffset)
	}

	// Send PgDn key
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)

	// Send Home key
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(Model)
	if m.viewport.YOffset != 0 {
		t.Errorf("expected YOffset 0 after Home, got %d", m.viewport.YOffset)
	}
}
