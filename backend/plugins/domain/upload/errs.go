// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package upload 提供文件上传与下载功能
package upload

import "Wavelet/plugins/domain/upload/shared"

// 文件管理常量
const (
	ErrNoFileSelected               = shared.ErrNoFileSelected
	ErrUnsupportedFormat            = shared.ErrUnsupportedFormat
	ErrProcessFileFailed            = shared.ErrProcessFileFailed
	ErrSaveFileFailed               = shared.ErrSaveFileFailed
	ErrOpenFileFailed               = shared.ErrOpenFileFailed
	ErrSaveUploadRecordFailed       = shared.ErrSaveUploadRecordFailed
	ErrGenericFileTooLarge          = shared.ErrGenericFileTooLarge
	ErrFileContentExtensionMismatch = shared.ErrFileContentExtensionMismatch
	ErrFileValidationFailed         = shared.ErrFileValidationFailed
	ErrInvalidMetadataJSON          = shared.ErrInvalidMetadataJSON
	ErrInvalidFileID                = shared.ErrInvalidFileID
	ErrQueryUploadRecordFailed      = shared.ErrQueryUploadRecordFailed
	ErrInvalidBatchDownloadRequest  = shared.ErrInvalidBatchDownloadRequest
	ErrInvalidIDValueFormat         = shared.ErrInvalidIDValueFormat
	ErrRetrieveUploadRecordsFailed  = shared.ErrRetrieveUploadRecordsFailed
	ErrNoValidFilesForArchive       = shared.ErrNoValidFilesForArchive
	ErrInvalidParams                = shared.ErrInvalidParams
	ErrQueryFileCountFailed         = shared.ErrQueryFileCountFailed
	ErrQueryFileListFailed          = shared.ErrQueryFileListFailed
	ErrDeleteFileFailed             = shared.ErrDeleteFileFailed
	ErrStorageReadOnly              = shared.ErrStorageReadOnly
	ErrS3KeyRequired                = shared.ErrS3KeyRequired
	ErrS3KeyTooLongFormat           = shared.ErrS3KeyTooLongFormat
	ErrS3KeyStartsWithSlash         = shared.ErrS3KeyStartsWithSlash
	ErrS3KeyContainsNullBytes       = shared.ErrS3KeyContainsNullBytes
	ErrQueryUnusedUploadsFailed     = shared.ErrQueryUnusedUploadsFailed
	ErrUnauthorized                 = shared.ErrUnauthorized
)
