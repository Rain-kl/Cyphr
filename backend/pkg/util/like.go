// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "strings"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike escapes SQL LIKE metacharacters (\, %, _) so a user-supplied
// value matches literally in LIKE patterns. Pair it with an explicit
// `ESCAPE '\'` clause where the dialect has no backslash default (SQLite);
// PostgreSQL and ClickHouse treat backslash as the default LIKE escape.
func EscapeLike(value string) string {
	return likeEscaper.Replace(value)
}
