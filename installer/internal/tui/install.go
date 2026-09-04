package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/agent"
)

type InstallStatusMsg struct {
	Stage    string
	Progress float64
	Message  string
	Err      error
	Done     bool
}

func (m Model) startInstallAgentCmd(opts agent.InstallOptions) tea.Cmd {
	return func() tea.Msg {
		err := m.agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
			// In Bubble Tea, progress callbacks can send messages or update state
		})
		return InstallStatusMsg{
			Done: true,
			Err:  err,
		}
	}
}

func (m Model) updateInstallView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if !m.installing {
			m.state = ViewMainMenu
		}
		return m, nil
	case "m":
		if !m.installing {
			m.installMirror = !m.installMirror
		}
		return m, nil
	case "v":
		if !m.installing {
			m.installSkipVenv = !m.installSkipVenv
		}
		return m, nil
	case "enter":
		if m.installing {
			return m, nil
		}

		m.installing = true
		m.installErr = nil
		m.installMsg = "正在连接 GitHub 下载 Agent 发布包..."

		opts := agent.InstallOptions{
			TargetDir:  m.paths.AgentDir,
			Version:    "latest",
			RepoOwner:  "Rain-kl",
			RepoName:   "Cyphr",
			UseMirror:  m.installMirror,
			SkipVenv:   m.installSkipVenv,
			AutoConfig: true,
		}

		return m, func() tea.Msg {
			err := m.agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
				// Progress updates
			})
			return InstallStatusMsg{Done: true, Err: err}
		}
	}
	return m, nil
}

func (m Model) viewInstallView() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("📦 安装 / 部署 / 更新 Cyphr Agent") + "\n\n")

	isInstalled := m.agentSvc.IsInstalled()
	if isInstalled {
		b.WriteString(fmt.Sprintf("当前状态: %s (目录: %s)\n\n", StyleBadgeSuccess.Render("● 已安装"), m.paths.AgentDir))
	} else {
		b.WriteString(fmt.Sprintf("当前状态: %s (目标路径: %s)\n\n", StyleBadgeWarning.Render("○ 尚未安装"), m.paths.AgentDir))
	}

	mirrorStr := StyleBadgeSuccess.Render("开启国内加速镜像 (ghproxy.net) [推荐]")
	if !m.installMirror {
		mirrorStr = StyleBadgeWarning.Render("直接连接 GitHub 官方源")
	}

	venvStr := StyleBadgeSuccess.Render("自动初始化 Python 虚拟环境 (uv sync / venv)")
	if m.installSkipVenv {
		venvStr = StyleBadgeWarning.Render("跳过虚拟环境初始化 (仅解压源码)")
	}

	b.WriteString(StyleCard.Render(fmt.Sprintf(
		"【安装选项与环境配置】\n\n"+
			"  • 下载加速 : %s\n"+
			"  • 依赖同步 : %s\n"+
			"  • 目标目录 : %s\n"+
			"  • 默认配置 : 自动生成 config.yaml\n",
		mirrorStr, venvStr, m.paths.AgentDir,
	)) + "\n\n")

	if m.installing {
		b.WriteString(StyleBadgeWarning.Render(fmt.Sprintf("%s %s", m.spinner.View(), m.installMsg)) + "\n\n")
		b.WriteString(StyleSubtitle.Render("正在后台下载与解压安装，请稍候...") + "\n\n")
	} else if m.installErr != nil {
		b.WriteString(StyleBadgeDanger.Render(fmt.Sprintf("✗ 安装失败: %v", m.installErr)) + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 重新尝试安装   [m] 切换镜像源   [v] 切换虚拟环境选项   [Esc/q] 返回主菜单"))
	} else if m.installDone {
		b.WriteString(StyleBadgeSuccess.Render("✓ Agent 已成功部署完成！可在主菜单启动服务。") + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 重新安装/更新   [Esc/q] 返回主菜单"))
	} else {
		b.WriteString(StyleSubtitle.Render("按 [Enter] 即可一键自动从 GitHub Release 下载 Agent 并部署到本地目录：") + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 开始一键安装/部署   [m] 切换镜像源   [v] 切换虚拟环境初始化   [Esc/q] 返回主菜单"))
	}

	return b.String()
}
