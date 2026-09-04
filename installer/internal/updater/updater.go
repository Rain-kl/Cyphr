package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ProgressCallback informs the caller about stages and progress percentage (0.0 to 1.0).
type ProgressCallback func(stage string, progress float64, message string)

// GitHubReleaseInfo holds release tag and asset metadata.
type GitHubReleaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// FetchLatestRelease queries the GitHub API for latest release metadata.
func FetchLatestRelease(owner, repo string) (*GitHubReleaseInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cyphr-Installer")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	var info GitHubReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// UpdateInstaller downloads the latest binary archive for the current OS/ARCH, extracts it, and atomically replaces the running executable.
func UpdateInstaller(owner, repo string, useMirror bool, cb ProgressCallback) error {
	if cb == nil {
		cb = func(string, float64, string) {}
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法确定当前可执行文件路径: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("解析可执行文件真实路径失败: %w", err)
	}

	cb("check", 0.1, "正在查询 GitHub 最新 Installer 发布版本...")
	rel, err := FetchLatestRelease(owner, repo)
	if err != nil {
		return fmt.Errorf("获取最新发布版本信息失败: %w", err)
	}

	// Target matching: cyphr-installer_<VERSION>_<GOOS>_<GOARCH>.(tar.gz|zip)
	targetOs := runtime.GOOS
	targetArch := runtime.GOARCH
	var downloadURL string
	var archiveName string

	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, "installer") && strings.Contains(name, targetOs) && strings.Contains(name, targetArch) {
			downloadURL = a.BrowserDownloadURL
			archiveName = a.Name
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("未在最新版本 %s 中找到适配当前平台 (%s/%s) 的发布包", rel.TagName, targetOs, targetArch)
	}

	if useMirror && !strings.Contains(downloadURL, "ghproxy") && !strings.Contains(downloadURL, "mirror") {
		downloadURL = "https://ghproxy.net/" + downloadURL
	}

	cb("download", 0.2, fmt.Sprintf("正在下载新版 Installer (%s)...", archiveName))
	tmpArchive, err := downloadToTemp(downloadURL, func(p float64) {
		cb("download", 0.2+(p*0.5), fmt.Sprintf("正在下载新版 Installer... %.1f%%", p*100))
	})
	if err != nil {
		return fmt.Errorf("下载发布包失败: %w", err)
	}
	defer os.Remove(tmpArchive)

	cb("extract", 0.75, "正在解压可执行文件...")
	tmpNewExe, err := extractBinaryFromArchive(tmpArchive, archiveName, "cyphr-installer")
	if err != nil {
		return fmt.Errorf("解压新版本失败: %w", err)
	}
	defer os.Remove(tmpNewExe)

	// Set executable permissions
	_ = os.Chmod(tmpNewExe, 0755)

	cb("replace", 0.9, "正在替换当前可执行文件...")
	if err := replaceExecutable(currentExe, tmpNewExe); err != nil {
		return fmt.Errorf("替换可执行程序失败: %w", err)
	}

	cb("finish", 1.0, fmt.Sprintf("✓ Installer 已成功更新至最新版本 %s！", rel.TagName))
	return nil
}

func downloadToTemp(url string, progressCb func(float64)) (string, error) {
	tmpFile, err := os.CreateTemp("", "cyphr-update-*")
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
	defer resp.Body.Close() //nolint:errcheck

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

	closed = true
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}

	return tmpName, nil
}

func extractBinaryFromArchive(archivePath, archiveName, binaryName string) (string, error) {
	tmpExe, err := os.CreateTemp("", binaryName+"-*")
	if err != nil {
		return "", err
	}
	tmpExeName := tmpExe.Name()
	var closed bool
	defer func() {
		if !closed {
			_ = tmpExe.Close()
		}
	}()

	if strings.HasSuffix(archiveName, ".zip") {
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			_ = os.Remove(tmpExeName)
			return "", err
		}
		defer zr.Close()

		for _, f := range zr.File {
			base := filepath.Base(f.Name)
			if base == binaryName || base == binaryName+".exe" {
				rc, err := f.Open()
				if err != nil {
					_ = os.Remove(tmpExeName)
					return "", err
				}
				defer rc.Close()
				if _, err = io.Copy(tmpExe, rc); err != nil {
					_ = os.Remove(tmpExeName)
					return "", err
				}
				closed = true
				if err := tmpExe.Close(); err != nil {
					_ = os.Remove(tmpExeName)
					return "", err
				}
				return tmpExeName, nil
			}
		}
		_ = os.Remove(tmpExeName)
		return "", fmt.Errorf("在 zip 归档中未找到目标文件 %s", binaryName)
	}

	// Assume .tar.gz
	f, err := os.Open(archivePath)
	if err != nil {
		_ = os.Remove(tmpExeName)
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		_ = os.Remove(tmpExeName)
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = os.Remove(tmpExeName)
			return "", err
		}
		base := filepath.Base(hdr.Name)
		if base == binaryName || base == binaryName+".exe" {
			if _, err = io.Copy(tmpExe, tr); err != nil {
				_ = os.Remove(tmpExeName)
				return "", err
			}
			closed = true
			if err := tmpExe.Close(); err != nil {
				_ = os.Remove(tmpExeName)
				return "", err
			}
			return tmpExeName, nil
		}
	}

	_ = os.Remove(tmpExeName)
	return "", fmt.Errorf("在 tar.gz 归档中未找到目标文件 %s", binaryName)
}

func replaceExecutable(dst, src string) error {
	oldExe := dst + ".old"
	_ = os.Remove(oldExe)

	if err := os.Rename(dst, oldExe); err != nil {
		return err
	}

	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(oldExe, dst)
		return err
	}

	_ = os.Remove(oldExe)
	return nil
}
