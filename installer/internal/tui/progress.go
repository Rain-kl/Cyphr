package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateDownloadProgress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = ViewMainMenu
		return m, nil
	case "x":
		return m.handleStopDownload()
	case "r":
		m.refreshData()
		return m, nil
	}
	return m, nil
}

func (m Model) viewDownloadProgress() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("⏳ 模型下载进度与实时日志") + "\n\n")

	if m.downStatus == nil || !m.downStatus.Running {
		b.WriteString(StyleBadgeMuted.Render("当前没有正在运行的后台下载任务。") + "\n\n")
		if m.downStatus != nil && len(m.downStatus.RecentLogs) > 0 {
			b.WriteString(StyleSubtitle.Render(fmt.Sprintf("上次任务: %s -> models/%s", m.downStatus.ModelID, m.downStatus.PkgDir)) + "\n\n")
			b.WriteString("最近日志末尾:\n")
			b.WriteString(StyleLogBox.Render(strings.Join(m.downStatus.RecentLogs, "\n")) + "\n\n")
		}
		b.WriteString(StyleKeyHelp.Render("[Esc/q] 返回主菜单"))
		return b.String()
	}

	// Active download card
	var card strings.Builder
	fmt.Fprintf(&card, "任务状态: %s\n", StyleBadgeWarning.Render(fmt.Sprintf("%s 正在后台下载 (PID: %d, 已运行: %s)", m.spinner.View(), m.downStatus.PID, m.downStatus.Uptime)))
	fmt.Fprintf(&card, "目标模型: %s\n", m.downStatus.ModelID)
	fmt.Fprintf(&card, "存储目录: models/%s\n", m.downStatus.PkgDir)
	fmt.Fprintf(&card, "下载来源: %s\n", m.downStatus.Endpoint)
	fmt.Fprintf(&card, "启动时间: %s\n", m.downStatus.StartTime)
	if m.downStatus.DiskUsage != "" {
		fmt.Fprintf(&card, "磁盘写入: %s\n", StyleBadgeSuccess.Render(m.downStatus.DiskUsage))
	}
	b.WriteString(StyleCard.Render(card.String()) + "\n\n")

	// Recent Logs
	b.WriteString(StyleSubtitle.Render(fmt.Sprintf("最新下载日志 (实时自动刷新，最后 %d 行)：", len(m.downStatus.RecentLogs))) + "\n")
	if len(m.downStatus.RecentLogs) == 0 {
		b.WriteString(StyleLogBox.Render("正在建立连接，请稍候...") + "\n\n")
	} else {
		b.WriteString(StyleLogBox.Render(strings.Join(m.downStatus.RecentLogs, "\n")) + "\n\n")
	}

	b.WriteString(StyleKeyHelp.Render("[Esc/q] 返回主菜单 (下载在后台继续)   [x] 停止当前下载   [r] 立即刷新"))
	return b.String()
}
