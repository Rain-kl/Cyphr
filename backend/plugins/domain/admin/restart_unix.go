//go:build !windows

// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"Wavelet/pkg/logger"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const installedBinaryMode = 0o755

func replaceAndRestart(executable, stagedBinary string) error {
	ctx := context.Background()
	logger.InfoF(ctx, "[Updater] Swapping executable: %s -> %s", executable, stagedBinary)
	backup := executable + ".old"

	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除旧备份失败: %w", err)
	}

	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("备份当前程序失败: %w", err)
	}

	if err := os.Rename(stagedBinary, executable); err != nil {
		_ = os.Rename(backup, executable)
		return fmt.Errorf("替换当前程序失败: %w", err)
	}

	if err := os.Chmod(executable, installedBinaryMode); err != nil {
		_ = os.Remove(executable)
		_ = os.Rename(backup, executable)
		return fmt.Errorf("设置程序执行权限失败: %w", err)
	}

	stagingDir := filepath.Dir(stagedBinary)
	_ = os.RemoveAll(stagingDir)

	logger.InfoF(ctx, "[Updater] Executing syscall.Exec to restart service: %s %v", executable, os.Args)
	//nolint:gosec // restart process via exec with same binary and args
	return syscall.Exec(executable, os.Args, os.Environ())
}
