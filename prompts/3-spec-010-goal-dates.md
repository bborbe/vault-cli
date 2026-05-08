---
spec: ["010"]
status: draft
created: "2026-05-08T00:00:00Z"
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
- `pkg/domain/task_frontmatter.go` — reference pattern: DeferDate/SetDeferDate, setDateField, formatDateOrDateTime
- `pkg/ops/goal_complete.go` — check if it calls SetStartDate/SetTargetDate with *time.Time
- Search for all callers: `grep -rn 'SetStartDate\|SetTargetDate\|\.StartDate()\|\.TargetDate()' pkg/ --include='*.go' | grep -i goal`
- `pkg/domain/goal_frontmatter_test.go` (if exists) — existing test patterns
- `vendor/github.com/bborbe/time/` — confirm libtime.DateOrDateTime construction from time.Time
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

This replaces the old `SetStartDate(t *time.Time)`. The signature changes from `*time.Time` to `*libtime.DateOrDateTime`.

#### 4b. Add SetStartDateFromTime() compat setter (if callers pass *time.Time)

Per spec Non-goals: "Removing the `*time.Time`-based getter/setter API. Decision: keep as compatibility layer."
Add a compat setter so existing callers compile unchanged:

```go
// SetStartDateFromTime stores start_date from a *time.Time.
// Kept for backward compatibility. New callers should use SetStartDate(*libtime.DateOrDateTime).
func (f *GoalFrontmatter) SetStartDateFromTime(t *time.Time) {
    if t == nil {
        f.Delete("start_date")
        return
    }
    f.Set("start_date", t.UTC().Format(time.DateOnly))
}
```

**Important:** If any existing caller called the OLD `SetStartDate(t *time.Time)`, it now calls a method that doesn't exist (signature changed). Options:
1. Rename the compat version to the OLD name `SetStartDate` and give the new typed version a new name — but this is counter-intuitive
2. OR: Update existing `*time.Time` callers to use `SetStartDateFromTime`

Prefer option 2: update callers (found in step 1 audit) to call `SetStartDateFromTime`. If the audit found no callers, skip the compat setter entirely.

### 5. Update pkg/domain/goal_frontmatter.go — SetTargetDate setter

Same pattern as SetStartDate (steps 4a/4b).

### 6. Update pkg/domain/goal_frontmatter.go — setDateFromString helper

The existing `setDateFromString` helper parses only `YYYY-MM-DD` via `time.Parse(time.DateOnly, value)`. After migration, `start_date` and `target_date` accept both date-only and RFC3339. Replace the `*time.Time` setter with a `*libtime.DateOrDateTime` setter using `setDateField` from task_frontmatter.go (which is not directly callable across packages — the equivalent logic must be inlined or a package-local helper added).

Inline the equivalent:

```go
func setGoalDateField(
    ctx context.Context,
    setter func(*libtime.DateOrDateTime),
    value string,
) error {
    if value == "" {
        setter(nil)
        return nil
    }
    t, err := libtime.ParseTime(ctx, value)
    if err != nil {
        return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD or RFC3339)")
    }
    d := libtime.DateOrDateTime(*t)
    setter(&d)
    return nil
}
```

This replaces `setDateFromString` which accepted only `YYYY-MM-DD`. If `setDateFromString` is used elsewhere in `goal_frontmatter.go` for the `*time.Time` compat setter, keep it for that case.

### 7. Update GetField in GoalFrontmatter

Change `start_date` and `target_date` cases:

```go
case "start_date":
    return formatDateOrDateTime(f.StartDate())
case "target_date":
    return formatDateOrDateTime(f.TargetDate())
```

Remove the old `t.UTC().Format(time.DateOnly)` formatting (the `formatDateOrDateTime` function preserves date-only as `YYYY-MM-DD` for midnight-UTC values — same output for existing date-only files).

### 8. Update SetField in GoalFrontmatter

Change `start_date` and `target_date` cases to use `setGoalDateField`:

```go
case "start_date":
    return setGoalDateField(ctx, f.SetStartDate, value)
case "target_date":
    return setGoalDateField(ctx, f.SetTargetDate, value)
```

Remove or rename the old `setDateFromString` method if no longer needed.

### 9. Import cleanup in goal_frontmatter.go

- `import "time"` — keep if used by `formatTimeAsDate` call or compat setter body
- `libtime "github.com/bborbe/time"` — already imported; confirm correct alias

### 10. Write tests in pkg/domain/goal_frontmatter_test.go (create if absent)

If `pkg/domain/goal_frontmatter_test.go` does not exist, create it with the domain test suite bootstrap. Check whether `pkg/domain/domain_suite_test.go` already exists — if so, do NOT recreate it; just add the test file for GoalFrontmatter.

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
- Existing `*time.Time` setter callers must NOT be silently broken — audit (step 1) and either update callers or add compat setters with renamed method names
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

# Confirm SetField uses setGoalDateField (not setDateFromString with time.Parse)
grep 'setGoalDateField\|ParseTime' pkg/domain/goal_frontmatter.go
# expected: setGoalDateField or libtime.ParseTime in SetField for start/target dates

# Confirm *time.Time grep on getters returns nothing (only compat setters may have it)
grep '\*time\.Time' pkg/domain/goal_frontmatter.go
# expected: only in compat setter methods (SetStartDateFromTime etc.) if kept, not in getters
```
</verification>
