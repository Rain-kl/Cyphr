// Package updater provides self-update functionality for the installer executable.
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

const (
	progressCheck        = 0.1
	progressDownload     = 0.2
	progressDownloadSpan = 0.5
	progressExtract      = 0.75
	progressReplace      = 0.9
	progressFinish       = 1.0

	downloadBufSize   = 32 * 1024
	percentMultiplier = 100
	httpTimeout       = 15 * time.Second
	downloadTimeout   = 5 * time.Minute
	executablePerm    = 0755
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

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

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

	cb("check", progressCheck, "正在查询 GitHub 最新 Installer 发布版本...")
	rel, err := FetchLatestRelease(owner, repo)
	if err != nil {
		return fmt.Errorf("获取最新发布版本信息失败: %w", err)
	}

	downloadURL, archiveName, err := findPlatformAsset(rel)
	if err != nil {
		return err
	}

	if useMirror && !strings.Contains(downloadURL, "ghproxy") && !strings.Contains(downloadURL, "mirror") {
		downloadURL = "https://ghproxy.net/" + downloadURL
	}

	cb("download", progressDownload, fmt.Sprintf("正在下载新版 Installer (%s)...", archiveName))
	tmpArchive, err := downloadToTemp(downloadURL, func(p float64) {
		cb("download", progressDownload+(p*progressDownloadSpan), fmt.Sprintf("正在下载新版 Installer... %.1f%%", p*percentMultiplier))
	})
	if err != nil {
		return fmt.Errorf("下载发布包失败: %w", err)
	}
	defer func() { _ = os.Remove(tmpArchive) }()

	cb("extract", progressExtract, "正在解压可执行文件...")
	targetDir := filepath.Dir(currentExe)
	tmpNewExe, err := extractBinaryFromArchive(tmpArchive, archiveName, "cyphr-installer", targetDir)
	if err != nil {
		return fmt.Errorf("解压新版本失败: %w", err)
	}
	defer func() { _ = os.Remove(tmpNewExe) }()

	_ = os.Chmod(tmpNewExe, executablePerm) //nolint:gosec // Executable file permission

	cb("replace", progressReplace, "正在替换当前可执行文件...")
	if err := replaceExecutable(currentExe, tmpNewExe); err != nil {
		return fmt.Errorf("替换可执行程序失败: %w", err)
	}

	cb("finish", progressFinish, fmt.Sprintf("✓ Installer 已成功更新至最新版本 %s！", rel.TagName))
	return nil
}

func findPlatformAsset(rel *GitHubReleaseInfo) (string, string, error) {
	targetOs := runtime.GOOS
	targetArch := runtime.GOARCH

	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, "installer") && strings.Contains(name, targetOs) && strings.Contains(name, targetArch) {
			return a.BrowserDownloadURL, a.Name, nil
		}
	}
	return "", "", fmt.Errorf("未在最新版本 %s 中找到适配当前平台 (%s/%s) 的发布包", rel.TagName, targetOs, targetArch)
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

func extractBinaryFromArchive(archivePath, archiveName, binaryName, targetDir string) (string, error) {
	tmpExe, err := os.CreateTemp(targetDir, binaryName+"-*")
	if err != nil {
		// Fallback to default temp dir if targetDir is unwritable
		tmpExe, err = os.CreateTemp("", binaryName+"-*")
		if err != nil {
			return "", err
		}
	}
	tmpExeName := tmpExe.Name()

	var extractErr error
	if strings.HasSuffix(archiveName, ".zip") {
		extractErr = extractFromZip(archivePath, binaryName, tmpExe)
	} else {
		extractErr = extractFromTarGz(archivePath, binaryName, tmpExe)
	}

	_ = tmpExe.Close()
	if extractErr != nil {
		_ = os.Remove(tmpExeName)
		return "", extractErr
	}

	return tmpExeName, nil
}

func extractFromZip(archivePath, binaryName string, tmpExe *os.File) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != binaryName && base != binaryName+".exe" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() { _ = rc.Close() }()

		_, err = io.Copy(tmpExe, rc) //nolint:gosec // Binary extraction
		return err
	}
	return fmt.Errorf("在 zip 归档中未找到目标文件 %s", binaryName)
}

func extractFromTarGz(archivePath, binaryName string, tmpExe *os.File) error {
	f, err := os.Open(filepath.Clean(archivePath)) //nolint:gosec // Archive path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		base := filepath.Base(hdr.Name)
		if base == binaryName || base == binaryName+".exe" {
			_, copyErr := io.Copy(tmpExe, tr) //nolint:gosec // Binary extraction
			return copyErr
		}
	}
	return fmt.Errorf("在 tar.gz 归档中未找到目标文件 %s", binaryName)
}

func replaceExecutable(dst, src string) error {
	dstDir := filepath.Dir(dst)
	stageFile := src

	if filepath.Clean(filepath.Dir(src)) != filepath.Clean(dstDir) {
		staged, cleanup, err := stageFileToDir(src, dstDir, filepath.Base(dst))
		if err != nil {
			return err
		}
		defer cleanup()
		stageFile = staged
	}

	oldExe := dst + ".old"
	_ = os.Remove(oldExe)

	if err := os.Rename(dst, oldExe); err != nil {
		return fmt.Errorf("备份当前可执行文件失败: %w", err)
	}

	if err := os.Rename(stageFile, dst); err != nil {
		_ = os.Rename(oldExe, dst) // Rollback
		return fmt.Errorf("替换可执行程序失败: %w", err)
	}

	_ = os.Remove(oldExe)
	return nil
}

func stageFileToDir(src, dstDir, baseName string) (string, func(), error) {
	tmp, err := os.CreateTemp(dstDir, baseName+"-stage-*")
	if err != nil {
		return "", nil, fmt.Errorf("在目标目录创建中转文件失败: %w", err)
	}
	stagePath := tmp.Name()
	cleanup := func() { _ = os.Remove(stagePath) }

	in, err := os.Open(filepath.Clean(src)) //nolint:gosec // Local file copy
	if err != nil {
		_ = tmp.Close()
		cleanup()
		return "", nil, fmt.Errorf("读取新版本可执行文件失败: %w", err)
	}

	_, copyErr := io.Copy(tmp, in)
	_ = in.Close()
	_ = tmp.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("复制新版本至目标目录失败: %w", copyErr)
	}

	perm := os.FileMode(executablePerm)
	if fi, statErr := os.Stat(src); statErr == nil {
		perm = fi.Mode()
	}
	_ = os.Chmod(stagePath, perm)

	return stagePath, cleanup, nil
}
