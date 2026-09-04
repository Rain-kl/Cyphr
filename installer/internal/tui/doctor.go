package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/doctor"
)

func (m Model) updateDoctorView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = ViewMainMenu
		return m, nil
	case "r":
		m.doctorOutput = doctor.Run(m.paths).Format()
		m.syncViewportContent(true)
		return m, nil
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) viewDoctorView() string {
	var b strings.Builder
	if m.doctorOutput == "" {
		b.WriteString(StyleBadgeWarning.Render("正在探测软硬件环境与 PyTorch CUDA 状态，请稍候...") + "\n\n")
	} else {
		b.WriteString(m.doctorOutput + "\n")
	}
	b.WriteString(StyleKeyHelp.Render("[Esc/q] 返回主菜单   [↑/↓, j/k, PgUp/PgDn, 鼠标滚轮] 上下滚动   [r] 重新诊断"))
	return b.String()
}
