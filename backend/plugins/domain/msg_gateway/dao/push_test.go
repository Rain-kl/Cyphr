// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao_test

import (
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/msg_gateway/dao"
	"Wavelet/plugins/domain/msg_gateway/model/entity"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type stubDBService struct{ db *gorm.DB }

func (s stubDBService) GORM() *gorm.DB                { return s.db }
func (s stubDBService) DB(_ context.Context) *gorm.DB { return s.db }
func (s stubDBService) Named(_ string) *gorm.DB       { return s.db }

func TestPushChannelDAO_CRUD(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.PushChannel{}, &entity.PushEvent{}, &entity.PushHistory{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	ch := entity.PushChannel{
		Name:    "test_webhook",
		Type:    "custom",
		URL:     "https://example.com/hook",
		Enabled: true,
	}
	require.NoError(t, dao.CreatePushChannelRecord(ctx, &ch))
	assert.NotZero(t, ch.ID)

	loaded, err := dao.GetPushChannelByIDRecord(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, "test_webhook", loaded.Name)

	active, err := dao.GetActivePushChannelByName(ctx, "test_webhook")
	require.NoError(t, err)
	assert.Equal(t, ch.ID, active.ID)

	channels, err := dao.ListPushChannelsRecord(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, channels)

	require.NoError(t, dao.DeletePushChannelRecord(ctx, &ch))
	_, err = dao.GetPushChannelByIDRecord(ctx, ch.ID)
	assert.Error(t, err)
}

func TestPushEventDAO_CRUD(t *testing.T) {
	_ = idgen.Init(1)
	db, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	require.NoError(t, db.AutoMigrate(&entity.PushChannel{}, &entity.PushEvent{}, &entity.PushHistory{}))

	dao.SetDBServiceForTest(stubDBService{db: db})
	t.Cleanup(func() { dao.SetDBServiceForTest(nil) })

	ctx := context.Background()

	ev := entity.PushEvent{
		EventKey: "test_event",
		Name:     "测试事件",
		Channels: []string{"test_webhook"},
		Targets:  []string{"admin"},
		Template: `{"title":"Hello"}`,
		Enabled:  true,
	}
	require.NoError(t, dao.CreatePushEventRecord(ctx, &ev))
	assert.NotZero(t, ev.ID)

	loaded, err := dao.GetPushEventByKeyRecord(ctx, "test_event")
	require.NoError(t, err)
	assert.Equal(t, "测试事件", loaded.Name)

	require.NoError(t, dao.UpdatePushEventEnabledRecord(ctx, &ev, false))
	loadedDisabled, err := dao.GetPushEventByIDRecord(ctx, ev.ID)
	require.NoError(t, err)
	assert.False(t, loadedDisabled.Enabled)

	require.NoError(t, dao.DeletePushEventRecord(ctx, &ev))
	_, err = dao.GetPushEventByIDRecord(ctx, ev.ID)
	assert.Error(t, err)
}
