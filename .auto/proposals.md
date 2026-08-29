# Deferred proposals — autoresearch run (iterations 27-36)

Five verified findings deliberately **not** changed by the loop: each needs either a
contract/API decision or a multi-package restructure, which this run was scoped to
propose rather than perform. Evidence is from reading the cited files in this
checkout at commit `ea97b64` plus iterations 27-35.

---

## P1 — Cross-driver storage migration cannot move objects (severity: data availability)

`plugins/domain/upload/task/storage_migration.go` computes `target` from the payload,
then calls:

```go
migrated, err := migrateObjects(ctx, storageSvc, storageSvc, total)
```

`sourceBackend` and `targetBackend` are the **same** `contracts.StorageService`. That
service resolves its backend per call and only ever to the currently active one
(`plugins/infra/storage/plugin.go:89` → `s.backend` or `objectstore.Active(ctx)`), and
the target config is persisted **after** the migration loop
(`uploadstorage.SaveActiveConfig(ctx, target)`).

Consequence for a non-empty source: `migrateSingleObject` reads and writes the same
backend; `shouldSkipMigration` finds every object already "present in the target" and
skips it, yet `migrated` is still incremented, so the task returns
`存储迁移完成，共迁移 N 个对象，活动存储已切换为 <driver>` having copied **zero** bytes,
and then points the platform at an empty backend. The same-driver and
`total == 0` branches are harmless and legitimately need no copying.

Why the tests miss it: `shared.MockStorageService` is one instance serving both
parameters, so a copy-to-self looks correct.

Proposed fix (needs a contract decision — this is a feature, not a patch):
1. Extend `contracts.StorageService` with the ability to operate against an explicitly
   supplied `StorageConfigDTO` (e.g. `BackendFor(ctx, cfg) (StorageReader, error)`),
   implemented in `plugins/infra/storage` where the `objectstore` backends live. They
   are unexported today and `plugins/domain/upload` must not import them (cross-plugin
   import ban), so the contract is the only correct route.
2. In the task, build the target from `target` and pass distinct source/target.
3. Only save the active config after a verified copy, and assert `src != dst` at
   entry.
4. Interim safety option if a decision is needed sooner: make the
   `target.Driver != active.Driver && total > 0` branch return an explicit
   not-implemented error instead of reporting success. Rejected by this loop because
   it disables an advertised admin operation, which is a product call, and because
   the machinery it would strand (`migrateObjects`, `migrateSingleObject`,
   `shouldSkipMigration`) becomes dead code the project gate then rejects.

Size: contract + infra impl + task wiring + a two-backend test double. Roughly one
focused session, not a loop iteration.

---

## P2 — `w_system_configs` has one migration owner and many writers (Cordis single-owner)

Owner per migrations: `plugins/domain/admin`. Still read/written with raw SQL from
`plugins/domain/system/repository.go:31`, `plugins/domain/cap/repository.go:50`,
`plugins/domain/auth/repository.go:122`, `plugins/domain/upload/storage/migration.go:96,108`,
`plugins/domain/upload/ingest/helpers.go:55`,
`plugins/domain/message_gateway/repository/push.go` and `plugins/drivers/driver_http`.

Iteration 34 fixed one instance of the real damage this causes (a failed read looked
identical to "unconfigured", silently dropping notifications); iteration 35 fixed
another (a failed read cached a narrowed whitelist for a whole TTL). The remaining
sites carry the same trap.

Proposed fix: one settings accessor contract (`Get(ctx, key) (string, error)` /
`GetAll(ctx, keys...)`) owned by the settings subsystem, then delete the raw table
access. Keys should be declared where they are used rather than string-matched.
Size: medium, touches seven plugins; do it key-group by key-group so each step is
independently revertable.

---

## P3 — Two tables are modelled twice (schema drift hazard)

* `w_task_executions`: `plugins/domain/admin/model/entity.go:207` **and**
  `plugins/drivers/driver_asynq_worker/types.go:49`. The two `TaskExecution` structs
  and their status enums are byte-for-byte identical today.
* `w_schedules`: `plugins/domain/admin/model/entity.go:169` **and**
  `plugins/drivers/driver_asynq_cron/schedule.go:25`.

Nothing is broken yet — that is the risk: the migration owner was only recently moved
to `admin` (`49f9d10`), and a column added to one struct will silently diverge from
the other, so whichever writer holds the stale struct zeroes or omits the new column.

Proposed fix: pick the single owner per P2's rules and have the other side go through
a contract (execution recording already has DTOs in `contracts`), then delete the
duplicate model. Consider a gate check rejecting two non-`testhelper` packages
declaring the same `w_` table — it will fail until these two are resolved, so land it
with the fix (the pattern that worked in iterations 5 and 19).

---

## P4 — `user` deletes rows from tables owned by `auth`

`plugins/domain/user/repository.go:327,330` issues `DELETE` against `w_access_tokens`
and `w_external_accounts`, both owned and migrated by `plugins/domain/auth`, inside
user deletion. It works, but ownership is inverted: revoke-on-delete is auth's
invariant, and encoding it in `user` means any other deletion path silently skips it.

Proposed fix: emit a typed `user:deleted` event from `user` and let `auth` cascade
within its own transaction boundary, or expose an explicit `AuthService.RevokeForUser`.
Size: small-to-medium; needs a test that the revocation still happens on delete.

---

## P5 — Package `cap` shadows the predeclared identifier (8 of 62 debt)

Every file in `plugins/domain/cap` declares `package cap`, which shadows the builtin.
It is the single largest block of non-cosmetic lint debt this run declined to chase,
and it is also a readability cost (`cap.Something` reads as a builtin call).

Proposed fix: rename to a non-shadowing identifier (e.g. `capacity` / `proofwork`,
matching what the plugin actually does) across its own files and importers. Mechanical
but wide; needs a decision on the new name first, which is why it is not done here.

---

## Explicitly rejected as metric-chasing

23 `funcorder`, 5 `exhaustive` (both flagged only because
`default-signifies-exhaustive` defaults to false), 4 `nonamedreturns` and the 17
`forcetypeassert` cluster in `core/events.go` and `core/extpoints/config_resolve.go`
— verified guarded by construction (`convertString` etc. return `(any, error)` and
always yield the asserted type when `err == nil`). Reordering functions or adding
unreachable `if !ok` branches would raise the score and lower the code.
