package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"cyphr/installer/internal/agent"
	"cyphr/installer/internal/config"
	"cyphr/installer/internal/doctor"
	"cyphr/installer/internal/model"
	"cyphr/installer/internal/tui"
	"cyphr/installer/internal/updater"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if parsed, err := zerolog.ParseLevel(lvl); err == nil {
			zerolog.SetGlobalLevel(parsed)
		}
	}
}

func main() {
	paths := config.NewAppPaths()

	// If CLI arguments are provided, handle them directly without launching TUI
	if len(os.Args) > 1 {
		handleCLI(paths, os.Args[1:])
		return
	}

	// Interactive Bubble Tea TUI
	// Do not use WithMouseCellMotion() so users can select and copy text in their terminal.
	p := tea.NewProgram(tui.NewModel(paths), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting TUI: %v\n", err)
		os.Exit(1)
	}
}

func handleCLI(paths *config.AppPaths, args []string) {
	agentSvc := agent.NewService(paths)
	modelSvc := model.NewService(paths)

	cmd := args[0]
	switch cmd {
	case "start":
		fmt.Println("正在启动 Agent 服务...")
		st, err := agentSvc.Start()
		if err != nil {
			fmt.Printf("✗ 启动失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Agent 服务启动成功！(PID: %d)\n", st.PID)
		fmt.Printf("  日志输出路径: %s\n", st.LogPath)

	case "stop":
		fmt.Println("正在停止 Agent 服务...")
		err := agentSvc.Stop()
		if err != nil {
			fmt.Printf("✗ 停止服务失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Agent 服务已成功停止。")

	case "restart":
		fmt.Println("正在重启 Agent 服务...")
		st, err := agentSvc.Restart()
		if err != nil {
			fmt.Printf("✗ 重启失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Agent 服务已成功重启！(PID: %d)\n", st.PID)

	case "install":
		targetDir := paths.AgentDir
		if len(args) >= 2 && !strings.HasPrefix(args[1], "-") {
			targetDir = args[1]
		}
		useMirror := true
		skipVenv := false
		for _, a := range args[1:] {
			if a == "--no-mirror" {
				useMirror = false
			}
			if a == "--skip-venv" {
				skipVenv = true
			}
		}

		fmt.Println("正在准备安装 / 更新 Cyphr Agent...")
		fmt.Printf("  目标安装目录: %s\n", targetDir)
		opts := agent.InstallOptions{
			TargetDir:  targetDir,
			Version:    "latest",
			RepoOwner:  "Rain-kl",
			RepoName:   "Cyphr",
			UseMirror:  useMirror,
			SkipVenv:   skipVenv,
			AutoConfig: true,
		}

		err := agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
			fmt.Printf("  [%-8s] %s\n", stage, message)
		})
		if err != nil {
			fmt.Printf("✗ 安装失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Cyphr Agent 安装完成！可执行 'installer start' 启动服务。")

	case "update":
		target := "all"
		if len(args) >= 2 && !strings.HasPrefix(args[1], "-") {
			target = args[1]
		}
		useMirror := true
		for _, a := range args[1:] {
			if a == "--no-mirror" {
				useMirror = false
			}
		}

		switch target {
		case "agent":
			fmt.Println("正在从 GitHub Release 更新 Cyphr Agent...")
			opts := agent.InstallOptions{
				TargetDir:  paths.AgentDir,
				Version:    "latest",
				RepoOwner:  "Rain-kl",
				RepoName:   "Cyphr",
				UseMirror:  useMirror,
				SkipVenv:   false,
				AutoConfig: true,
			}
			err := agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
				fmt.Printf("  [%-8s] %s\n", stage, message)
			})
			if err != nil {
				fmt.Printf("✗ Agent 更新失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Cyphr Agent 已成功更新！")

		case "installer", "self":
			fmt.Println("正在从 GitHub Release 更新 Installer 自身程序...")
			err := updater.UpdateInstaller("Rain-kl", "Cyphr", useMirror, func(stage string, progress float64, message string) {
				fmt.Printf("  [%-8s] %s\n", stage, message)
			})
			if err != nil {
				fmt.Printf("✗ Installer 更新失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Cyphr Installer 已成功更新至最新版本！")

		default:
			// Update both
			fmt.Println("=== 1/2 更新 Cyphr Agent ===")
			opts := agent.InstallOptions{
				TargetDir:  paths.AgentDir,
				Version:    "latest",
				RepoOwner:  "Rain-kl",
				RepoName:   "Cyphr",
				UseMirror:  useMirror,
				SkipVenv:   false,
				AutoConfig: true,
			}
			_ = agentSvc.InstallAgent(opts, func(stage string, progress float64, message string) {
				fmt.Printf("  [%-8s] %s\n", stage, message)
			})

			fmt.Println("\n=== 2/2 更新 Cyphr Installer ===")
			err := updater.UpdateInstaller("Rain-kl", "Cyphr", useMirror, func(stage string, progress float64, message string) {
				fmt.Printf("  [%-8s] %s\n", stage, message)
			})
			if err != nil {
				fmt.Printf("✗ Installer 更新提示: %v\n", err)
			} else {
				fmt.Println("✓ Cyphr Installer 已更新至最新版本！")
			}
		}

	case "status":
		st := agentSvc.Status()
		fmt.Println("=== Agent 服务状态 ===")
		if st.Running {
			fmt.Printf("服务状态: 运行中 (Running)\n")
			fmt.Printf("进程 PID: %d\n", st.PID)
			fmt.Printf("运行时长: %s\n", st.Uptime)
			fmt.Printf("物理内存: ~%d MB\n", st.RSSMB)
		} else {
			fmt.Println("服务状态: 未运行 (Stopped)")
		}

		dst := modelSvc.Status()
		fmt.Println("\n=== 模型与下载状态 ===")
		if dst.Running {
			fmt.Printf("下载任务: 正在后台下载中 (Downloading)\n")
			fmt.Printf("  下载模型: %s\n", dst.ModelID)
			fmt.Printf("  存储目录: models/%s\n", dst.PkgDir)
			fmt.Printf("  进程 PID: %d (已运行 %s)\n", dst.PID, dst.Uptime)
		} else {
			fmt.Println("下载任务: 当前无后台下载任务在运行")
		}

		fmt.Println("\n本地已安装模型 (models/ 目录):")
		models, _ := modelSvc.ListLocalModels()
		if len(models) == 0 {
			fmt.Println("  (暂无已安装模型)")
		} else {
			for _, m := range models {
				fmt.Printf("  - %s (%s) [%s]\n", m.DirName, m.DiskSize, m.Status)
			}
		}

	case "_worker-download":
		// Hidden internal worker command invoked via proc.Daemonize for background download
		if len(args) < 5 {
			fmt.Println("用法: installer _worker-download <MODEL_ID> <PKG_DIR> <SOURCE> <ENDPOINT>")
			os.Exit(1)
		}
		workerModelID := args[1]
		workerPkgDir := args[2]
		workerSource := args[3]
		workerEndpoint := args[4]
		err := modelSvc.ExecuteDownloadTask(model.DownloadOptions{
			ModelID:  workerModelID,
			PkgDir:   workerPkgDir,
			Source:   workerSource,
			Endpoint: workerEndpoint,
			Mode:     "fg",
		})
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)

	case "download":
		if len(args) < 2 {
			fmt.Println("用法: installer download <MODEL_ID> [PKG_DIR] [SOURCE: hf/modelscope] [ENDPOINT] [MODE: bg/fg]")
			fmt.Println("示例: installer download Qwen/Qwen3-ASR-0.6B qwen3-asr-0.6b modelscope")
			fmt.Println("示例: installer download Qwen/Qwen3-ASR-0.6B qwen3-asr-0.6b hf https://hf-mirror.com bg")
			os.Exit(1)
		}
		modelID := args[1]
		pkgDir := ""
		if len(args) >= 3 && !strings.HasPrefix(args[2], "-") {
			pkgDir = args[2]
		}
		source := "hf"
		if len(args) >= 4 {
			source = args[3]
		}
		endpoint := ""
		if len(args) >= 5 {
			endpoint = args[4]
		}
		mode := "bg"
		if len(args) >= 6 {
			mode = args[5]
		}

		if strings.ToLower(source) == "modelscope" {
			source = "modelscope"
			if endpoint == "" {
				endpoint = "https://modelscope.cn"
			}
		} else {
			source = "huggingface"
			if endpoint == "" {
				endpoint = "https://hf-mirror.com"
			}
		}

		fmt.Println("正在启动下载任务...")
		fmt.Printf("  模型 ID : %s\n", modelID)
		fmt.Printf("  平台源  : %s\n", source)
		fmt.Printf("  下载源  : %s\n", endpoint)
		pid, err := modelSvc.StartDownload(model.DownloadOptions{
			ModelID:  modelID,
			PkgDir:   pkgDir,
			Source:   source,
			Endpoint: endpoint,
			Mode:     mode,
		})
		if err != nil {
			fmt.Printf("✗ 启动下载失败: %v\n", err)
			os.Exit(1)
		}
		if mode == "bg" {
			fmt.Printf("✓ 后台下载任务已成功启动！(PID: %d)\n", pid)
		}

	case "progress":
		dst := modelSvc.Status()
		fmt.Println("=== 模型下载状态与进度 ===")
		if dst.Running {
			fmt.Printf("运行状态: 正在后台下载中 (Downloading)\n")
			fmt.Printf("进程 PID: %d (已运行 %s)\n", dst.PID, dst.Uptime)
			fmt.Printf("当前模型: %s\n", dst.ModelID)
			fmt.Printf("存储路径: models/%s\n", dst.PkgDir)
			fmt.Printf("下载来源: %s\n", dst.Endpoint)
			fmt.Printf("启动时间: %s\n", dst.StartTime)
			if dst.DiskUsage != "" {
				fmt.Printf("当前落盘: %s\n", dst.DiskUsage)
			}
			fmt.Println("\n最新下载日志预览:")
			for _, line := range dst.RecentLogs {
				fmt.Println(line)
			}
		} else {
			fmt.Println("运行状态: 当前无后台下载任务在运行")
			if len(dst.RecentLogs) > 0 {
				fmt.Println("\n最近下载日志:")
				for _, line := range dst.RecentLogs {
					fmt.Println(line)
				}
			}
		}

	case "stop-download":
		fmt.Println("正在停止后台下载任务...")
		err := modelSvc.StopDownload()
		if err != nil {
			fmt.Printf("✗ 停止下载失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ 下载任务已停止 (分块已妥善保留，下次自动断点续传)。")

	case "models":
		fmt.Println("=== 本地已安装模型列表 ===")
		models, _ := modelSvc.ListLocalModels()
		if len(models) == 0 {
			fmt.Println("  (暂无已下载模型)")
		} else {
			for _, m := range models {
				fmt.Printf("  - %s (%s) [%s]\n", m.DirName, m.DiskSize, m.Status)
			}
		}

	case "doctor":
		rep := doctor.Run(paths)
		fmt.Print(rep.Format())

	case "help", "-h", "--help":
		printHelp()

	default:
		fmt.Printf("未知命令: %s\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Cyphr Agent 管理面板 & 安装工具")
	fmt.Println("\n用法:")
	fmt.Println("  installer             启动交互式 Bubble Tea TUI 面板")
	fmt.Println("  installer [命令]      以无界面 CLI 模式直接执行命令")
	fmt.Println("\n可用命令:")
	fmt.Println("  start                 后台启动 Agent 服务")
	fmt.Println("  stop                  停止 Agent 服务")
	fmt.Println("  restart               重启 Agent 服务")
	fmt.Println("  install [DIR]         从 GitHub 下载并安装/部署 Agent 运行时环境")
	fmt.Println("  update [agent|self]   在线更新 Agent 运行时或 Installer 自身程序")
	fmt.Println("  status                查看综合运行状态与已安装模型")
	fmt.Println("  download <ID> [DIR]   下载指定 ASR 模型到 models/ 目录 (支持 hf 与 modelscope 源)")
	fmt.Println("  progress              查看当前后台下载进度与日志")
	fmt.Println("  stop-download         停止当前正在运行的后台模型下载任务")
	fmt.Println("  models                列出本地已下载的模型包")
	fmt.Println("  doctor                诊断并检测本地软硬件环境 (GPU、驱动、CUDA、PyTorch、FFmpeg 等)")
	fmt.Println("  help                  查看帮助信息")
}
