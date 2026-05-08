---
spec: ["010"]
status: draft
created: "2026-05-08T00:00:00Z"
---

<summary>
- bborbe/time dependency bumped from v1.25.11 to v1.27.0 in go.mod and vendor
- Local pkg/domain/date_or_datetime.go and its test file deleted entirely
- All 17 files that referenced domain.DateOrDateTime now import and use libtime.DateOrDateTime instead
- Task DeferDate/PlannedDate/DueDate getters and Goal DeferDate getter return *libtime.DateOrDateTime
- Task SetDeferDate/SetPlannedDate/SetDueDate and Goal SetDeferDate setters take *libtime.DateOrDateTime
- formatDateOrDateTime helper in both pkg/domain/ and pkg/ops/ updated to use libtime.DateOrDateTime
- setDateField helper in pkg/domain/task_frontmatter.go updated for new type
- parseDeferDate and isDeferDateInPast in pkg/ops/defer_date_parser.go use libtime.DateOrDateTime
- defer.go operations updated to construct/use libtime.DateOrDateTime
- All existing tests continue to pass with the migrated type
</summary>

<objective>
Switch from the vault-cli-local `domain.DateOrDateTime` type alias to `libtime.DateOrDateTime` from `github.com/bborbe/time@v1.27.0`. Bump the dependency, delete the local copy, and retarget every reference across 17 files. This is the prerequisite for all subsequent date-field migration prompts in spec 010.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `go-error-wrapping-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for error wrapping rules.
Read `go-testing-guide.md` in `~/.claude/plugins/marketplaces/coding/docs/` for Ginkgo/Gomega conventions.
Read `test-pyramid-triggers.md` in `~/.claude/plugins/marketplaces/coding/docs/` for which test types to write for each code change.

Key files to read before making changes:
- `pkg/domain/date_or_datetime.go` — the local type to delete; note its full method set
- `pkg/domain/task_frontmatter.go` — DeferDate/PlannedDate/DueDate getters, SetDeferDate/SetPlannedDate/SetDueDate, formatDateOrDateTime, setDateField, formatTimeAsDate
- `pkg/domain/goal_frontmatter.go` — DeferDate/SetDeferDate getters using local type
- `pkg/domain/frontmatter_map.go` — GetTime() returns *time.Time (does NOT change in this prompt)
- `pkg/ops/defer.go` — constructs domain.DateOrDateTime directly via type conversion
- `pkg/ops/defer_date_parser.go` — parseDeferDate and isDeferDateInPast take/return domain.DateOrDateTime
- `pkg/ops/frontmatter.go` — has its own formatDateOrDateTime(d *domain.DateOrDateTime) function
- `go.mod` — current: github.com/bborbe/time v1.25.11
</context>

<requirements>
### 1. Verify libtime.DateOrDateTime API before writing any code

Run these greps against the vendored v1.27.0 source after bumping (step 2 below), then read the output to determine the correct construction and method names:

```bash
# After step 2: confirm type definition
grep -rn "type DateOrDateTime" $(go env GOPATH)/pkg/mod/github.com/bborbe/time@v1.27.0/ 2>/dev/null
# or check vendor
grep -rn "type DateOrDateTime" vendor/github.com/bborbe/time/

# Check method set
grep -rn "func.*DateOrDateTime" vendor/github.com/bborbe/time/ | head -30

# Check if .Time() method exists (used in formatDateOrDateTime)
grep -rn "func.*DateOrDateTime.*Time()" vendor/github.com/bborbe/time/

# Check constructors (how to build from time.Time)
grep -rn "func.*DateOrDateTime" vendor/github.com/bborbe/time/ | grep -i "new\|from\|make"
```

Use the output to fill in the blanks in steps 4–7. Do NOT guess — grep confirms.

### 2. Bump bborbe/time in go.mod and vendor

```bash
go get github.com/bborbe/time@v1.27.0
go mod tidy
go mod vendor
```

Verify: `grep 'bborbe/time' go.mod` shows `v1.27.0`.

### 3. Delete the local type files

```bash
rm pkg/domain/date_or_datetime.go
rm pkg/domain/date_or_datetime_test.go
```

After deletion, run `make test` to see all compile errors that need fixing. Fix them all before proceeding.

### 4. Update pkg/domain/task_frontmatter.go

All changes in this file substitute `domain.DateOrDateTime` (previously the local type, now `libtime.DateOrDateTime`):

**Imports:** Confirm `libtime "github.com/bborbe/time"` is already imported. Remove `import "time"` only if no remaining usages (check: `formatTimeAsDate` still uses `time.Time`).

**Getters** — DeferDate, PlannedDate, DueDate: change return type from `*DateOrDateTime` to `*libtime.DateOrDateTime`. The body converts the `*time.Time` from `GetTime()` to `*libtime.DateOrDateTime`. Use the verified constructor from step 1 (e.g., `d := libtime.DateOrDateTime(*t)` if it is a named type alias, or whatever the correct constructor is).

Example pattern (verify constructor via step 1):
```go
func (f TaskFrontmatter) DeferDate() *libtime.DateOrDateTime {
    t := f.GetTime("defer_date")
    if t == nil {
        return nil
    }
    // use verified constructor here
    d := libtime.DateOrDateTime(*t)  // adjust if libtime.DateOrDateTime is not a named time.Time type
    return &d
}
```

**Setters** — SetDeferDate, SetPlannedDate, SetDueDate: change parameter type from `*DateOrDateTime` to `*libtime.DateOrDateTime`.

**formatDateOrDateTime:** change parameter type from `*DateOrDateTime` to `*libtime.DateOrDateTime`. Update the body to use the verified method for extracting time.Time (e.g., `.Time()` if available, or use `MarshalText()` and drop `formatTimeAsDate` from this code path):

```go
func formatDateOrDateTime(d *libtime.DateOrDateTime) string {
    if d == nil {
        return ""
    }
    // If libtime.DateOrDateTime has .Time(): return formatTimeAsDate(d.Time())
    // If not: text, _ := d.MarshalText(); return string(text)
    // Verify via step 1 grep.
    ...
}
```

**setDateField:** change setter parameter type from `func(*DateOrDateTime)` to `func(*libtime.DateOrDateTime)`. Update body construction accordingly.

### 5. Update pkg/domain/goal_frontmatter.go

Same pattern as task_frontmatter.go for:
- `DeferDate() *DateOrDateTime` → `DeferDate() *libtime.DateOrDateTime`
- `SetDeferDate(d *DateOrDateTime)` → `SetDeferDate(d *libtime.DateOrDateTime)`
- `GetField("defer_date")` case: `formatDateOrDateTime(f.DeferDate())` — signature already matches after step 4

### 6. Update pkg/ops/defer_date_parser.go

- Return type of `parseDeferDate`: `domain.DateOrDateTime` → `libtime.DateOrDateTime`
- Parameter type of `isDeferDateInPast`: `domain.DateOrDateTime` → `libtime.DateOrDateTime`
- All `domain.DateOrDateTime(t)` constructions → `libtime.DateOrDateTime(t)` (or verified constructor)
- Zero value `domain.DateOrDateTime{}` → `libtime.DateOrDateTime{}` (or appropriate zero)
- Import: add `libtime "github.com/bborbe/time"` if not already present; remove `domain` import if no longer needed

### 7. Update pkg/ops/defer.go

- All `domain.DateOrDateTime` references → `libtime.DateOrDateTime`
- `domain.DateOrDateTime(existingT.AddDate(...))` → `libtime.DateOrDateTime(existingT.AddDate(...))` (adjust for verified constructor)
- `targetDate domain.DateOrDateTime` parameter types → `libtime.DateOrDateTime`
- Check `.Time()` call on existing DeferDate: `task.DeferDate().Time()` — this calls the method on `*libtime.DateOrDateTime`; verify it exists in step 1

### 8. Update pkg/ops/frontmatter.go

- `formatDateOrDateTime(d *domain.DateOrDateTime)` → `formatDateOrDateTime(d *libtime.DateOrDateTime)`
- Update body same as step 4
- Import adjustments: add libtime if not present, remove domain if no longer needed

**Note:** After this change, both `pkg/domain/task_frontmatter.go` and `pkg/ops/frontmatter.go` will have a `formatDateOrDateTime` function with the same signature. This duplication pre-existed; do NOT merge them in this prompt.

### 9. Update all test files

Files to update (each may contain `domain.DateOrDateTime` references):
- `pkg/domain/task_frontmatter_test.go`
- `pkg/ops/complete_test.go`
- `pkg/ops/defer_test.go`
- `pkg/ops/frontmatter_test.go`
- `pkg/ops/frontmatter_entity_test.go`
- `pkg/ops/list_test.go`
- `pkg/ops/show_test.go`

For each: replace `domain.DateOrDateTime(...)` → `libtime.DateOrDateTime(...)` (or verified constructor). Update imports accordingly.

### 10. Iterative verification

After each file change, run `make test` to catch compile errors immediately. Do not wait until all files are changed.

After all changes compile and tests pass, run:
```bash
go test -coverprofile=/tmp/cover.out -mod=vendor ./pkg/domain/... ./pkg/ops/... && go tool cover -func=/tmp/cover.out | grep -E 'domain|ops'
```
</requirements>

<constraints>
- `github.com/bborbe/time` must be bumped to v1.27.0 — no lower version
- Local `pkg/domain/date_or_datetime.go` must be completely deleted — zero references to it remain
- `grep -r 'domain\.DateOrDateTime' pkg/` must return no matches after this prompt
- `pkg/domain/frontmatter_map.go` `GetTime()` return type stays `*time.Time` — do NOT change it in this prompt
- `formatTimeAsDate` in `task_frontmatter.go` stays as-is — it operates on `time.Time` and is called by `formatDateOrDateTime` if libtime provides `.Time()`
- The `setDateField` function signature changes; all callers of `setDateField` in task_frontmatter.go must be updated to pass setters with the new type
- All existing tests must continue to pass — no behavior changes, only type-substitution
- Do NOT add or change any other date fields (that is for prompts 2–5)
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```bash
# Confirm local type is gone
ls pkg/domain/date_or_datetime.go 2>&1
# expected: No such file

# Confirm no remaining references to local domain.DateOrDateTime
grep -r 'domain\.DateOrDateTime' pkg/
# expected: no output

# Confirm go.mod version
grep 'bborbe/time' go.mod
# expected: github.com/bborbe/time v1.27.0

# Confirm libtime.DateOrDateTime is used in key files
grep 'libtime\.DateOrDateTime' pkg/domain/task_frontmatter.go pkg/ops/defer_date_parser.go pkg/ops/defer.go pkg/ops/frontmatter.go
# expected: multiple matches in each file
```
</verification>
