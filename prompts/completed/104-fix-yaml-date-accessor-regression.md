---
status: completed
summary: Added FrontmatterMap.GetTime helper and routed all six Task/Goal date accessors through it, fixing the regression where YAML-native time.Time values produced nil or corrupted strings; added unit and integration tests including golden schema test and negative regression assertions.
container: vault-cli-104-fix-yaml-date-accessor-regression
dark-factory-version: v0.108.0-dirty
created: "2026-04-11T09:42:32Z"
queued: "2026-04-11T09:42:32Z"
started: "2026-04-11T09:43:40Z"
completed: "2026-04-11T09:49:04Z"
---
<summary>
- Tasks with `defer_date: 2026-04-13` in YAML frontmatter now correctly expose the date through `task.DeferDate()` instead of returning nil
- `vault-cli task show --output json` and `task list --output json` include `defer_date`, `planned_date`, `due_date`, `completed_date` for every task whose YAML has them
- `CompletedDate()` and `LastCompleted()` return clean `YYYY-MM-DD` / RFC3339 strings instead of Go's ugly `2026-03-08 00:00:00 +0000 UTC` default format
- A new `FrontmatterMap.GetTime(key)` helper cleanly handles both `time.Time` (YAML-parsed) and string (manually-authored) values
- All five Task date accessors plus `GoalFrontmatter.DeferDate` route through the new helper — no remaining GetString+ParseTime callsites for date fields
- Golden JSON schema test locks the field set of `task show` and `task list` output so this class of silent-field-drop cannot regress again
- Downstream task-orchestrator defer filter now sees real dates and hides deferred tasks from the UI as intended (no task-orchestrator code change)
</summary>

<objective>
Fix the post-spec-008 regression where YAML-parsed `time.Time` frontmatter values (e.g. `defer_date: 2026-04-13`) are stringified via `fmt.Sprintf("%v", v)` and then fail to re-parse, causing Task date accessors (DeferDate, PlannedDate, DueDate, LastCompleted, CompletedDate) plus `GoalFrontmatter.DeferDate` to return nil or ship corrupted `"2026-04-13 00:00:00 +0000 UTC"` strings to JSON output. Introduce a type-safe `FrontmatterMap.GetTime` helper, route every affected accessor through it, and add a golden JSON schema test so the class of bug cannot recur silently.

**Deeper root cause (for future maintainers):** `DateOrDateTime` at `pkg/domain/date_or_datetime.go` implements `encoding.TextMarshaler` / `TextUnmarshaler`. In the pre-spec-008 struct-based code, YAML invoked `UnmarshalText` automatically because struct fields declared typed `*DateOrDateTime` targets. In the new map-based model, data lands in `map[string]any` — YAML has no target-type hint and parses bare `defer_date: 2026-04-13` directly into `time.Time`, so `DateOrDateTime.UnmarshalText` is never called. The type's custom format logic is entirely bypassed for unquoted YAML dates. Quoted YAML (`defer_date: "2026-04-13"`) happens to work because the value stays a string; unquoted does not. Real vault files use BOTH forms, so the fix must handle both paths.

**Scope note:** `ObjectiveFrontmatter`, `ThemeFrontmatter`, and `GoalFrontmatter.StartDate`/`TargetDate`/`Completed` already have correct `time.Time` type assertion fallbacks and are NOT affected by this bug. Do not modify them. VisionFrontmatter has no date fields. The affected surface is exactly six accessors listed above.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions.

**Bug root cause:** Spec-008 migrated domain types to `map[string]any` frontmatter. YAML unmarshaling parses date strings like `defer_date: 2026-04-13` as `time.Time`, not `string`. The new accessors call `FrontmatterMap.GetString("defer_date")`, which hits the `default` case in `pkg/domain/frontmatter_map.go` and returns `fmt.Sprintf("%v", v)` — producing `"2026-04-13 00:00:00 +0000 UTC"`. That string then fails `libtime.ParseTime` at `pkg/domain/task_frontmatter.go:79` with `extra text: 00:00:00 +0000 UTC`, so `Task.DeferDate()` returns nil for every task with a YAML-native `defer_date`.

**Verified failing case (do not modify, read-only):** `~/Documents/Obsidian/Personal/24 Tasks/Aquascape PWC.md` has `defer_date: 2026-04-13` on disk, and `vault-cli task show "Aquascape PWC" --output json` does NOT include `defer_date` in output today. After this fix, it must.

**Partial precedent already in the codebase:** `pkg/domain/goal_frontmatter.go` `GoalFrontmatter.DeferDate()` (lines ~132-152) already contains a hand-rolled `time.Time` type assertion fallback. This prompt replaces that ad-hoc pattern with a shared helper and applies it consistently to Task accessors too.

**Files to read in full before making changes:**
- `pkg/domain/frontmatter_map.go` (new helper goes here)
- `pkg/domain/frontmatter_map_test.go` (new GetTime tests go here)
- `pkg/domain/task_frontmatter.go` (five broken accessors: DeferDate, LastCompleted, CompletedDate, PlannedDate, DueDate)
- `pkg/domain/task_frontmatter_test.go` (existing DeferDate test at the `Describe("DeferDate")` block; add cases alongside it)
- `pkg/domain/goal_frontmatter.go` (GoalFrontmatter.DeferDate — the sixth broken accessor; remove ad-hoc fallback, use GetTime)
- `pkg/domain/goal_frontmatter_test.go` (add GoalFrontmatter DeferDate time.Time case)
- `pkg/domain/date_or_datetime.go` (where `DateOrDateTime` is defined), plus `formatDateOrDateTime` at the bottom of `pkg/domain/task_frontmatter.go` (already implements the YYYY-MM-DD vs RFC3339 formatting rule we want to mirror for string accessors)
- `integration/cli_test.go` (follow the `createTempVault` + `gexec.Start` pattern in the existing `vault-cli defer` Describe block around the `task show` tests)

**Libraries:**
- `libtime "github.com/bborbe/time"` — already imported in task_frontmatter.go; `libtime.ParseTime(ctx, s) (*time.Time, error)` is the existing parser
- `"context"` — use `context.Background()` inside GetTime to match the existing accessor style
- Testing: `github.com/onsi/ginkgo/v2` and `github.com/onsi/gomega`, same as sibling test files
</context>

<requirements>

### 1. Add `FrontmatterMap.GetTime` helper

In `pkg/domain/frontmatter_map.go`, add a new method directly below `GetString`. Use this exact signature and semantics:

```go
// GetTime returns the time.Time value stored for key.
// Handles three shapes:
//   - time.Time (YAML parses date/datetime literals into this automatically)
//   - string (falls back to libtime.ParseTime for manually-authored values)
//   - anything else → nil
// Returns nil on missing key, empty string, parse failure, or unsupported type.
func (f FrontmatterMap) GetTime(key string) *time.Time {
    v := f.data[key]
    if v == nil {
        return nil
    }
    switch t := v.(type) {
    case time.Time:
        copy := t
        return &copy
    case string:
        if t == "" {
            return nil
        }
        parsed, err := libtime.ParseTime(context.Background(), t)
        if err != nil {
            return nil
        }
        return parsed
    default:
        return nil
    }
}
```

Add the required imports to `pkg/domain/frontmatter_map.go`:
- `"context"`
- `"time"`
- `libtime "github.com/bborbe/time"`

Keep the existing `"fmt"` and `"strings"` imports. Sort the import block per `goimports` conventions (stdlib, then third-party).

**IMPORTANT — do not introduce an import cycle.** `libtime` is a sibling library, not a project package, so this is safe. Verify with `go build ./pkg/domain/...` after editing.

### 2. Rewrite `TaskFrontmatter.DeferDate` to use GetTime

In `pkg/domain/task_frontmatter.go`, replace the body of `DeferDate()` (the method that today reads `GetString("defer_date")` and calls `libtime.ParseTime`) with:

```go
// DeferDate reads "defer_date" key as *DateOrDateTime.
// Handles both time.Time (YAML-parsed) and string (hand-authored) forms.
// Returns nil on missing or unparseable value.
func (f TaskFrontmatter) DeferDate() *DateOrDateTime {
    t := f.GetTime("defer_date")
    if t == nil {
        return nil
    }
    d := DateOrDateTime(*t)
    return &d
}
```

### 3. Rewrite `TaskFrontmatter.PlannedDate` and `TaskFrontmatter.DueDate`

Apply the exact same pattern as step 2, with the respective keys `"planned_date"` and `"due_date"`. Each becomes three lines of logic (get, nil-check, wrap). Remove the `libtime.ParseTime` calls from these two methods — they are now redundant because `GetTime` handles parsing.

### 4. Rewrite `TaskFrontmatter.LastCompleted` and `TaskFrontmatter.CompletedDate`

These return `string`, not `*DateOrDateTime`. Do NOT change their signatures. Replace their bodies to call `GetTime` and format the result using the same rule as `formatDateOrDateTime` (the helper at the bottom of `task_frontmatter.go`):

```go
// LastCompleted reads "last_completed" as a formatted date string.
// Returns "" on missing value. Date-only values (midnight UTC) format as
// "2006-01-02"; values with a time component format as RFC3339.
func (f TaskFrontmatter) LastCompleted() string {
    t := f.GetTime("last_completed")
    if t == nil {
        return ""
    }
    return formatTimeAsDate(*t)
}

// CompletedDate reads "completed_date" as a formatted date string.
// Same formatting rules as LastCompleted.
func (f TaskFrontmatter) CompletedDate() string {
    t := f.GetTime("completed_date")
    if t == nil {
        return ""
    }
    return formatTimeAsDate(*t)
}
```

Add a new package-private helper `formatTimeAsDate` near `formatDateOrDateTime` at the bottom of `task_frontmatter.go`:

```go
// formatTimeAsDate serializes a time.Time using the same rule as formatDateOrDateTime:
// YYYY-MM-DD for midnight-UTC values, RFC3339 preserving timezone otherwise.
func formatTimeAsDate(t time.Time) string {
    tUTC := t.UTC()
    if tUTC.Hour() == 0 && tUTC.Minute() == 0 && tUTC.Second() == 0 && tUTC.Nanosecond() == 0 {
        return tUTC.Format(time.DateOnly)
    }
    return t.Format(time.RFC3339)
}
```

**Refactor opportunity:** `formatDateOrDateTime` can now call `formatTimeAsDate` internally — do that reduction to keep the two formatters consistent. The refactored version:

```go
func formatDateOrDateTime(d *DateOrDateTime) string {
    if d == nil {
        return ""
    }
    return formatTimeAsDate(d.Time())
}
```

### 5. Rewrite `GoalFrontmatter.DeferDate` to use GetTime

In `pkg/domain/goal_frontmatter.go`, remove the ad-hoc `if t, ok := v.(time.Time); ok` fallback and the subsequent `GetString` + `libtime.ParseTime` block. Replace with the same three-line pattern as `TaskFrontmatter.DeferDate`:

```go
func (f GoalFrontmatter) DeferDate() *DateOrDateTime {
    t := f.GetTime("defer_date")
    if t == nil {
        return nil
    }
    d := DateOrDateTime(*t)
    return &d
}
```

After this edit, check whether `goal_frontmatter.go` still needs `context` / `libtime` / `time` imports for OTHER methods. If any of those imports become unused, remove them. Run `go build ./pkg/domain/...` to confirm.

Similarly audit `task_frontmatter.go` imports — `context` and `libtime` are still used by other methods (`setDateField`, `SetStatus`, etc.) so they stay, but verify with a build.

### 6. Unit tests — `frontmatter_map_test.go` GetTime block

Add a new `Describe("GetTime", ...)` block to `pkg/domain/frontmatter_map_test.go`. Follow the existing style (each scenario uses its own `Context` or `When` sub-block where appropriate, or inline `It` nodes for simple cases). Cover every branch:

1. **time.Time value** — `domain.NewFrontmatterMap(map[string]any{"d": time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)})` → `GetTime("d")` returns non-nil, `.UTC().Format("2006-01-02")` equals `"2026-04-13"`
2. **ISO-8601 date-only string** — `"2026-04-13"` → parses to correct time (year/month/day match)
3. **RFC3339 datetime string** — `"2026-03-08T00:00:00Z"` → parses correctly
4. **nil value** — key exists with nil value → returns nil
5. **empty string** — `""` → returns nil
6. **wrong type** — integer `42` → returns nil
7. **missing key** — `GetTime("absent")` on empty map → returns nil
8. **unparseable string** — `"not-a-date"` → returns nil (parse failure path)

Import `"time"` in `frontmatter_map_test.go` if not already present.

### 7. Unit tests — `task_frontmatter_test.go` date accessor cases

The existing `Describe("DeferDate", ...)` block already covers the string input path. Add sibling `It` nodes (in the same `Describe` block) for the `time.Time` input path:

- **`time.Time` defer_date** — construct the frontmatter with `map[string]any{"defer_date": time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)}` and assert `DeferDate()` returns non-nil with `Time().UTC().Format("2006-01-02") == "2026-04-13"`

Add new `Describe` blocks for the other accessors that previously had no test coverage. For each accessor, cover both the `string` and `time.Time` input paths:

- **`Describe("PlannedDate", ...)`** — `"planned_date"` as string `"2026-05-01"` and as `time.Time`; both return non-nil with matching date
- **`Describe("DueDate", ...)`** — `"due_date"` as string `"2026-06-15"` and as `time.Time`; both return non-nil with matching date
- **`Describe("LastCompleted", ...)`** — covers the regression directly:
  - missing key → `""`
  - `"last_completed": time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)` → returns `"2026-03-08"` (NOT `"2026-03-08 00:00:00 +0000 UTC"`)
  - `"last_completed": "2026-03-08"` (string input) → returns `"2026-03-08"`
  - datetime with non-zero time → returns RFC3339 format
- **`Describe("CompletedDate", ...)`** — same four cases as LastCompleted but with the `"completed_date"` key

### 8. Unit tests — `goal_frontmatter_test.go` DeferDate cases

Locate the existing `GoalFrontmatter` test file. Add a `Describe("DeferDate", ...)` block (or extend the existing one) with both input paths:
- `"defer_date": "2026-04-13"` → non-nil, date matches
- `"defer_date": time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)` → non-nil, date matches
- missing key → nil

### 9. Integration test — end-to-end YAML date round-trip

Add a new `Describe` block to `integration/cli_test.go` (below the existing `"vault-cli defer"` block). Use the existing `createTempVault` helper. The task file content must embed the defer_date as a YAML-native date literal (no quotes) so that YAML actually parses it as `time.Time`:

```go
Describe("vault-cli task show with YAML date literal", func() {
    var vaultPath, configPath string
    var cleanup func()

    BeforeEach(func() {
        vaultPath, configPath, cleanup = createTempVault(map[string]string{
            "aqua": `---
status: todo
priority: 2
defer_date: 2026-04-13
---
# Aqua
`,
        })
    })

    AfterEach(func() {
        cleanup()
    })

    It("outputs defer_date in JSON when YAML has a native date literal", func() {
        cmd := exec.Command(
            binPath,
            "--config", configPath,
            "--vault", "test",
            "task", "show", "aqua",
            "--output", "json",
        )
        session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
        Expect(err).NotTo(HaveOccurred())
        Eventually(session).Should(gexec.Exit(0))
        Expect(session.Out).To(gbytes.Say(`"defer_date":\s*"2026-04-13"`))
    })
})
```

Adjust the exact JSON field name (`"defer_date"`) if `task show --output json` uses a different key — run `vault-cli task show --output json` against any existing task beforehand to confirm the field name, or grep the JSON marshaling code under `pkg/cli/` for the task show output type. Do not assume.

**Also assert the negative regression:** the JSON output MUST NOT contain the substring `"00:00:00 +0000 UTC"` anywhere. Add:

```go
Expect(session.Out.Contents()).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
```

This is the exact malformed string produced by `fmt.Sprintf("%v", time.Time)` and catches any future regression that re-introduces the default case stringification.

### 10. Golden JSON schema test for Task output — lock the output shape

The architectural gap that let this bug ship is: no test asserts the full set of fields present in `task show` / `task list` JSON output. Add a single golden test in `integration/cli_test.go` for Task (scope intentionally limited — other entities are not affected by this bug and their output shapes are out of scope for this prompt).

First, confirm the current `TaskDetail` struct shape by reading `pkg/ops/show.go` (struct `TaskDetail`) and `pkg/ops/list.go` (struct `TaskListItem`). Use the set of `json:"..."` tags on those structs as the authoritative expected key list — do not hand-guess.

Add a new `Describe("vault-cli task JSON schema", ...)` block with one fixture task that populates every supported date field, using a MIX of quoted and unquoted date forms:

```yaml
---
status: in_progress
priority: 2
assignee: bborbe
recurring: weekly
phase: todo
defer_date: 2026-04-13                  # unquoted (YAML time.Time path)
planned_date: "2026-04-15"              # quoted (string path)
due_date: 2026-04-20T10:30:00Z          # unquoted RFC3339 (YAML time.Time path)
completed_date: "2026-03-09T12:30:00Z"  # quoted RFC3339 with non-midnight time (exercises RFC3339 preservation path)
last_completed: 2026-03-08              # unquoted date (not in JSON output today, but read by LastCompleted accessor)
task_identifier: 043d9cac-d56b-4a36-921e-b0e35819fb66
goals:
  - "[[Example Goal]]"
tags:
  - alpha
---
body
```

Test assertions:

1. Run `vault-cli task show <name> --output json` and parse into `map[string]any`
2. Assert the required date fields are present with exact expected values:
   ```go
   var parsed map[string]any
   Expect(json.Unmarshal(session.Out.Contents(), &parsed)).To(Succeed())
   Expect(parsed).To(HaveKeyWithValue("defer_date", "2026-04-13"))
   Expect(parsed).To(HaveKeyWithValue("planned_date", "2026-04-15"))
   Expect(parsed).To(HaveKeyWithValue("due_date", "2026-04-20T10:30:00Z"))
   Expect(parsed).To(HaveKeyWithValue("completed_date", "2026-03-09T12:30:00Z"))
   ```
3. **Negative regression assertion** — the raw JSON bytes MUST NOT contain the forbidden substring:
   ```go
   Expect(string(session.Out.Contents())).NotTo(ContainSubstring("00:00:00 +0000 UTC"))
   ```
4. Repeat steps 1-3 with `vault-cli task list --output json`, selecting the fixture task from the array and asserting the same date field values.

The negative assertion is the high-signal guard: it catches any future regression where `fmt.Sprintf("%v", time.Time)` leaks into JSON output.

### 11. Verify the fix end-to-end against the real failing case (manual check, not committed)

After all the above edits compile and pass `make test`, manually build and run against the real vault to confirm the bug is actually gone:

```bash
make build
./bin/vault-cli task show "Aquascape PWC" --output json | grep defer_date
```

Expected: output contains `"defer_date": "2026-04-13"` (exact date may differ if the task was updated). If the field is still absent, stop and diagnose before marking the prompt complete.

**Do NOT modify the Aquascape PWC file or any file in `~/Documents/Obsidian/Personal/`.** This step is read-only verification.

</requirements>

<constraints>
- Must pass `make precommit` (tests + lint + gosec)
- Do NOT change the storage layer — only `pkg/domain/*` and `integration/cli_test.go`
- Do NOT add defer_date filtering to task list — that is separate future work
- Do NOT touch `task-orchestrator` or any Python code
- Keep one type per file convention — the new `GetTime` method goes on the existing `FrontmatterMap` type in its existing file, not a new file
- Do NOT change the public signatures of any accessor (`DeferDate`, `LastCompleted`, etc.) — only their bodies
- Do NOT introduce an import cycle — `libtime` is `github.com/bborbe/time`, a sibling library, not a project package; verify with `go build ./pkg/domain/...`
- Remove `libtime.ParseTime` direct calls from individual date accessors — they should all go through `GetTime` so the bug cannot recur in one place and not another
- Existing tests must still pass without modification (except where you are specifically adding new cases)
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
Run from the project root:

```bash
make precommit
```

Must pass clean (tests + lint + gosec + format).

Targeted unit-test runs for sanity:

```bash
go test ./pkg/domain/... -run 'FrontmatterMap|TaskFrontmatter|GoalFrontmatter' -count=1
go test ./integration/... -run 'task show with YAML date literal' -count=1
```

Greps to confirm no stray direct `libtime.ParseTime` calls remain inside the six date accessors:

```bash
grep -n 'libtime.ParseTime' pkg/domain/task_frontmatter.go
# expected: matches ONLY inside setDateField (the SETTER path), NOT inside any getter
grep -n 'libtime.ParseTime' pkg/domain/goal_frontmatter.go
# expected: no matches (getter was the only place; setter uses its own helper if any)
```

Confirm GetTime handles all three branches:

```bash
grep -n 'case time.Time' pkg/domain/frontmatter_map.go
# expected: exactly one match (inside GetTime)
grep -n 'GetTime' pkg/domain/frontmatter_map.go pkg/domain/task_frontmatter.go pkg/domain/goal_frontmatter.go
# expected: matches in all three files
```

Confirm the negative regression assertion is in the integration test:

```bash
grep -n '00:00:00 +0000 UTC' integration/cli_test.go
# expected: at least one match (the NotTo(ContainSubstring) assertion)
```

Confirm no GetString-for-dates callsites remain in the six affected accessors:

```bash
grep -nE 'GetString\("(defer_date|planned_date|due_date|last_completed|completed_date)"' pkg/domain/task_frontmatter.go pkg/domain/goal_frontmatter.go
# expected: no output — every date field in Task + Goal.DeferDate must go through GetTime
```

Confirm Task golden schema test exists:

```bash
grep -n 'HaveKeyWithValue.*defer_date\|NotTo(ContainSubstring.*00:00:00' integration/cli_test.go
# expected: matches — the golden test and negative regression assertion
```

End-to-end sanity against the user-reported bug (writes to /tmp only, does not touch real vault):

```bash
mkdir -p "/tmp/vfix/24 Tasks"
cat > "/tmp/vfix/24 Tasks/Deferred.md" <<'YAML'
---
status: in_progress
defer_date: 2026-04-13
planned_date: 2026-04-15
due_date: 2026-04-20T10:30:00Z
last_completed: 2026-03-08
---
body
YAML
cat > /tmp/vfix.yaml <<CFG
current_user: test
default_vault: t
vaults:
  t:
    name: t
    path: /tmp/vfix
    tasks_dir: "24 Tasks"
CFG
./bin/vault-cli --config /tmp/vfix.yaml task show "Deferred" --output json
# expected: output contains "defer_date":"2026-04-13", "planned_date":"2026-04-15",
# "due_date":"2026-04-20T10:30:00Z", "last_completed":"2026-03-08"
# MUST NOT contain "00:00:00 +0000 UTC"
rm -rf /tmp/vfix /tmp/vfix.yaml
```
</verification>
