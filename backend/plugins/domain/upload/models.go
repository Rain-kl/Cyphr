// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package upload 提供上传域的门面与类型重导出。
package upload

import (
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/models"
)

// UploadStatus 上传状态类型别名
//
//nolint:revive
type UploadStatus = models.UploadStatus

// UploadMetadata 上传元数据类型别名
//
//nolint:revive
type UploadMetadata = models.UploadMetadata

// Upload 上传实体类型别名
type Upload = models.Upload

// UploadStat 上传统计实体类型别名
//
//nolint:revive
type UploadStat = models.UploadStat

// 上传状态常量别名
const (
	UploadStatusPending = models.UploadStatusPending
	UploadStatusUsed    = models.UploadStatusUsed
	UploadStatusDeleted = models.UploadStatusDeleted
)
