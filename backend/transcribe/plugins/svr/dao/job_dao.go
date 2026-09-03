// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/consts"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"context"
	"time"

	"gorm.io/gorm"
)

const columnStatus = "status"

// JobDAO defines data access methods for t_jobs and t_job_logs.
type JobDAO interface {
	Create(ctx context.Context, job *entity.JobEntity) error
	GetByID(ctx context.Context, id uint64) (*entity.JobEntity, error)
	ListByUserID(ctx context.Context, uid uint64, page, size int, status string, keyword ...string) ([]entity.JobEntity, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status string) error
	AppendLogs(ctx context.Context, jobID uint64, logs []entity.JobLogEntity) error
	GetLogsByJobID(ctx context.Context, jobID uint64) ([]entity.JobLogEntity, error)
	ListPendingJobs(ctx context.Context) ([]entity.JobEntity, error)
	UpdateNodeID(ctx context.Context, id uint64, nodeID uint64, status string) error
	UpdateCompletion(ctx context.Context, id uint64, status string, duration float64, resultText, resultJSON, errorMsg string) error
	ResetRunningJobsToPending(ctx context.Context, nodeID uint64) error
}

// GormJobDAO implements JobDAO using GORM.
type GormJobDAO struct {
	db *gorm.DB
}

var _ JobDAO = (*GormJobDAO)(nil)

// NewJobDAO creates a new JobDAO.
func NewJobDAO(db *gorm.DB) JobDAO {
	return &GormJobDAO{db: db}
}

// Create inserts a new job record.
func (d *GormJobDAO) Create(ctx context.Context, job *entity.JobEntity) error {
	if job.ID == 0 {
		job.ID = generateID(job.ID)
	}
	if job.Status == "" {
		job.Status = consts.StatusPending
	}
	if job.TaskType == "" {
		job.TaskType = consts.TaskTypeASR
	}
	return d.db.WithContext(ctx).Create(job).Error
}

// GetByID finds a job by ID.
func (d *GormJobDAO) GetByID(ctx context.Context, id uint64) (*entity.JobEntity, error) {
	var job entity.JobEntity
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &job, nil
}

// ListByUserID lists jobs for a specific user with pagination, optional status, and optional keyword filter on file name.
func (d *GormJobDAO) ListByUserID(ctx context.Context, uid uint64, page, size int, status string, keyword ...string) ([]entity.JobEntity, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	offset := (page - 1) * size

	query := d.db.WithContext(ctx).Model(&entity.JobEntity{}).Where("user_id = ?", uid)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if len(keyword) > 0 && keyword[0] != "" {
		escaped := util.EscapeLike(keyword[0])
		query = query.Where("original_file_name LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.JobEntity
	if err := query.Order("id DESC").Offset(offset).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// UpdateStatus updates the job's status and records timestamps.
func (d *GormJobDAO) UpdateStatus(ctx context.Context, id uint64, status string) error {
	now := time.Now()
	updates := map[string]any{
		columnStatus: status,
	}
	switch status {
	case consts.StatusRunning:
		updates["started_at"] = &now
	case consts.StatusCompleted, consts.StatusFailed:
		updates["completed_at"] = &now
	}

	res := d.db.WithContext(ctx).Model(&entity.JobEntity{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return mapNotFound(gorm.ErrRecordNotFound)
	}
	return nil
}

// AppendLogs appends logs for a job in a transaction, auto-assigning IDs and sequence numbers if needed.
func (d *GormJobDAO) AppendLogs(ctx context.Context, jobID uint64, logs []entity.JobLogEntity) error {
	if len(logs) == 0 {
		return nil
	}

	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxSeq int
		row := tx.Model(&entity.JobLogEntity{}).Where("job_id = ?", jobID).Select("COALESCE(MAX(seq), 0)").Row()
		_ = row.Scan(&maxSeq)

		maxProgress := 0
		for i := range logs {
			logs[i].JobID = jobID
			if logs[i].ID == 0 {
				logs[i].ID = generateID(0)
			}
			if logs[i].Seq == 0 {
				maxSeq++
				logs[i].Seq = maxSeq
			} else if logs[i].Seq > maxSeq {
				maxSeq = logs[i].Seq
			}
			if logs[i].Progress > maxProgress {
				maxProgress = logs[i].Progress
			}
		}

		if err := tx.Create(&logs).Error; err != nil {
			return err
		}

		if maxProgress > 0 {
			if err := tx.Model(&entity.JobEntity{}).Where("id = ?", jobID).Update("progress", maxProgress).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetLogsByJobID retrieves all log rows for a job ordered by seq ASC.
func (d *GormJobDAO) GetLogsByJobID(ctx context.Context, jobID uint64) ([]entity.JobLogEntity, error) {
	var logs []entity.JobLogEntity
	if err := d.db.WithContext(ctx).Where("job_id = ?", jobID).Order("seq ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ListPendingJobs finds all jobs with pending status ordered by ID ASC.
func (d *GormJobDAO) ListPendingJobs(ctx context.Context) ([]entity.JobEntity, error) {
	var list []entity.JobEntity
	if err := d.db.WithContext(ctx).Where("status = ?", consts.StatusPending).Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateNodeID assigns a node to the job and updates status.
func (d *GormJobDAO) UpdateNodeID(ctx context.Context, id uint64, nodeID uint64, status string) error {
	now := time.Now()
	updates := map[string]any{
		"node_id":    nodeID,
		columnStatus: status,
	}
	if status == consts.StatusRunning {
		updates["started_at"] = &now
	}
	res := d.db.WithContext(ctx).Model(&entity.JobEntity{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return mapNotFound(gorm.ErrRecordNotFound)
	}
	return nil
}

// UpdateCompletion records job completion results, timing, and status.
func (d *GormJobDAO) UpdateCompletion(ctx context.Context, id uint64, status string, duration float64, resultText, resultJSON, errorMsg string) error {
	now := time.Now()
	updates := map[string]any{
		columnStatus:       status,
		"duration_seconds": duration,
		"result_text":      resultText,
		"result_json":      resultJSON,
		"error_msg":        errorMsg,
		"completed_at":     &now,
	}
	if status == consts.StatusCompleted {
		updates["progress"] = 100
	}

	res := d.db.WithContext(ctx).Model(&entity.JobEntity{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return mapNotFound(gorm.ErrRecordNotFound)
	}
	return nil
}

// ResetRunningJobsToPending resets all running jobs on a disconnected node back to pending.
func (d *GormJobDAO) ResetRunningJobsToPending(ctx context.Context, nodeID uint64) error {
	return d.db.WithContext(ctx).Model(&entity.JobEntity{}).
		Where("node_id = ? AND status = ?", nodeID, consts.StatusRunning).
		Updates(map[string]any{
			columnStatus: consts.StatusPending,
			"node_id":    nil,
		}).Error
}
