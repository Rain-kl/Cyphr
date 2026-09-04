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
	if m.agentStatus != nil && m.agentStatus.Installed {
		if m.agentStatus.Running {
			fmt.Fprintf(&agentInfo, "运行状态: %s\n", StyleBadgeSuccess.Render("● 运行中 (Running)"))
			fmt.Fprintf(&agentInfo, "进程 PID: %d\n", m.agentStatus.PID)
			fmt.Fprintf(&agentInfo, "运行时长: %s\n", m.agentStatus.Uptime)
			fmt.Fprintf(&agentInfo, "物理内存: ~%d MB\n", m.agentStatus.RSSMB)
		} else {
			fmt.Fprintf(&agentInfo, "运行状态: %s\n", StyleBadgeDanger.Render("● 已安装但未运行 (Stopped)"))
		}
		fmt.Fprintf(&agentInfo, "安装路径: %s\n", m.agentStatus.AgentDir)
		fmt.Fprintf(&agentInfo, "日志路径: %s\n", m.agentStatus.LogPath)
	} else {
		fmt.Fprintf(&agentInfo, "安装状态: %s\n", StyleBadgeWarning.Render("○ 尚未安装 (可在菜单选择【安装/更新 Agent】)"))
		fmt.Fprintf(&agentInfo, "目标路径: %s\n", m.paths.AgentDir)
	}
	b.WriteString(StyleCard.Render("【Agent 服务】\n"+agentInfo.String()) + "\n")

	// Download Task Card
	var downInfo strings.Builder
	if m.downStatus != nil && m.downStatus.Running {
		fmt.Fprintf(&downInfo, "任务状态: %s\n", StyleBadgeWarning.Render(fmt.Sprintf("%s 正在后台下载中 (Downloading)", m.spinner.View())))
		fmt.Fprintf(&downInfo, "进程 PID: %d (已运行 %s)\n", m.downStatus.PID, m.downStatus.Uptime)
		fmt.Fprintf(&downInfo, "下载模型: %s\n", m.downStatus.ModelID)
		fmt.Fprintf(&downInfo, "本地目录: models/%s\n", m.downStatus.PkgDir)
		fmt.Fprintf(&downInfo, "镜像来源: %s\n", m.downStatus.Endpoint)
		fmt.Fprintf(&downInfo, "启动时间: %s\n", m.downStatus.StartTime)
		if m.downStatus.DiskUsage != "" {
			fmt.Fprintf(&downInfo, "当前落盘: %s\n", m.downStatus.DiskUsage)
		}
	} else {
		fmt.Fprintf(&downInfo, "任务状态: %s\n", StyleBadgeMuted.Render("○ 当前无后台下载任务在运行"))
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
			fmt.Fprintf(&modelInfo, "  • %-22s %-10s %s\n", lm.DirName, "("+lm.DiskSize+")", statusBadge)
		}
	}
	b.WriteString(StyleCard.Render("【已安装模型库】(models/)\n"+modelInfo.String()) + "\n")

	b.WriteString(StyleKeyHelp.Render("[Esc/q/Enter] 返回主菜单   [r] 刷新数据"))
	return b.String()
}
