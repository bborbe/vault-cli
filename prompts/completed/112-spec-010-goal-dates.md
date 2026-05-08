---
status: completed
spec: [010-unify-date-fields-to-dateordatetime]
summary: Migrated Goal start_date and target_date from *time.Time to *libtime.DateOrDateTime; setters/getters/GetField/SetField all updated; tests extended; fixed pre-existing plugin version mismatch (0.58.7 → 0.59.0) that blocked make precommit
container: vault-cli-112-spec-010-goal-dates
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-08T00:00:00Z"
queued: "2026-05-08T19:04:32Z"
started: "2026-05-08T20:27:40Z"
completed: "2026-05-08T20:30:08Z"
branch: dark-factory/unify-date-fields-to-dateordatetime
---

<summary>
- Goal StartDate() getter changed from *time.Time to *libtime.DateOrDateTime
- Goal TargetDate() getter changed from *time.Time to *libtime.DateOrDateTime
- New typed SetStartDate(*libtime.DateOrDateTime) and SetTargetDate(*libtime.DateOrDateTime) setters added
- Legacy *time.Time setters kept as compat wrappers that delegate to new typed setters
- Goal DeferDate() confirmed consistent with Task DeferDate() (already *libtime.DateOrDateTime after Prompt 1)
- GetField("start_date") and GetField("target_date") updated to format via formatDateOrDateTime
- SetField("start_date", value) and SetField("target_date", value) updated to use setDateField with *libtime.DateOrDateTime setters; now accept both YYYY-MM-DD and RFC3339 (not just YYYY-MM-DD)
- Existing compat *time.Time setters compile and pass tests
- All existing tests pass; new tests cover round-trip fidelity for both date-only and RFC3339 values
</summary>

<objective>
Migrate Goal `start_date` and `target_date` from `*time.Time` storage to `*libtime.DateOrDateTime`, enabling RFC3339 round-trip fidelity. Keep legacy `*time.Time` setters as compat wrappers to avoid breaking callers. Goal `defer_date` is already typed correctly after Prompt 1. Requires Prompts 1 and 2 to be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega conventions.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

**Prompts 1 and 2 must be completed first.** This prompt depends on:
- `libtime.DateOrDateTime` available as the project-wide date type
- `setDateField` helper in task_frontmatter.go accepting `func(*libtime.DateOrDateTime)` (changed in Prompt 1)
- `formatDateOrDateTime` accepting `*libtime.DateOrDateTime` (changed in Prompt 1)

Key files to read before making changes:
- `pkg/domain/goal_frontmatter.go` — full file; current StartDate/TargetDate return *time.Time; DeferDate already returns *libtime.DateOrDateTime after Prompt 1
- `pkg/domain/task_frontmatter.go` — reference pattern: DeferDate/SetDeferDate, setDateField, formatDateOrDateTime. Both files are in the same `package domain` — `setDateField` is callable directly from `goal_frontmatter.go`.
- Search for all callers: `grep -rn 'SetStartDate\|SetTargetDate\|\.StartDate()\|\.TargetDate()' --include='*.go' .`
- `pkg/domain/goal_frontmatter_test.go` exists — extend it.
- `github.com/bborbe/time` library: this repo is non-vendored. Use `go doc github.com/bborbe/time.DateOrDateTime`. Library type is `type DateOrDateTime stdtime.Time`; `libtime.DateOrDateTime(*t)` works directly.

**Spec note (revised 2026-05-08)**: Compat-layer constraint dropped. Audit confirms zero external callers. Canonical `StartDate()`/`SetStartDate()`/`TargetDate()`/`SetTargetDate()` signatures change to `*libtime.DateOrDateTime`. No `*FromTime` shims required.

**Precondition**: Prompt 1 must have completed. Verify with `grep -q 'libtime\.DateOrDateTime' pkg/domain/task_frontmatter.go`.
</context>

<requirements>
### 1. Audit callers before changing signatures

Run this before writing any code:

```bash
# Find all callers of Goal StartDate/TargetDate getters
grep -rn '\.StartDate()\|\.TargetDate()' pkg/ --include='*.go' | grep -v '_test.go'

# Find all callers of Goal SetStartDate/SetTargetDate setters
grep -rn 'SetStartDate\|SetTargetDate' pkg/ --include='*.go' | grep -v '_test.go'
```

Review the output. Callers that pass `*time.Time` to the setters will use the compat layer added in step 3. Callers that consume `*time.Time` from the getters must be updated or can rely on the compat getter (step 2b).

### 2. Update pkg/domain/goal_frontmatter.go — StartDate getter

#### 2a. Change StartDate() to return *libtime.DateOrDateTime

Replace the current implementation with the DeferDate pattern:

```go
// StartDate reads "start_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f GoalFrontmatter) StartDate() *libtime.DateOrDateTime {
    t := f.GetTime("start_date")
    if t == nil {
        return nil
    }
    d := libtime.DateOrDateTime(*t)
    return &d
}
```

Remove the old implementation that returned `*time.Time`.

#### 2b. Add StartDateAsTime() compat getter (only if callers consume *time.Time)

If step 1 audit shows callers that consume `*time.Time` from `StartDate()`, add a compat getter:

```go
// StartDateAsTime returns StartDate as *time.Time for backward compatibility.
// New callers should use StartDate() which returns *libtime.DateOrDateTime.
func (f GoalFrontmatter) StartDateAsTime() *time.Time {
    d := f.StartDate()
    if d == nil {
        return nil
    }
    t := d.Time()
    return &t
}
```

If no callers consume the old `*time.Time` return, skip this compat getter.

### 3. Update pkg/domain/goal_frontmatter.go — TargetDate getter

Same pattern as StartDate (steps 2a/2b):

```go
// TargetDate reads "target_date" as *libtime.DateOrDateTime.
func (f GoalFrontmatter) TargetDate() *libtime.DateOrDateTime {
    t := f.GetTime("target_date")
    if t == nil {
        return nil
    }
    d := libtime.DateOrDateTime(*t)
    return &d
}
```

### 4. Update pkg/domain/goal_frontmatter.go — SetStartDate setter

#### 4a. Add new typed setter

```go
// SetStartDate stores the start_date in the map. Deletes key if d is nil.
func (f *GoalFrontmatter) SetStartDate(d *libtime.DateOrDateTime) {
    if d == nil {
        f.Delete("start_date")
        return
    }
    f.Set("start_date", formatDateOrDateTime(d))
}
```

This replaces the old `SetStartDate(t *time.Time)`. The signature changes from `*time.Time` to `*libtime.DateOrDateTime`. Update any in-tree callers found in step 1 audit to pass `*libtime.DateOrDateTime`.

### 5. Update pkg/domain/goal_frontmatter.go — SetTargetDate setter

Same pattern as SetStartDate.

### 6. Update pkg/domain/goal_frontmatter.go — SetField helper

`setDateField` in `task_frontmatter.go` is in the **same `domain` package** and is callable directly from `goal_frontmatter.go` — do NOT duplicate it. Use it directly:

```go
case "start_date":
    return setDateField(ctx, f.SetStartDate, value)
case "target_date":
    return setDateField(ctx, f.SetTargetDate, value)
```

Remove `setDateFromString` (the `time.Parse(time.DateOnly, ...)` helper) once no caller remains.

### 7. Update GetField in GoalFrontmatter

Change `start_date` and `target_date` cases:

```go
case "start_date":
    return formatDateOrDateTime(f.StartDate())
case "target_date":
    return formatDateOrDateTime(f.TargetDate())
```

Remove the old `t.UTC().Format(time.DateOnly)` formatting.

### 8. Update SetField in GoalFrontmatter

Already covered in step 6.

### 9. Import cleanup in goal_frontmatter.go

- `import "time"` — keep if used by `formatTimeAsDate` or other paths; otherwise remove.
- `libtime "github.com/bborbe/time"` — already imported.

### 10. Extend tests in pkg/domain/goal_frontmatter_test.go

`pkg/domain/goal_frontmatter_test.go` and `pkg/domain/domain_suite_test.go` already exist — extend the existing suite, do NOT recreate.

Cover:
- `StartDate()` returns nil when key absent
- `StartDate()` returns non-nil *DateOrDateTime when `start_date: 2025-01-15` (YAML date literal)
- `StartDate()` returns non-nil *DateOrDateTime when `start_date: "2025-01-15T14:30:00+01:00"` (RFC3339 string)
- `SetStartDate(nil)` deletes the key
- `SetStartDate(&d)` stores formatted value; subsequent `StartDate()` retrieves it
- Round-trip: date-only value formats back as `YYYY-MM-DD` (midnight UTC → date-only)
- Round-trip: RFC3339 value preserves timezone in formatted output
- Same tests for `TargetDate` / `SetTargetDate`
- `DeferDate()` still works (regression: was already *libtime.DateOrDateTime after Prompt 1)
- `GetField("start_date")` returns formatted string
- `SetField(ctx, "start_date", "2025-01-15")` round-trips via GetField

Use Ginkgo/Gomega style matching the rest of the domain test suite.

### 11. Iterative verification

After each method change, run `make test` to catch compile errors immediately.
</requirements>

<constraints>
- `StartDate()` and `TargetDate()` getters MUST return `*libtime.DateOrDateTime` (not `*time.Time`)
- Audit (step 1) and update any in-tree caller that previously called `*time.Time`-typed Goal accessors. Spec compat-layer constraint dropped (revised 2026-05-08); no `*FromTime` shims required.
- `DeferDate()` / `SetDeferDate()` must NOT be touched — they were already migrated in Prompt 1
- `formatDateOrDateTime` and `GetTime` are shared helpers; do NOT redefine them in goal_frontmatter.go — use as-is from their respective packages
- Round-trip rule: midnight-UTC values format as `YYYY-MM-DD`; values with time components format as RFC3339 with timezone — this is libtime.DateOrDateTime's public contract
- `import "time"` removal is optional — only remove if zero remaining usages after changes
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```bash
# Confirm StartDate and TargetDate return *libtime.DateOrDateTime
grep -n 'func.*StartDate\|func.*TargetDate' pkg/domain/goal_frontmatter.go
# expected: both return *libtime.DateOrDateTime

# Confirm no *time.Time storage in getters
grep -A 10 'func.*GoalFrontmatter.*StartDate()' pkg/domain/goal_frontmatter.go
# expected: uses GetTime() and DateOrDateTime, not time.Parse(time.DateOnly)

# Confirm SetField uses setDateField (the same helper task_frontmatter.go uses)
grep 'setDateField\|setDateFromString' pkg/domain/goal_frontmatter.go
# expected: setDateField in SetField for start/target dates; no setDateFromString remaining

# Confirm no *time.Time on canonical accessors
grep -E 'StartDate\(|TargetDate\(|SetStartDate|SetTargetDate' pkg/domain/goal_frontmatter.go | grep '\*time\.Time'
# expected: no output
```
</verification>
