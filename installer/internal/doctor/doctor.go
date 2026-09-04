// Package doctor provides diagnostic checks and reporting for host environment, GPU, and agent health.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"cyphr/installer/internal/config"
)

const (
	defaultProbeTimeout = 3 * time.Second
	smiProbeTimeout     = 4 * time.Second
	deepProbeTimeout    = 8 * time.Second
	minCSVFields        = 4
)

// GPUDevice represents an inspected GPU hardware device.
type GPUDevice struct {
	Name        string `json:"name"`
	DriverVer   string `json:"driver_ver"`
	TotalVRAMMB int    `json:"total_vram_mb"`
	FreeVRAMMB  int    `json:"free_vram_mb"`
}

// PythonProbeResult captures runtime reflections from within Python.
type PythonProbeResult struct {
	PythonVersion string `json:"python"`
	Is64Bit       bool   `json:"is_64bit"`
	Platform      string `json:"platform"`
	Torch         struct {
		Installed     bool    `json:"installed"`
		Version       string  `json:"version"`
		CUDAVersion   *string `json:"cuda_version"`
		CUDAAvailable bool    `json:"cuda_available"`
		DeviceCount   int     `json:"device_count"`
		DeviceName    string  `json:"device_name"`
		Error         string  `json:"error"`
	} `json:"torch"`
	QwenASR bool `json:"qwen_asr"`
	PyNVML  struct {
		Available bool   `json:"available"`
		Driver    string `json:"driver"`
		Error     string `json:"error"`
	} `json:"pynvml"`
}

// Report holds all diagnostic results.
type Report struct {
	Timestamp time.Time

	// 1. System
	OS       string
	Arch     string
	CPUs     int
	Hostname string

	// 2. Hardware GPU
	PhysicalGPUs  []string
	NvidiaSmiPath string
	NvidiaDriver  string
	NvidiaCUDA    string
	GPUDevices    []GPUDevice

	// 3. System Tooling
	SysPythonPath string
	SysPythonVer  string
	SysPython64   bool
	UvPath        string
	UvVer         string
	FFmpegPath    string
	FFmpegVer     string

	// 4. Agent Installation
	AgentDir       string
	AgentInstalled bool
	VenvDir        string
	VenvExists     bool
	AgentPython    string

	// 5. Python Probe
	ProbeRun    bool
	ProbeErr    string
	ProbeResult PythonProbeResult

	// 6. Config & Connectivity
	ConfigFileExists bool
	ControllerURL    string
	ControllerReach  bool
	ControllerStatus string

	// 7. Overall Assessment
	HasGPUAcceleration bool
	Issues             []string
	Suggestions        []string
}

// Run executes the complete diagnostic suite and returns the report.
func Run(paths *config.AppPaths) *Report {
	r := &Report{
		Timestamp: time.Now(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
	}

	if h, err := os.Hostname(); err == nil {
		r.Hostname = h
	}

	// 1. Physical Display Adapters
	r.PhysicalGPUs = querySystemDisplayAdapters()

	// 2. nvidia-smi & GPU Details
	r.inspectNvidiaSmi()

	// 3. System Tools (Python, uv, ffmpeg)
	r.inspectSystemTools()

	// 4. Agent Directory & Venv
	r.inspectAgent(paths)

	// 5. Deep Python Probe
	r.runPythonProbe()

	// 6. Config & Connectivity
	r.inspectConfigAndNetwork(paths)

	// 7. Synthesize Diagnostics & Recommendations
	r.synthesize()

	return r
}

func (r *Report) inspectNvidiaSmi() {
	smiPath, err := exec.LookPath("nvidia-smi")
	if err != nil && runtime.GOOS == "windows" {
		commonPaths := []string{
			`C:\Windows\System32\nvidia-smi.exe`,
			`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		}
		for _, p := range commonPaths {
			if fi, serr := os.Stat(p); serr == nil && !fi.IsDir() {
				smiPath = p
				break
			}
		}
	}

	if smiPath == "" {
		return
	}
	r.NvidiaSmiPath = smiPath

	r.queryNvidiaCSV(smiPath)
	r.queryNvidiaCUDAHeader(smiPath)
}

func (r *Report) queryNvidiaCSV(smiPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), smiProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, smiPath, "--query-gpu=name,driver_version,memory.total,memory.free", "--format=csv,noheader,nounits") //nolint:gosec // Diagnostic probe
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Split(l, ",")
		if len(parts) >= minCSVFields {
			name := strings.TrimSpace(parts[0])
			driver := strings.TrimSpace(parts[1])
			var totalMB, freeMB int
			_, _ = fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &totalMB)
			_, _ = fmt.Sscanf(strings.TrimSpace(parts[3]), "%d", &freeMB)
			r.GPUDevices = append(r.GPUDevices, GPUDevice{
				Name:        name,
				DriverVer:   driver,
				TotalVRAMMB: totalMB,
				FreeVRAMMB:  freeMB,
			})
			if r.NvidiaDriver == "" {
				r.NvidiaDriver = driver
			}
		}
	}
}

func (r *Report) queryNvidiaCUDAHeader(smiPath string) {
	ctxHeader, cancelHeader := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancelHeader()

	hCmd := exec.CommandContext(ctxHeader, smiPath) //nolint:gosec // Diagnostic probe
	out, err := hCmd.Output()
	if err != nil {
		return
	}

	s := string(out)
	if idx := strings.Index(s, "CUDA Version:"); idx != -1 {
		sub := s[idx+len("CUDA Version:"):]
		fields := strings.Fields(sub)
		if len(fields) > 0 {
			r.NvidiaCUDA = fields[0]
		}
	}
}

func (r *Report) inspectSystemTools() {
	r.inspectPython()
	r.inspectUv()
	r.inspectFFmpeg()
}

func (r *Report) inspectPython() {
	for _, bin := range []string{"python3", "python", "py"} {
		p, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		r.SysPythonPath = p
		ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
		if out, cmdErr := exec.CommandContext(ctx, p, "--version").CombinedOutput(); cmdErr == nil { //nolint:gosec // Diagnostic probe
			r.SysPythonVer = strings.TrimSpace(string(out))
		}
		cancel()

		ctxBit, cancelBit := context.WithTimeout(context.Background(), defaultProbeTimeout)
		if out, cmdErr := exec.CommandContext(ctxBit, p, "-c", "import sys; print(sys.maxsize > 2**32)").CombinedOutput(); cmdErr == nil { //nolint:gosec // Diagnostic probe
			r.SysPython64 = strings.TrimSpace(string(out)) == "True"
		}
		cancelBit()
		break
	}
}

func (r *Report) inspectUv() {
	p, err := exec.LookPath("uv")
	if err != nil {
		return
	}
	r.UvPath = p
	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()
	if out, cmdErr := exec.CommandContext(ctx, p, "--version").CombinedOutput(); cmdErr == nil { //nolint:gosec // Diagnostic probe
		r.UvVer = strings.TrimSpace(string(out))
	}
}

func (r *Report) inspectFFmpeg() {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return
	}
	r.FFmpegPath = p
	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()
	if out, cmdErr := exec.CommandContext(ctx, p, "-version").CombinedOutput(); cmdErr == nil { //nolint:gosec // Diagnostic probe
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			r.FFmpegVer = strings.TrimSpace(lines[0])
		}
	}
}

func (r *Report) inspectAgent(paths *config.AppPaths) {
	r.AgentDir = paths.AgentDir
	mainPy := filepath.Join(paths.AgentDir, "main.py")
	if fi, err := os.Stat(mainPy); err == nil && !fi.IsDir() {
		r.AgentInstalled = true
	}

	r.VenvDir = filepath.Join(paths.AgentDir, ".venv")
	if fi, err := os.Stat(r.VenvDir); err == nil && fi.IsDir() {
		r.VenvExists = true
	}

	r.AgentPython = paths.PythonBin
}

func (r *Report) runPythonProbe() {
	if r.AgentPython == "" {
		return
	}

	script := `
import sys, json
res = {
    "python": sys.version.split()[0],
    "is_64bit": sys.maxsize > 2**32,
    "platform": sys.platform,
}
try:
    import torch
    res["torch"] = {
        "installed": True,
        "version": getattr(torch, "__version__", ""),
        "cuda_version": getattr(torch.version, "cuda", None),
        "cuda_available": bool(torch.cuda.is_available()),
        "device_count": torch.cuda.device_count() if torch.cuda.is_available() else 0,
        "device_name": torch.cuda.get_device_name(0) if torch.cuda.is_available() and torch.cuda.device_count() > 0 else "",
    }
except Exception as e:
    res["torch"] = {"installed": False, "error": str(e)}

try:
    import qwen_asr
    res["qwen_asr"] = True
except Exception:
    res["qwen_asr"] = False

try:
    import pynvml
    pynvml.nvmlInit()
    res["pynvml"] = {"available": True, "driver": pynvml.nvmlSystemGetDriverVersion()}
except Exception as e:
    res["pynvml"] = {"available": False, "error": str(e)}

print(json.dumps(res))
`

	ctx, cancel := context.WithTimeout(context.Background(), deepProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.AgentPython, "-c", script) //nolint:gosec // Diagnostic probe
	cmd.Dir = r.AgentDir
	out, err := cmd.CombinedOutput()
	r.ProbeRun = true
	if err != nil {
		r.ProbeErr = fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))
		return
	}

	raw := strings.TrimSpace(string(out))
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		raw = raw[start : end+1]
	}

	if unmarshalErr := json.Unmarshal([]byte(raw), &r.ProbeResult); unmarshalErr != nil {
		r.ProbeErr = fmt.Sprintf("解析探针输出失败: %v, 原始输出:\n%s", unmarshalErr, raw)
	}
}

func parseControllerURL(cfgFile string) string {
	data, readErr := os.ReadFile(cfgFile) //nolint:gosec // Config file path is from AppPaths
	if readErr != nil {
		return ""
	}
	var m map[string]any
	if yamlErr := yaml.Unmarshal(data, &m); yamlErr != nil {
		return ""
	}
	if u, ok := m["controller_url"].(string); ok {
		return strings.TrimSpace(u)
	}
	return ""
}

func (r *Report) inspectConfigAndNetwork(paths *config.AppPaths) {
	cfgFile := filepath.Clean(paths.ConfigFile)
	if _, err := os.Stat(cfgFile); err == nil {
		r.ConfigFileExists = true
		r.ControllerURL = parseControllerURL(cfgFile)
	}

	if r.ControllerURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultProbeTimeout)
	defer cancel()

	pingURL := strings.TrimRight(r.ControllerURL, "/") + "/api/v1/ping"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: defaultProbeTimeout}
	resp, reqErr := client.Do(req)
	if reqErr != nil {
		r.ControllerReach = false
		r.ControllerStatus = fmt.Sprintf("连接失败 (%v)", reqErr)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	r.ControllerReach = true
	r.ControllerStatus = fmt.Sprintf("HTTP %d OK", resp.StatusCode)
}

func (r *Report) synthesize() {
	hasNvidiaHardware := r.detectNvidiaHardware()
	r.checkPythonCompatibility()
	r.checkTorchCUDA(hasNvidiaHardware)
	r.checkNvidiaDriver(hasNvidiaHardware)
	r.checkFFmpeg()
	r.checkController()
}

func (r *Report) detectNvidiaHardware() bool {
	if len(r.GPUDevices) > 0 || r.NvidiaSmiPath != "" {
		return true
	}
	for _, a := range r.PhysicalGPUs {
		lower := strings.ToLower(a)
		if strings.Contains(lower, "nvidia") || strings.Contains(lower, "geforce") ||
			strings.Contains(lower, "quadro") || strings.Contains(lower, "tesla") ||
			strings.Contains(lower, "rtx") {
			return true
		}
	}
	return false
}

func (r *Report) checkPythonCompatibility() {
	if !r.ProbeRun || r.ProbeResult.PythonVersion == "" {
		return
	}
	if !r.ProbeResult.Is64Bit {
		r.Issues = append(r.Issues, "检测到 Agent 运行在 32 位 Python 解释器环境下。PyTorch CUDA 仅支持 64 位 (x64) 架构！")
		r.Suggestions = append(r.Suggestions, "请使用 64 位 Python 3.12 重新构建虚拟环境：uv venv --python 3.12")
	}
	if strings.HasPrefix(r.ProbeResult.PythonVersion, "3.14") || strings.HasPrefix(r.ProbeResult.PythonVersion, "3.15") {
		r.Issues = append(r.Issues, fmt.Sprintf("检测到 Agent 虚拟环境使用了 Python %s。PyTorch 官方目前最高仅支持至 Python 3.13（推荐 Python 3.12）！", r.ProbeResult.PythonVersion))
		r.Suggestions = append(r.Suggestions, "请重新使用 Python 3.12 创建虚拟环境并同步依赖：\n"+
			"   1) 删除现有 .venv: Remove-Item -Recurse -Force .venv\n"+
			"   2) 用 uv 指定 3.12: uv venv --python 3.12\n"+
			"   3) 安装 CUDA 依赖: uv pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124")
	}
}

func (r *Report) checkTorchCUDA(hasNvidiaHardware bool) {
	if !r.ProbeRun {
		return
	}
	if !r.ProbeResult.Torch.Installed {
		r.Issues = append(r.Issues, "Agent Python 环境未安装 PyTorch 深度学习框架。")
		r.Suggestions = append(r.Suggestions, "请在 Agent 目录执行安装: "+r.buildTorchInstallCmd())
		return
	}

	if r.ProbeResult.Torch.CUDAAvailable {
		r.HasGPUAcceleration = true
		return
	}

	tVer := r.ProbeResult.Torch.Version
	tCUDA := r.ProbeResult.Torch.CUDAVersion
	if tCUDA == nil || *tCUDA == "" || strings.Contains(tVer, "+cpu") {
		r.Issues = append(r.Issues, fmt.Sprintf("当前 Python 环境安装的是 PyTorch 纯 CPU 版本 (%s)，缺少 CUDA 加速核心。", tVer))
		if hasNvidiaHardware {
			r.Suggestions = append(r.Suggestions,
				"【重要修复】本机配备有 NVIDIA 显卡硬件，但当前 PyTorch 未集成 CUDA 支持。在 Windows 上通过普通 pip/uv 可能会默认拉取 CPU 版本。\n"+
					"   请进入 Agent 目录并执行以下命令强制安装带 CUDA 12.4 支持的 PyTorch：\n"+
					"   >> "+r.buildTorchInstallCmd())
		}
		return
	}

	r.Issues = append(r.Issues, fmt.Sprintf("PyTorch 已编译 CUDA 支持 (CUDA %s)，但在本系统初始化失败 (torch.cuda.is_available() == False)。", *tCUDA))
	if r.NvidiaDriver == "" {
		r.Issues = append(r.Issues, "未检测到有效的 NVIDIA 驱动版本。")
		r.Suggestions = append(r.Suggestions, "请前往 NVIDIA 官方驱动网站下载安装最新的显卡驱动程序 (Game Ready 或 Studio Driver)。")
	} else {
		r.Suggestions = append(r.Suggestions, fmt.Sprintf("当前 NVIDIA 驱动版本为 %s，可能与 PyTorch 内置的 CUDA %s 不兼容，建议更新 NVIDIA 显卡驱动至最新版。", r.NvidiaDriver, *tCUDA))
	}
}

func (r *Report) checkNvidiaDriver(hasNvidiaHardware bool) {
	if hasNvidiaHardware && r.NvidiaSmiPath == "" {
		r.Issues = append(r.Issues, "系统硬件列表中检测到 NVIDIA 显示芯片，但未找到 nvidia-smi 诊断管理工具。")
		r.Suggestions = append(r.Suggestions, "建议重新安装或更新 NVIDIA 官方显卡驱动，确保驱动附带管理工具完整就绪。")
	}
}

func (r *Report) checkFFmpeg() {
	if r.FFmpegPath != "" {
		return
	}
	r.Issues = append(r.Issues, "系统 PATH 中未找到 ffmpeg 工具，音视频预处理与格式转换可能会受影响。")
	if runtime.GOOS == "windows" {
		r.Suggestions = append(r.Suggestions, "建议通过 Windows 终端执行: winget install Gyan.FFmpeg 或下载 ffmpeg.exe 并加入系统环境变量 PATH。")
	} else {
		r.Suggestions = append(r.Suggestions, "建议通过包管理器安装: sudo apt install ffmpeg 或 brew install ffmpeg。")
	}
}

func (r *Report) checkController() {
	if r.ConfigFileExists && r.ControllerURL != "" && !r.ControllerReach {
		r.Issues = append(r.Issues, fmt.Sprintf("无法连接至中央调度控制器 (Controller URL: %s): %s", r.ControllerURL, r.ControllerStatus))
		r.Suggestions = append(r.Suggestions, "请检查 config.yaml 中的 controller_url 地址是否正确，并确认服务端服务已正常启动。")
	}
}

func (r *Report) buildTorchInstallCmd() string {
	if r.UvPath != "" {
		return "uv pip install --upgrade --force-reinstall torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124"
	}
	py := r.AgentPython
	if py == "" {
		py = "python"
	}
	return py + " -m pip install --upgrade --force-reinstall torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu124"
}

// Format returns a formatted, human-readable terminal report.
func (r *Report) Format() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38BDF8"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC"))
	tagSuccess := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render("✓")
	tagWarn := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B")).Render("!")
	tagFail := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")).Render("✗")

	b.WriteString("\n" + titleStyle.Render("================ Cyphr Agent 环境健康诊断 (Doctor) ================") + "\n\n")

	r.renderHost(&b, sectionStyle)
	r.renderGPU(&b, sectionStyle, tagSuccess, tagWarn)
	r.renderTooling(&b, sectionStyle, tagSuccess, tagWarn)
	r.renderAgent(&b, sectionStyle, tagSuccess, tagWarn, tagFail)
	r.renderConfigAndNetwork(&b, sectionStyle, tagSuccess, tagWarn, tagFail)
	r.renderSummary(&b, sectionStyle, tagFail)

	return b.String()
}

func (r *Report) renderHost(b *strings.Builder, sectionStyle lipgloss.Style) {
	b.WriteString(sectionStyle.Render("【1】操作系统与宿主信息 (Host Environment)") + "\n")
	fmt.Fprintf(b, "  • 操作系统: %s %s (CPU 核心数: %d)\n", r.OS, r.Arch, r.CPUs)
	if r.Hostname != "" {
		fmt.Fprintf(b, "  • 机器名称: %s\n", r.Hostname)
	}
	b.WriteString("\n")
}

func (r *Report) renderGPU(b *strings.Builder, sectionStyle lipgloss.Style, tagSuccess, tagWarn string) {
	b.WriteString(sectionStyle.Render("【2】GPU 物理硬件与显卡驱动 (GPU & Driver)") + "\n")
	if len(r.PhysicalGPUs) > 0 {
		b.WriteString("  • 系统检测到的显示适配器:\n")
		for _, g := range r.PhysicalGPUs {
			fmt.Fprintf(b, "    - %s\n", g)
		}
	} else {
		b.WriteString("  • 系统检测到的显示适配器: (未能通过系统接口枚举到独立显卡)\n")
	}

	if r.NvidiaSmiPath != "" {
		fmt.Fprintf(b, "  • %s NVIDIA 工具链: %s\n", tagSuccess, r.NvidiaSmiPath)
		if r.NvidiaDriver != "" {
			fmt.Fprintf(b, "  • %s 显卡驱动版本: %s\n", tagSuccess, r.NvidiaDriver)
		}
		if r.NvidiaCUDA != "" {
			fmt.Fprintf(b, "  • %s 驱动最高支持 CUDA: %s\n", tagSuccess, r.NvidiaCUDA)
		}
		if len(r.GPUDevices) > 0 {
			b.WriteString("  • 识别到的 NVIDIA 设备:\n")
			for i, dev := range r.GPUDevices {
				fmt.Fprintf(b, "    [%d] %s (显存: %d MB / 剩余: %d MB)\n", i, dev.Name, dev.TotalVRAMMB, dev.FreeVRAMMB)
			}
		}
	} else {
		fmt.Fprintf(b, "  • %s NVIDIA 工具链: 未找到 nvidia-smi (请确认是否安装官方显卡驱动)\n", tagWarn)
	}
	b.WriteString("\n")
}

func (r *Report) renderTooling(b *strings.Builder, sectionStyle lipgloss.Style, tagSuccess, tagWarn string) {
	b.WriteString(sectionStyle.Render("【3】系统工具与包管理器 (Tooling)") + "\n")
	if r.SysPythonPath != "" {
		bitStr := "64-bit"
		if !r.SysPython64 {
			bitStr = "32-bit (注意: 不支持 CUDA)"
		}
		fmt.Fprintf(b, "  • %s 系统 Python: %s (%s, %s)\n", tagSuccess, r.SysPythonPath, r.SysPythonVer, bitStr)
	} else {
		fmt.Fprintf(b, "  • %s 系统 Python: 系统 PATH 中未找到 python/python3\n", tagWarn)
	}

	if r.UvPath != "" {
		fmt.Fprintf(b, "  • %s 高性能包管理 (uv): %s (%s)\n", tagSuccess, r.UvPath, r.UvVer)
	} else {
		fmt.Fprintf(b, "  • %s 高性能包管理 (uv): 未安装 (建议安装 uv 获得极速依赖同步)\n", tagWarn)
	}

	if r.FFmpegPath != "" {
		fmt.Fprintf(b, "  • %s 多媒体工具 (FFmpeg): %s (%s)\n", tagSuccess, r.FFmpegPath, r.FFmpegVer)
	} else {
		fmt.Fprintf(b, "  • %s 多媒体工具 (FFmpeg): 未在 PATH 中找到 ffmpeg.exe\n", tagWarn)
	}
	b.WriteString("\n")
}

func (r *Report) renderAgent(b *strings.Builder, sectionStyle lipgloss.Style, tagSuccess, tagWarn, tagFail string) {
	b.WriteString(sectionStyle.Render("【4】Agent 虚拟环境与 PyTorch 深度探针 (PyTorch & CUDA Probe)") + "\n")
	fmt.Fprintf(b, "  • Agent 安装路径: %s\n", r.AgentDir)
	if r.AgentInstalled {
		fmt.Fprintf(b, "  • %s Agent 源码主入口: 已就绪 (main.py 存在)\n", tagSuccess)
	} else {
		fmt.Fprintf(b, "  • %s Agent 源码主入口: 未安装 (可执行 'installer install' 部署)\n", tagFail)
	}

	if r.VenvExists {
		fmt.Fprintf(b, "  • %s 虚拟环境 (.venv): 已创建 (%s)\n", tagSuccess, r.VenvDir)
	} else {
		fmt.Fprintf(b, "  • %s 虚拟环境 (.venv): 未检测到 .venv 目录\n", tagWarn)
	}

	if r.AgentPython != "" {
		fmt.Fprintf(b, "  • 使用的 Python 解释器: %s\n", r.AgentPython)
	}

	if r.ProbeRun {
		r.renderProbeResult(b, tagSuccess, tagWarn, tagFail)
	}
	b.WriteString("\n")
}

func (r *Report) renderProbeResult(b *strings.Builder, tagSuccess, tagWarn, tagFail string) {
	if r.ProbeErr != "" {
		fmt.Fprintf(b, "  • %s 探针执行异常: %s\n", tagFail, r.ProbeErr)
		return
	}
	pr := r.ProbeResult
	if pr.Torch.Installed {
		cudaBuiltStr := "无 CUDA 编译支持"
		if pr.Torch.CUDAVersion != nil && *pr.Torch.CUDAVersion != "" {
			cudaBuiltStr = fmt.Sprintf("CUDA %s", *pr.Torch.CUDAVersion)
		}
		fmt.Fprintf(b, "  • %s PyTorch 库: 已安装 (版本: %s, %s)\n", tagSuccess, pr.Torch.Version, cudaBuiltStr)

		if pr.Torch.CUDAAvailable {
			fmt.Fprintf(b, "  • %s PyTorch CUDA 加速: 【已启用】(可用 GPU 数量: %d, 设备: %s)\n",
				tagSuccess, pr.Torch.DeviceCount, pr.Torch.DeviceName)
		} else {
			fmt.Fprintf(b, "  • %s PyTorch CUDA 加速: 【不可用 / 未启用】(torch.cuda.is_available() == False)\n", tagFail)
		}
	} else {
		fmt.Fprintf(b, "  • %s PyTorch 库: 未安装或导入失败 (%s)\n", tagFail, pr.Torch.Error)
	}

	if pr.QwenASR {
		fmt.Fprintf(b, "  • %s Qwen-ASR 推理依赖: 已安装就绪\n", tagSuccess)
	} else {
		fmt.Fprintf(b, "  • %s Qwen-ASR 推理依赖: 未安装 (缺少 qwen_asr 模块)\n", tagWarn)
	}
}

func (r *Report) renderConfigAndNetwork(b *strings.Builder, sectionStyle lipgloss.Style, tagSuccess, tagWarn, tagFail string) {
	b.WriteString(sectionStyle.Render("【5】Agent 配置与服务端网络连通性 (Config & Network)") + "\n")
	if !r.ConfigFileExists {
		fmt.Fprintf(b, "  • %s 配置文件: 未找到 config.yaml (请从 config.example.yaml 复制并配置)\n\n", tagWarn)
		return
	}
	fmt.Fprintf(b, "  • %s 配置文件: config.yaml 存在\n", tagSuccess)
	if r.ControllerURL == "" {
		fmt.Fprintf(b, "  • %s 调度控制器地址: 未在 config.yaml 中配置 controller_url\n\n", tagWarn)
		return
	}
	fmt.Fprintf(b, "  • 调度控制器地址: %s\n", r.ControllerURL)
	if r.ControllerReach {
		fmt.Fprintf(b, "  • %s 服务端连通性: 正常 (%s)\n", tagSuccess, r.ControllerStatus)
	} else {
		fmt.Fprintf(b, "  • %s 服务端连通性: 异常 (%s)\n", tagFail, r.ControllerStatus)
	}
	b.WriteString("\n")
}

func (r *Report) renderSummary(b *strings.Builder, sectionStyle lipgloss.Style, tagFail string) {
	b.WriteString(sectionStyle.Render("==================== 综合诊断结论与建议 ====================") + "\n")
	if r.HasGPUAcceleration {
		badge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981")).Render("【状态极佳】GPU 硬件加速与 CUDA 环境已完全就绪！")
		b.WriteString("  " + badge + "\n")
		b.WriteString("  Agent 在启动后将自动注册为 GPU 模式，并可全速执行 ASR 本地推理。\n")
	} else {
		badge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444")).Render("【排查提示】当前 Agent 无法使用 GPU 进行推理加速（将回落至 CPU 模式）")
		b.WriteString("  " + badge + "\n")
	}

	if len(r.Issues) > 0 {
		b.WriteString("\n发现的问题:\n")
		for i, issue := range r.Issues {
			fmt.Fprintf(b, "  [%d] %s %s\n", i+1, tagFail, issue)
		}
	}

	if len(r.Suggestions) > 0 {
		b.WriteString("\n推荐解决步骤:\n")
		for i, sug := range r.Suggestions {
			fmt.Fprintf(b, "  [%d] %s\n", i+1, sug)
		}
	} else if r.HasGPUAcceleration {
		b.WriteString("  ✓ 软硬件环境未发现异常，可随时执行 'installer start' 启动服务。\n")
	}
	b.WriteString("\n")
}
