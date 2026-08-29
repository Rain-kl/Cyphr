// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import "sort"

// Entries returns the effective configuration as redacted, key-sorted entries.
func (r *ConfigRegistry) Entries() []ConfigEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := append([]string(nil), r.order...)
	sort.Strings(keys)

	out := make([]ConfigEntry, 0, len(keys))
	for _, key := range keys {
		d := r.decls[key]
		out = append(out, ConfigEntry{
			Key: d.key, PluginID: d.pluginID, Env: d.env,
			Origin: r.origins[key], Value: "pending",
		})
	}
	return out
}
