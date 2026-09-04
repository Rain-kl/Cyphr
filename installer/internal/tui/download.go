package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/model"
)

func (m Model) updateDownloadCatalog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalOptions := len(model.PresetCatalog)

	switch msg.String() {
	case "up", "k":
		if m.catalogIndex > 0 {
			m.catalogIndex--
		} else {
			m.catalogIndex = totalOptions - 1
		}
	case "down", "j":
		if m.catalogIndex < totalOptions-1 {
			m.catalogIndex++
		} else {
			m.catalogIndex = 0
		}
	case "m":
		// Toggle mirror
		if m.downloadMirror == "hf-mirror" {
			m.downloadMirror = "hf"
		} else {
			m.downloadMirror = "hf-mirror"
		}
	case "enter":
		if m.downStatus != nil && m.downStatus.Running {
			m.err = fmt.Errorf("当前已有下载任务正在运行中 (PID: %d)，请先停止或等待完成", m.downStatus.PID)
			return m, nil
		}

		selectedPreset := model.PresetCatalog[m.catalogIndex]
		endpoint := "https://hf-mirror.com"
		if m.downloadMirror == "hf" {
			endpoint = "https://huggingface.co"
		}

		opts := model.DownloadOptions{
			ModelID:  selectedPreset.ID,
			PkgDir:   selectedPreset.PkgDir,
			Endpoint: endpoint,
			Mode:     "bg",
		}

		pid, err := m.modelSvc.StartDownload(opts)
		if err != nil {
			m.err = err
		} else {
			m.statusMsg = fmt.Sprintf("已成功启动后台下载任务 (PID: %d)，目标: models/%s", pid, selectedPreset.PkgDir)
			m.state = ViewDownloadProgress
		}
		m.refreshData()
		return m, nil
	}
	return m, nil
}

func (m Model) viewDownloadCatalog() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("📥 下载 ASR 模型库") + "\n\n")

	mirrorText := StyleBadgeSuccess.Render("国内镜像 (https://hf-mirror.com) [推荐]")
	if m.downloadMirror == "hf" {
		mirrorText = StyleBadgeWarning.Render("官方源 (https://huggingface.co)")
	}
	b.WriteString("当前下载源: " + mirrorText + "  " + StyleSubtitle.Render("(按 [m] 键快速切换镜像)") + "\n\n")

	if m.downStatus != nil && m.downStatus.Running {
		b.WriteString(StyleBadgeWarning.Render(fmt.Sprintf("⚠️ 提示: 当前已有模型正在下载中 (PID: %d, 模型: %s)。", m.downStatus.PID, m.downStatus.ModelID)) + "\n\n")
	}

	b.WriteString(StyleSubtitle.Render("选择需要下载的模型后按 [Enter] 即可后台静默下载（支持断点续传）：") + "\n\n")

	for i, p := range model.PresetCatalog {
		isSelected := i == m.catalogIndex
		prefix := "  "
		if isSelected {
			prefix = "❯ "
		}

		title := fmt.Sprintf("%-20s  %-12s  %s", p.Name, "["+p.Tag+"]", p.SizeEst)
		desc := p.Description

		if isSelected {
			b.WriteString(StyleMenuItemSelected.Render(prefix+title) + "\n")
			b.WriteString("    " + StyleSubtitle.Render(desc) + "\n")
		} else {
			b.WriteString(StyleMenuItem.Render(prefix+title) + "\n")
			b.WriteString("    " + StyleSubtitle.Render(desc) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(StyleKeyHelp.Render("[Enter] 开始后台下载   [m] 切换镜像源   [↑/↓, j/k] 选择模型   [Esc/q] 返回"))
	return b.String()
}
