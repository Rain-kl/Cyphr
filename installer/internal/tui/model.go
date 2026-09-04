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
	agentStatus *agent.AgentStatus
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
		downloadSource: "modelscope",
		downloadMirror: "modelscope",
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

func (m *Model) renderFooter() string {
	if m.viewport.TotalLineCount() <= m.viewport.Height {
		return ""
	}
	pct := int(m.viewport.ScrollPercent() * 100)
	footerText := fmt.Sprintf("─── [ 滚动位置: %d%% | 滚轮或 ↑/↓/PgUp/PgDn 翻页 ] ───", pct)
	return StyleSubtitle.Render(footerText)
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
	if vpHeight < 4 {
		vpHeight = 4
	}
	vpWidth := m.width - 4
	if vpWidth < 20 {
		vpWidth = 20
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
		// Mouse wheel scrolling
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "pgup", "ctrl+b":
			m.viewport.HalfViewUp()
			return m, nil
		case "pgdown", "ctrl+f":
			m.viewport.HalfViewDown()
			return m, nil
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "end":
			m.viewport.GotoBottom()
			return m, nil
		case "q", "esc":
			if m.state != ViewMainMenu {
				m.state = ViewMainMenu
				m.statusMsg = ""
				m.err = nil
				m.syncViewportContent(false)
				return m, nil
			}
			return m, tea.Quit
		}

		// Delegate to current view updater
		var updatedModel tea.Model
		var cmd tea.Cmd
		switch m.state {
		case ViewMainMenu:
			updatedModel, cmd = m.updateMainMenu(msg)
		case ViewInstallAgent:
			updatedModel, cmd = m.updateInstallView(msg)
		case ViewUpdateMenu:
			updatedModel, cmd = m.updateUpdateView(msg)
		case ViewStatusDashboard:
			updatedModel, cmd = m.updateStatusDashboard(msg)
		case ViewDownloadCatalog:
			updatedModel, cmd = m.updateDownloadCatalog(msg)
		case ViewDownloadProgress:
			updatedModel, cmd = m.updateDownloadProgress(msg)
		case ViewAgentLogs:
			updatedModel, cmd = m.updateAgentLogs(msg)
		case ViewDoctor:
			updatedModel, cmd = m.updateDoctorView(msg)
		default:
			updatedModel = m
		}

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

	footer := m.renderFooter()
	if footer != "" {
		b.WriteString("\n" + footer)
	}

	return StyleApp.Render(b.String())
}
