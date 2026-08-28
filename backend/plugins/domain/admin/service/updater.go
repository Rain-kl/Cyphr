// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/pkg/buildinfo"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	githubAPIBaseURL = "https://api.github.com"
	maxArchiveSize   = int64(1024 * 1024 * 1024)
	maxReleaseSize   = int64(4 * 1024 * 1024)
	repositoryParts  = 2
	windowsOS        = "windows"
	archiveFileMode  = 0o600
	stagedBinaryMode = 0o700
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	State              string `json:"state"`
}

type githubRelease struct {
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	Body       string         `json:"body"`
	HTMLURL    string         `json:"html_url"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Published  time.Time      `json:"published_at"`
	Assets     []releaseAsset `json:"assets"`
}

type releaseClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// UpdaterManager manages application binary updates from GitHub releases.
type UpdaterManager struct {
	client    releaseClient
	mu        sync.Mutex
	upgrading bool
}

// DefaultUpdaterManager is the default singleton update manager.
var DefaultUpdaterManager = &UpdaterManager{
	client: &http.Client{Timeout: 10 * time.Minute},
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) {
		return ""
	}
	return version
}

func parseRepository(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New(errs.ErrInvalidRepository)
	}

	if !strings.Contains(raw, "://") {
		repo := strings.TrimSuffix(strings.Trim(raw, "/"), ".git")
		if len(strings.Split(repo, "/")) == repositoryParts {
			return repo, nil
		}
		return "", errors.New(errs.ErrInvalidRepository)
	}

	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", errors.New(errs.ErrInvalidRepository)
	}
	repo := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	if len(strings.Split(repo, "/")) != repositoryParts {
		return "", errors.New(errs.ErrInvalidRepository)
	}
	return repo, nil
}

func expectedAssetName(tag string) string {
	extension := "tar.gz"
	if runtime.GOOS == windowsOS {
		extension = "zip"
	}
	return fmt.Sprintf("wavelet_%s_%s_%s.%s", tag, runtime.GOOS, runtime.GOARCH, extension)
}

func expectedAssetNames(repo, tag string) []string {
	names := []string{expectedAssetName(tag)}
	if parts := strings.Split(repo, "/"); len(parts) == repositoryParts {
		repoName := parts[1]
		if repoName != "wavelet" {
			extension := "tar.gz"
			if runtime.GOOS == windowsOS {
				extension = "zip"
			}
			names = append(names, fmt.Sprintf("%s_%s_%s_%s.%s", repoName, tag, runtime.GOOS, runtime.GOARCH, extension))
		}
	}
	return names
}

func selectLatestRelease(repo string, releases []githubRelease) (githubRelease, releaseAsset, error) {
	var selected githubRelease
	var selectedAsset releaseAsset
	selectedVersion := ""

	for _, release := range releases {
		version := normalizeVersion(release.TagName)
		if release.Draft || version == "" {
			continue
		}
		expectedNames := expectedAssetNames(repo, release.TagName)
		for _, asset := range release.Assets {
			matched := false
			for _, name := range expectedNames {
				if asset.Name == name {
					matched = true
					break
				}
			}
			if !matched || asset.BrowserDownloadURL == "" || asset.State != "uploaded" {
				continue
			}
			if selectedVersion == "" || semver.Compare(version, selectedVersion) > 0 {
				selected = release
				selectedAsset = asset
				selectedVersion = version
			}
		}
	}

	if selectedVersion == "" {
		return githubRelease{}, releaseAsset{}, errors.New(errs.ErrNoCompatibleRelease)
	}
	return selected, selectedAsset, nil
}

func (m *UpdaterManager) fetchRelease(ctx context.Context, repo string) (githubRelease, releaseAsset, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/repos/%s/releases?per_page=30", githubAPIBaseURL, repo),
		nil,
	)
	if err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errs.ErrReleaseRequestFailed, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Wavelet-Updater")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := m.client.Do(req)
	if err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errs.ErrReleaseRequestFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: HTTP %d", errs.ErrReleaseRequestFailed, resp.StatusCode)
	}

	var releases []githubRelease
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseSize))
	if err := decoder.Decode(&releases); err != nil {
		return githubRelease{}, releaseAsset{}, fmt.Errorf("%s: %w", errs.ErrReleaseResponseInvalid, err)
	}

	release, asset, err := selectLatestRelease(repo, releases)
	if err != nil {
		return githubRelease{}, releaseAsset{}, err
	}
	logger.InfoF(ctx, "[Updater] Selected latest compatible release: %s (Asset: %s)", release.TagName, asset.Name)
	return release, asset, nil
}

func loadRepository(ctx context.Context) (string, error) {
	cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyUpdateUpstreamRepository)
	if err != nil {
		return "", fmt.Errorf("%s: %w", errs.ErrInvalidRepository, err)
	}
	return parseRepository(cfg.Value)
}

// status returns current version and update status.
func (m *UpdaterManager) status(ctx context.Context) (model.UpdaterStatus, releaseAsset, error) {
	upstreamRepo, err := loadRepository(ctx)
	if err != nil {
		return model.UpdaterStatus{}, releaseAsset{}, err
	}
	release, asset, err := m.fetchRelease(ctx, upstreamRepo)
	if err != nil {
		return model.UpdaterStatus{}, releaseAsset{}, err
	}

	currentVersion := normalizeVersion(buildinfo.Version)
	latestVersion := normalizeVersion(release.TagName)
	updateAvailable := currentVersion != "" && semver.Compare(latestVersion, currentVersion) > 0

	logger.InfoF(ctx, "[Updater] Check update complete. current: %s, latest: %s, update_available: %t", buildinfo.Version, release.TagName, updateAvailable)

	return model.UpdaterStatus{
		CurrentVersion:     buildinfo.Version,
		BuildTime:          buildinfo.BuildTime,
		LatestVersion:      release.TagName,
		UpdateAvailable:    updateAvailable,
		CanUpgrade:         updateAvailable && runtime.GOOS != windowsOS,
		Prerelease:         release.Prerelease,
		ReleaseName:        release.Name,
		ReleaseNotes:       release.Body,
		ReleaseURL:         release.HTMLURL,
		PublishedAt:        release.Published.Format(time.RFC3339),
		UpstreamRepository: upstreamRepo,
		AssetName:          asset.Name,
		Platform:           runtime.GOOS + "/" + runtime.GOARCH,
	}, asset, nil
}

// GetUpdateStatus returns current updater status.
func GetUpdateStatus(ctx context.Context) (model.UpdaterStatus, error) {
	status, _, err := DefaultUpdaterManager.status(ctx)
	return status, err
}

func downloadArchive(ctx context.Context, client releaseClient, asset releaseAsset, destination string) error {
	if asset.Size <= 0 || asset.Size > maxArchiveSize {
		return fmt.Errorf(errs.ErrReleaseAssetSizeInvalid, asset.Size)
	}
	logger.InfoF(ctx, "[Updater] Downloading release asset: %s", asset.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf(errs.ErrCreateUpgradeRequestFailed, err)
	}
	req.Header.Set("User-Agent", "Wavelet-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(errs.ErrDownloadUpgradeAssetFailed, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(errs.ErrUpgradeAssetHTTPFailed, resp.StatusCode)
	}

	//nolint:gosec // updater download destination is validated
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, archiveFileMode)
	if err != nil {
		return fmt.Errorf(errs.ErrCreateUpgradeArchiveFailed, err)
	}

	written, err := io.Copy(file, io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		_ = file.Close()
		return fmt.Errorf(errs.ErrWriteUpgradeArchiveFailed, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf(errs.ErrCloseUpgradeArchiveFailed, err)
	}
	if written > maxArchiveSize || written != asset.Size {
		return fmt.Errorf(errs.ErrUpgradeArchiveSizeMismatch, written, asset.Size)
	}
	logger.InfoF(ctx, "[Updater] Successfully downloaded release asset to %s", destination)
	return nil
}

func safeArchivePath(destination, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) || cleanName == "." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(errs.ErrArchiveContainsIllegalPath, name)
	}
	target := filepath.Join(destination, cleanName)
	relative, err := filepath.Rel(destination, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(errs.ErrArchivePathOutOfDestination, name)
	}
	return target, nil
}

func matchBinaryName(name string, candidates []string) bool {
	for _, candidate := range candidates {
		if runtime.GOOS == windowsOS {
			if strings.EqualFold(name, candidate) {
				return true
			}
		} else {
			if name == candidate {
				return true
			}
		}
	}
	return false
}

func getCandidateBinaryNames(executable, repo string) []string {
	execName := filepath.Base(executable)
	names := []string{execName}

	addName := func(base string) {
		name := base
		if runtime.GOOS == windowsOS && !strings.HasSuffix(strings.ToLower(name), ".exe") {
			name += ".exe"
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}

	if parts := strings.Split(repo, "/"); len(parts) == repositoryParts {
		addName(parts[1])
	}
	addName("wavelet")

	return names
}

func isLikelyBinary(name string, isDir bool, mode os.FileMode) bool {
	if isDir {
		return false
	}
	base := strings.ToLower(filepath.Base(name))

	exclusions := []string{
		"license", "licence", "copying", "notice", "readme", "changelog",
	}
	for _, excl := range exclusions {
		if strings.HasPrefix(base, excl) {
			return false
		}
	}

	if runtime.GOOS == windowsOS {
		return filepath.Ext(base) == ".exe"
	}

	return (mode.Perm()&0o111 != 0) || (filepath.Ext(base) == "")
}

func findBinaryInTarGz(archivePath string, candidates []string) (string, error) {
	//nolint:gosec // updater archivePath is verified
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	reader := tar.NewReader(gzipReader)
	var binaries []string
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag == tar.TypeReg && isLikelyBinary(header.Name, false, header.FileInfo().Mode()) {
			binaries = append(binaries, header.Name)
		}
	}

	if len(binaries) == 1 {
		return binaries[0], nil
	}

	for _, name := range binaries {
		if matchBinaryName(filepath.Base(name), candidates) {
			return name, nil
		}
	}

	return "", errors.New(errs.ErrNoCompatibleAsset)
}

func findBinaryInZip(archivePath string, candidates []string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()

	var binaries []string
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && isLikelyBinary(file.Name, false, file.FileInfo().Mode()) {
			binaries = append(binaries, file.Name)
		}
	}

	if len(binaries) == 1 {
		return binaries[0], nil
	}

	for _, name := range binaries {
		if matchBinaryName(filepath.Base(name), candidates) {
			return name, nil
		}
	}

	return "", errors.New(errs.ErrNoCompatibleAsset)
}

func extractTarGz(ctx context.Context, archivePath, destination, targetName string, candidates []string) (string, error) {
	binaryPathInArchive, err := findBinaryInTarGz(archivePath, candidates)
	if err != nil {
		return "", err
	}

	logger.InfoF(ctx, "[Updater] Extracting tar.gz archive: %s (extracting: %s)", archivePath, binaryPathInArchive)
	//nolint:gosec // updater archivePath is verified
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Name != binaryPathInArchive {
			continue
		}
		target, err := safeArchivePath(destination, targetName)
		if err != nil {
			return "", err
		}
		//nolint:gosec // updater destination is sanitized
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, stagedBinaryMode)
		if err != nil {
			return "", err
		}
		written, copyErr := io.Copy(output, io.LimitReader(reader, maxArchiveSize+1))
		closeErr := output.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if written > maxArchiveSize {
			return "", errors.New(errs.ErrExtractedBinaryTooLarge)
		}
		logger.InfoF(ctx, "[Updater] Successfully extracted binary to %s", target)
		return target, nil
	}
	return "", errors.New(errs.ErrNoCompatibleAsset)
}

func extractZip(ctx context.Context, archivePath, destination, targetName string, candidates []string) (string, error) {
	binaryPathInArchive, err := findBinaryInZip(archivePath, candidates)
	if err != nil {
		return "", err
	}

	logger.InfoF(ctx, "[Updater] Extracting zip archive: %s (extracting: %s)", archivePath, binaryPathInArchive)
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = reader.Close()
	}()
	for _, file := range reader.File {
		if file.Name != binaryPathInArchive {
			continue
		}
		target, err := safeArchivePath(destination, targetName)
		if err != nil {
			return "", err
		}
		input, err := file.Open()
		if err != nil {
			return "", err
		}
		//nolint:gosec // updater extraction target is safe
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, stagedBinaryMode)
		if err != nil {
			return "", err
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxArchiveSize+1))
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if inputCloseErr != nil {
			return "", inputCloseErr
		}
		if outputCloseErr != nil {
			return "", outputCloseErr
		}
		if written > maxArchiveSize {
			return "", errors.New(errs.ErrExtractedBinaryTooLarge)
		}
		logger.InfoF(ctx, "[Updater] Successfully extracted binary to %s", target)
		return target, nil
	}
	return "", errors.New(errs.ErrNoCompatibleAsset)
}

// PrepareUpgrade validates preconditions and downloads the newest binary.
func (m *UpdaterManager) PrepareUpgrade(ctx context.Context) (string, string, error) {
	if runtime.GOOS == windowsOS {
		return "", "", errors.New(errs.ErrAutomaticUpgradeBlocked)
	}
	if normalizeVersion(buildinfo.Version) == "" {
		return "", "", errors.New(errs.ErrDevelopmentBuild)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.upgrading {
		return "", "", errors.New(errs.ErrUpgradeAlreadyRunning)
	}

	status, asset, err := m.status(ctx)
	if err != nil {
		return "", "", err
	}
	if !status.UpdateAvailable {
		return "", "", errors.New(errs.ErrAlreadyUpToDate)
	}

	logger.InfoF(ctx, "[Updater] Preparing upgrade. current: %s, latest: %s", status.CurrentVersion, status.LatestVersion)

	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf(errs.ErrLocateExecutableFailed, err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", "", fmt.Errorf(errs.ErrResolveExecutablePathFailed, err)
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(executable), ".wavelet-update-*")
	if err != nil {
		return "", "", fmt.Errorf(errs.ErrCreateUpgradeDirFailed, err)
	}

	archivePath := filepath.Join(tempDir, asset.Name)
	if err := downloadArchive(ctx, m.client, asset, archivePath); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", "", err
	}

	targetName := filepath.Base(executable)
	candidates := getCandidateBinaryNames(executable, status.UpstreamRepository)

	var stagedBinary string
	if strings.HasSuffix(asset.Name, ".zip") {
		stagedBinary, err = extractZip(ctx, archivePath, tempDir, targetName, candidates)
	} else {
		stagedBinary, err = extractTarGz(ctx, archivePath, tempDir, targetName, candidates)
	}
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", "", fmt.Errorf(errs.ErrExtractUpgradeAssetFailed, err)
	}
	logger.InfoF(ctx, "[Updater] Staged binary successfully prepared: %s", stagedBinary)
	m.upgrading = true
	return executable, stagedBinary, nil
}

// FinishUpgrade resets the upgrading flag.
func (m *UpdaterManager) FinishUpgrade() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upgrading = false
}
