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
		Version:    "latest",
		RepoOwner:  "Rain-kl",
		RepoName:   "Cyphr",
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

	cb("prepare", 0.05, fmt.Sprintf("准备安装目标目录: %s", targetDir))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	var zipReader *zip.Reader
	var zipCloser io.Closer

	// Determine download source
	if opts.ZipURL != "" && !strings.HasPrefix(opts.ZipURL, "http://") && !strings.HasPrefix(opts.ZipURL, "https://") {
		// Local ZIP file path
		cb("read", 0.1, fmt.Sprintf("读取本地 Agent 压缩包: %s", opts.ZipURL))
		r, err := zip.OpenReader(opts.ZipURL)
		if err != nil {
			return fmt.Errorf("读取本地 ZIP 文件失败: %w", err)
		}
		zipReader = &r.Reader
		zipCloser = r
	} else {
		// Download from GitHub or specified URL
		downloadURL := opts.ZipURL
		if downloadURL == "" {
			cb("resolve", 0.1, "正在查询 GitHub 最新 Agent 发布包...")
			resolvedURL, err := resolveReleaseAssetURL(opts.RepoOwner, opts.RepoName, opts.Version)
			if err != nil {
				return fmt.Errorf("解析 GitHub Release 资源失败: %w", err)
			}
			downloadURL = resolvedURL
		}

		if opts.UseMirror && !strings.Contains(downloadURL, "ghproxy") && !strings.Contains(downloadURL, "mirror") {
			// Prepend fast accelerator mirror for Chinese users
			downloadURL = "https://ghproxy.net/" + downloadURL
		}

		cb("download", 0.2, fmt.Sprintf("正在下载 Agent 离线包: %s", downloadURL))
		tempZip, err := downloadFile(downloadURL, func(p float64) {
			cb("download", 0.2+(p*0.4), fmt.Sprintf("正在下载 Agent 资源包... %.1f%%", p*100))
		})
		if err != nil {
			return fmt.Errorf("下载 Agent 包失败: %w", err)
		}
		defer os.Remove(tempZip)

		r, err := zip.OpenReader(tempZip)
		if err != nil {
			return fmt.Errorf("解压读取下载文件失败: %w", err)
		}
		zipReader = &r.Reader
		zipCloser = r
	}
	defer zipCloser.Close()

	// Extract files
	cb("extract", 0.65, "正在解压安装 Agent 代码文件...")
	if err := extractZip(zipReader, targetDir); err != nil {
		return fmt.Errorf("解压 Agent 代码失败: %w", err)
	}

	// Ensure config.yaml is generated from config.example.yaml if missing
	if opts.AutoConfig {
		cfgPath := filepath.Join(targetDir, "config.yaml")
		cfgExample := filepath.Join(targetDir, "config.example.yaml")
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			if _, err := os.Stat(cfgExample); err == nil {
				cb("config", 0.75, "生成初始默认配置 config.yaml...")
				_ = copyFile(cfgExample, cfgPath)
			}
		}
	}

	// Python Virtual Environment initialization (uv sync or venv)
	if !opts.SkipVenv {
		cb("venv", 0.8, "正在初始化 Python 虚拟环境与依赖...")
		if err := setupPythonEnv(targetDir, cb); err != nil {
			cb("warning", 0.95, fmt.Sprintf("Python 环境自动初始化提示: %v", err))
		}
	}

	// Update service paths to reflect installed target
	s.paths = config.NewAppPathsFromDir(targetDir)

	cb("finish", 1.0, "✓ Agent 安装完成！可直接在主菜单启动服务。")
	return nil
}

func resolveReleaseAssetURL(owner, repo, version string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	if version != "" && version != "latest" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, version)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Cyphr-Installer")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Fallback to standard release asset URL directly
		if version == "" || version == "latest" {
			return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/cyphr-agent.zip", owner, repo), nil
		}
		return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/cyphr-agent.zip", owner, repo, version), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
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
	}

	// Fallback pattern
	if version != "" && version != "latest" {
		return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/cyphr-agent_%s.zip", owner, repo, version, version), nil
	}
	return fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/cyphr-agent.zip", owner, repo), nil
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

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("下载失败 (HTTP %d): %s", resp.StatusCode, resp.Status)
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
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
		if err != nil {
			if err == io.EOF {
				break
			}
			_ = os.Remove(tmpName)
			return "", err
		}
	}

	// Close file explicitly before returning so other processes / zip.OpenReader can open it on Windows
	closed = true
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	return tmpName, nil
}

func extractZip(zr *zip.Reader, destDir string) error {
	for _, f := range zr.File {
		// Strip leading "agent/" prefix if present in zip
		relPath := f.Name
		relPath = strings.TrimPrefix(relPath, "agent/")
		cleanRel := filepath.Clean(relPath)
		if cleanRel == "." || cleanRel == "/" || strings.HasPrefix(cleanRel, "..") {
			continue
		}

		target := filepath.Join(destDir, relPath)
		// Security check against ZipSlip vulnerability
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		_ = rc.Close()
		_ = outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func setupPythonEnv(agentDir string, cb InstallProgressCallback) error {
	// 1. Try uv if installed
	if _, err := exec.LookPath("uv"); err == nil {
		cb("venv", 0.85, "检测到 uv 包管理器，正在同步 Python 3.12 虚拟环境与依赖...")
		// Use --python 3.12 explicitly to prevent pulling incompatible Python versions (e.g. 3.14)
		cmd := exec.CommandContext(context.Background(), "uv", "sync", "--python", "3.12")
		cmd.Dir = agentDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("uv sync 失败: %w, 日志:\n%s", err, string(out))
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
		cb("venv", 0.85, "创建 Python 虚拟环境 (.venv)...")
		args := []string{"-m", "venv", ".venv"}
		if pyBin == "py" {
			args = []string{"-3.12", "-m", "venv", ".venv"}
		}
		cmd := exec.CommandContext(context.Background(), pyBin, args...)
		cmd.Dir = agentDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("创建虚拟环境失败: %w, 日志:\n%s", err, string(out))
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
