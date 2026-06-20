---
status: completed
spec: [018-typed-date-storage]
summary: Added libtime.DateOrDateTime arm to FrontmatterMap.GetTime and one corresponding Ginkgo test; make precommit exited 0.
container: vault-cli-date-storage-exec-142-reader-side-date-multi-shape
dark-factory-version: v0.182.0
created: "2026-06-20T13:48:15Z"
queued: "2026-06-20T13:48:15Z"
started: "2026-06-20T13:48:16Z"
completed: "2026-06-20T13:49:51Z"
branch: dark-factory/typed-date-storage
---

<summary>
- Reading a date frontmatter field now works whether the stored value is a legacy on-disk string, a yaml-parsed `time.Time`, or an in-memory typed date value
- This makes "set a date, then read it back without saving to disk first" return the value you set, instead of nothing
- Existing behavior for legacy string values and yaml-parsed timestamps is unchanged
- Missing keys, empty strings, unparseable strings, and unsupported types still return "no value" without crashing
- No setters change in this prompt — this is purely a reader-side tolerance addition
- New unit tests cover all four input shapes plus the failure cases
</summary>

<objective>
Extend `FrontmatterMap.GetTime` in `pkg/domain/frontmatter_map.go` so it returns a non-nil result when the stored value is a `libtime.DateOrDateTime` (the in-memory typed shape), in addition to the existing `time.Time` and `string` shapes. This is the reader-side precondition for the later prompts that change setters to store typed values directly. No setter changes in this prompt.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` for Ginkgo v2 / Gomega conventions and coverage rules.

Read these files in full before changing anything:
- `/workspace/pkg/domain/frontmatter_map.go` — `GetTime` is at lines 52-80. The three existing arms are `time.Time`, `string`, and `default`.
- `/workspace/pkg/domain/frontmatter_map_test.go` (if present) and `/workspace/pkg/domain/task_frontmatter_test.go` — for Ginkgo `Describe`/`It` test style in package `domain_test`.

`libtime` is imported as `libtime "github.com/bborbe/time"` (already imported in `frontmatter_map.go` at line 13).

`libtime.DateOrDateTime` exposes a value-receiver method `Time() time.Time` (verified in `github.com/bborbe/time@v1.27.1/time_date-or-date-time.go`). A `libtime.DateOrDateTime` is constructed from a `time.Time` via `libtime.DateOrDateTime(someTime)`.

Current `GetTime` body (verbatim, lines 59-80):

```go
func (f FrontmatterMap) GetTime(key string) *time.Time {
	v := f.data[key]
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		tc := t
		return &tc
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
</context>

<requirements>

### 1. Add a `libtime.DateOrDateTime` arm to `GetTime` in `pkg/domain/frontmatter_map.go`

Insert a new `case` BETWEEN the existing `case time.Time:` arm and the `case string:` arm:

```go
	case libtime.DateOrDateTime:
		tc := t.Time()
		return &tc
```

Do NOT replace or reorder the existing `time.Time`, `string`, or `default` arms — this is additive. The final switch must have four arms in this order: `time.Time`, `libtime.DateOrDateTime`, `string`, `default`.

### 2. Update the `GetTime` doc comment

Update the doc comment above `GetTime` (lines 52-58) to mention the new shape. The comment must list all three handled shapes: `time.Time`, `libtime.DateOrDateTime`, and `string`. Keep the existing sentence about returning nil on missing key, empty string, parse failure, or unsupported type.

### 3. Add ONE new `It(...)` for the `libtime.DateOrDateTime` shape

The file `pkg/domain/frontmatter_map_test.go` ALREADY exists and ALREADY contains a `Describe("GetTime", ...)` block at line 169 with `It` cases for `time.Time`, ISO date string, RFC3339 string, nil value, empty string, int (unsupported type), missing key, and unparseable string. The ONLY genuinely new case is `libtime.DateOrDateTime` (AC #6).

Add exactly ONE new `It(...)` inside the existing `Describe("GetTime", ...)` block at `pkg/domain/frontmatter_map_test.go:169` for the `libtime.DateOrDateTime` shape:

- Construct via `libtime.DateOrDateTime(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))`
- Store under a key via `domain.NewFrontmatterMap(map[string]any{"d": <typed value>})`
- Assert `GetTime("d")` returns non-nil AND the dereferenced `*time.Time` equals the source instant via `time.Time.Equal` (NOT `==` — RFC3339 round-trip may differ in monotonic clock reading)

Use a fixed `time.Time` literal. Do NOT use `time.Now()`. Do NOT duplicate the existing 8 cases — they already cover their respective ACs (#7, #8, #9, plus the edge cases). Do NOT add a parallel `Describe` block.

### 4. Iterative verification

After editing, run `make test` from the repo root to confirm tests pass. Do NOT run `make precommit` until the final verification step.

</requirements>

<constraints>
- Public setter and reader signatures MUST NOT change — `GetTime(key string) *time.Time` stays exactly as is.
- Existing `time.Time`, `string`, and `default` arms MUST NOT be replaced or reordered — the new arm is additive only.
- Do NOT change any setter in this prompt. Setters are migrated in the following prompts.
- Do NOT modify `formatTimeAsDate` or `formatDateOrDateTime` — they are unchanged in this prompt.
- Tests use Ginkgo v2 / Gomega in package `domain_test` per project convention. No Counterfeiter mocks needed (`FrontmatterMap` is concrete).
- Coverage for the modified `GetTime` MUST stay ≥80% per `docs/definition-of-done.md` — every arm including the new one must be exercised by a test.
- `make precommit` MUST stay green from the repo root (no per-package Makefile in this repo).
- **`ParseDateOrDateTime` / `ParseDateOrDateTimeDefault` helpers are OUT OF SCOPE for this prompt.** Spec AC #16 says "IF a new `ParseDateOrDateTime` is added…" — it's conditional. This prompt adds none. Both helpers already exist on the `libtime.DateOrDateTime` type in `bborbe/time@v1.27.1` (`ParseDateOrDateTime(ctx, value) (*DateOrDateTime, error)` at line 57 and `ParseDateOrDateTimeDefault(ctx, value, default) DateOrDateTime` at line 45). Do NOT create local wrappers in `pkg/domain/`. AC #16 is vacuously satisfied for this prompt.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. AC #6: the new DateOrDateTime arm is present in GetTime
grep -n 'case libtime.DateOrDateTime:' pkg/domain/frontmatter_map.go
# Expected: 1 match inside GetTime

# 2. The existing arms are still present (additive, not replaced)
grep -n 'case time.Time:' pkg/domain/frontmatter_map.go   # Expected: 1 match
grep -n 'case string:' pkg/domain/frontmatter_map.go      # Expected: ≥1 match

# 3. Reader-shape tests pass (AC #6, #7, #8, #9)
go test -v ./pkg/domain/... -run GetTime
# Expected: PASS

# 4. Coverage of the modified function
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/domain/... && go tool cover -func=/tmp/cover.out | grep -i 'GetTime'
# Expected: GetTime at 100%
```
</verification>

<!-- DARK-FACTORY-REPORT -->
