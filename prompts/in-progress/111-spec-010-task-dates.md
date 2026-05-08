---
status: committing
spec: [010-unify-date-fields-to-dateordatetime]
summary: Migrated Task completed_date and last_completed/last_completed_date from plain string storage to *libtime.DateOrDateTime, added created_date field, implemented dual-write for last_completed legacy key, updated ops/complete.go, show.go, list.go, and frontmatter_entity.go accordingly, and fixed pre-existing plugin version mismatch (0.58.6 → 0.58.7).
container: vault-cli-111-spec-010-task-dates
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-08T00:00:00Z"
queued: "2026-05-08T19:04:32Z"
started: "2026-05-08T19:11:11Z"
branch: dark-factory/unify-date-fields-to-dateordatetime
---

<summary>
- Task completed_date getter returns *libtime.DateOrDateTime (was string); setter takes *libtime.DateOrDateTime
- last_completed renamed to last_completed_date: new getter reads last_completed_date first, falls back to last_completed legacy key; new setter writes both keys (dual-write window)
- Legacy LastCompleted() and SetLastCompleted() kept as compat wrappers backed by new typed primitives
- New created_date field added to Task frontmatter: getter and setter using *libtime.DateOrDateTime
- GetField/SetField in TaskFrontmatter updated for completed_date, last_completed_date, created_date, and last_completed compat key
- ops/complete.go updated to call new typed setters for completed_date and last_completed_date
- ops/show.go and ops/list.go updated to format *libtime.DateOrDateTime for JSON string output
- ops/frontmatter_entity.go updated to include last_completed_date and created_date in allowed fields
- All existing tests pass; new tests cover round-trip and dual-write behavior
</summary>

<objective>
Migrate Task `completed_date` and `last_completed`/`last_completed_date` from plain string storage to `*libtime.DateOrDateTime`, and add a new `created_date` field. Producers gain RFC3339 round-trip fidelity for all three fields. The dual-write window for `last_completed` → `last_completed_date` allows external consumers one release to adapt. Requires Prompt 1 (libtime upgrade) to be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega conventions.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

**Prompt 1 must be completed first.** This prompt depends on:
- `domain.DateOrDateTime` replaced by `libtime.DateOrDateTime` throughout the codebase
- `pkg/domain/date_or_datetime.go` deleted
- `github.com/bborbe/time@v1.27.0` in go.mod

Key files to read before making changes:
- `pkg/domain/task_frontmatter.go` — full file; reference pattern is DeferDate/SetDeferDate (already *libtime.DateOrDateTime after Prompt 1), and helpers setDateField/formatDateOrDateTime
- `pkg/ops/complete.go` — calls SetCompletedDate and SetLastCompleted; need to understand how it sources the timestamp from currentDateTime
- `pkg/ops/show.go` — TaskDetail struct has CompletedDate string; set from task.CompletedDate()
- `pkg/ops/list.go` — TaskItem struct has CompletedDate string; set from task.CompletedDate()
- `pkg/ops/frontmatter_entity.go` — `knownTaskScalarFields` map (around line 547)
- `pkg/domain/task_frontmatter_test.go` — existing test patterns
- `github.com/bborbe/time` library: this repo is non-vendored. Use `go doc github.com/bborbe/time.DateOrDateTime` or read from `$(go env GOMODCACHE)/github.com/bborbe/time@v1.27.0/`. Library type is `type DateOrDateTime stdtime.Time`; construction `libtime.DateOrDateTime(*t)` from `*time.Time` works directly. `.Time() time.Time` accessor exists.

**Precondition**: Prompt 1 must have completed. Verify with `grep -q 'libtime\.DateOrDateTime' pkg/domain/task_frontmatter.go` — if no match, stop and run Prompt 1 first.
</context>

<requirements>
### 1. Confirm conversion idiom (read existing post-Prompt-1 code)

Read `pkg/domain/task_frontmatter.go` after Prompt 1 has shipped. The existing `DeferDate()` body is the reference: it converts `*time.Time` → `*libtime.DateOrDateTime` via `d := libtime.DateOrDateTime(*t)`. Use the same idiom in steps 4 and 5.

For `c.currentDateTime.Now()` (returns `libtime.DateTime`), get the underlying `time.Time` via `.Time()`, then construct the same way: `libtime.DateOrDateTime(c.currentDateTime.Now().Time())`.

### 2. Update pkg/domain/task_frontmatter.go — completed_date field

#### 2a. Change CompletedDate() getter

Replace the string-returning getter with a typed getter (follow the DeferDate pattern exactly):

```go
// CompletedDate reads "completed_date" as *libtime.DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) CompletedDate() *libtime.DateOrDateTime {
    t := f.GetTime("completed_date")
    if t == nil {
        return nil
    }
    // Use same construction as DeferDate() — verified in Prompt 1
    d := libtime.DateOrDateTime(*t)
    return &d
}
```

#### 2b. Change SetCompletedDate() setter

Replace the string-taking setter with a typed setter (follow SetDeferDate pattern):

```go
// SetCompletedDate stores the completed_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetCompletedDate(d *libtime.DateOrDateTime) {
    if d == nil {
        f.Delete("completed_date")
        return
    }
    f.Set("completed_date", formatDateOrDateTime(d))
}
```

#### 2c. Update GetField("completed_date") case

```go
case "completed_date":
    return formatDateOrDateTime(f.CompletedDate())
```

#### 2d. Update SetField("completed_date", value) case

```go
case "completed_date":
    return setDateField(ctx, f.SetCompletedDate, value)
```

### 3. Update pkg/domain/task_frontmatter.go — last_completed / last_completed_date

#### 3a. Add LastCompletedDate() getter (new canonical getter)

Reads `last_completed_date` first; falls back to legacy `last_completed` key:

```go
// LastCompletedDate reads "last_completed_date" as *libtime.DateOrDateTime.
// Falls back to the legacy "last_completed" key for backward compatibility.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) LastCompletedDate() *libtime.DateOrDateTime {
    // Prefer canonical key
    if t := f.GetTime("last_completed_date"); t != nil {
        d := libtime.DateOrDateTime(*t)
        return &d
    }
    // Legacy key fallback
    if t := f.GetTime("last_completed"); t != nil {
        d := libtime.DateOrDateTime(*t)
        return &d
    }
    return nil
}
```

#### 3b. Keep LastCompleted() as a compat string getter (backed by new typed getter)

Replace the old implementation so it delegates to LastCompletedDate():

```go
// LastCompleted reads "last_completed" (legacy) or "last_completed_date" (canonical)
// as a formatted date string. Kept for backward compatibility.
func (f TaskFrontmatter) LastCompleted() string {
    return formatDateOrDateTime(f.LastCompletedDate())
}
```

#### 3c. Add SetLastCompletedDate() setter (new canonical setter — dual-write)

Writes both `last_completed_date` AND `last_completed` during the dual-write window:

```go
// SetLastCompletedDate stores the last_completed_date in the map.
// Dual-writes to both "last_completed_date" (canonical) and "last_completed" (legacy)
// for one release cycle to allow external consumers to migrate.
// Deletes both keys if d is nil.
func (f *TaskFrontmatter) SetLastCompletedDate(d *libtime.DateOrDateTime) {
    if d == nil {
        f.Delete("last_completed_date")
        f.Delete("last_completed")
        return
    }
    formatted := formatDateOrDateTime(d)
    f.Set("last_completed_date", formatted)
    f.Set("last_completed", formatted)  // dual-write window
}
```

#### 3d. Keep SetLastCompleted() as a compat string setter (delegates to SetLastCompletedDate)

```go
// SetLastCompleted stores the last_completed value. Kept for backward compatibility.
// Delegates to SetLastCompletedDate for dual-write behavior.
func (f *TaskFrontmatter) SetLastCompleted(v string) {
    if v == "" {
        f.SetLastCompletedDate(nil)
        return
    }
    // Parse string to *libtime.DateOrDateTime, then delegate
    ctx := context.Background()
    t, err := libtime.ParseTime(ctx, v)
    if err != nil {
        // Fallback: store raw string if unparseable (preserves old behavior)
        f.Set("last_completed", v)
        f.Set("last_completed_date", v)
        return
    }
    d := libtime.DateOrDateTime(*t)
    f.SetLastCompletedDate(&d)
}
```

Add `"context"` import if not already present (it is — used in other methods).

#### 3e. Update GetField for last_completed_date (add new case) and last_completed (compat case)

```go
case "last_completed_date":
    return formatDateOrDateTime(f.LastCompletedDate())
case "last_completed":
    return f.LastCompleted()  // compat — returns same value, reads canonical or legacy key
```

#### 3f. Update SetField for last_completed_date (add new case) and last_completed (compat)

```go
case "last_completed_date":
    return setDateField(ctx, f.SetLastCompletedDate, value)
case "last_completed":
    f.SetLastCompleted(value)  // compat — dual-writes via SetLastCompletedDate internally
```

### 4. Update pkg/domain/task_frontmatter.go — created_date (new field)

#### 4a. Add CreatedDate() getter

```go
// CreatedDate reads "created_date" as *libtime.DateOrDateTime.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) CreatedDate() *libtime.DateOrDateTime {
    t := f.GetTime("created_date")
    if t == nil {
        return nil
    }
    d := libtime.DateOrDateTime(*t)
    return &d
}
```

#### 4b. Add SetCreatedDate() setter

```go
// SetCreatedDate stores the created_date in the map. Deletes the key if d is nil.
func (f *TaskFrontmatter) SetCreatedDate(d *libtime.DateOrDateTime) {
    if d == nil {
        f.Delete("created_date")
        return
    }
    f.Set("created_date", formatDateOrDateTime(d))
}
```

#### 4c. Add GetField case for created_date

```go
case "created_date":
    return formatDateOrDateTime(f.CreatedDate())
```

#### 4d. Add SetField case for created_date

```go
case "created_date":
    return setDateField(ctx, f.SetCreatedDate, value)
```

### 5. Update pkg/ops/complete.go

**completed_date:** Change the call to `SetCompletedDate`. The old code passed a formatted string. Now pass a `*libtime.DateOrDateTime` built from the current time:

```go
// Old:
task.SetCompletedDate(c.currentDateTime.Now().Time().UTC().Format("2006-01-02T15:04:05Z"))

// New (verify exact construction via step 1 grep):
now := c.currentDateTime.Now()
t := now.Time()
d := libtime.DateOrDateTime(t)  // or use verified constructor
task.SetCompletedDate(&d)
```

**last_completed:** The old code built a `today` string and called `SetLastCompleted(today)`. Update to use the new canonical setter:

```go
// Old:
today := c.currentDateTime.Now().Time().UTC().Format(time.DateOnly)
task.SetLastCompleted(today)

// New: build *libtime.DateOrDateTime for midnight-UTC (date-only)
t := libtime.ToDate(c.currentDateTime.Now().Time()).Time()  // midnight UTC
d := libtime.DateOrDateTime(t)
task.SetLastCompletedDate(&d)
```

`libtime.ToDate(t).Time()` normalizes to midnight UTC, ensuring the value serializes as `YYYY-MM-DD`. Confirm `libtime.ToDate` exists: `go doc github.com/bborbe/time.ToDate`.

### 6. Update pkg/ops/show.go and pkg/ops/list.go

Both have a `CompletedDate string` field in their response struct that was assigned directly from `task.CompletedDate()`. Now `task.CompletedDate()` returns `*libtime.DateOrDateTime`. Use `formatDateOrDateTime` to format it as a string for JSON output.

In `pkg/ops/show.go`:
```go
// Old:
detail.CompletedDate = task.CompletedDate()

// New:
detail.CompletedDate = formatDateOrDateTime(task.CompletedDate())
```

In `pkg/ops/list.go`:
```go
// Old:
items[i].CompletedDate = task.CompletedDate()

// New:
items[i].CompletedDate = formatDateOrDateTime(task.CompletedDate())
```

Both files should already have access to `formatDateOrDateTime` (it's in the same `ops` package, defined in `frontmatter.go`).

### 7. Update pkg/ops/frontmatter_entity.go — allowed fields

Locate the `knownTaskScalarFields` map (around line 547). Add the new and renamed keys:

```go
"last_completed_date": true,
"created_date": true,
```

Keep `"last_completed": true` for the dual-write window.

### 8. Write tests in pkg/domain/task_frontmatter_test.go

Add a `Describe` block (or extend existing) covering:

**completed_date:**
- `CompletedDate()` returns nil when key absent
- `CompletedDate()` returns non-nil *DateOrDateTime when date-only YAML value present
- `CompletedDate()` returns non-nil *DateOrDateTime when RFC3339 string present
- `SetCompletedDate(nil)` deletes the key
- `SetCompletedDate(&d)` stores the formatted value; subsequent `CompletedDate()` retrieves it
- Round-trip: date-only value writes as `YYYY-MM-DD`, RFC3339 value preserves timezone

**last_completed / last_completed_date:**
- `LastCompletedDate()` returns nil when both keys absent
- `LastCompletedDate()` reads `last_completed_date` when present
- `LastCompletedDate()` falls back to `last_completed` when only legacy key present
- `LastCompletedDate()` prefers `last_completed_date` when both keys present
- `SetLastCompletedDate(&d)` writes both `last_completed_date` AND `last_completed`
- `SetLastCompletedDate(nil)` deletes both keys
- `SetLastCompleted("2025-01-15")` dual-writes both keys
- `LastCompleted()` returns formatted string (compat getter)

**created_date:**
- `CreatedDate()` returns nil when key absent
- `CreatedDate()` round-trips date-only and RFC3339 values
- `SetCreatedDate(nil)` deletes the key

Use the same Ginkgo/Gomega pattern as the existing test file (BeforeEach, It, Expect with Gomega matchers).

### 9. Iterative verification

After each section of changes, run `make test` to catch issues early. Fix errors before moving to the next section.
</requirements>

<constraints>
- `completed_date` getter must return `*libtime.DateOrDateTime` (not string) — callers in ops/show.go and ops/list.go must use formatDateOrDateTime to convert for JSON
- Dual-write for `last_completed` is mandatory for this release cycle — SetLastCompletedDate must write BOTH `last_completed_date` AND `last_completed`
- The dual-write period ends in the NEXT release — a follow-up spec/issue must be opened (document in `## Unreleased` changelog entry or code comment)
- `created_date` field is only a getter/setter — vault-cli does NOT auto-populate it on task creation; the agent task-controller sets it externally
- `SetLastCompleted(string)` compat setter MUST be kept — it is called from ops/complete.go and possibly tests
- `LastCompleted() string` compat getter MUST be kept — it may be referenced in tests or frontmatter entity allowed-fields logic
- Do NOT change `DeferDate`, `PlannedDate`, `DueDate` (already migrated in Prompt 1)
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```bash
# Confirm completed_date getter returns *libtime.DateOrDateTime (not string)
grep -n 'func.*CompletedDate()' pkg/domain/task_frontmatter.go
# expected: returns *libtime.DateOrDateTime

# Confirm dual-write in SetLastCompletedDate
grep -A 10 'func.*SetLastCompletedDate' pkg/domain/task_frontmatter.go
# expected: both "last_completed_date" and "last_completed" Set() calls

# Confirm created_date getter exists
grep -n 'func.*CreatedDate' pkg/domain/task_frontmatter.go
# expected: CreatedDate() and SetCreatedDate()

# Confirm ops/show.go and ops/list.go use formatDateOrDateTime
grep 'formatDateOrDateTime.*CompletedDate' pkg/ops/show.go pkg/ops/list.go
# expected: matches in both files

# Confirm no *time.Time storage in task_frontmatter.go
grep '\*time\.Time' pkg/domain/task_frontmatter.go
# expected: no output (time.Time may appear in formatTimeAsDate helper, that's ok if still used)
```
</verification>
