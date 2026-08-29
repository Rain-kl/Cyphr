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
