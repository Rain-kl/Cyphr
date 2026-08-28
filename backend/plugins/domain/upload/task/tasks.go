// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/upload/filesrv"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// 上传域任务的通用元数据常量（TaskMetaDTO Category/Queue 复用）
const (
	taskCategoryUpload    = "upload"
	taskQueueDefault      = "default"
)

const (
	// WarmImageCacheTask 图片压缩缓存预热任务标识
	WarmImageCacheTask = "upload:warm_image_cache"
	// TaskTypeWarmImageCache 图片压缩缓存预热管理类型
	TaskTypeWarmImageCache = "warm_image_cache"
)

var warmImageCacheMu sync.Mutex

// WarmImageCacheMeta represents the image cache warmup task metadata.
var WarmImageCacheMeta = contracts.TaskMetaDTO{
	Name:        WarmImageCacheTask,
	DisplayName: "预热图片压缩缓存",
	Description: "串行将文件管理中的图片转换为指定质量的 WebP 并写入永久缓存",
	Category:    taskCategoryUpload,
	MaxRetry:    3,
	Queue:       taskQueueDefault,
	Params: []contracts.TaskParamDTO{
		{
			Name:        "quality",
			Type:        "string",
			Required:    true,
			Description: "WebP 压缩质量，仅支持 low、medium、high",
		},
	},
}

// WarmImageCachePayload is the image cache warmup task payload.
type WarmImageCachePayload struct {
	Quality string `json:"quality"`
}

// WarmImageCacheHandler serially warms compressed image cache entries.
type WarmImageCacheHandler struct{}

// ValidatePayload validates and normalizes image cache warmup parameters.
func (h *WarmImageCacheHandler) ValidatePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, errors.New(shared.ErrImageCacheWarmupPayloadRequired)
	}

	var req WarmImageCachePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf(shared.ErrInvalidImageCacheWarmupPayload, err)
	}

	req.Quality = strings.ToLower(strings.TrimSpace(req.Quality))
	if req.Quality != shared.ImageQualityLow &&
		req.Quality != shared.ImageQualityMedium &&
		req.Quality != shared.ImageQualityHigh {
		return nil, errors.New(shared.ErrInvalidImageCacheWarmupQuality)
	}

	return json.Marshal(req)
}

// Execute serially converts all managed images to WebP cache entries.
func (h *WarmImageCacheHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	normalizedPayload, err := h.ValidatePayload(payload)
	if err != nil {
		logger.WarnF(ctx, "图片缓存预热参数无效: %v", err)
		return nil, err
	}

	var req WarmImageCachePayload
	if err := json.Unmarshal(normalizedPayload, &req); err != nil {
		return nil, fmt.Errorf(shared.ErrParseImageCacheWarmupPayload, err)
	}

	logger.InfoF(ctx, "等待获取图片缓存预热执行锁，质量: %s", req.Quality)
	warmImageCacheMu.Lock()
	defer warmImageCacheMu.Unlock()

	const (
		batchSize      = 50
		maxFailureLogs = 5
	)
	var lastID uint64
	var totalProcessed int
	var totalCached int
	var totalGenerated int
	var totalFailed int

	logger.InfoF(ctx, "开始串行预热图片压缩缓存，质量: %s，每批: %d", req.Quality, batchSize)

	db := shared.GetDB(ctx)
	if db == nil {
		return nil, errors.New("database service not available")
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("image cache warmup canceled: %w", err)
		}

		var uploads []models.Upload
		if err := db.
			Where("id > ? AND status != ? AND (LOWER(mime_type) LIKE ? OR LOWER(extension) IN ?)",
				lastID,
				models.UploadStatusDeleted,
				"image/%",
				[]string{"jpg", "jpeg", "png", "webp", "gif"},
			).
			Order("id ASC").
			Limit(batchSize).
			Find(&uploads).Error; err != nil {
			logger.ErrorF(ctx, "查询图片上传记录失败: %v", err)
			return nil, fmt.Errorf(shared.ErrQueryImagesForCacheWarmup, err)
		}

		if len(uploads) == 0 {
			break
		}

		batchGenerated := 0
		batchCached := 0
		batchFailed := 0
		for i := range uploads {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("image cache warmup canceled: %w", err)
			}

			upload := &uploads[i]
			totalProcessed++
			lastID = upload.ID

			_, cacheHit, err := filesrv.EnsureCompressedImageCache(ctx, upload, req.Quality)
			if err != nil {
				totalFailed++
				batchFailed++
				if totalFailed <= maxFailureLogs {
					logger.WarnF(ctx, "图片处理失败 [ID:%d]: %v", upload.ID, err)
				}
				continue
			}
			if cacheHit {
				totalCached++
				batchCached++
				continue
			}
			totalGenerated++
			batchGenerated++
		}

		logger.InfoF(
			ctx,
			"批次完成，末尾 ID: %d，生成: %d，命中: %d，失败: %d",
			lastID,
			batchGenerated,
			batchCached,
			batchFailed,
		)
	}

	msg := fmt.Sprintf(
		"图片缓存预热完成，共处理 %d 张，生成 %d 张，命中 %d 张，失败 %d 张",
		totalProcessed,
		totalGenerated,
		totalCached,
		totalFailed,
	)
	logger.InfoF(ctx, "%s", msg)
	return &contracts.TaskResultDTO{Message: msg}, nil
}
