// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

// 模块内专用错误文案常量，集中在此文件维护，禁止在 handler 中内联。
const (
	// errSystemBusy 访问日志缓冲队列满载时的限流提示，刻意使用模糊文案避免泄露内部容量细节。
	errSystemBusy = "系统繁忙，请稍后再试"
)
