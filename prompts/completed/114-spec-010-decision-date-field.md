---
status: completed
spec: [010-unify-date-fields-to-dateordatetime]
summary: Migrated Decision.ReviewedDate from string to *libtime.DateOrDateTime; updated storage read (time.Time/string switch), write (formatReviewedDate helper), decision_ack (ToDate conversion), decision_list (formatDateOrDateTime at response site); updated all affected tests.
container: vault-cli-114-spec-010-decision-date-field
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-08T00:00:00Z"
queued: "2026-05-08T19:04:32Z"
started: "2026-05-08T21:22:27Z"
completed: "2026-05-08T21:26:19Z"
branch: dark-factory/unify-date-fields-to-dateordatetime
---

<summary>
- Decision.ReviewedDate changed from string to *libtime.DateOrDateTime in the domain struct
- Decision storage read updated: data["reviewed_date"] now handled by GetTime-style logic (accepts time.Time or string from YAML)
- Decision storage write updated: ReviewedDate formatted via formatDateOrDateTime (not stored as raw string)
- decision_ack.go updated: sets ReviewedDate as *libtime.DateOrDateTime built from currentDateTime.Now()
- decision_list.go updated: ReviewedDate JSON field formatted from *libtime.DateOrDateTime to string for output
- Round-trip: existing date-only vault files read as YYYY-MM-DD and write back as YYYY-MM-DD with no churn
- All existing tests pass; new tests cover date-only and RFC3339 round-trip for reviewed_date
</summary>

<objective>
Migrate Decision `reviewed_date` from a plain `string` field to `*libtime.DateOrDateTime`, completing the last entity migration in spec 010. Unlike Task/Goal/Objective/Theme (which use FrontmatterMap-embedded accessors), Decision uses a plain struct with manual map read/write in storage — those callsites drive the scope. Requires Prompts 1–4 to be completed first.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega conventions.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

**Prompts 1–4 must be completed first.** This prompt depends on:
- `libtime.DateOrDateTime` available throughout the codebase
- `formatDateOrDateTime(*libtime.DateOrDateTime)` available in the `ops` package (pkg/ops/frontmatter.go)
- A `GetTime`-equivalent approach for reading `reviewed_date` from the frontmatter map

Key files to read before making changes:
- `pkg/domain/decision.go` — Decision struct with `ReviewedDate string`; also check yaml tags and how they are actually used
- `pkg/storage/decision.go` — readDecisionFromPath (reads reviewed_date manually from map); WriteDecision or equivalent write path (writes reviewed_date to map)
- `pkg/ops/decision_ack.go` — sets `decision.ReviewedDate = d.currentDateTime.Now().Format("2006-01-02")`
- `pkg/ops/decision_list.go` — DecisionListItem struct has `ReviewedDate string`; set from `dec.ReviewedDate`
- `pkg/domain/frontmatter_map.go` — GetTime(key string) method returns *time.Time; use this pattern for the storage read
- `github.com/bborbe/time` library: this repo is non-vendored. Use `go doc github.com/bborbe/time.DateOrDateTime`. Library type is `type DateOrDateTime stdtime.Time`; `libtime.DateOrDateTime(*t)` works directly. `.MarshalText()` is the canonical formatter (midnight-UTC → date-only, else RFC3339).

**Precondition**: Prompts 1-4 must have completed. Verify with `grep -q 'libtime\.DateOrDateTime' pkg/domain/task_frontmatter.go` — if no match, stop.
</context>

<requirements>
### 1. Audit the full scope before writing any code

```bash
# All usages of ReviewedDate in non-test code
grep -rn 'ReviewedDate\|reviewed_date' pkg/ --include='*.go' | grep -v '_test.go'
```

Confirm the list matches: `pkg/domain/decision.go`, `pkg/storage/decision.go`, `pkg/ops/decision_ack.go`, `pkg/ops/decision_list.go`. If any additional files appear, add them to the scope.

### 2. Update pkg/domain/decision.go — change ReviewedDate field type

Change:

```go
ReviewedDate string `yaml:"reviewed_date,omitempty"`
```

To:

```go
ReviewedDate *libtime.DateOrDateTime `yaml:"-"` // managed by storage layer, not YAML struct tags
```

The `yaml:"-"` tag prevents the YAML library from trying to marshal/unmarshal this field directly — the storage layer manages it via the FrontmatterMap explicitly (as it already does for all Decision fields). The `yaml:"reviewed_date,omitempty"` tag was technically unused for read/write since storage uses the manual map approach; replacing with `yaml:"-"` is correct.

Add import `libtime "github.com/bborbe/time"` to `pkg/domain/decision.go`.

### 3. Update pkg/storage/decision.go — read path

Locate `readDecisionFromPath`. The current read for `reviewed_date`:

```go
if v, ok := data["reviewed_date"].(string); ok {
    decision.ReviewedDate = v
}
```

This only handles `string` values. Replace with a helper that accepts both `time.Time` (YAML-auto-parsed date literals) and `string` (hand-authored values), following the `FrontmatterMap.GetTime` pattern:

```go
// Read reviewed_date: accepts time.Time (YAML-parsed) or string (hand-authored)
if raw := data["reviewed_date"]; raw != nil {
    switch v := raw.(type) {
    case time.Time:
        d := libtime.DateOrDateTime(v)
        decision.ReviewedDate = &d
    case string:
        if v != "" {
            ctx2 := context.Background()
            if t, err := libtime.ParseTime(ctx2, v); err == nil {
                d := libtime.DateOrDateTime(*t)
                decision.ReviewedDate = &d
            }
            // If parse fails, leave ReviewedDate nil (invalid value — don't crash)
        }
    }
}
```

Add imports as needed: `"time"` (if not already present), `libtime "github.com/bborbe/time"`, `"context"` (if not already present).

### 4. Update pkg/storage/decision.go — write path

Locate the write method (likely `WriteDecision` or `writeDecision`). The current write for `reviewed_date`:

```go
if decision.ReviewedDate != "" {
    data["reviewed_date"] = decision.ReviewedDate
}
```

Replace with:

```go
if decision.ReviewedDate != nil {
    data["reviewed_date"] = formatReviewedDate(decision.ReviewedDate)
}
```

Add a package-local helper in `decision.go` (or inline if simple):

```go
// formatReviewedDate serializes a *libtime.DateOrDateTime to string for YAML storage.
// Midnight-UTC values format as YYYY-MM-DD; others as RFC3339 with timezone.
func formatReviewedDate(d *libtime.DateOrDateTime) string {
    if d == nil {
        return ""
    }
    // Use the same logic as formatDateOrDateTime in the ops package:
    t := d.Time()  // if libtime.DateOrDateTime has .Time(); otherwise use MarshalText
    if t.UTC().Hour() == 0 && t.UTC().Minute() == 0 && t.UTC().Second() == 0 && t.UTC().Nanosecond() == 0 {
        return t.UTC().Format("2006-01-02")
    }
    return t.Format(time.RFC3339)
}
```

**Alternative:** If `libtime.DateOrDateTime.MarshalText()` is available and produces the same output, use it instead to avoid duplicating the midnight-UTC logic:

```go
func formatReviewedDate(d *libtime.DateOrDateTime) string {
    if d == nil {
        return ""
    }
    text, err := d.MarshalText()
    if err != nil {
        return ""
    }
    return string(text)
}
```

Library exposes both `MarshalText() ([]byte, error)` and `Time() time.Time`. Prefer reusing `pkg/ops/frontmatter.go` `formatDateOrDateTime` if exported / accessible from `pkg/storage`; otherwise use the library's `MarshalText` directly.

### 5. Update pkg/ops/decision_ack.go

Current code:
```go
decision.ReviewedDate = d.currentDateTime.Now().Format("2006-01-02")
```

`d.currentDateTime.Now()` returns a `libtime.DateTime`. Replace with a `*libtime.DateOrDateTime` assignment.

The goal is a date-only value (midnight UTC), since the decision is acknowledged "today" at date granularity:

```go
// Get the current date as midnight UTC for date-only serialization
t := libtime.ToDate(d.currentDateTime.Now().Time()).Time()
reviewedDate := libtime.DateOrDateTime(t)
decision.ReviewedDate = &reviewedDate
```

Verify `libtime.ToDate` exists:
```bash
go doc github.com/bborbe/time.ToDate
```

If `libtime.ToDate` doesn't exist, use an alternative:
```go
now := d.currentDateTime.Now().Time().UTC()
midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
reviewedDate := libtime.DateOrDateTime(midnight)
decision.ReviewedDate = &reviewedDate
```

### 6. Update pkg/ops/decision_list.go

The `DecisionListItem` (or equivalent) struct has:
```go
ReviewedDate string `json:"reviewed_date,omitempty"`
```

This is the JSON response — it should remain a `string` for API stability. Update the assignment from `dec.ReviewedDate` (was string) to format the new `*libtime.DateOrDateTime`:

```go
// Old:
ReviewedDate: dec.ReviewedDate,

// New (use ops package formatDateOrDateTime from frontmatter.go):
ReviewedDate: formatDateOrDateTime(dec.ReviewedDate),
```

`formatDateOrDateTime` is defined in `pkg/ops/frontmatter.go` and is accessible within the `ops` package.

### 7. Write tests

#### 7a. Storage round-trip test in pkg/storage/decision_test.go

Add test cases (extend existing Ginkgo suite — do NOT recreate suite bootstrap):

- **Date-only YAML literal round-trip:** Write a decision fixture with `reviewed_date: 2025-01-15` as a YAML date, read via `readDecisionFromPath`, confirm `ReviewedDate` is non-nil and `formatReviewedDate(ReviewedDate)` returns `"2025-01-15"`
- **RFC3339 string round-trip:** Write a fixture with `reviewed_date: "2025-01-15T14:30:00+01:00"`, confirm ReviewedDate is non-nil and its formatted value is `"2025-01-15T14:30:00+01:00"`
- **Absent key:** Fixture with no `reviewed_date`, confirm `ReviewedDate` is nil
- **Write path:** Build a Decision with ReviewedDate set, write it, read it back, confirm the frontmatter contains `reviewed_date: 2025-01-15` (for a midnight-UTC value)

Use temp files and the existing storage test infrastructure.

#### 7b. Decision ack test in pkg/ops/decision_ack_test.go (extend existing)

- After `Execute`, confirm `decision.ReviewedDate` is non-nil
- Confirm `decision.ReviewedDate` formats as a `YYYY-MM-DD` string (date-only, not RFC3339)

### 8. Iterative verification

Run `make test` after each file change to catch compile errors early. The Decision struct field type change will cause compile errors in decision_ack.go and decision_list.go — fix them in sequence.
</requirements>

<constraints>
- `Decision.ReviewedDate` MUST change from `string` to `*libtime.DateOrDateTime` — there is no compat layer for the string type here (unlike Goal's `*time.Time` setters, the legacy `string` type has no downstream consumers that need a compat wrapper)
- The YAML struct tag must change to `yaml:"-"` since the storage layer manages this field via the FrontmatterMap (direct YAML deserialization of the struct was never actually used)
- `formatReviewedDate` in decision.go must produce the same output format as `formatDateOrDateTime` in ops/frontmatter.go — verify they agree (midnight-UTC → `YYYY-MM-DD`, others → RFC3339)
- `decision_list.go` JSON response struct keeps `ReviewedDate string` — do NOT change the API shape; format `*libtime.DateOrDateTime` to string at the response-building callsite
- No new CLI subcommands or flags
- All existing tests must continue to pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```bash
# Confirm ReviewedDate is now *libtime.DateOrDateTime
grep 'ReviewedDate' pkg/domain/decision.go
# expected: *libtime.DateOrDateTime

# Confirm storage read handles both time.Time and string
grep -A 15 'reviewed_date' pkg/storage/decision.go
# expected: switch statement with time.Time and string cases

# Confirm storage write uses formatReviewedDate (not raw string assignment)
grep 'formatReviewedDate\|MarshalText' pkg/storage/decision.go
# expected: at least one match

# Confirm decision_ack.go no longer uses .Format("2006-01-02") string assignment
grep 'Format.*2006\|ReviewedDate.*=' pkg/ops/decision_ack.go
# expected: no string .Format() call; assignment to *libtime.DateOrDateTime

# Confirm decision_list.go formats to string via formatDateOrDateTime
grep 'formatDateOrDateTime.*ReviewedDate\|ReviewedDate.*format' pkg/ops/decision_list.go
# expected: one match

# Final check: no remaining *_date fields stored as plain string
grep '\*time\.Time\|string.*_date\|_date.*string' pkg/domain/decision.go
# expected: no matches (ReviewedDate is *libtime.DateOrDateTime, not string or *time.Time)
```

### Add changelog entry (in CHANGELOG.md under ## Unreleased)

After all changes pass `make precommit`, add the following entry to `## Unreleased` in `CHANGELOG.md`:

```
- feat: Unify all *_date frontmatter fields across Task, Goal, Objective, Theme, Decision to use libtime.DateOrDateTime for RFC3339 round-trip fidelity [spec 010]
```

And add a follow-up tracking note for the dual-write window cleanup:

```
- chore: Drop legacy last_completed write after one release cycle (dual-write window from spec 010 task migration)
```
</verification>
