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
	case KeyDown, "j":
		if m.catalogIndex < totalOptions-1 {
			m.catalogIndex++
		} else {
			m.catalogIndex = 0
		}
	case "m", "s":
		// Cycle sources: modelscope -> hf-mirror -> hf -> modelscope
		switch m.downloadSource {
		case SourceModelScope:
			m.downloadSource = SourceHFMirror
		case SourceHFMirror:
			m.downloadSource = SourceHF
		default:
			m.downloadSource = SourceModelScope
		}
	case "p", "P":
		if m.downStatus != nil && m.downStatus.Running {
			m.state = ViewDownloadProgress
			return m, nil
		}
	case "x", "X":
		if m.downStatus != nil && m.downStatus.Running {
			return m.handleStopDownload()
		}
	case KeyEnter:
		return m.handleCatalogEnter()
	}
	return m, nil
}

func (m Model) resolveDownloadTarget(preset model.PresetModel) (modelID, source, endpoint string, err error) {
	switch m.downloadSource {
	case SourceModelScope:
		source = SourceModelScope
		endpoint = "https://modelscope.cn"
		modelID = preset.ModelScopeID
		if modelID == "" {
			err = fmt.Errorf("模型 %s 暂无 ModelScope 官方源，请按 [m] 切换至 Hugging Face 镜像或官方源", preset.Name)
			return
		}
	case SourceHF:
		source = "huggingface"
		endpoint = "https://huggingface.co"
		modelID = preset.HuggingFaceID
		if modelID == "" {
			modelID = preset.ID
		}
	default: // SourceHFMirror
		source = "huggingface"
		endpoint = "https://hf-mirror.com"
		modelID = preset.HuggingFaceID
		if modelID == "" {
			modelID = preset.ID
		}
	}
	return
}

func (m Model) handleCatalogEnter() (tea.Model, tea.Cmd) {
	if m.downStatus != nil && m.downStatus.Running {
		m.err = fmt.Errorf("当前已有下载任务正在运行中 (PID: %d)，请先停止或等待完成", m.downStatus.PID)
		return m, nil
	}

	selectedPreset := model.PresetCatalog[m.catalogIndex]
	modelID, source, endpoint, err := m.resolveDownloadTarget(selectedPreset)
	if err != nil {
		m.err = err
		return m, nil
	}

	opts := model.DownloadOptions{
		ModelID:  modelID,
		PkgDir:   selectedPreset.PkgDir,
		Source:   source,
		Endpoint: endpoint,
		Mode:     "bg",
	}

	pid, err := m.modelSvc.StartDownload(opts)
	if err != nil {
		m.err = err
	} else {
		m.statusMsg = fmt.Sprintf("已成功启动后台下载任务 (PID: %d)，目标: models/%s (源: %s)", pid, selectedPreset.PkgDir, source)
		m.state = ViewDownloadProgress
	}
	m.refreshData()
	return m, nil
}

func (m Model) viewDownloadCatalog() string {
	var b strings.Builder

	b.WriteString(StyleCardTitle.Render("📥 下载 ASR 模型库") + "\n\n")

	var sourceText string
	switch m.downloadSource {
	case SourceModelScope:
		sourceText = StyleBadgeSuccess.Render("ModelScope (阿里魔搭社区) [国内极速]")
	case SourceHFMirror:
		sourceText = StyleBadgeSuccess.Render("Hugging Face 国内镜像 (https://hf-mirror.com) [推荐]")
	default:
		sourceText = StyleBadgeWarning.Render("Hugging Face 官方源 (https://huggingface.co)")
	}
	b.WriteString("当前下载平台: " + sourceText + "  " + StyleSubtitle.Render("(按 [m] 或 [s] 键快速切换下载源)") + "\n\n")

	if m.downStatus != nil && m.downStatus.Running {
		runningCard := fmt.Sprintf("⚠️ 当前已有模型正在后台下载中 (PID: %d, 模型: %s)\n   按 [p] 查看实时进度与日志   按 [x] 终止该下载任务", m.downStatus.PID, m.downStatus.ModelID)
		b.WriteString(StyleCard.Render(runningCard) + "\n\n")
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

		// If current source is modelscope and model has no modelscope repo, show hint
		if m.downloadSource == SourceModelScope && p.ModelScopeID == "" {
			desc += " [ModelScope 暂未收录]"
		}

		if isSelected {
			b.WriteString(StyleMenuItemSelected.Render(prefix+title) + "\n")
			b.WriteString("    " + StyleSubtitle.Render(desc) + "\n")
		} else {
			b.WriteString(StyleMenuItem.Render(prefix+title) + "\n")
			b.WriteString("    " + StyleSubtitle.Render(desc) + "\n")
		}
		b.WriteString("\n")
	}

	footerHelp := "[Enter] 开始后台下载   [m/s] 切换下载平台(ModelScope/HF)   [↑/↓, j/k] 选择模型   [Esc/q] 返回"
	if m.downStatus != nil && m.downStatus.Running {
		footerHelp = "[p] 查看实时进度   [x] 停止当前下载   " + footerHelp
	}
	b.WriteString(StyleKeyHelp.Render(footerHelp))
	return b.String()
}
