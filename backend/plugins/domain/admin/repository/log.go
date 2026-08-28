// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"Wavelet/pkg/util"
	"context"
)

// UserDisplayName is the minimal user projection needed to decorate access log rows.
type UserDisplayName struct {
	Username string
	Nickname string
}

// SearchUserIDsByUsername is the database fallback used when the user contract is absent.
func SearchUserIDsByUsername(ctx context.Context, username string) ([]uint64, error) {
	gormDB := GetDB(ctx)
	if gormDB == nil {
		return nil, nil
	}

	var ids []uint64
	if err := gormDB.Table("w_users").
		Where("username LIKE ? ESCAPE '\\'", "%"+util.EscapeLike(username)+"%").
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// LoadUserDisplayNames resolves usernames and nicknames for the given ids.
func LoadUserDisplayNames(ctx context.Context, userIDs []uint64) (map[uint64]UserDisplayName, error) {
	result := make(map[uint64]UserDisplayName, len(userIDs))
	gormDB := GetDB(ctx)
	if gormDB == nil || len(userIDs) == 0 {
		return result, nil
	}

	var users []struct {
		ID       uint64
		Username string
		Nickname string
	}
	if err := gormDB.Table("w_users").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, u := range users {
		result[u.ID] = UserDisplayName{Username: u.Username, Nickname: u.Nickname}
	}
	return result, nil
}
