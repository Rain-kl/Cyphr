package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/replicate/pget/pkg/download"
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

// RemoteFile represents a file to be downloaded in a model repository.
type RemoteFile struct {
	Path   string
	Size   int64
	Sha256 string
	URL    string
}

// DownloadProgressCallback notifies about overall download progress and current file progress.
type DownloadProgressCallback func(completedFiles, totalFiles int, currentFile string, currentBytes, totalBytes int64, speedBytesPerSec float64)

// FetchRepoFiles queries HuggingFace or ModelScope for file list.
func FetchRepoFiles(ctx context.Context, source, modelID, endpoint string) ([]RemoteFile, error) {
	switch strings.ToLower(source) {
	case "modelscope":
		return fetchModelScopeFiles(ctx, modelID)
	default:
		return fetchHuggingFaceFiles(ctx, modelID, endpoint)
	}
}

func fetchHuggingFaceFiles(ctx context.Context, modelID, endpoint string) ([]RemoteFile, error) {
	if endpoint == "" {
		endpoint = "https://hf-mirror.com"
	}
	apiURL := fmt.Sprintf("%s/api/models/%s?blobs=true", strings.TrimRight(endpoint, "/"), modelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cyphr-Downloader/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Hugging Face API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hugging Face API 返回状态码 %d: %s (模型 ID: %s)", resp.StatusCode, resp.Status, modelID)
	}

	var payload struct {
		Siblings []struct {
			Rfilename string `json:"rfilename"`
			Size      *int64 `json:"size"`
			LFS       *struct {
				Size   int64  `json:"size"`
				Sha256 string `json:"sha256"`
			} `json:"lfs"`
		} `json:"siblings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 Hugging Face 元数据失败: %w", err)
	}

	var files []RemoteFile
	for _, s := range payload.Siblings {
		if s.Rfilename == "" || strings.HasPrefix(s.Rfilename, ".") && s.Rfilename != ".gitattributes" {
			continue
		}
		var size int64
		var sha256 string
		if s.LFS != nil {
			size = s.LFS.Size
			sha256 = s.LFS.Sha256
		} else if s.Size != nil {
			size = *s.Size
		}

		downloadURL := fmt.Sprintf("%s/%s/resolve/main/%s", strings.TrimRight(endpoint, "/"), modelID, s.Rfilename)
		files = append(files, RemoteFile{
			Path:   s.Rfilename,
			Size:   size,
			Sha256: sha256,
			URL:    downloadURL,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("模型仓库 %s 中未找到有效文件", modelID)
	}
	return files, nil
}

func fetchModelScopeFiles(ctx context.Context, modelID string) ([]RemoteFile, error) {
	apiURL := fmt.Sprintf("https://modelscope.cn/api/v1/models/%s/repo/files?Revision=master&Recursive=true", modelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cyphr-Downloader/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 ModelScope API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ModelScope API 返回状态码 %d: %s (模型 ID: %s)", resp.StatusCode, resp.Status, modelID)
	}

	var payload struct {
		Code int `json:"Code"`
		Data struct {
			Files []struct {
				Name   string `json:"Name"`
				Path   string `json:"Path"`
				Size   int64  `json:"Size"`
				Sha256 string `json:"Sha256"`
				Type   string `json:"Type"`
			} `json:"Files"`
		} `json:"Data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 ModelScope 元数据失败: %w", err)
	}

	var files []RemoteFile
	for _, f := range payload.Data.Files {
		if f.Type == "tree" || f.Path == "" {
			continue
		}
		downloadURL := fmt.Sprintf("https://modelscope.cn/api/v1/models/%s/repo?Revision=master&FilePath=%s", modelID, url.QueryEscape(f.Path))
		files = append(files, RemoteFile{
			Path:   f.Path,
			Size:   f.Size,
			Sha256: f.Sha256,
			URL:    downloadURL,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("ModelScope 仓库 %s 中未找到有效文件", modelID)
	}
	return files, nil
}

// DownloadFile downloads a single file using replicate/pget for large files (> 5MB) or direct HTTP stream for small files.
func DownloadFile(ctx context.Context, f RemoteFile, destDir string, cb func(bytesWritten int64, speed float64)) error {
	targetPath := filepath.Join(destDir, f.Path)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	// Verify existing file
	if fi, err := os.Stat(targetPath); err == nil {
		if f.Size > 0 && fi.Size() == f.Size {
			if cb != nil {
				cb(f.Size, 0)
			}
			return nil
		}
	}

	tmpPath := targetPath + ".part"
	_ = os.Remove(tmpPath)

	// Small files (< 5MB) or files with unknown size: download via standard HTTP stream
	if f.Size > 0 && f.Size < 5*1024*1024 {
		return downloadSmallFile(ctx, f, targetPath, tmpPath, cb)
	}

	// Large files: use replicate/pget parallel chunk engine
	pgetOpts := download.Options{
		MaxConcurrency: 8,
		ChunkSize:      10 * 1024 * 1024,
	}
	bufferMode := download.GetBufferMode(pgetOpts)

	reader, _, err := bufferMode.Fetch(ctx, f.URL)
	if err != nil {
		// Fallback to direct HTTP stream if pget Fetch errors
		return downloadSmallFile(ctx, f, targetPath, tmpPath, cb)
	}
	defer func() {
		if rc, ok := reader.(io.Closer); ok {
			rc.Close()
		}
	}()

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	var written int64
	buf := make([]byte, 256*1024)
	startTime := time.Now()
	lastReport := startTime

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, rErr := reader.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				return wErr
			}
			written += int64(n)

			now := time.Now()
			if now.Sub(lastReport) >= 200*time.Millisecond {
				elapsed := now.Sub(startTime).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(written) / elapsed
				}
				if cb != nil {
					cb(written, speed)
				}
				lastReport = now
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return rErr
		}
	}

	_ = out.Close()

	if f.Size > 0 && written != f.Size {
		return fmt.Errorf("文件大小不匹配: 预期 %d 字节, 实际下载 %d 字节", f.Size, written)
	}

	return os.Rename(tmpPath, targetPath)
}

func downloadSmallFile(ctx context.Context, f RemoteFile, targetPath, tmpPath string, cb func(bytesWritten int64, speed float64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", f.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Cyphr-Downloader/1.0")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	var written int64
	buf := make([]byte, 128*1024)
	startTime := time.Now()
	lastReport := startTime

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := out.Write(buf[:n]); wErr != nil {
				return wErr
			}
			written += int64(n)

			now := time.Now()
			if now.Sub(lastReport) >= 200*time.Millisecond {
				elapsed := now.Sub(startTime).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(written) / elapsed
				}
				if cb != nil {
					cb(written, speed)
				}
				lastReport = now
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return rErr
		}
	}

	_ = out.Close()

	if f.Size > 0 && written != f.Size {
		return fmt.Errorf("文件大小不匹配: 预期 %d 字节, 实际下载 %d 字节", f.Size, written)
	}

	return os.Rename(tmpPath, targetPath)
}

// RunModelDownload downloads all files of a model repository, reporting progress through a callback.
func RunModelDownload(ctx context.Context, source, modelID, endpoint, destDir string, cb DownloadProgressCallback) error {
	files, err := FetchRepoFiles(ctx, source, modelID, endpoint)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建模型目标目录失败: %w", err)
	}

	totalFiles := len(files)
	for i, f := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := DownloadFile(ctx, f, destDir, func(written int64, speed float64) {
			if cb != nil {
				cb(i, totalFiles, f.Path, written, f.Size, speed)
			}
		})
		if err != nil {
			return fmt.Errorf("下载文件 %s 失败: %w", f.Path, err)
		}

		if cb != nil {
			cb(i+1, totalFiles, f.Path, f.Size, f.Size, 0)
		}
	}

	return nil
}

// FormatBytes formats byte counts into human readable strings.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
