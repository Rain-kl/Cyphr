// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"Wavelet/pkg/config"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultSQLiteDBPath = "./data/wavelet.db"
	logDBNameSQLite     = "sqlite"
)

// sqliteDatabasePath resolves the effective SQLite file path from configuration.
func sqliteDatabasePath() string {
	name := config.Config.Database.SQLitePath
	if name == "" {
		name = defaultSQLiteDBPath
	}
	return name
}

// QuoteTableName escapes a raw identifier for use inside a quoted SQL fragment.
func QuoteTableName(table string) string {
	return `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
}

// GetSQLiteOverview collects the SQLite runtime overview.
func GetSQLiteOverview(ctx context.Context) (model.DBOverviewResponse, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return model.DBOverviewResponse{}, errs.ErrDatabaseUninitialized
	}

	name := sqliteDatabasePath()

	var version string
	var ver string
	if err := gormDB.Raw("SELECT sqlite_version()").Scan(&ver).Error; err == nil {
		version = "SQLite " + ver
	} else {
		version = "SQLite"
	}

	var sizeStr string
	if fi, err := os.Stat(name); err == nil {
		size := fi.Size()
		if size < 0 {
			size = 0
		}
		sizeStr = model.FormatBytes(uint64(size))
	} else {
		sizeStr = "0 B"
	}

	var tableCount int64
	if err := gormDB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount).Error; err != nil {
		tableCount = 0
	}

	var connCount int64
	if sqlDB, err := gormDB.DB(); err == nil {
		connCount = int64(sqlDB.Stats().OpenConnections)
	} else {
		connCount = 1
	}

	return model.DBOverviewResponse{
		Type:        logDBNameSQLite,
		Version:     version,
		Name:        name,
		Size:        sizeStr,
		TableCount:  tableCount,
		Connections: connCount,
	}, nil
}

// GetPostgresOverview collects the PostgreSQL runtime overview.
func GetPostgresOverview(ctx context.Context) (model.DBOverviewResponse, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return model.DBOverviewResponse{}, errs.ErrDatabaseUninitialized
	}

	name := config.Config.Database.Database

	var version string
	var ver string
	if err := gormDB.Raw("SELECT version()").Scan(&ver).Error; err == nil {
		version = ver
	} else {
		version = "PostgreSQL"
	}

	var sizeStr string
	var sizeBytes sql.NullInt64
	if err := gormDB.Raw("SELECT pg_database_size(current_database())").Scan(&sizeBytes).Error; err == nil && sizeBytes.Valid {
		size := sizeBytes.Int64
		if size < 0 {
			size = 0
		}
		sizeStr = model.FormatBytes(uint64(size))
	} else {
		sizeStr = "0 B"
	}

	var tableCount int64
	if err := gormDB.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()").Scan(&tableCount).Error; err != nil {
		tableCount = 0
	}

	var connCount int64
	var pgc sql.NullInt64
	if err := gormDB.Raw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()").Scan(&pgc).Error; err == nil && pgc.Valid {
		connCount = pgc.Int64
	} else {
		if sqlDB, err := gormDB.DB(); err == nil {
			connCount = int64(sqlDB.Stats().OpenConnections)
		} else {
			connCount = 1
		}
	}

	return model.DBOverviewResponse{
		Type:        "postgres",
		Version:     version,
		Name:        name,
		Size:        sizeStr,
		TableCount:  tableCount,
		Connections: connCount,
	}, nil
}

// ListDatabaseTableNames returns every user table of the active database.
func ListDatabaseTableNames(ctx context.Context) ([]string, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return nil, errs.ErrDatabaseUninitialized
	}

	var tables []string
	var err error

	if !config.Config.Database.Enabled {
		err = gormDB.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tables).Error
	} else {
		err = gormDB.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() ORDER BY table_name").Scan(&tables).Error
	}
	if err != nil {
		return nil, err
	}
	return tables, nil
}

// CountDatabaseTableRows counts the rows of the quoted table.
func CountDatabaseTableRows(ctx context.Context, quotedTable string) (int64, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return 0, errs.ErrDatabaseUninitialized
	}

	var total int64
	if err := gormDB.Raw("SELECT count(*) FROM " + quotedTable).Scan(&total).Error; err != nil {
		return 0, errs.NewInvalidInputError(err.Error())
	}
	return total, nil
}

// QueryDatabaseTableRows loads one page of raw rows from the quoted table.
func QueryDatabaseTableRows(
	ctx context.Context,
	quotedTable string,
	limit int,
	offset int,
) ([]string, []map[string]any, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return nil, nil, errs.ErrDatabaseUninitialized
	}

	rows, err := gormDB.Raw("SELECT * FROM "+quotedTable+" LIMIT ? OFFSET ?", limit, offset).Rows()
	if err != nil {
		return nil, nil, errs.NewInvalidInputError(err.Error())
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	results, err := scanTableRows(rows, cols)
	if err != nil {
		return nil, nil, err
	}
	return cols, results, nil
}

// RunSelectSQL executes an arbitrary select-like statement.
func RunSelectSQL(ctx context.Context, sqlStr string) ([]string, []map[string]any, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return nil, nil, errs.ErrDatabaseUninitialized
	}

	rows, err := gormDB.Raw(sqlStr).Rows()
	if err != nil {
		return nil, nil, errs.NewInvalidInputError(err.Error())
	}
	defer func() {
		_ = rows.Close()
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	results, err := scanTableRows(rows, cols)
	if err != nil {
		return nil, nil, err
	}
	return cols, results, nil
}

// RunMutationSQL executes a non-query statement and reports affected rows.
func RunMutationSQL(ctx context.Context, sqlStr string) (int64, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return 0, errs.ErrDatabaseUninitialized
	}
	tx := gormDB.Exec(sqlStr)
	if tx.Error != nil {
		return 0, errs.NewInvalidInputError(tx.Error.Error())
	}
	return tx.RowsAffected, nil
}

// scanTableRows decodes every row of the result set into a column keyed map.
func scanTableRows(rows *sql.Rows, cols []string) ([]map[string]any, error) {
	results := make([]map[string]any, 0)
	for rows.Next() {
		row, err := scanRowAsMap(rows, cols)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	return results, nil
}

// scanRowAsMap decodes a single row, normalising driver byte slices to strings.
func scanRowAsMap(rows *sql.Rows, cols []string) (map[string]any, error) {
	columns := make([]any, len(cols))
	columnPointers := make([]any, len(cols))
	for i := range columns {
		columnPointers[i] = &columns[i]
	}

	if err := rows.Scan(columnPointers...); err != nil {
		return nil, err
	}

	rowMap := make(map[string]any)
	for i, colName := range cols {
		val := columns[i]
		if b, ok := val.([]byte); ok {
			rowMap[colName] = string(b)
			continue
		}
		rowMap[colName] = val
	}
	return rowMap, nil
}

// GetSQLiteInfo collects the SQLite type/name/version triple.
func GetSQLiteInfo(ctx context.Context) model.DatabaseInfoResponse {
	info := model.DatabaseInfoResponse{
		Type:    logDBNameSQLite,
		Name:    config.Config.Database.SQLitePath,
		Version: "SQLite",
	}
	if info.Name == "" {
		info.Name = defaultSQLiteDBPath
	}
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return info
	}
	var ver string
	if err := gormDB.Raw("SELECT sqlite_version()").Scan(&ver).Error; err == nil && ver != "" {
		info.Version = "SQLite " + ver
	}
	return info
}

// GetPostgresInfo collects the PostgreSQL type/name/version triple.
func GetPostgresInfo(ctx context.Context) model.DatabaseInfoResponse {
	info := model.DatabaseInfoResponse{
		Type:    "postgres",
		Name:    config.Config.Database.Database,
		Version: "PostgreSQL",
	}
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return info
	}
	var ver string
	if err := gormDB.Raw("SELECT version()").Scan(&ver).Error; err == nil && ver != "" {
		info.Version = ver
	}
	return info
}

// OpenSQLiteExportFile opens the active SQLite database file together with its stat info.
func OpenSQLiteExportFile() (*os.File, os.FileInfo, error) {
	//nolint:gosec // export db file path is trusted
	f, err := os.Open(sqliteDatabasePath())
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", errs.ErrOpenDatabaseFileFailed, err)
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("%s: %w", errs.ErrReadDatabaseFileInfoFailed, err)
	}
	return f, fi, nil
}

// NewPgDumpCommand builds the streaming pg_dump command for the active database.
func NewPgDumpCommand(ctx context.Context) (*exec.Cmd, string, error) {
	dbCfg := config.Config.Database

	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, "", errors.New(errs.ErrPgDumpUnavailable)
	}

	args := []string{
		"--no-password",
		"-h", dbCfg.Host,
		"-p", fmt.Sprintf("%d", dbCfg.Port),
		"-U", dbCfg.Username,
		dbCfg.Database,
	}

	//nolint:gosec // pg_dump args are constructed from validated db config
	cmd := exec.CommandContext(ctx, pgDumpPath, args...)
	if dbCfg.Password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+dbCfg.Password)
	} else {
		cmd.Env = os.Environ()
	}

	fileName := fmt.Sprintf("wavelet_%s.sql", time.Now().Format("20060102_150405"))
	return cmd, fileName, nil
}
