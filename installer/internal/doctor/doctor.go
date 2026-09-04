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
	// Look in PATH first
	smiPath, err := exec.LookPath("nvidia-smi")
	if err != nil && runtime.GOOS == "windows" {
		// Common Windows fallbacks
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

	// Query GPU details via nvidia-smi csv
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, smiPath, "--query-gpu=name,driver_version,memory.total,memory.free", "--format=csv,noheader,nounits")
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			parts := strings.Split(l, ",")
			if len(parts) >= 4 {
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

	// Also extract supported CUDA version from header
	ctxHeader, cancelHeader := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelHeader()
	hCmd := exec.CommandContext(ctxHeader, smiPath)
	if out, err := hCmd.Output(); err == nil {
		s := string(out)
		if idx := strings.Index(s, "CUDA Version:"); idx != -1 {
			sub := s[idx+len("CUDA Version:"):]
			fields := strings.Fields(sub)
			if len(fields) > 0 {
				r.NvidiaCUDA = fields[0]
			}
		}
	}
}

func (r *Report) inspectSystemTools() {
	// 1. System Python
	for _, bin := range []string{"python3", "python", "py"} {
		if p, err := exec.LookPath(bin); err == nil {
			r.SysPythonPath = p
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if out, err := exec.CommandContext(ctx, p, "--version").CombinedOutput(); err == nil {
				r.SysPythonVer = strings.TrimSpace(string(out))
			}
			cancel()

			ctxBit, cancelBit := context.WithTimeout(context.Background(), 3*time.Second)
			if out, err := exec.CommandContext(ctxBit, p, "-c", "import sys; print(sys.maxsize > 2**32)").CombinedOutput(); err == nil {
				r.SysPython64 = strings.TrimSpace(string(out)) == "True"
			}
			cancelBit()
			break
		}
	}

	// 2. uv
	if p, err := exec.LookPath("uv"); err == nil {
		r.UvPath = p
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if out, err := exec.CommandContext(ctx, p, "--version").CombinedOutput(); err == nil {
			r.UvVer = strings.TrimSpace(string(out))
		}
		cancel()
	}

	// 3. ffmpeg
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		r.FFmpegPath = p
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if out, err := exec.CommandContext(ctx, p, "-version").CombinedOutput(); err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 0 {
				r.FFmpegVer = strings.TrimSpace(lines[0])
			}
		}
		cancel()
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

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.AgentPython, "-c", script)
	cmd.Dir = r.AgentDir
	out, err := cmd.CombinedOutput()
	r.ProbeRun = true
	if err != nil {
		r.ProbeErr = fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))
		return
	}

	// Parse JSON from output
	raw := strings.TrimSpace(string(out))
	// In case there are warnings before json, find first '{' and last '}'
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start != -1 && end != -1 && end > start {
		raw = raw[start : end+1]
	}

	if err := json.Unmarshal([]byte(raw), &r.ProbeResult); err != nil {
		r.ProbeErr = fmt.Sprintf("解析探针输出失败: %v, 原始输出:\n%s", err, raw)
	}
}

func (r *Report) inspectConfigAndNetwork(paths *config.AppPaths) {
	cfgFile := paths.ConfigFile
	if _, err := os.Stat(cfgFile); err == nil {
		r.ConfigFileExists = true
		data, err := os.ReadFile(cfgFile)
		if err == nil {
			var m map[string]any
			if err := yaml.Unmarshal(data, &m); err == nil {
				if u, ok := m["controller_url"].(string); ok {
					r.ControllerURL = strings.TrimSpace(u)
				}
			}
		}
	}

	if r.ControllerURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		pingURL := strings.TrimRight(r.ControllerURL, "/") + "/api/v1/ping"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err == nil {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				r.ControllerReach = true
				r.ControllerStatus = fmt.Sprintf("HTTP %d OK", resp.StatusCode)
			} else {
				r.ControllerReach = false
				r.ControllerStatus = fmt.Sprintf("连接失败 (%v)", err)
			}
		}
	}
}

func (r *Report) synthesize() {
	// Has NVIDIA physical GPU?
	hasNvidiaHardware := len(r.GPUDevices) > 0 || r.NvidiaSmiPath != ""
	for _, a := range r.PhysicalGPUs {
		if strings.Contains(strings.ToLower(a), "nvidia") || strings.Contains(strings.ToLower(a), "geforce") || strings.Contains(strings.ToLower(a), "quadro") || strings.Contains(strings.ToLower(a), "tesla") || strings.Contains(strings.ToLower(a), "rtx") {
			hasNvidiaHardware = true
			break
		}
	}

	// 1. Check Python Bit width & Version Compatibility
	if r.ProbeRun && r.ProbeResult.PythonVersion != "" {
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

	// 2. Check PyTorch CUDA capability
	if r.ProbeRun && r.ProbeResult.Torch.Installed {
		if r.ProbeResult.Torch.CUDAAvailable {
			r.HasGPUAcceleration = true
		} else {
			// Why is CUDA unavailable?
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
			} else {
				r.Issues = append(r.Issues, fmt.Sprintf("PyTorch 已编译 CUDA 支持 (CUDA %s)，但在本系统初始化失败 (torch.cuda.is_available() == False)。", *tCUDA))
				if r.NvidiaDriver == "" {
					r.Issues = append(r.Issues, "未检测到有效的 NVIDIA 驱动版本。")
					r.Suggestions = append(r.Suggestions, "请前往 NVIDIA 官方驱动网站下载安装最新的显卡驱动程序 (Game Ready 或 Studio Driver)。")
				} else {
					r.Suggestions = append(r.Suggestions, fmt.Sprintf("当前 NVIDIA 驱动版本为 %s，可能与 PyTorch 内置的 CUDA %s 不兼容，建议更新 NVIDIA 显卡驱动至最新版。", r.NvidiaDriver, *tCUDA))
				}
			}
		}
	} else if r.ProbeRun && !r.ProbeResult.Torch.Installed {
		r.Issues = append(r.Issues, "Agent Python 环境未安装 PyTorch 深度学习框架。")
		r.Suggestions = append(r.Suggestions, "请在 Agent 目录执行安装: "+r.buildTorchInstallCmd())
	}

	// 3. Check NVIDIA Driver when Hardware exists
	if hasNvidiaHardware && r.NvidiaSmiPath == "" {
		r.Issues = append(r.Issues, "系统硬件列表中检测到 NVIDIA 显示芯片，但未找到 nvidia-smi 诊断管理工具。")
		r.Suggestions = append(r.Suggestions, "建议重新安装或更新 NVIDIA 官方显卡驱动，确保驱动附带管理工具完整就绪。")
	}

	// 4. Check FFmpeg
	if r.FFmpegPath == "" {
		r.Issues = append(r.Issues, "系统 PATH 中未找到 ffmpeg 工具，音视频预处理与格式转换可能会受影响。")
		if runtime.GOOS == "windows" {
			r.Suggestions = append(r.Suggestions, "建议通过 Windows 终端执行: winget install Gyan.FFmpeg 或下载 ffmpeg.exe 并加入系统环境变量 PATH。")
		} else {
			r.Suggestions = append(r.Suggestions, "建议通过包管理器安装: sudo apt install ffmpeg 或 brew install ffmpeg。")
		}
	}

	// 5. Check Controller URL
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

	// 1. System Info
	b.WriteString(sectionStyle.Render("【1】操作系统与宿主信息 (Host Environment)") + "\n")
	b.WriteString(fmt.Sprintf("  • 操作系统: %s %s (CPU 核心数: %d)\n", r.OS, r.Arch, r.CPUs))
	if r.Hostname != "" {
		b.WriteString(fmt.Sprintf("  • 机器名称: %s\n", r.Hostname))
	}
	b.WriteString("\n")

	// 2. Hardware GPU & Driver
	b.WriteString(sectionStyle.Render("【2】GPU 物理硬件与显卡驱动 (GPU & Driver)") + "\n")
	if len(r.PhysicalGPUs) > 0 {
		b.WriteString("  • 系统检测到的显示适配器:\n")
		for _, g := range r.PhysicalGPUs {
			b.WriteString(fmt.Sprintf("    - %s\n", g))
		}
	} else {
		b.WriteString("  • 系统检测到的显示适配器: (未能通过系统接口枚举到独立显卡)\n")
	}

	if r.NvidiaSmiPath != "" {
		b.WriteString(fmt.Sprintf("  • %s NVIDIA 工具链: %s\n", tagSuccess, r.NvidiaSmiPath))
		if r.NvidiaDriver != "" {
			b.WriteString(fmt.Sprintf("  • %s 显卡驱动版本: %s\n", tagSuccess, r.NvidiaDriver))
		}
		if r.NvidiaCUDA != "" {
			b.WriteString(fmt.Sprintf("  • %s 驱动最高支持 CUDA: %s\n", tagSuccess, r.NvidiaCUDA))
		}
		if len(r.GPUDevices) > 0 {
			b.WriteString("  • 识别到的 NVIDIA 设备:\n")
			for i, dev := range r.GPUDevices {
				b.WriteString(fmt.Sprintf("    [%d] %s (显存: %d MB / 剩余: %d MB)\n", i, dev.Name, dev.TotalVRAMMB, dev.FreeVRAMMB))
			}
		}
	} else {
		b.WriteString(fmt.Sprintf("  • %s NVIDIA 工具链: 未找到 nvidia-smi (请确认是否安装官方显卡驱动)\n", tagWarn))
	}
	b.WriteString("\n")

	// 3. Python & Tooling
	b.WriteString(sectionStyle.Render("【3】系统工具与包管理器 (Tooling)") + "\n")
	if r.SysPythonPath != "" {
		bitStr := "64-bit"
		if !r.SysPython64 {
			bitStr = "32-bit (注意: 不支持 CUDA)"
		}
		b.WriteString(fmt.Sprintf("  • %s 系统 Python: %s (%s, %s)\n", tagSuccess, r.SysPythonPath, r.SysPythonVer, bitStr))
	} else {
		b.WriteString(fmt.Sprintf("  • %s 系统 Python: 系统 PATH 中未找到 python/python3\n", tagWarn))
	}

	if r.UvPath != "" {
		b.WriteString(fmt.Sprintf("  • %s 高性能包管理 (uv): %s (%s)\n", tagSuccess, r.UvPath, r.UvVer))
	} else {
		b.WriteString(fmt.Sprintf("  • %s 高性能包管理 (uv): 未安装 (建议安装 uv 获得极速依赖同步)\n", tagWarn))
	}

	if r.FFmpegPath != "" {
		b.WriteString(fmt.Sprintf("  • %s 多媒体工具 (FFmpeg): %s (%s)\n", tagSuccess, r.FFmpegPath, r.FFmpegVer))
	} else {
		b.WriteString(fmt.Sprintf("  • %s 多媒体工具 (FFmpeg): 未在 PATH 中找到 ffmpeg.exe\n", tagWarn))
	}
	b.WriteString("\n")

	// 4. Agent Environment & Deep Python Probe
	b.WriteString(sectionStyle.Render("【4】Agent 虚拟环境与 PyTorch 深度探针 (PyTorch & CUDA Probe)") + "\n")
	b.WriteString(fmt.Sprintf("  • Agent 安装路径: %s\n", r.AgentDir))
	if r.AgentInstalled {
		b.WriteString(fmt.Sprintf("  • %s Agent 源码主入口: 已就绪 (main.py 存在)\n", tagSuccess))
	} else {
		b.WriteString(fmt.Sprintf("  • %s Agent 源码主入口: 未安装 (可执行 'installer install' 部署)\n", tagFail))
	}

	if r.VenvExists {
		b.WriteString(fmt.Sprintf("  • %s 虚拟环境 (.venv): 已创建 (%s)\n", tagSuccess, r.VenvDir))
	} else {
		b.WriteString(fmt.Sprintf("  • %s 虚拟环境 (.venv): 未检测到 .venv 目录\n", tagWarn))
	}

	if r.AgentPython != "" {
		b.WriteString(fmt.Sprintf("  • 使用的 Python 解释器: %s\n", r.AgentPython))
	}

	if r.ProbeRun {
		if r.ProbeErr != "" {
			b.WriteString(fmt.Sprintf("  • %s 探针执行异常: %s\n", tagFail, r.ProbeErr))
		} else {
			pr := r.ProbeResult
			// PyTorch status
			if pr.Torch.Installed {
				cudaBuiltStr := "无 CUDA 编译支持"
				if pr.Torch.CUDAVersion != nil && *pr.Torch.CUDAVersion != "" {
					cudaBuiltStr = fmt.Sprintf("CUDA %s", *pr.Torch.CUDAVersion)
				}
				b.WriteString(fmt.Sprintf("  • %s PyTorch 库: 已安装 (版本: %s, %s)\n", tagSuccess, pr.Torch.Version, cudaBuiltStr))

				if pr.Torch.CUDAAvailable {
					b.WriteString(fmt.Sprintf("  • %s PyTorch CUDA 加速: 【已启用】(可用 GPU 数量: %d, 设备: %s)\n",
						tagSuccess, pr.Torch.DeviceCount, pr.Torch.DeviceName))
				} else {
					b.WriteString(fmt.Sprintf("  • %s PyTorch CUDA 加速: 【不可用 / 未启用】(torch.cuda.is_available() == False)\n", tagFail))
				}
			} else {
				b.WriteString(fmt.Sprintf("  • %s PyTorch 库: 未安装或导入失败 (%s)\n", tagFail, pr.Torch.Error))
			}

			// Qwen-ASR package
			if pr.QwenASR {
				b.WriteString(fmt.Sprintf("  • %s Qwen-ASR 推理依赖: 已安装就绪\n", tagSuccess))
			} else {
				b.WriteString(fmt.Sprintf("  • %s Qwen-ASR 推理依赖: 未安装 (缺少 qwen_asr 模块)\n", tagWarn))
			}
		}
	}
	b.WriteString("\n")

	// 5. Config & Network
	b.WriteString(sectionStyle.Render("【5】Agent 配置与服务端网络连通性 (Config & Network)") + "\n")
	if r.ConfigFileExists {
		b.WriteString(fmt.Sprintf("  • %s 配置文件: config.yaml 存在\n", tagSuccess))
		if r.ControllerURL != "" {
			b.WriteString(fmt.Sprintf("  • 调度控制器地址: %s\n", r.ControllerURL))
			if r.ControllerReach {
				b.WriteString(fmt.Sprintf("  • %s 服务端连通性: 正常 (%s)\n", tagSuccess, r.ControllerStatus))
			} else {
				b.WriteString(fmt.Sprintf("  • %s 服务端连通性: 异常 (%s)\n", tagFail, r.ControllerStatus))
			}
		} else {
			b.WriteString(fmt.Sprintf("  • %s 调度控制器地址: 未在 config.yaml 中配置 controller_url\n", tagWarn))
		}
	} else {
		b.WriteString(fmt.Sprintf("  • %s 配置文件: 未找到 config.yaml (请从 config.example.yaml 复制并配置)\n", tagWarn))
	}
	b.WriteString("\n")

	// 6. Summary & Recommendations
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
			b.WriteString(fmt.Sprintf("  [%d] %s %s\n", i+1, tagFail, issue))
		}
	}

	if len(r.Suggestions) > 0 {
		b.WriteString("\n推荐解决步骤:\n")
		for i, sug := range r.Suggestions {
			b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, sug))
		}
	} else if r.HasGPUAcceleration {
		b.WriteString("  ✓ 软硬件环境未发现异常，可随时执行 'installer start' 启动服务。\n")
	}
	b.WriteString("\n")

	return b.String()
}
