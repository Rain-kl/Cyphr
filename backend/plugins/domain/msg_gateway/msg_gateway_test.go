// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package msg_gateway_test

import (
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/msg_gateway"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

type mockDBService struct {
	db *gorm.DB
}

func (m *mockDBService) GORM() *gorm.DB {
	return m.db
}

func (m *mockDBService) DB(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func (m *mockDBService) Named(_ string) *gorm.DB {
	return m.db
}

func TestUpsertPairingCode_ReusesUnexpired(t *testing.T) {
	testDB, _, cleanup := testhelper.SetupTestEnvironment(t)
	msg_gateway.SetDBServiceForTest(&mockDBService{db: testDB})
	defer func() {
		msg_gateway.SetDBServiceForTest(nil)
		cleanup()
	}()
	ctx := context.Background()
	first, err := msg_gateway.UpsertPairingCode(ctx, 1, "tg-1", "ABCD1234", time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := msg_gateway.UpsertPairingCode(ctx, 1, "tg-1", "ZZZZ9999", time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.Code != second.Code || first.Code != "ABCD1234" {
		t.Fatalf("reuse failed: %+v %+v", first, second)
	}
}
