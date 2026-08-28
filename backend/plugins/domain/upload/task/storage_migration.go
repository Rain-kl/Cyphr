// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	uploadstats "Wavelet/plugins/domain/upload/stats"
	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

const (
	// StorageMigrationTask is the task name for storage migration.
	StorageMigrationTask = uploadstorage.StorageMigrationTask
	// TaskTypeStorageMigration is the task metadata type for storage migration.
	TaskTypeStorageMigration = "storage_migration"
)

// StorageMigrationMeta describes the manually dispatchable migration task.
var StorageMigrationMeta = contracts.TaskMetaDTO{
	Name:        StorageMigrationTask,
	DisplayName: "迁移文件存储",
	Description: "将活动存储中的文件迁移到待切换的目标存储，迁移期间文件系统保持只读",
	Category:    "upload",
	MaxRetry:    3,
	Queue:       "default",
	Params: []contracts.TaskParamDTO{
		{
			Name:        "target",
			Type:        "text",
			Required:    true,
			Description: "待迁移到的目标存储引擎完整配置 JSON 字符串",
		},
		{
			Name:        "batch_size",
			Type:        "number",
			Required:    false,
			Description: "每批扫描的文件数量（默认 100）",
		},
		{
			Name:        "concurrency",
			Type:        "number",
			Required:    false,
			Description: "并发迁移 worker 数量（默认 4）",
		},
	},
}

// MigrationHandler copies stored objects and activates the target backend.
type MigrationHandler struct{}

// ValidatePayload rejects duplicate active migrations through the task framework.
func (h *MigrationHandler) ValidatePayload(payload []byte) ([]byte, error) {
	normalized, _, err := uploadstorage.NormalizeMigrationPayload(context.Background(), payload)
	if err != nil {
		return payload, err
	}
	active, err := hasUnresolvedMigrationTask(context.Background())
	if err != nil {
		return payload, err
	}
	if active {
		return payload, fmt.Errorf("storage migration task is already unresolved")
	}
	return normalized, nil
}

// Execute migrates all unique active-storage objects to the pending backend.
func (h *MigrationHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	cache := shared.GetCache(ctx)
	if cache != nil {
		const (
			cleanupTimeout  = 5 * time.Second
			renewalInterval = 10 * time.Minute
		)

		lockKey := "lock:storage:migrate"
		var lockVal string
		if err := cache.Get(ctx, lockKey, &lockVal); err == nil && lockVal != "" {
			return nil, errors.New("另一个存储迁移任务正在运行中")
		}
		_ = cache.Set(ctx, lockKey, "locked", time.Hour)

		stopRenewal := make(chan struct{})
		//nolint:contextcheck
		defer func() {
			close(stopRenewal)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			_ = cache.Delete(cleanupCtx, lockKey)
		}()

		//nolint:contextcheck,gosec
		util.Go(func() {
			ticker := time.NewTicker(renewalInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					renewCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
					_ = cache.Set(renewCtx, lockKey, "locked", time.Hour)
					cancel()
				case <-stopRenewal:
					return
				case <-ctx.Done():
					return
				}
			}
		})
	}

	active, err := loadActiveStorageConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load active storage config: %w", err)
	}
	target, err := uploadstorage.ParseMigrationTargetConfig(ctx, payload)
	if err != nil {
		return nil, err
	}
	if target.Driver == active.Driver {
		if err := saveActiveStorageConfig(ctx, target); err != nil {
			return nil, fmt.Errorf("activate same-driver storage config: %w", err)
		}
		message := fmt.Sprintf("存储配置已更新，活动存储保持为 %s", target.Driver)
		logger.InfoF(ctx, "%s", message)
		return &contracts.TaskResultDTO{Message: message}, nil
	}

	total, err := countStorageObjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("count source objects: %w", err)
	}
	if total == 0 {
		if err := saveActiveStorageConfig(ctx, target); err != nil {
			return nil, fmt.Errorf("activate empty storage config: %w", err)
		}
		message := fmt.Sprintf("当前存储没有需要迁移的对象，活动存储已切换为 %s", target.Driver)
		logger.InfoF(ctx, "%s", message)
		return &contracts.TaskResultDTO{Message: message}, nil
	}

	storageSvc := shared.GetStorage(ctx)
	if storageSvc == nil {
		return nil, errors.New("source storage service not available")
	}

	logger.InfoF(ctx, "开始存储迁移: %s -> %s，总对象数: %d", active.Driver, target.Driver, total)
	migrated, err := migrateObjects(ctx, storageSvc, storageSvc, total)
	if err != nil {
		return nil, err
	}

	if err := saveActiveStorageConfig(ctx, target); err != nil {
		return nil, fmt.Errorf("activate target storage: %w", err)
	}
	message := fmt.Sprintf("存储迁移完成，共迁移 %d 个对象，活动存储已切换为 %s", migrated, target.Driver)
	logger.InfoF(ctx, "%s", message)
	return &contracts.TaskResultDTO{Message: message}, nil
}

func loadActiveStorageConfig(ctx context.Context) (contracts.StorageConfigDTO, error) {
	var val string
	db := shared.GetDB(ctx)
	if db != nil {
		_ = db.Table("w_system_configs").Where("key = ?", "storage_config").Pluck("value", &val).Error
	}
	var cfg contracts.StorageConfigDTO
	if val != "" {
		_ = json.Unmarshal([]byte(val), &cfg)
	}
	return cfg, nil
}

func saveActiveStorageConfig(ctx context.Context, cfg contracts.StorageConfigDTO) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	db := shared.GetDB(ctx)
	if db == nil {
		return errors.New("database not available")
	}
	return db.Table("w_system_configs").
		Where("key = ?", "storage_config").
		Update("value", string(data)).Error
}

func countStorageObjects(ctx context.Context) (int64, error) {
	var count int64
	db := shared.GetDB(ctx)
	if db == nil {
		return 0, errors.New("database not available")
	}
	err := db.Model(&models.Upload{}).
		Where("status != ?", models.UploadStatusDeleted).
		Distinct("file_path").
		Count(&count).Error
	return count, err
}

func hasUnresolvedMigrationTask(ctx context.Context) (bool, error) {
	execution, ok, err := uploadstorage.LatestMigrationExecution(ctx)
	if err != nil || !ok || execution == nil {
		return false, err
	}
	return execution.Status == "pending" || execution.Status == "running", nil
}

type migrationObject struct {
	FilePath string `gorm:"column:file_path"`
	FileSize int64  `gorm:"column:file_size"`
	MimeType string `gorm:"column:mime_type"`
	Hash     string `gorm:"column:hash"`
}

type storageReaderWriter interface {
	Get(ctx context.Context, key string) (*contracts.StorageObject, error)
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (contracts.StoragePutResult, error)
}

func migrateObjects(
	ctx context.Context,
	sourceBackend storageReaderWriter,
	targetBackend storageReaderWriter,
	total int64,
) (int64, error) {
	const batchSize = 50
	const migrationConcurrency = 10
	const sha256HexLength = 64
	var migrated int64
	var lastFilePath string
	db := shared.GetDB(ctx)
	if db == nil {
		return 0, errors.New("database not available")
	}
	for {
		if err := ctx.Err(); err != nil {
			return atomic.LoadInt64(&migrated), fmt.Errorf("storage migration canceled: %w", err)
		}

		logger.InfoF(ctx, "正在查询待迁移对象批次，当前已完成迁移: %d/%d", atomic.LoadInt64(&migrated), total)

		var objects []migrationObject
		query := db.Model(&models.Upload{}).
			Select("file_path, MAX(file_size) AS file_size, MAX(mime_type) AS mime_type, MAX(hash) AS hash").
			Where("status != ?", models.UploadStatusDeleted)
		if lastFilePath != "" {
			query = query.Where("file_path > ?", lastFilePath)
		}
		if err := query.Group("file_path").
			Order("file_path ASC").
			Limit(batchSize).
			Scan(&objects).Error; err != nil {
			return atomic.LoadInt64(&migrated), fmt.Errorf("query source objects: %w", err)
		}
		if len(objects) == 0 {
			logger.InfoF(ctx, "所有对象迁移完毕")
			break
		}

		lastFilePath = objects[len(objects)-1].FilePath
		logger.InfoF(ctx, "获取当前批次迁移对象，批次大小: %d，实际获取对象数: %d", batchSize, len(objects))

		var g errgroup.Group
		g.SetLimit(migrationConcurrency)

		for _, object := range objects {
			obj := object
			g.Go(func() error {
				if err := migrateSingleObject(ctx, sourceBackend, targetBackend, obj, sha256HexLength); err != nil {
					return err
				}
				atomic.AddInt64(&migrated, 1)
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return atomic.LoadInt64(&migrated), err
		}

		logger.InfoF(ctx, "当前批次迁移完成。迁移进度: %d/%d", atomic.LoadInt64(&migrated), total)
	}
	return atomic.LoadInt64(&migrated), nil
}

func migrateSingleObject(
	ctx context.Context,
	sourceBackend storageReaderWriter,
	targetBackend storageReaderWriter,
	obj migrationObject,
	sha256HexLength int,
) error {
	if shouldSkipMigration(ctx, sourceBackend, targetBackend, obj) {
		logger.InfoF(ctx, "[跳过迁移] 目标存储已存在相同文件: %s", obj.FilePath)
		return nil
	}

	logger.InfoF(ctx, "[迁移开始] 正在从源存储读取文件: %s", obj.FilePath)
	source, err := sourceBackend.Get(ctx, obj.FilePath)
	if err != nil {
		if isNotFoundError(err) {
			return markMissingMigrationObjectDeleted(ctx, obj.FilePath, err)
		}
		return fmt.Errorf("open source object %q: %w", obj.FilePath, err)
	}
	logger.InfoF(ctx, "[传输中] 正在向目标存储上传文件: %s (大小: %d 字节, 类型: %s)", obj.FilePath, obj.FileSize, obj.MimeType)
	targetResult, putErr := targetBackend.Put(ctx, obj.FilePath, source.Body, obj.FileSize, obj.MimeType)
	closeErr := source.Body.Close()
	if putErr != nil {
		return fmt.Errorf("copy object %q: %w", obj.FilePath, putErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close source object %q: %w", obj.FilePath, closeErr)
	}

	if len(obj.Hash) == sha256HexLength {
		logger.InfoF(ctx, "[校验中] 正在对目标文件进行数据一致性校验 (SHA-256): %s", targetResult.Key)
		targetObj, getErr := targetBackend.Get(ctx, targetResult.Key)
		if getErr != nil {
			return fmt.Errorf("retrieve target object for verification %q: %w", obj.FilePath, getErr)
		}
		if targetObj == nil || targetObj.Body == nil {
			return fmt.Errorf("retrieve target object for verification %q: object or body is nil", obj.FilePath)
		}
		h := sha256.New()
		if _, copyErr := io.Copy(h, targetObj.Body); copyErr != nil {
			_ = targetObj.Body.Close()
			return fmt.Errorf("read target object for verification %q: %w", obj.FilePath, copyErr)
		}
		_ = targetObj.Body.Close()

		computedHash := hex.EncodeToString(h.Sum(nil))
		if computedHash != obj.Hash {
			return fmt.Errorf("integrity check failed for %q: got hash %s, want %s", obj.FilePath, computedHash, obj.Hash)
		}
		logger.InfoF(ctx, "[校验通过] 文件一致性校验成功: %s", targetResult.Key)
	}

	db := shared.GetDB(ctx)
	if targetResult.Key != obj.FilePath && db != nil {
		logger.InfoF(ctx, "[更新数据库] 正在更新文件路径: %s -> %s", obj.FilePath, targetResult.Key)
		if err := db.Model(&models.Upload{}).
			Where("file_path = ? AND status != ?", obj.FilePath, models.UploadStatusDeleted).
			Update("file_path", targetResult.Key).Error; err != nil {
			return fmt.Errorf("update migrated object %q: %w", obj.FilePath, err)
		}
	}
	logger.InfoF(ctx, "[迁移成功] 文件已完成迁移: %s", targetResult.Key)
	return nil
}

func shouldSkipMigration(
	ctx context.Context,
	sourceBackend storageReaderWriter,
	targetBackend storageReaderWriter,
	obj migrationObject,
) bool {
	if sourceBackend == targetBackend {
		return false
	}
	targetObj, err := targetBackend.Get(ctx, obj.FilePath)
	if err != nil || targetObj == nil || targetObj.Body == nil {
		return false
	}
	defer func() {
		_ = targetObj.Body.Close()
	}()

	return targetObj.ContentLength == obj.FileSize
}

func markMissingMigrationObjectDeleted(
	ctx context.Context,
	filePath string,
	sourceErr error,
) error {
	logger.WarnF(ctx, "警告: 源存储中物理文件不存在，标记为已删除并跳过: %s (错误: %v)", filePath, sourceErr)
	db := shared.GetDB(ctx)
	if db == nil {
		return errors.New("database not available")
	}

	var affectedUploads []models.Upload
	if err := db.
		Where("file_path = ? AND status != ?", filePath, models.UploadStatusDeleted).
		Find(&affectedUploads).Error; err != nil {
		return fmt.Errorf("load missing object uploads %q: %w", filePath, err)
	}
	if err := db.Model(&models.Upload{}).
		Where("file_path = ?", filePath).
		Update("status", models.UploadStatusDeleted).Error; err != nil {
		return fmt.Errorf("update missing object %q: %w", filePath, err)
	}
	for i := range affectedUploads {
		_ = uploadstats.ApplyUploadStatsRemove(ctx, &affectedUploads[i])
	}
	return nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	for _, sub := range []string{"not found", "nosuchkey", "nosuchbucket", "404", "does not exist"} {
		if strings.Contains(errStr, sub) {
			return true
		}
	}
	return false
}
