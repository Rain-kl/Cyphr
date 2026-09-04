package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cyphr/installer/internal/agent"
	"cyphr/installer/internal/config"
	"cyphr/installer/internal/model"
)

// ViewState defines which screen is currently visible.
type ViewState int

// View states supported by the TUI.
const (
	ViewMainMenu ViewState = iota
	ViewInstallAgent
	ViewUpdateMenu
	ViewStatusDashboard
	ViewDownloadCatalog
	ViewDownloadProgress
	ViewAgentLogs
	ViewDoctor
)

const (
	minVpHeight = 4
	minVpWidth  = 20
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
	paths     *config.AppPaths
	agentSvc  *agent.Service
	modelSvc  *model.Service
	state     ViewState
	lastState ViewState
	width     int
	height    int
	ready     bool
	viewport  viewport.Model
	spinner   spinner.Model
	statusMsg string
	err       error

	// Sub-view state
	menuIndex      int
	catalogIndex   int
	downloadSource string // "hf-mirror", "modelscope", "hf"
	downloadMirror string // backward-compat or alias
	downloadMode   string // "bg" or "fg"

	// Install view state
	installing      bool
	installDone     bool
	installMsg      string
	installErr      error
	installMirror   bool // use ghproxy
	installSkipVenv bool

	// Update view state
	updateTarget UpdateTarget
	updating     bool
	updateDone   bool
	updateMsg    string
	updateErr    error
	updateMirror bool
	doctorOutput string

	// Cached data
	agentStatus *agent.Status
	downStatus  *model.DownloadStatus
	localModels []model.LocalModel
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
		lastState:      ViewMainMenu,
		spinner:        s,
		downloadSource: SourceModelScope,
		downloadMirror: SourceModelScope,
		downloadMode:   "bg",
		installMirror:  true,
		updateMirror:   true,
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

func (m *Model) renderHeader() string {
	var b strings.Builder
	headerText := "  CYPHR AGENT MANAGER & INSTALLER  "
	b.WriteString(StyleHeader.Render(headerText) + "\n")

	if m.err != nil {
		b.WriteString(StyleBadgeDanger.Render(fmt.Sprintf(" ✗ 错误: %v", m.err)) + "\n\n")
	} else if m.statusMsg != "" {
		b.WriteString(StyleBadgeSuccess.Render(fmt.Sprintf(" ✓ %s", m.statusMsg)) + "\n\n")
	}
	return b.String()
}

func (m *Model) currentViewContent() string {
	switch m.state {
	case ViewMainMenu:
		return m.viewMainMenu()
	case ViewInstallAgent:
		return m.viewInstallView()
	case ViewUpdateMenu:
		return m.viewUpdateView()
	case ViewStatusDashboard:
		return m.viewStatusDashboard()
	case ViewDownloadCatalog:
		return m.viewDownloadCatalog()
	case ViewDownloadProgress:
		return m.viewDownloadProgress()
	case ViewAgentLogs:
		return m.viewAgentLogs()
	case ViewDoctor:
		return m.viewDoctorView()
	default:
		return ""
	}
}

func (m *Model) syncViewportSize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	headerHeight := lipgloss.Height(m.renderHeader())
	// Account for app padding (vertical: 2 lines) and footer (1 line)
	vpHeight := m.height - headerHeight - 3
	if vpHeight < minVpHeight {
		vpHeight = minVpHeight
	}
	vpWidth := m.width - 4
	if vpWidth < minVpWidth {
		vpWidth = minVpWidth
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
}

func (m *Model) syncViewportContent(preserveOffset bool) {
	if !m.ready {
		m.syncViewportSize()
	}
	content := m.currentViewContent()
	m.viewport.SetContent(content)

	if !preserveOffset {
		if m.state == ViewAgentLogs {
			m.viewport.GotoBottom()
		} else {
			m.viewport.GotoTop()
		}
	}
}

func (m Model) handleKeyNav(k string) (tea.Model, tea.Cmd, bool) {
	switch k {
	case "ctrl+c":
		return m, tea.Quit, true
	case "pgup", "ctrl+b":
		m.viewport.HalfViewUp()
		return m, nil, true
	case "pgdown", "ctrl+f":
		m.viewport.HalfViewDown()
		return m, nil, true
	case "home":
		m.viewport.GotoTop()
		return m, nil, true
	case "end":
		m.viewport.GotoBottom()
		return m, nil, true
	case KeyQ, KeyEsc:
		if m.state != ViewMainMenu {
			m.state = ViewMainMenu
			m.statusMsg = ""
			m.err = nil
			m.syncViewportContent(false)
			return m, nil, true
		}
		return m, tea.Quit, true
	}
	return m, nil, false
}

func (m Model) dispatchViewUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case ViewMainMenu:
		return m.updateMainMenu(msg)
	case ViewInstallAgent:
		return m.updateInstallView(msg)
	case ViewUpdateMenu:
		return m.updateUpdateView(msg)
	case ViewStatusDashboard:
		return m.updateStatusDashboard(msg)
	case ViewDownloadCatalog:
		return m.updateDownloadCatalog(msg)
	case ViewDownloadProgress:
		return m.updateDownloadProgress(msg)
	case ViewAgentLogs:
		return m.updateAgentLogs(msg)
	case ViewDoctor:
		return m.updateDoctorView(msg)
	default:
		return m, nil
	}
}

// Update handles messages and keybindings.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncViewportSize()
		m.syncViewportContent(true)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TickMsg:
		m.refreshData()
		m.syncViewportContent(true)
		cmds = append(cmds, tickCmd())

	case InstallStatusMsg:
		m.installing = false
		if msg.Err != nil {
			m.installErr = msg.Err
			m.installDone = false
		} else {
			m.installDone = true
			m.installErr = nil
			m.installMsg = "Agent 安装成功！"
		}
		m.refreshData()
		m.syncViewportContent(true)

	case UpdateProgressMsg:
		m.updating = false
		if msg.Err != nil {
			m.updateErr = msg.Err
			m.updateDone = false
		} else {
			m.updateDone = true
			m.updateErr = nil
			m.updateMsg = "更新成功！"
		}
		m.refreshData()
		m.syncViewportContent(true)

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if updatedM, cmd, handled := m.handleKeyNav(msg.String()); handled {
			return updatedM, cmd
		}

		updatedModel, cmd := m.dispatchViewUpdate(msg)
		if newM, ok := updatedModel.(Model); ok {
			m = newM
			if m.state != m.lastState {
				m.lastState = m.state
				m.syncViewportContent(false)
			} else {
				m.syncViewportContent(true)
			}
		}
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

// View renders the current active screen.
func (m Model) View() string {
	if !m.ready {
		m.syncViewportSize()
		m.syncViewportContent(true)
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString(m.viewport.View())

	return StyleApp.Render(b.String())
}
