// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository_test

import (
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/message_gateway/errs"
	"Wavelet/plugins/domain/message_gateway/repository"
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// stubDBService satisfies contracts.DBService over a test database handle.
type stubDBService struct{ db *gorm.DB }

func (s stubDBService) GORM() *gorm.DB { return s.db }

func (s stubDBService) DB(_ context.Context) *gorm.DB { return s.db }

func (s stubDBService) Named(_ string) *gorm.DB { return s.db }

// TestFindUserByFieldRecordRejectsUnlistedColumns pins the column allow-list. The
// lookup column is interpolated into SQL, so an unlisted name must be refused before
// any query is built rather than trusted because call sites happen to pass literals.
func TestFindUserByFieldRecordRejectsUnlistedColumns(t *testing.T) {
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	if err := db.Table("w_users").Create(map[string]any{"id": 77, "username": "seeded"}).Error; err != nil {
		t.Fatalf("seed user failed: %v", err)
	}

	repository.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { repository.SetDBServiceForTest(nil) })

	ctx := context.Background()

	user, err := repository.FindUserByFieldRecord(ctx, "username", "seeded")
	if err != nil {
		t.Fatalf("allowlisted lookup by username failed: %v", err)
	}
	if user.ID != 77 {
		t.Errorf("allowlisted lookup returned ID %d, want 77", user.ID)
	}
	if _, err := repository.FindUserByFieldRecord(ctx, "id", uint64(77)); err != nil {
		t.Errorf("allowlisted lookup by id failed: %v", err)
	}

	cases := []struct {
		name  string
		field string
	}{
		{"tautology injection", `username = '' OR 1=1 --`},
		{"stacked statement", "id; DROP TABLE w_users"},
		{"column outside allow-list", "password"},
		{"empty field", ""},
	}
	for _, tc := range cases {
		if _, err := repository.FindUserByFieldRecord(ctx, tc.field, "seeded"); !errors.Is(err, errs.ErrUnsupportedUserLookupField) {
			t.Errorf("%s: got err %v, want ErrUnsupportedUserLookupField", tc.name, err)
		}
	}

	var remaining int64
	if err := db.Table("w_users").Count(&remaining).Error; err != nil || remaining != 1 {
		t.Fatalf("w_users damaged by rejected lookups: count=%d err=%v", remaining, err)
	}
}

// smtpTestValues are the four system-config rows the built-in email channel reads.
var smtpTestValues = map[string]string{
	"smtp_host":     "mail.example.test",
	"smtp_port":     "465",
	"smtp_username": "notify@example.test",
	"smtp_password": "s3cret-value",
}

// TestLoadSMTPConfigRecordMapsEveryKey guards the single-query rewrite: every field
// must still be filled from its own row.
func TestLoadSMTPConfigRecordMapsEveryKey(t *testing.T) {
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	keys := make([]string, 0, len(smtpTestValues))
	for key := range smtpTestValues {
		keys = append(keys, key)
	}
	if err := db.Table("w_system_configs").Where("key IN ?", keys).Delete(map[string]any{}).Error; err != nil {
		t.Fatalf("clear smtp rows: %v", err)
	}
	for _, key := range keys {
		row := map[string]any{"key": key, "value": smtpTestValues[key], "type": "system"}
		if err := db.Table("w_system_configs").Create(row).Error; err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}

	repository.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { repository.SetDBServiceForTest(nil) })

	cfg, err := repository.LoadSMTPConfigRecord(context.Background())
	if err != nil {
		t.Fatalf("LoadSMTPConfigRecord: %v", err)
	}
	if cfg.Host != smtpTestValues["smtp_host"] || cfg.Port != smtpTestValues["smtp_port"] ||
		cfg.Username != smtpTestValues["smtp_username"] || cfg.Password != smtpTestValues["smtp_password"] {
		t.Errorf("got %+v, want every SMTP field mapped from its own row", cfg)
	}
}

// TestLoadSMTPConfigRecordSurfacesReadFailure pins the actual defect: a read that
// fails used to be discarded, returning four blank strings that callers could only
// interpret as "SMTP was never configured", so the notification was dropped silently.
func TestLoadSMTPConfigRecordSurfacesReadFailure(t *testing.T) {
	bare, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open bare sqlite: %v", err)
	}

	repository.SetDBServiceForTest(stubDBService{db: bare})
	t.Cleanup(func() { repository.SetDBServiceForTest(nil) })

	if _, err := repository.LoadSMTPConfigRecord(context.Background()); err == nil {
		t.Fatal("LoadSMTPConfigRecord returned nil error although the config table cannot be read")
	}
}
