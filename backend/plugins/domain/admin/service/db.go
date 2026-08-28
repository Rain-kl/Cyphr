// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/pkg/config"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// selectSQLKeywords marks statements that return a result set instead of a row count.
var selectSQLKeywords = []string{"select", "show", "explain", "describe", "pragma"}

// DatabaseOverview collects the runtime overview of the active database.
func DatabaseOverview(ctx context.Context) (model.DBOverviewResponse, error) {
	if !config.Config.Database.Enabled {
		return repository.GetSQLiteOverview(ctx)
	}
	return repository.GetPostgresOverview(ctx)
}

// DatabaseTableNames returns every user table of the active database.
func DatabaseTableNames(ctx context.Context) ([]string, error) {
	return repository.ListDatabaseTableNames(ctx)
}

// DatabaseTableData loads one page of a table with its column layout and total row count.
func DatabaseTableData(ctx context.Context, req model.GetTableDataRequest) (model.TableDataResponse, error) {
	quotedTable := repository.QuoteTableName(req.Table)

	total, err := repository.CountDatabaseTableRows(ctx, quotedTable)
	if err != nil {
		return model.TableDataResponse{}, err
	}

	offset := (req.Page - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	limit := req.PageSize
	if limit <= 0 {
		limit = 10
	}

	cols, results, err := repository.QueryDatabaseTableRows(ctx, quotedTable, limit, offset)
	if err != nil {
		return model.TableDataResponse{}, err
	}

	return model.TableDataResponse{
		Columns: cols,
		Total:   total,
		Results: truncateCellValues(results),
	}, nil
}

// truncateCellValues caps oversized string cells before they reach the console grid.
func truncateCellValues(rows []map[string]any) []map[string]any {
	for _, row := range rows {
		for column, value := range row {
			if str, ok := value.(string); ok {
				row[column] = model.TruncateDisplayValue(str)
			}
		}
	}
	return rows
}

// ExecuteCustomSQL runs an arbitrary statement issued from the console SQL runner.
func ExecuteCustomSQL(ctx context.Context, trimmedSQL string) (model.ExecuteSQLResponse, error) {
	startTime := time.Now()

	if isSelectStatement(trimmedSQL) {
		cols, results, err := repository.RunSelectSQL(ctx, trimmedSQL)
		if err != nil {
			return model.ExecuteSQLResponse{}, err
		}
		return model.ExecuteSQLResponse{
			Type:            "select",
			Columns:         cols,
			Results:         results,
			AffectedRows:    int64(len(results)),
			ExecutionTimeMs: time.Since(startTime).Milliseconds(),
		}, nil
	}

	affectedRows, err := repository.RunMutationSQL(ctx, trimmedSQL)
	if err != nil {
		return model.ExecuteSQLResponse{}, err
	}
	return model.ExecuteSQLResponse{
		Type:            "exec",
		AffectedRows:    affectedRows,
		ExecutionTimeMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// isSelectStatement reports whether the statement yields a result set.
func isSelectStatement(trimmedSQL string) bool {
	lowerSQL := strings.ToLower(trimmedSQL)
	for _, kw := range selectSQLKeywords {
		if strings.HasPrefix(lowerSQL, kw) {
			return true
		}
	}
	return false
}

// DatabaseInfo returns the active database type, name and version.
func DatabaseInfo(ctx context.Context) model.DatabaseInfoResponse {
	if !config.Config.Database.Enabled {
		return repository.GetSQLiteInfo(ctx)
	}
	return repository.GetPostgresInfo(ctx)
}

// OpenSQLiteExportFile opens the active SQLite database file together with its stat info.
func OpenSQLiteExportFile() (*os.File, os.FileInfo, error) {
	return repository.OpenSQLiteExportFile()
}

// NewPgDumpCommand builds the streaming pg_dump command for the active database.
func NewPgDumpCommand(ctx context.Context) (*exec.Cmd, string, error) {
	return repository.NewPgDumpCommand(ctx)
}
