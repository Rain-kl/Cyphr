package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/agent"
	"cyphr/installer/internal/updater"
)

type UpdateTarget int

const (
	UpdateTargetAgent UpdateTarget = iota
	UpdateTargetInstaller
)

type UpdateProgressMsg struct {
	Stage    string
	Progress float64
	Message  string
	Err      error
	Done     bool
}

func (m Model) updateUpdateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		if !m.updating {
			m.state = ViewMainMenu
		}
		return m, nil

	case "up", "k", "down", "j", "tab":
		if !m.updating {
			if m.updateTarget == UpdateTargetAgent {
				m.updateTarget = UpdateTargetInstaller
			} else {
				m.updateTarget = UpdateTargetAgent
			}
		}
		return m, nil

	case "m":
		if !m.updating {
			m.updateMirror = !m.updateMirror
		}
		return m, nil

	case "enter":
		if m.updating {
			return m, nil
		}

		m.updating = true
		m.updateErr = nil
		m.updateDone = false

		if m.updateTarget == UpdateTargetAgent {
			m.updateMsg = "正在从 GitHub 获取并更新 Agent..."
			opts := agent.InstallOptions{
				TargetDir:  m.paths.AgentDir,
				Version:    "latest",
				RepoOwner:  "Rain-kl",
				RepoName:   "Cyphr",
				UseMirror:  m.updateMirror,
				SkipVenv:   false,
				AutoConfig: true,
			}
			return m, func() tea.Msg {
				err := m.agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
					// Progress callback
				})
				return UpdateProgressMsg{Done: true, Err: err}
			}
		}

		// Update Installer
		m.updateMsg = "正在从 GitHub 下载最新版本 Installer 并自动替换..."
		useMirror := m.updateMirror
		return m, func() tea.Msg {
			err := updater.UpdateInstaller("Rain-kl", "Cyphr", useMirror, func(stage string, progress float64, message string) {
				// Progress callback
			})
			return UpdateProgressMsg{Done: true, Err: err}
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) viewUpdateView() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("🔄 软件版本在线更新") + "\n\n")

	mirrorStr := StyleBadgeSuccess.Render("开启国内加速 (ghproxy.net) [推荐]")
	if !m.updateMirror {
		mirrorStr = StyleBadgeWarning.Render("直接连接 GitHub 官方源")
	}
	b.WriteString("下载通道: " + mirrorStr + "  " + StyleSubtitle.Render("(按 [m] 切换加速镜像)") + "\n\n")

	// Target options
	agentPrefix := "  "
	installerPrefix := "  "
	if m.updateTarget == UpdateTargetAgent {
		agentPrefix = "❯ "
	} else {
		installerPrefix = "❯ "
	}

	var agentCard strings.Builder
	fmt.Fprintf(&agentCard, "%s【1】更新 Agent 运行时\n", agentPrefix)
	if m.agentStatus != nil && m.agentStatus.Installed {
		fmt.Fprintf(&agentCard, "   当前状态: %s (目录: %s)\n", StyleBadgeSuccess.Render("已安装"), m.paths.AgentDir)
	} else {
		fmt.Fprintf(&agentCard, "   当前状态: %s (目录: %s)\n", StyleBadgeWarning.Render("未安装"), m.paths.AgentDir)
	}
	agentCard.WriteString("   从 GitHub Release 拉取最新 cyphr-agent.zip 源码并同步虚拟环境。")

	var installerCard strings.Builder
	fmt.Fprintf(&installerCard, "%s【2】更新 Installer 自身可执行程序\n", installerPrefix)
	installerCard.WriteString("   从 GitHub Release 自动匹配并下载当前平台的最新二进制并完成热替换。")

	if m.updateTarget == UpdateTargetAgent {
		b.WriteString(StyleMenuItemSelected.Render(StyleCard.Render(agentCard.String())) + "\n")
		b.WriteString(StyleMenuItem.Render(StyleCard.Render(installerCard.String())) + "\n\n")
	} else {
		b.WriteString(StyleMenuItem.Render(StyleCard.Render(agentCard.String())) + "\n")
		b.WriteString(StyleMenuItemSelected.Render(StyleCard.Render(installerCard.String())) + "\n\n")
	}

	switch {
	case m.updating:
		b.WriteString(StyleBadgeWarning.Render(fmt.Sprintf("%s %s", m.spinner.View(), m.updateMsg)) + "\n\n")
		b.WriteString(StyleSubtitle.Render("正在联网执行更新操作，请耐心等待...") + "\n\n")
	case m.updateErr != nil:
		b.WriteString(StyleBadgeDanger.Render(fmt.Sprintf("✗ 更新失败: %v", m.updateErr)) + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 重新尝试更新   [↑/↓] 切换更新目标   [m] 切换加速镜像   [Esc/q] 返回主菜单"))
	case m.updateDone:
		b.WriteString(StyleBadgeSuccess.Render("✓ 更新完成！") + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 再次更新   [↑/↓] 切换更新目标   [Esc/q] 返回主菜单"))
	default:
		b.WriteString(StyleSubtitle.Render("使用 [↑/↓] 选择需要更新的组件，按 [Enter] 开始在线更新：") + "\n\n")
		b.WriteString(StyleKeyHelp.Render("[Enter] 开始更新所选组件   [↑/↓] 切换更新组件   [m] 切换镜像源   [Esc/q] 返回主菜单"))
	}

	return b.String()
}
