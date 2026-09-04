package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/agent"
	"cyphr/installer/internal/config"
	"cyphr/installer/internal/model"
)

// ViewState defines which screen is currently visible.
type ViewState int

const (
	ViewMainMenu ViewState = iota
	ViewStatusDashboard
	ViewDownloadCatalog
	ViewDownloadProgress
	ViewAgentLogs
)

// TickMsg triggers periodic state refresh (e.g. status & log tailing).
type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Model is the root Bubble Tea application model.
type Model struct {
	paths       *config.AppPaths
	agentSvc    *agent.Service
	modelSvc    *model.Service
	state       ViewState
	prevView    ViewState
	width       int
	height      int
	spinner     spinner.Model
	statusMsg   string
	isBusy      bool
	err         error

	// Sub-view state
	menuIndex          int
	catalogIndex       int
	downloadMirror     string // "hf-mirror" or "hf"
	downloadMode       string // "bg" or "fg"
	customModelInput   string
	isCustomModelInput bool

	// Cached data
	agentStatus  *agent.AgentStatus
	downStatus   *model.DownloadStatus
	localModels  []model.LocalModel
}

// NewModel creates a new root TUI Model.
func NewModel(paths *config.AppPaths) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = StyleBadgeSuccess

	agentSvc := agent.NewService(paths)
	modelSvc := model.NewService(paths)

	m := Model{
		paths:          paths,
		agentSvc:       agentSvc,
		modelSvc:       modelSvc,
		state:          ViewMainMenu,
		spinner:        s,
		downloadMirror: "hf-mirror",
		downloadMode:   "bg",
	}
	m.refreshData()
	return m
}

func (m *Model) refreshData() {
	m.agentStatus = m.agentSvc.Status()
	m.downStatus = m.modelSvc.Status()
	m.localModels, _ = m.modelSvc.ListLocalModels()
}

// Init sets up initial commands (spinner ticks, status timer).
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

// Update handles messages and keybindings.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TickMsg:
		m.refreshData()
		cmds = append(cmds, tickCmd())

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q", "esc":
			if m.state != ViewMainMenu {
				m.state = ViewMainMenu
				m.statusMsg = ""
				m.err = nil
				return m, nil
			}
			return m, tea.Quit
		}

		// Delegate to current view updater
		switch m.state {
		case ViewMainMenu:
			return m.updateMainMenu(msg)
		case ViewStatusDashboard:
			return m.updateStatusDashboard(msg)
		case ViewDownloadCatalog:
			return m.updateDownloadCatalog(msg)
		case ViewDownloadProgress:
			return m.updateDownloadProgress(msg)
		case ViewAgentLogs:
			return m.updateAgentLogs(msg)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the current active screen.
func (m Model) View() string {
	var b strings.Builder

	// Top App Header
	headerText := fmt.Sprintf("  CYPHR AGENT MANAGER & INSTALLER  ")
	b.WriteString(StyleHeader.Render(headerText) + "\n")

	// Notice or Error Banner
	if m.err != nil {
		b.WriteString(StyleBadgeDanger.Render(fmt.Sprintf(" ✗ 错误: %v", m.err)) + "\n\n")
	} else if m.statusMsg != "" {
		b.WriteString(StyleBadgeSuccess.Render(fmt.Sprintf(" ✓ %s", m.statusMsg)) + "\n\n")
	}

	switch m.state {
	case ViewMainMenu:
		b.WriteString(m.viewMainMenu())
	case ViewStatusDashboard:
		b.WriteString(m.viewStatusDashboard())
	case ViewDownloadCatalog:
		b.WriteString(m.viewDownloadCatalog())
	case ViewDownloadProgress:
		b.WriteString(m.viewDownloadProgress())
	case ViewAgentLogs:
		b.WriteString(m.viewAgentLogs())
	}

	return StyleApp.Render(b.String())
}
