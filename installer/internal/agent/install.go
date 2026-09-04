package agent

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cyphr/installer/internal/config"
)

const (
	defaultVersion   = "latest"
	defaultRepoOwner = "Rain-kl"
	defaultRepoName  = "Cyphr"

	progressPrepare       = 0.05
	progressResolve       = 0.1
	progressDownloadStart = 0.2
	progressDownloadSpan  = 0.4
	progressExtract       = 0.65
	progressConfig        = 0.75
	progressVenv          = 0.8
	progressVenvSync      = 0.85
	progressWarning       = 0.95
	progressFinish        = 1.0

	percentMultiplier = 100
	downloadBufSize   = 32 * 1024
	httpTimeout       = 15 * time.Second
	downloadTimeout   = 5 * time.Minute
	dirPerm           = 0750
)

// GitHubRelease represents a minimal GitHub Release payload.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// InstallProgressCallback reports download and setup events.
type InstallProgressCallback func(stage string, progress float64, message string)

// InstallOptions contains parameters for installing the agent.
type InstallOptions struct {
	TargetDir  string // Destination directory (defaults to paths.AgentDir)
	Version    string // Specific release tag e.g. "v1.0.0" or "latest"
	RepoOwner  string // Defaults to "Rain-kl" or "arctel-net"
	RepoName   string // Defaults to "Cyphr"
	ZipURL     string // Optional direct zip URL or local file path
	UseMirror  bool   // If true, route GitHub download through fast accelerator
	SkipVenv   bool   // If true, skip uv sync / venv creation
	AutoConfig bool   // If true, copy config.example.yaml to config.yaml if absent
}

// DefaultInstallOptions returns sensible defaults for installation.
func DefaultInstallOptions(paths *config.AppPaths) InstallOptions {
	return InstallOptions{
		TargetDir:  paths.AgentDir,
		Version:    defaultVersion,
		RepoOwner:  defaultRepoOwner,
		RepoName:   defaultRepoName,
		UseMirror:  true,
		AutoConfig: true,
	}
}

// InstallAgent downloads, extracts and sets up the Python Agent into target directory.
func (s *Service) InstallAgent(opts InstallOptions, cb InstallProgressCallback) error {
	if cb == nil {
		cb = func(string, float64, string) {}
	}

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = s.paths.AgentDir
	}

	cb("prepare", progressPrepare, fmt.Sprintf("准备安装目标目录: %s", targetDir))
	if err := os.MkdirAll(targetDir, dirPerm); err != nil { //nolint:gosec // Directory permissions
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	zipReader, cleanup, err := s.resolveZipReader(opts, cb)
	if err != nil {
		return err
	}
	defer cleanup()

	cb("extract", progressExtract, "正在解压安装 Agent 代码文件...")
	if err := extractZip(zipReader, targetDir); err != nil {
		return fmt.Errorf("解压 Agent 代码失败: %w", err)
	}

	if opts.AutoConfig {
		ensureDefaultConfig(targetDir, cb)
	}

	if !opts.SkipVenv {
		cb("venv", progressVenv, "正在初始化 Python 虚拟环境与依赖...")
		if err := setupPythonEnv(targetDir, cb); err != nil {
			cb("warning", progressWarning, fmt.Sprintf("Python 环境自动初始化提示: %v", err))
		}
	}

	s.paths = config.NewAppPathsFromDir(targetDir)
	cb("finish", progressFinish, "✓ Agent 安装完成！可直接在主菜单启动服务。")
	return nil
}

func (s *Service) resolveZipReader(opts InstallOptions, cb InstallProgressCallback) (*zip.Reader, func(), error) {
	if opts.ZipURL != "" && !strings.HasPrefix(opts.ZipURL, "http://") && !strings.HasPrefix(opts.ZipURL, "https://") {
		cb("read", progressResolve, fmt.Sprintf("读取本地 Agent 压缩包: %s", opts.ZipURL))
		r, err := zip.OpenReader(opts.ZipURL)
		if err != nil {
			return nil, nil, fmt.Errorf("读取本地 ZIP 文件失败: %w", err)
		}
		return &r.Reader, func() { _ = r.Close() }, nil
	}

	downloadURL := opts.ZipURL
	if downloadURL == "" {
		cb("resolve", progressResolve, "正在查询 GitHub 最新 Agent 发布包...")
		resolvedURL, err := resolveReleaseAssetURL(opts.RepoOwner, opts.RepoName, opts.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("解析 GitHub Release 资源失败: %w", err)
		}
		downloadURL = resolvedURL
	}

	if opts.UseMirror && !strings.Contains(downloadURL, "ghproxy") && !strings.Contains(downloadURL, "mirror") {
		downloadURL = "https://ghproxy.net/" + downloadURL
	}

	cb("download", progressDownloadStart, fmt.Sprintf("正在下载 Agent 离线包: %s", downloadURL))
	tempZip, err := downloadFile(downloadURL, func(p float64) {
		cb("download", progressDownloadStart+(p*progressDownloadSpan), fmt.Sprintf("正在下载 Agent 资源包... %.1f%%", p*percentMultiplier))
	})
	if err != nil {
		return nil, nil, fmt.Errorf("下载 Agent 包失败: %w", err)
	}

	r, err := zip.OpenReader(tempZip)
	if err != nil {
		_ = os.Remove(tempZip)
		return nil, nil, fmt.Errorf("解压读取下载文件失败: %w", err)
	}

	cleanup := func() {
		_ = r.Close()
		_ = os.Remove(tempZip)
	}
	return &r.Reader, cleanup, nil
}

func ensureDefaultConfig(targetDir string, cb InstallProgressCallback) {
	cfgPath := filepath.Join(targetDir, "config.yaml")
	cfgExample := filepath.Join(targetDir, "config.example.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if _, err := os.Stat(cfgExample); err == nil {
			cb("config", progressConfig, "生成初始默认配置 config.yaml...")
			_ = copyFile(cfgExample, cfgPath)
		}
	}
}

func resolveReleaseAssetURL(owner, repo, version string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	if version != "" && version != defaultVersion {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, version)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Cyphr-Installer")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fallbackAssetURL(owner, repo, version), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fallbackAssetURL(owner, repo, version), nil
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err == nil {
		for _, a := range release.Assets {
			if strings.Contains(a.Name, "cyphr-agent") && strings.HasSuffix(a.Name, ".zip") {
				return a.BrowserDownloadURL, nil
			}
		}
		if release.TagName != "" {
			version = release.TagName
		}
	}

	return fallbackAssetURL(owner, repo, version), nil
}

func fallbackAssetURL(owner, repo, version string) string {
	if version != "" && version != defaultVersion {
		return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/cyphr-agent_%s.zip", owner, repo, version, version)
	}
	return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/cyphr-agent.zip", owner, repo)
}

func downloadFile(url string, progressCb func(float64)) (string, error) {
	tmpFile, err := os.CreateTemp("", "cyphr-agent-*.zip")
	if err != nil {
		return "", err
	}
	tmpName := tmpFile.Name()
	var closed bool
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("下载失败 (HTTP %d): %s", resp.StatusCode, resp.Status)
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, downloadBufSize)

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmpFile.Write(buf[:n]); werr != nil {
				_ = os.Remove(tmpName)
				return "", werr
			}
			downloaded += int64(n)
			if total > 0 && progressCb != nil {
				progressCb(float64(downloaded) / float64(total))
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			_ = os.Remove(tmpName)
			return "", rerr
		}
	}

	closed = true
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	return tmpName, nil
}

func extractZip(zr *zip.Reader, destDir string) error {
	cleanDest := filepath.Clean(destDir)
	for _, f := range zr.File {
		relPath := strings.TrimPrefix(f.Name, "agent/")
		cleanRel := filepath.Clean(relPath)
		if cleanRel == "." || cleanRel == "/" || strings.HasPrefix(cleanRel, "..") {
			continue
		}

		target := filepath.Join(cleanDest, cleanRel)
		// Security check against ZipSlip vulnerability
		if !strings.HasPrefix(target, cleanDest+string(filepath.Separator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, dirPerm); err != nil { //nolint:gosec // Directory permissions
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil { //nolint:gosec // Directory permissions
			return err
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, target string) error {
	target = filepath.Clean(target)
	outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()) //nolint:gosec // Safe extraction target
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	_, err = io.Copy(outFile, rc) //nolint:gosec // Archive extraction
	return err
}

func setupPythonEnv(agentDir string, cb InstallProgressCallback) error {
	// 1. Try uv if installed
	if _, err := exec.LookPath("uv"); err == nil {
		cb("venv", progressVenvSync, "检测到 uv 包管理器，正在同步 Python 3.12 虚拟环境与依赖...")
		cmd := exec.CommandContext(context.Background(), "uv", "sync", "--python", "3.12")
		cmd.Dir = agentDir
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return fmt.Errorf("uv sync 失败: %w, 日志:\n%s", cmdErr, string(out))
		}
		return nil
	}

	// 2. Try python3 / python (prefer 3.12 if available)
	pyBin := ""
	for _, candidate := range []string{"python3.12", "py", "python3", "python"} {
		if _, err := exec.LookPath(candidate); err == nil {
			pyBin = candidate
			break
		}
	}
	if pyBin == "" {
		return fmt.Errorf("未在系统 PATH 中检测到 python3 或 uv，请先安装 Python 3.12")
	}

	venvDir := filepath.Join(agentDir, ".venv")
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		cb("venv", progressVenvSync, "创建 Python 虚拟环境 (.venv)...")
		args := []string{"-m", "venv", ".venv"}
		if pyBin == "py" {
			args = []string{"-3.12", "-m", "venv", ".venv"}
		}
		cmd := exec.CommandContext(context.Background(), pyBin, args...) //nolint:gosec // Agent venv creation
		cmd.Dir = agentDir
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return fmt.Errorf("创建虚拟环境失败: %w, 日志:\n%s", cmdErr, string(out))
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src)) //nolint:gosec // Local file copy
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(filepath.Clean(dst)) //nolint:gosec // Local file copy
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
