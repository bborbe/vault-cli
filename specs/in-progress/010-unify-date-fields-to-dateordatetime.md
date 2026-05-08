---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-05-08T17:52:30Z"
generating: "2026-05-08T18:03:07Z"
prompted: "2026-05-08T18:13:30Z"
verifying: "2026-05-08T21:26:20Z"
branch: dark-factory/unify-date-fields-to-dateordatetime
---

## Summary

- vault-cli stores `*_date` frontmatter values in three inconsistent ways: `*DateOrDateTime` (good), `*time.Time` (lossy, date-only), and plain `string` (raw).
- Same field name (`defer_date`) has different types across entity domains — Task uses the polymorphic type, Goal uses `*time.Time`.
- Goal: every `*_date` frontmatter field across Task, Goal, Objective, Theme, Decision uses `*DateOrDateTime` so producers (agent task-controller, OpenClaw, humans) have one mental model: "any date field accepts `YYYY-MM-DD` or RFC3339".
- The `DateOrDateTime` type lives in **`github.com/bborbe/time` v1.27.0** (alongside existing `Date` and `DateTime`, with the full peer API surface including `Clone`/`ClonePtr`, `AsDate`/`AsDateTime`/`IsDateOnly`). vault-cli imports the library type rather than maintaining its own copy. **Dependency satisfied — ready to execute.**
- Migration is read-safe: existing date-only YAML round-trips to date-only; producers wanting timestamps gain that capability without rewriting old files.
- All open questions resolved; libtime dependency satisfied. Ready for approval.

## Problem

The codebase has accreted three storage patterns for date-shaped frontmatter:

1. `*DateOrDateTime` — Task's `defer_date`, `planned_date`, `due_date`. Accepts both `YYYY-MM-DD` and RFC3339 with timezone. Round-trip preserves the input form.
2. `*time.Time` formatted as `time.DateOnly` — Goal/Objective/Theme `start_date`, `target_date`, plus Goal `defer_date`. Truncates time-of-day on write.
3. Plain `string` — Task `completed_date`, `last_completed`; Decision `reviewed_date`. Raw, no validation.

Consequences:

- The same field name (`defer_date`) has different types in Goal vs Task. A producer writing `defer_date` to either entity must remember which form is expected.
- Producers that need real timestamps (agent task-controller wants RFC3339 `created_date`) cannot use `*time.Time`-as-date-only fields without losing precision.
- Three code paths for parsing and formatting dates means three places where bugs hide (timezone handling, midnight-UTC boundary, YAML auto-parsing of date literals).

## Goal

After this work:

- Every `*_date` frontmatter field across Task, Goal, Objective, Theme, Decision uses `*DateOrDateTime` (or is consciously excluded with a documented reason).
- Reading existing vault files (with date-only YAML literals) continues to work unchanged.
- Writing back a value that was read as date-only emits date-only — no churn in existing files.
- Producers can supply RFC3339 timestamps for any date field and have them round-trip faithfully.
- The `defer_date` semantics are identical across Goal and Task.

## Non-goals

- Renaming any frontmatter field.
- Schema versioning or a frontmatter-format migration that rewrites existing vault files in place.
- Changing CLI command surfaces (subcommands, flag names).
- Introducing new date fields beyond Task `created_date` (which is in scope here — see Desired Behavior #1).
- Maintaining `*time.Time`-based getter/setter signatures. **Decision (revised 2026-05-08): signatures may change.** A pre-execution audit confirmed zero external callers of `StartDate()`/`SetStartDate()`/`TargetDate()`/`SetTargetDate()` outside `pkg/domain/`. The compat-layer constraint was over-conservative; canonical accessors switch to `*libtime.DateOrDateTime`. Optional renamed `*FromTime` shims may be added if any in-tree caller cannot trivially migrate, but no spec contract requires them.
- Permanently keeping the legacy `last_completed` frontmatter key. **In-spec rename to `last_completed_date`** with a one-release dual-write window (read either key, write both `last_completed` AND `last_completed_date`); old key is dropped in a follow-up release.

## Desired Behavior

1. Task `created_date` (new field), `completed_date`, and `last_completed_date` (renamed from `last_completed`) accept and round-trip both date-only and RFC3339 values via `*DateOrDateTime`. For `last_completed_date`, reads accept either the new or legacy key; writes emit both keys for the duration of the dual-write window. `created_date` is set by the agent task-controller on task creation (RFC3339 timestamp).
2. Goal `start_date`, `target_date`, `defer_date` accept and round-trip both forms.
3. Objective `start_date`, `target_date` accept and round-trip both forms.
4. Theme `start_date`, `target_date` accept and round-trip both forms.
5. Decision `reviewed_date` accepts and round-trips both forms.
6. A vault file authored with date-only YAML literals (`start_date: 2025-01-15`) reads cleanly and writes back as `2025-01-15`, not `2025-01-15T00:00:00Z`.
7. A producer writing an RFC3339 timestamp (`2025-01-15T14:30:00+01:00`) reads back the same string with timezone preserved.
8. The `defer_date` getter/setter signatures match between Task and Goal.
9. Canonical `StartDate()`/`TargetDate()` etc. on Goal/Objective/Theme switch to `*libtime.DateOrDateTime`. No requirement to preserve `*time.Time` signatures (audit confirmed zero external callers). All in-tree call sites are updated as part of the relevant prompt.

## Constraints

- **Dependency satisfied**: `github.com/bborbe/time` v1.27.0 exports `DateOrDateTime` with the full peer API. Bumping the `bborbe/time` go.mod version + deleting vault-cli's local `pkg/domain/date_or_datetime.go` is the first prompt of this migration.
- Existing vault files must not break on read. `DateOrDateTime` accepts both `time.Time` (YAML-auto-parsed) and string forms — this is the load-bearing primitive.
- Existing tests must continue to pass. Tests that assert `*time.Time` return types may need to switch to `*DateOrDateTime`, but the asserted behavior (parsed value, round-trip output) must remain.
- Round-trip rule: midnight-UTC values format as `YYYY-MM-DD`, all others as RFC3339. This is the public contract on `libtime.DateOrDateTime` — vault-cli relies on it but does not own it.
- See `pkg/domain/task_frontmatter.go` `DeferDate` / `SetDeferDate` and the `setDateField` / `formatDateOrDateTime` helpers as the reference pattern (post-migration `formatDateOrDateTime` becomes a thin wrapper around `libtime.DateOrDateTime.MarshalText` or is dropped if no longer needed).
- See `docs/development-patterns.md` for established repo conventions any implementation prompt should follow.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Vault file has `start_date: 2025-01-15` (YAML date literal) | Read as `*DateOrDateTime`, formats back as `2025-01-15` | None needed |
| Vault file has `start_date: "2025-01-15T14:30:00+01:00"` (RFC3339 string) | Read with timezone preserved, round-trips identically | None needed |
| Caller previously passed non-midnight `*time.Time` to `SetStartDate` | Audit confirmed zero such callers. Canonical setter signature switches to `*libtime.DateOrDateTime`; in-tree callers updated. | None — no external callers |
| Producer writes garbage string to `defer_date` | Parse error surfaces at read time with field name and offending value | Fix producer |
| Two date fields on the same entity disagree on form (one date-only, one RFC3339) | Both round-trip correctly in their own form | None needed |

## Security / Abuse Cases

Not applicable — this is internal data-model refactoring. No new HTTP, file, or user-input surface. Existing YAML parsing already validates date strings.

## Acceptance Criteria

- [x] `github.com/bborbe/time` exports `DateOrDateTime` — shipped in v1.27.0.
- [ ] vault-cli go.mod is bumped to `bborbe/time@v1.27.0` (or later) and `pkg/domain/date_or_datetime.go` is deleted in favour of `libtime.DateOrDateTime`. All references in `pkg/domain/*_frontmatter.go` switch to the library type.
- [ ] Migration is split into the 5 sequential prompts listed under `## Resolved Decisions` → "Migration ordering".
- [ ] Follow-up issue/spec opened to drop the legacy `last_completed` write after one release cycle. (Closes the dual-write window.)
- [ ] `grep -rn '\*time.Time' pkg/domain/*_frontmatter.go` returns no canonical date-field accessors (only places where `time.Time` appears via YAML auto-parsing helpers; no `Start/Target/Defer/Planned/Due/Reviewed` accessor returns or accepts `*time.Time`).
- [ ] Task `created_date` (new), `completed_date`, and `last_completed_date` use `*DateOrDateTime`. `last_completed_date` is the canonical key; legacy `last_completed` is read as a fallback and dual-written for one release cycle.
- [ ] Goal `start_date`, `target_date`, `defer_date` use `*DateOrDateTime`.
- [ ] Objective `start_date`, `target_date` use `*DateOrDateTime`.
- [ ] Theme `start_date`, `target_date` use `*DateOrDateTime`.
- [ ] Decision `reviewed_date` uses `*DateOrDateTime`.
- [ ] Cross-domain `defer_date` has identical type and behavior on Goal and Task.
- [ ] All in-tree call sites compile against the new `*libtime.DateOrDateTime` signatures. (Compat-layer constraint dropped — audit confirmed zero external callers.)
- [ ] A round-trip test loads a fixture with mixed date-only and RFC3339 values across all entity types, performs get/set via the CLI, and asserts no semantic loss and no form churn.
- [ ] All existing tests pass.

No scenario test required — round-trip coverage fits as a unit/integration test on the frontmatter layer.

## Verification

```
make precommit
```

## Resolved Decisions

- **Dual-write release count**: ONE release cycle. Read both `last_completed` and `last_completed_date`; write both. Drop legacy `last_completed` write in the next release. External consumers (recurring-task automation) get one release window to update.
- **Test fixture strategy**: in-tree fixtures under `pkg/storage/testdata/` covering each entity type with mixed date-only and RFC3339 values. Ephemeral fixtures rejected — round-trip tests benefit from human-reviewable expected output.
- **Migration ordering** (5 prompts, sequential):
  1. Bump go.mod to `bborbe/time@v1.27.0`, delete `pkg/domain/date_or_datetime.go`, retarget existing Task `DeferDate`/`PlannedDate`/`DueDate` accessors at `libtime.DateOrDateTime`.
  2. Task migration: add `created_date`, migrate `completed_date`, rename `last_completed` → `last_completed_date` with dual-write.
  3. Goal `start_date` / `target_date` / `defer_date` migration. `defer_date` cross-domain consistency falls out here.
  4. Objective + Theme `start_date` / `target_date` migration (mechanical, parallel to Goal).
  5. Decision `reviewed_date` migration.
- **Domain-knowledge doc**: rely on libtime's `DateOrDateTime` GoDoc as the canonical reference for the round-trip contract. vault-cli adds a one-paragraph note in `docs/development-patterns.md` listing which entity fields use `*DateOrDateTime` and linking to the library type. No new `docs/date-fields.md`.

## Do-Nothing Option

Tolerable but degrading. Producers (notably agent task-controller) increasingly want RFC3339 timestamps; each new field added without unification widens the inconsistency. The cross-domain `defer_date` collision is already a footgun. Doing nothing means every new producer integration re-discovers the three-pattern problem. Acceptable to defer this spec while higher-priority work ships, but not acceptable as a permanent state.

## Verification Result

**Verified:** 2026-05-08T21:50:10Z (HEAD a7f54c9)
**Binary:** /Users/bborbe/Documents/workspaces/go/bin/dark-factory (v0.156.1-1-g04f3863-dirty)
**Scenario:** No scenario file — verification is `make precommit` (unit/integration round-trip coverage). Re-verification covered the previously-missing follow-up artifact AC.
**Evidence:**
- Follow-up spec present at `specs/ideas/drop-legacy-last-completed-key.md` (status: `idea`); scope: drop dual-write of legacy `last_completed`, retain read-fallback for one more release; explicitly cross-references parent spec 010.
- All other ACs verified in prior run (libtime v1.27.0 bump, deletion of local `date_or_datetime.go`, 5-prompt sequential migration, `*DateOrDateTime` adoption across Task/Goal/Objective/Theme/Decision, cross-domain `defer_date` parity, `make precommit` green).
**Verdict:** PASS
