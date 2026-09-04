package model

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cyphr/installer/internal/config"
	"cyphr/installer/internal/proc"
)

// LocalModel represents an installed model folder on disk.
type LocalModel struct {
	DirName  string
	FullPath string
	DiskSize string
	Status   string // "完整/就绪", "未完成分块", "缺少 config.json"
	IsReady  bool
}

// DownloadMetadata contains metadata recorded when a download starts.
type DownloadMetadata struct {
	ModelID   string
	PkgDir    string
	Source    string // "huggingface" or "modelscope"
	Endpoint  string
	StartTime string
	Mode      string // "bg" or "fg"
}

// DownloadStatus represents the current background download task state.
type DownloadStatus struct {
	Running    bool
	PID        int
	Uptime     string
	ModelID    string
	PkgDir     string
	Source     string
	Endpoint   string
	StartTime  string
	DiskUsage  string
	RecentLogs []string
}

// DownloadOptions defines options for starting a download.
type DownloadOptions struct {
	ModelID  string
	PkgDir   string
	Source   string // "huggingface" or "modelscope"
	Endpoint string
	Mode     string // "bg" or "fg"
}

// Service provides model listing and download management.
type Service struct {
	paths *config.AppPaths
}

// NewService creates a new model Service.
func NewService(paths *config.AppPaths) *Service {
	return &Service{paths: paths}
}

// ListLocalModels scans the models directory and returns metadata for each installed model.
func (s *Service) ListLocalModels() ([]LocalModel, error) {
	modelsDir := s.paths.ModelsDir
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var list []LocalModel
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		fullPath := filepath.Join(modelsDir, dirName)

		size := getDirSize(fullPath)
		status, isReady := checkModelIntegrity(fullPath)

		list = append(list, LocalModel{
			DirName:  dirName,
			FullPath: fullPath,
			DiskSize: size,
			Status:   status,
			IsReady:  isReady,
		})
	}
	return list, nil
}

func checkModelIntegrity(dir string) (string, bool) {
	// Check for .part or .aria2 files
	hasPart := false
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if strings.HasSuffix(info.Name(), ".part") || strings.HasSuffix(info.Name(), ".aria2") {
				hasPart = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	if hasPart {
		return "未完成分块", false
	}

	// Check for config.json
	configJson := filepath.Join(dir, "config.json")
	if _, err := os.Stat(configJson); err != nil {
		return "缺少 config.json", false
	}

	return "完整/就绪", true
}

func getDirSize(dir string) string {
	out, err := exec.Command("du", "-sh", dir).Output()
	if err != nil {
		return "未知"
	}
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return fields[0]
	}
	return "未知"
}

// ReadMetadata parses download.info.log if present.
func (s *Service) ReadMetadata() *DownloadMetadata {
	meta := &DownloadMetadata{
		ModelID:  "未知",
		PkgDir:   "未知",
		Source:   "huggingface",
		Endpoint: "未知",
	}
	f, err := os.Open(s.paths.DownloadInfoFile)
	if err != nil {
		return meta
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := parts[0]
			v := parts[1]
			switch k {
			case "MODEL_ID":
				meta.ModelID = v
			case "PKG_DIR":
				meta.PkgDir = v
			case "SOURCE":
				meta.Source = v
			case "ENDPOINT":
				meta.Endpoint = v
			case "START_TIME":
				meta.StartTime = v
			case "MODE":
				meta.Mode = v
			}
		}
	}
	return meta
}

// Status returns current download task status.
func (s *Service) Status() *DownloadStatus {
	meta := s.ReadMetadata()
	pid, err := proc.ReadPid(s.paths.DownloadPidFile)
	if err != nil || pid <= 0 || !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.DownloadPidFile)
		return &DownloadStatus{
			Running:    false,
			ModelID:    meta.ModelID,
			PkgDir:     meta.PkgDir,
			Source:     meta.Source,
			Endpoint:   meta.Endpoint,
			StartTime:  meta.StartTime,
			RecentLogs: proc.TailLines(s.paths.DownloadLogFile, 12),
		}
	}

	stats := proc.GetStats(pid)
	targetDir := filepath.Join(s.paths.ModelsDir, meta.PkgDir)
	diskUsage := ""
	if _, err := os.Stat(targetDir); err == nil {
		diskUsage = getDirSize(targetDir)
	}

	return &DownloadStatus{
		Running:    true,
		PID:        pid,
		Uptime:     stats.Uptime,
		ModelID:    meta.ModelID,
		PkgDir:     meta.PkgDir,
		Source:     meta.Source,
		Endpoint:   meta.Endpoint,
		StartTime:  meta.StartTime,
		DiskUsage:  diskUsage,
		RecentLogs: proc.TailLines(s.paths.DownloadLogFile, 12),
	}
}

// StartDownload starts a background or foreground download using the native Go downloader.
func (s *Service) StartDownload(opts DownloadOptions) (int, error) {
	st := s.Status()
	if st.Running {
		return st.PID, fmt.Errorf("已有模型下载任务正在运行中 (PID: %d)，请勿重复启动", st.PID)
	}

	proc.RemovePid(s.paths.DownloadPidFile)
	_ = os.MkdirAll(s.paths.ModelsDir, 0755)

	if opts.Source == "" {
		opts.Source = "huggingface"
	}
	if opts.Endpoint == "" {
		if strings.ToLower(opts.Source) == "modelscope" {
			opts.Endpoint = "https://modelscope.cn"
		} else {
			opts.Endpoint = "https://hf-mirror.com"
		}
	}
	if opts.PkgDir == "" {
		parts := strings.Split(opts.ModelID, "/")
		opts.PkgDir = strings.ToLower(parts[len(parts)-1])
	}
	if opts.Mode == "" {
		opts.Mode = "bg"
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	// Write metadata file
	infoContent := fmt.Sprintf("MODEL_ID=%s\nPKG_DIR=%s\nSOURCE=%s\nENDPOINT=%s\nSTART_TIME=%s\nMODE=%s\n",
		opts.ModelID, opts.PkgDir, opts.Source, opts.Endpoint, now, opts.Mode)
	_ = os.WriteFile(s.paths.DownloadInfoFile, []byte(infoContent), 0644)

	// Append banner to download log
	banner := fmt.Sprintf("\n========================================================\n下载任务启动时间: %s\n平台平台源: %s\n模型 ID: %s\n目标目录: models/%s\n下载源地址: %s\n下载模式: %s\n========================================================\n",
		now, opts.Source, opts.ModelID, opts.PkgDir, opts.Endpoint, opts.Mode)
	if f, err := os.OpenFile(s.paths.DownloadLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		_, _ = f.WriteString(banner)
		_ = f.Close()
	}

	if opts.Mode == "fg" {
		return 0, s.ExecuteDownloadTask(opts)
	}

	// For background mode, daemonize the current installer executable with the hidden _worker-download command
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	command := []string{
		exe,
		"_worker-download",
		opts.ModelID,
		opts.PkgDir,
		opts.Source,
		opts.Endpoint,
	}

	pid, err := proc.Daemonize(command, nil, s.paths.AgentDir, s.paths.DownloadLogFile)
	if err != nil {
		return 0, fmt.Errorf("启动下载守护进程失败: %w", err)
	}

	if err := proc.WritePid(s.paths.DownloadPidFile, pid); err != nil {
		return pid, fmt.Errorf("记录下载 PID 失败: %w", err)
	}

	time.Sleep(800 * time.Millisecond)
	if !proc.IsRunning(pid) {
		proc.RemovePid(s.paths.DownloadPidFile)
		recent := proc.TailLines(s.paths.DownloadLogFile, 12)
		return 0, fmt.Errorf("下载任务启动后立即异常退出，最新日志:\n%s", strings.Join(recent, "\n"))
	}

	return pid, nil
}

// ExecuteDownloadTask synchronously performs the download, writing progress to stdout.
func (s *Service) ExecuteDownloadTask(opts DownloadOptions) error {
	destDir := filepath.Join(s.paths.ModelsDir, opts.PkgDir)
	if opts.Source == "" {
		opts.Source = "huggingface"
	}

	fmt.Printf("开始下载模型: %s\n平台源: %s\n目标目录: %s\n源地址: %s\n\n", opts.ModelID, opts.Source, destDir, opts.Endpoint)

	lastLoggedFile := ""
	lastPercent := -1

	ctx := context.Background()
	err := RunModelDownload(ctx, opts.Source, opts.ModelID, opts.Endpoint, destDir, func(completedFiles, totalFiles int, currentFile string, currentBytes, totalBytes int64, speedBytesPerSec float64) {
		if currentFile != lastLoggedFile {
			lastLoggedFile = currentFile
			lastPercent = -1
			fmt.Printf("\n[%d/%d] 正在下载: %s (%s)\n", completedFiles+1, totalFiles, currentFile, FormatBytes(totalBytes))
		}

		pct := 0
		if totalBytes > 0 {
			pct = int(float64(currentBytes) / float64(totalBytes) * 100)
		}

		// Throttle log writes to every 10% progress or file completion to avoid log flooding
		if pct/10 != lastPercent/10 || currentBytes == totalBytes {
			lastPercent = pct
			speedStr := ""
			if speedBytesPerSec > 0 {
				speedStr = fmt.Sprintf(" | 速度: %s/s", FormatBytes(int64(speedBytesPerSec)))
			}
			fmt.Printf("  -> 进度: %d%% (%s / %s)%s\n", pct, FormatBytes(currentBytes), FormatBytes(totalBytes), speedStr)
		}
	})

	if err != nil {
		fmt.Printf("\n✗ 下载失败: %v\n", err)
		return err
	}

	fmt.Println("\n✓ 模型所有文件已下载并校验完成！")
	return nil
}

// StopDownload terminates the active background download task.
func (s *Service) StopDownload() error {
	st := s.Status()
	if !st.Running {
		proc.RemovePid(s.paths.DownloadPidFile)
		return nil
	}

	err := proc.GracefulStopDownload(st.PID, 5*time.Second)
	proc.RemovePid(s.paths.DownloadPidFile)
	return err
}
