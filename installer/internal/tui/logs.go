package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/proc"
)

const maxLogLines = 100

func (m Model) updateAgentLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyEsc, KeyQ:
		m.state = ViewMainMenu
		return m, nil
	case "r":
		m.refreshData()
		m.syncViewportContent(false)
		return m, nil
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) viewAgentLogs() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("📋 Agent 服务运行日志 (实时自动刷新)") + "\n\n")

	lines := proc.TailLines(m.paths.LogFile, maxLogLines)
	if len(lines) == 0 {
		b.WriteString(StyleBadgeMuted.Render("暂无服务日志输出。") + "\n\n")
	} else {
		b.WriteString(StyleLogBox.Render(strings.Join(lines, "\n")) + "\n\n")
	}

	b.WriteString(StyleKeyHelp.Render(fmt.Sprintf("日志路径: %s   [Esc/q] 返回主菜单   [↑/↓, j/k, PgUp/PgDn] 翻页滚动   [鼠标划选] 复制文本   [r] 立即刷新", m.paths.LogFile)))
	return b.String()
}
