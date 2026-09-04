package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateStatusDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter":
		m.state = ViewMainMenu
		return m, nil
	case "r":
		m.refreshData()
		return m, nil
	}
	return m, nil
}

func (m Model) viewStatusDashboard() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("📊 系统与服务综合看板") + "\n\n")

	// Agent Service Card
	var agentInfo strings.Builder
	if m.agentStatus != nil && m.agentStatus.Running {
		agentInfo.WriteString(fmt.Sprintf("运行状态: %s\n", StyleBadgeSuccess.Render("● 运行中 (Running)")))
		agentInfo.WriteString(fmt.Sprintf("进程 PID: %d\n", m.agentStatus.PID))
		agentInfo.WriteString(fmt.Sprintf("运行时长: %s\n", m.agentStatus.Uptime))
		agentInfo.WriteString(fmt.Sprintf("物理内存: ~%d MB\n", m.agentStatus.RSSMB))
		agentInfo.WriteString(fmt.Sprintf("日志路径: %s\n", m.agentStatus.LogPath))
	} else {
		agentInfo.WriteString(fmt.Sprintf("运行状态: %s\n", StyleBadgeDanger.Render("● 未运行 (Stopped)")))
		agentInfo.WriteString(fmt.Sprintf("日志路径: %s\n", m.paths.LogFile))
	}
	b.WriteString(StyleCard.Render("【Agent 服务】\n"+agentInfo.String()) + "\n")

	// Download Task Card
	var downInfo strings.Builder
	if m.downStatus != nil && m.downStatus.Running {
		downInfo.WriteString(fmt.Sprintf("任务状态: %s\n", StyleBadgeWarning.Render(fmt.Sprintf("%s 正在后台下载中 (Downloading)", m.spinner.View()))))
		downInfo.WriteString(fmt.Sprintf("进程 PID: %d (已运行 %s)\n", m.downStatus.PID, m.downStatus.Uptime))
		downInfo.WriteString(fmt.Sprintf("下载模型: %s\n", m.downStatus.ModelID))
		downInfo.WriteString(fmt.Sprintf("本地目录: models/%s\n", m.downStatus.PkgDir))
		downInfo.WriteString(fmt.Sprintf("镜像来源: %s\n", m.downStatus.Endpoint))
		downInfo.WriteString(fmt.Sprintf("启动时间: %s\n", m.downStatus.StartTime))
		if m.downStatus.DiskUsage != "" {
			downInfo.WriteString(fmt.Sprintf("当前落盘: %s\n", m.downStatus.DiskUsage))
		}
	} else {
		downInfo.WriteString(fmt.Sprintf("任务状态: %s\n", StyleBadgeMuted.Render("○ 当前无后台下载任务在运行")))
	}
	b.WriteString(StyleCard.Render("【模型下载任务】\n"+downInfo.String()) + "\n")

	// Local Models Card
	var modelInfo strings.Builder
	if len(m.localModels) == 0 {
		modelInfo.WriteString(StyleBadgeMuted.Render("暂无本地已安装模型。可在主菜单选择【下载 ASR 模型】。") + "\n")
	} else {
		for _, lm := range m.localModels {
			statusBadge := StyleBadgeSuccess.Render("[" + lm.Status + "]")
			if !lm.IsReady {
				statusBadge = StyleBadgeWarning.Render("[" + lm.Status + "]")
			}
			modelInfo.WriteString(fmt.Sprintf("  • %-22s %-10s %s\n", lm.DirName, "("+lm.DiskSize+")", statusBadge))
		}
	}
	b.WriteString(StyleCard.Render("【已安装模型库】(models/)\n"+modelInfo.String()) + "\n")

	b.WriteString(StyleKeyHelp.Render("[Esc/q/Enter] 返回主菜单   [r] 刷新数据"))
	return b.String()
}
