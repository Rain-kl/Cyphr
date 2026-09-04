package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/doctor"
)

// MenuItem represents a single selectable menu entry.
type MenuItem struct {
	Title string
	Desc  string
	Key   string
}

var mainMenuItems = []MenuItem{
	{Title: "启动 Agent 服务", Desc: "在后台常驻运行推理 Agent 服务进程", Key: "1"},
	{Title: "停止 Agent 服务", Desc: "优雅停止当前运行中的 Agent 服务", Key: "2"},
	{Title: "重启 Agent 服务", Desc: "停止并重新在后台加载启动服务", Key: "3"},
	{Title: "安装 Agent 部署", Desc: "从 GitHub Release 下载并部署 Agent 运行时环境", Key: "i"},
	{Title: "检查与在线更新", Desc: "在线检查并更新 Agent 代码或 Installer 自身程序", Key: "u"},
	{Title: "查看综合状态", Desc: "查看 Agent 资源、下载任务及模型库综合看板", Key: "4"},
	{Title: "ASR 模型与下载管理", Desc: "浏览/下载预设模型、追踪实时下载进度或终止任务", Key: "5"},
	{Title: "查看服务实时日志", Desc: "浏览 agent.log 详细输出与错误排查", Key: "6"},
	{Title: "环境健康诊断 (Doctor)", Desc: "检测 GPU、CUDA、PyTorch 驱动及硬件加速就绪情况", Key: "d"},
	{Title: "退出管理面板", Desc: "安全退出控制台 (后台服务与下载不受影响)", Key: "q"},
}

func (m Model) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuIndex > 0 {
			m.menuIndex--
		} else {
			m.menuIndex = len(mainMenuItems) - 1
		}
	case KeyDown, "j":
		if m.menuIndex < len(mainMenuItems)-1 {
			m.menuIndex++
		} else {
			m.menuIndex = 0
		}
	case KeyEnter:
		return m.handleMenuEnter()
	default:
		return m.handleMenuKeyShortcuts(msg.String())
	}
	return m, nil
}

func (m Model) handleMenuKeyShortcuts(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "1":
		return m.handleStartService()
	case "2":
		return m.handleStopService()
	case "3":
		return m.handleRestartService()
	case "i", "I":
		m.state = ViewInstallAgent
		return m, nil
	case "u", "U":
		m.state = ViewUpdateMenu
		return m, nil
	case "4":
		m.state = ViewStatusDashboard
		return m, nil
	case "5":
		return m.handleModelManagement()
	case "6":
		m.state = ViewAgentLogs
		return m, nil
	case "d", "D":
		m.state = ViewDoctor
		m.doctorOutput = doctor.Run(m.paths).Format()
		return m, nil
	}
	return m, nil
}

func (m Model) handleModelManagement() (tea.Model, tea.Cmd) {
	if m.downStatus != nil && m.downStatus.Running {
		m.state = ViewDownloadProgress
	} else {
		m.state = ViewDownloadCatalog
	}
	return m, nil
}

func (m Model) handleMenuEnter() (tea.Model, tea.Cmd) {
	switch m.menuIndex {
	case 0:
		return m.handleStartService()
	case 1:
		return m.handleStopService()
	case 2:
		return m.handleRestartService()
	case 3:
		m.state = ViewInstallAgent
	case 4:
		m.state = ViewUpdateMenu
	case 5:
		m.state = ViewStatusDashboard
	case 6:
		return m.handleModelManagement()
	case 7:
		m.state = ViewAgentLogs
	case 8:
		m.state = ViewDoctor
		m.doctorOutput = doctor.Run(m.paths).Format()
	case 9:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleStartService() (tea.Model, tea.Cmd) {
	m.err = nil
	m.statusMsg = ""
	st, err := m.agentSvc.Start()
	if err != nil {
		m.err = err
	} else {
		m.statusMsg = fmt.Sprintf("Agent 服务已在后台成功启动 (PID: %d)", st.PID)
	}
	m.refreshData()
	return m, nil
}

func (m Model) handleStopService() (tea.Model, tea.Cmd) {
	m.err = nil
	m.statusMsg = ""
	err := m.agentSvc.Stop()
	if err != nil {
		m.err = err
	} else {
		m.statusMsg = "Agent 服务已成功停止。"
	}
	m.refreshData()
	return m, nil
}

func (m Model) handleRestartService() (tea.Model, tea.Cmd) {
	m.err = nil
	m.statusMsg = ""
	st, err := m.agentSvc.Restart()
	if err != nil {
		m.err = err
	} else {
		m.statusMsg = fmt.Sprintf("Agent 服务已成功重启 (PID: %d)", st.PID)
	}
	m.refreshData()
	return m, nil
}

func (m Model) handleStopDownload() (tea.Model, tea.Cmd) {
	m.err = nil
	m.statusMsg = ""
	err := m.modelSvc.StopDownload()
	if err != nil {
		m.err = err
	} else {
		m.statusMsg = "后台模型下载任务已停止 (分块文件已妥善保存，下次将自动续传)。"
	}
	m.refreshData()
	return m, nil
}

func (m Model) viewMainMenu() string {
	var b strings.Builder

	// Mini Status Bar Card
	agentBadge := StyleBadgeDanger.Render("● 未运行")
	if m.agentStatus != nil {
		if !m.agentStatus.Installed {
			agentBadge = StyleBadgeWarning.Render("○ 未安装 (按 [i] 安装)")
		} else if m.agentStatus.Running {
			agentBadge = StyleBadgeSuccess.Render(fmt.Sprintf("● 运行中 (PID: %d, 内存: %dMB)", m.agentStatus.PID, m.agentStatus.RSSMB))
		}
	}

	downBadge := StyleBadgeMuted.Render("○ 无下载任务")
	if m.downStatus != nil && m.downStatus.Running {
		downBadge = StyleBadgeWarning.Render(fmt.Sprintf("%s 正在下载: %s (PID: %d)", m.spinner.View(), m.downStatus.ModelID, m.downStatus.PID))
	}

	statusBar := fmt.Sprintf("【Agent 服务】: %s    【模型下载】: %s", agentBadge, downBadge)
	b.WriteString(StyleCard.Render(statusBar) + "\n\n")

	b.WriteString(StyleSubtitle.Render("请使用方向键 [↑/↓] 或快捷键选择功能操作，回车确认：") + "\n\n")

	for i, item := range mainMenuItems {
		isSelected := i == m.menuIndex
		var itemStr string
		if isSelected {
			itemStr = fmt.Sprintf("❯ [%s] %-18s  %s", item.Key, item.Title, StyleSubtitle.Render(item.Desc))
			b.WriteString(StyleMenuItemSelected.Render(itemStr) + "\n")
		} else {
			itemStr = fmt.Sprintf("  [%s] %-18s  %s", item.Key, item.Title, StyleSubtitle.Render(item.Desc))
			b.WriteString(StyleMenuItem.Render(itemStr) + "\n")
		}
	}

	b.WriteString("\n" + StyleKeyHelp.Render("[Enter] 确认选择   [↑/↓, j/k] 移动光标   [i] 安装 Agent   [u] 更新组件   [d] 环境诊断   [1-6] 快捷键   [q/Esc] 退出"))
	return b.String()
}
