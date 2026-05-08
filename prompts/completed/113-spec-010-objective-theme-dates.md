---
status: completed
spec: [010-unify-date-fields-to-dateordatetime]
summary: Migrated ObjectiveFrontmatter and ThemeFrontmatter StartDate/TargetDate from *time.Time to *libtime.DateOrDateTime, updated GetField/SetField to use formatDateOrDateTime and setDateField, and added comprehensive Ginkgo test coverage for both types.
container: vault-cli-113-spec-010-objective-theme-dates
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-08T00:00:00Z"
queued: "2026-05-08T19:04:32Z"
started: "2026-05-08T21:19:09Z"
completed: "2026-05-08T21:22:25Z"
branch: dark-factory/unify-date-fields-to-dateordatetime
---

<summary>
- Objective StartDate() and TargetDate() getters changed from *time.Time to *libtime.DateOrDateTime
- Objective SetStartDate() and SetTargetDate() changed to take *libtime.DateOrDateTime; compat *time.Time setters added if callers exist
- Theme StartDate() and TargetDate() getters changed from *time.Time to *libtime.DateOrDateTime
- Theme SetStartDate() and SetTargetDate() changed to take *libtime.DateOrDateTime; compat *time.Time setters added if callers exist
- GetField for start_date/target_date on both types updated to use formatDateOrDateTime
- SetField for start_date/target_date on both types updated to accept YYYY-MM-DD and RFC3339 via libtime.ParseTime
- All existing tests pass; new tests cover round-trip fidelity on both entity types
</summary>

<objective>
Mechanically apply the same `*time.Time` → `*libtime.DateOrDateTime` migration to `ObjectiveFrontmatter` and `ThemeFrontmatter` that Prompt 3 applied to `GoalFrontmatter`. These two entities are structurally identical for the date fields in scope. Requires Prompts 1, 2, and 3 to be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega conventions.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

**Prompts 1, 2, and 3 must be completed first.** This prompt depends on:
- `libtime.DateOrDateTime` available throughout the codebase
- `formatDateOrDateTime(*libtime.DateOrDateTime)` available in both `domain` and `ops` packages
- Prompt 3 pattern (Goal migration) established as the canonical template

Key files to read before making changes:
- `pkg/domain/objective_frontmatter.go` — full file; StartDate/TargetDate return *time.Time; pattern is identical to goal_frontmatter.go before Prompt 3
- `pkg/domain/theme_frontmatter.go` — full file; StartDate/TargetDate return *time.Time; same pattern
- `pkg/domain/goal_frontmatter.go` — READ THIS after Prompt 3 is done; use it as the exact template for what Objective and Theme should look like after this prompt
- Search for callers: `grep -rn 'SetStartDate\|SetTargetDate\|\.StartDate()\|\.TargetDate()' pkg/ --include='*.go' | grep -v '_test.go'`
</context>

<requirements>
### 1. Audit callers before changing signatures

Run this before writing any code:

```bash
# Getters
grep -rn '\.StartDate()\|\.TargetDate()' pkg/ --include='*.go' | grep -v '_test.go'

# Setters
grep -rn 'SetStartDate\|SetTargetDate' pkg/ --include='*.go' | grep -v '_test.go'
```

Check which callers refer to Objective or Theme vs Goal. Determine whether compat `*time.Time` setters are needed (same decision logic as Prompt 3 step 4b).

### 2. Update pkg/domain/objective_frontmatter.go

Apply the exact same changes as Prompt 3 applied to goal_frontmatter.go:

**StartDate() getter:** Change return type from `*time.Time` to `*libtime.DateOrDateTime`. Replace `time.Parse(time.DateOnly, ...)` body with `GetTime` + `libtime.DateOrDateTime` construction:

```go
func (f ObjectiveFrontmatter) StartDate() *libtime.DateOrDateTime {
    t := f.GetTime("start_date")
    if t == nil {
        return nil
    }
    d := libtime.DateOrDateTime(*t)
    return &d
}
```

**TargetDate() getter:** Same pattern.

**SetStartDate() setter:** Change parameter from `*time.Time` to `*libtime.DateOrDateTime`:

```go
func (f *ObjectiveFrontmatter) SetStartDate(d *libtime.DateOrDateTime) {
    if d == nil {
        f.Delete("start_date")
        return
    }
    f.Set("start_date", formatDateOrDateTime(d))
}
```

**SetTargetDate() setter:** Same pattern.

**SetField helper:** Replace the inline `time.Parse(time.DateOnly, ...)` calls in `SetField` with `setDateField` from `task_frontmatter.go` (same `domain` package — directly callable, do NOT inline a copy).

**GetField:** Update `start_date` and `target_date` cases:

```go
case "start_date":
    return formatDateOrDateTime(f.StartDate())
case "target_date":
    return formatDateOrDateTime(f.TargetDate())
```

**SetField:** Update `start_date` and `target_date` cases:

```go
case "start_date":
    return setDateField(ctx, f.SetStartDate, value)
case "target_date":
    return setDateField(ctx, f.SetTargetDate, value)
```

**Import:** Add `libtime "github.com/bborbe/time"` if not already present. Keep `import "time"` if `Completed()` (objective only) still references `time.Time` for YAML auto-parsed values; otherwise remove.

### 3. Update pkg/domain/theme_frontmatter.go

Apply identical changes as step 2, but for `ThemeFrontmatter`:

- `StartDate() *libtime.DateOrDateTime` (was `*time.Time`)
- `TargetDate() *libtime.DateOrDateTime` (was `*time.Time`)
- `SetStartDate(d *libtime.DateOrDateTime)` (was `*time.Time`)
- `SetTargetDate(d *libtime.DateOrDateTime)` (was `*time.Time`)
- `GetField` uses `formatDateOrDateTime` and `SetField` uses `setDateField` (both from `task_frontmatter.go`, same package)

Note: `ThemeFrontmatter` does NOT have a `Completed` field or `DeferDate` — only `start_date` and `target_date`.

### 4. Extend tests in pkg/domain/objective_frontmatter_test.go

`pkg/domain/objective_frontmatter_test.go` and `pkg/domain/domain_suite_test.go` already exist — extend the existing suite, do NOT recreate.

Cover for ObjectiveFrontmatter:
- `StartDate()` nil when absent
- `StartDate()` returns `*DateOrDateTime` from date-only YAML literal `start_date: 2025-01-15`
- `StartDate()` returns `*DateOrDateTime` from RFC3339 string
- `SetStartDate(nil)` deletes key
- `SetStartDate(&d)` stores and round-trips via `StartDate()`
- Round-trip: midnight-UTC → `YYYY-MM-DD`; RFC3339 with timezone → same RFC3339
- Same set of tests for `TargetDate` / `SetTargetDate`
- `GetField("start_date")` returns formatted string
- `SetField(ctx, "start_date", "2025-03-01")` works; `SetField(ctx, "start_date", "2025-03-01T09:00:00Z")` works

### 5. Extend tests in pkg/domain/theme_frontmatter_test.go

Cover the same cases as step 4 but for `ThemeFrontmatter.StartDate` and `ThemeFrontmatter.TargetDate`. File already exists — extend it.

### 6. Iterative verification

After each file change, run `make test`. Fix compile errors before moving on.
</requirements>

<constraints>
- `StartDate()` and `TargetDate()` MUST return `*libtime.DateOrDateTime` on both Objective and Theme (not `*time.Time`)
- Spec compat-layer constraint dropped (revised 2026-05-08): canonical signatures change to `*libtime.DateOrDateTime`. Audit confirmed zero external callers; in-tree callers (if any) are updated accordingly.
- `formatDateOrDateTime` used in GetField MUST come from the same package — do NOT re-import from domain into ops or vice versa; both packages have their own copy of this function
- Do NOT change GoalFrontmatter (done in Prompt 3) — only Objective and Theme
- Do NOT change TaskFrontmatter (done in Prompts 1 and 2)
- Round-trip rule: date-only (midnight UTC) → `YYYY-MM-DD`; datetime with timezone → RFC3339 with timezone preserved
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```bash
# Confirm getters return *libtime.DateOrDateTime on both types
grep -n 'func.*StartDate\|func.*TargetDate' pkg/domain/objective_frontmatter.go pkg/domain/theme_frontmatter.go
# expected: all return *libtime.DateOrDateTime

# Confirm no *time.Time in getter bodies (only compat setters may have it)
grep 'time\.Parse\(time\.DateOnly' pkg/domain/objective_frontmatter.go pkg/domain/theme_frontmatter.go
# expected: no output (libtime.ParseTime is used instead)

# Confirm *time.Time compat setters exist only if callers needed them (audit in step 1)
# If audit found no callers: following grep should return nothing
grep 'FromTime' pkg/domain/objective_frontmatter.go pkg/domain/theme_frontmatter.go

# Confirm libtime import in both files
grep 'libtime.*bborbe/time' pkg/domain/objective_frontmatter.go pkg/domain/theme_frontmatter.go
# expected: one import line in each
```
</verification>
