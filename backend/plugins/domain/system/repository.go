// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
)

// publicSystemConfig 前端公共配置接口的只读投影。
type publicSystemConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// listPublicSystemConfigs 读取对前端可见的系统配置项。
//
// 注意：w_system_configs 的所有者插件是 admin，contracts 目前尚未暴露读取契约，
// 因此此处仍只能直连只读查询；待 admin 提供 SettingsService 契约后应改为调用契约。
func listPublicSystemConfigs(ctx context.Context, appCtx *core.Context) ([]publicSystemConfig, error) {
	dbSvc, err := core.Inject[contracts.DBService](appCtx)
	if err != nil {
		return nil, err
	}
	if dbSvc == nil {
		return nil, nil
	}
	var configs []publicSystemConfig
	err = dbSvc.DB(ctx).Table("w_system_configs").
		Where("visibility = ?", "visible").
		Find(&configs).Error
	return configs, err
}
