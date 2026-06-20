---
status: completed
spec: [018-typed-date-storage]
summary: Changed six TaskFrontmatter date setters to store typed libtime.DateOrDateTime values instead of pre-stringified strings, added TypedDateRoundTrip and golden-file YAML tests, created testdata/task_frontmatter_golden.yaml; all 304 specs pass and make precommit exits 0.
container: vault-cli-date-storage-exec-143-task-setter-typed-storage
dark-factory-version: v0.182.0
created: "2026-06-20T13:48:15Z"
queued: "2026-06-20T13:48:15Z"
started: "2026-06-20T13:49:53Z"
completed: "2026-06-20T13:55:04Z"
branch: dark-factory/typed-date-storage
---

<summary>
- Task date fields (defer, planned, due, completed, created, last-completed) are now stored as typed date values in memory instead of being pre-converted to strings at the setter
- Reading a date back immediately after setting it (without saving to disk first) returns exactly the value that was set
- The on-disk YAML and CLI JSON output for tasks are unchanged — the date type produces the same `YYYY-MM-DD` / RFC3339 text it did before
- The `last_completed` + `last_completed_date` dual-write behavior is preserved exactly
- A new round-trip test proves set-then-get equality for every task date field
- A new golden-file test pins the exact YAML produced when a task with all date fields set is serialized
- The shared string helper is intentionally NOT removed yet — Goal, Objective, and Theme still use it; its removal happens in the next prompt
</summary>

<objective>
Change the six `Set*Date` setters on `TaskFrontmatter` in `pkg/domain/task_frontmatter.go` to store the dereferenced typed `libtime.DateOrDateTime` value into the underlying map (instead of pre-stringifying it via `formatDateOrDateTime`). The `last_completed_date` + `last_completed` dual-write window is preserved. On-disk YAML and CLI JSON output must stay byte-identical because `libtime.DateOrDateTime` emits the same text via its own `MarshalText` / `MarshalJSON`. Add a `TypedDateRoundTrip` test and a golden-file YAML test. `formatDateOrDateTime` is NOT removed in this prompt — Goal/Objective/Theme still depend on it.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` for Ginkgo v2 / Gomega and coverage rules.

PRECONDITION: prompt `1-reader-side-date-multi-shape` must already be shipped — `FrontmatterMap.GetTime` must already handle the `libtime.DateOrDateTime` arm. If `grep -n 'case libtime.DateOrDateTime:' pkg/domain/frontmatter_map.go` returns no match, STOP and report `status: failed` with message `"GetTime DateOrDateTime arm not yet deployed (prompt 1)"`. Without it, the round-trip test will fail because getters read back through `GetTime`.

Read these files in full before changing anything:
- `/workspace/pkg/domain/task_frontmatter.go` — the six setters (`SetLastCompletedDate` 216-225, `SetCompletedDate` 245-251, `SetCreatedDate` 254-260, `SetDeferDate` 294-300, `SetPlannedDate` 303-309, `SetDueDate` 312-318), `GetField` (323-369), `LastCompleted` (121-123), `formatDateOrDateTime` (488-495), `formatTimeAsDate` (478-486), and the getters (`CompletedDate` 128+, `DeferDate`, `PlannedDate`, `DueDate`, `CreatedDate`, `LastCompletedDate`).
- `/workspace/pkg/domain/task_frontmatter_test.go` — existing Ginkgo `Describe`/`It` style in package `domain_test`.
- `/workspace/pkg/storage/base.go` lines 54-80 — `serializeMapAsFrontmatter` calls `yaml.Marshal(data)` on the raw map. This is WHY storing a typed value keeps YAML byte-identical: `libtime.DateOrDateTime` implements `MarshalText` (verified in `github.com/bborbe/time@v1.27.1/time_date-or-date-time.go:201`), and yaml.v3 emits via `MarshalText`, producing the same `YYYY-MM-DD` text that `formatDateOrDateTime` produced.

KEY FACTS (verified in source):
- `libtime.DateOrDateTime` has value-receiver `MarshalText() ([]byte, error)` and `MarshalJSON() ([]byte, error)` and `String() string`. `String()` emits `YYYY-MM-DD` for midnight-UTC values, else RFC3339Nano.
- `f.Set(key, value any)` (`pkg/domain/frontmatter_map.go:110`) and `f.Delete(key)` (line 122) are the storage primitives.
- `formatDateOrDateTime` in `task_frontmatter.go` (488-495) calls `formatTimeAsDate(d.Time())`. It is STILL USED by Goal/Objective/Theme `GetField` and is NOT deleted in this prompt.
</context>

<requirements>

### 1. Change the five simple Task date setters to store the typed value

In `pkg/domain/task_frontmatter.go`, change the body of each setter so the non-nil branch stores `*d` (the dereferenced typed value) instead of `formatDateOrDateTime(d)`. The nil branch (Delete) is unchanged.

- `SetDeferDate` (294-300): `f.Set("defer_date", formatDateOrDateTime(d))` → `f.Set("defer_date", *d)`
- `SetPlannedDate` (303-309): `f.Set("planned_date", formatDateOrDateTime(d))` → `f.Set("planned_date", *d)`
- `SetDueDate` (312-318): `f.Set("due_date", formatDateOrDateTime(d))` → `f.Set("due_date", *d)`
- `SetCompletedDate` (245-251): `f.Set("completed_date", formatDateOrDateTime(d))` → `f.Set("completed_date", *d)`
- `SetCreatedDate` (254-260): `f.Set("created_date", formatDateOrDateTime(d))` → `f.Set("created_date", *d)`

### 2. Change `SetLastCompletedDate` to store typed values to both keys (preserve dual-write)

`SetLastCompletedDate` (216-225) currently computes `formatted := formatDateOrDateTime(d)` and sets both keys to the string. Change the non-nil branch to store the typed value to both keys:

```go
func (f *TaskFrontmatter) SetLastCompletedDate(d *libtime.DateOrDateTime) {
	if d == nil {
		f.Delete("last_completed_date")
		f.Delete("last_completed")
		return
	}
	f.Set("last_completed_date", *d)
	f.Set("last_completed", *d) // dual-write window
}
```

The dual-write to both `last_completed_date` (canonical) and `last_completed` (legacy) MUST be preserved. Do NOT touch `SetLastCompleted(v string)` (227-242) — it delegates to `SetLastCompletedDate` and is unchanged.

### 3. Do NOT change `GetField` or `LastCompleted` in this prompt

The `GetField` date arms (340, 356-364) and `LastCompleted` (121-123) still call `formatDateOrDateTime`. This continues to work because `formatDateOrDateTime` is NOT deleted in this prompt. Leave them exactly as they are. They are migrated in the next prompt when the helper is deleted.

### 4. Do NOT delete `formatDateOrDateTime`

`formatDateOrDateTime` (488-495) stays in place — Goal, Objective, and Theme `GetField` still call it. `formatTimeAsDate` (478-486) also stays unchanged. Removal of `formatDateOrDateTime` happens in the next prompt.

### 5. Add a `TypedDateRoundTrip` test (AC #5)

Add a Ginkgo block whose description string contains `TypedDateRoundTrip` (so `go test -run TypedDateRoundTrip` selects it) to `pkg/domain/task_frontmatter_test.go`. For each task date field, construct a `*libtime.DateOrDateTime`, call the setter, then call the matching getter WITHOUT any YAML round-trip, and assert the returned value equals the input instant:

- `SetDeferDate(d)` then `DeferDate()` → non-nil, `.Time()` equals `d.Time()`
- `SetPlannedDate(d)` then `PlannedDate()` → equal
- `SetDueDate(d)` then `DueDate()` → equal
- `SetCompletedDate(d)` then `CompletedDate()` → equal
- `SetCreatedDate(d)` then `CreatedDate()` → equal
- `SetLastCompletedDate(d)` then `LastCompletedDate()` → equal, AND the legacy `last_completed` key (via `Get("last_completed")`) is also populated (dual-write check)

Construct the input from a fixed instant, e.g.:
```go
t := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
d := libtime.DateOrDateTime(t).Ptr()
```
(`Ptr()` is a value-receiver method on `DateOrDateTime` returning `*DateOrDateTime`, verified in `time_date-or-date-time.go:150`.) Use a fixed instant — never the wall clock.

### 6. Add a golden-file YAML test (AC #10)

Create `pkg/domain/testdata/task_frontmatter_golden.yaml` and a test that serializes a `TaskFrontmatter` with all date fields set and compares the marshaled YAML byte-for-byte to the golden file.

- Build a `TaskFrontmatter` via `domain.NewTaskFrontmatter(...)` (read the constructor signature in `task_frontmatter.go` first), set every date field with fixed instants (use `time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)` for date-only fields), and set a couple of non-date fields (e.g. status, task_identifier) so the golden file is realistic.
- Marshal via `yaml.Marshal(tf.RawMap())` (import the same yaml package the codebase uses — check `pkg/storage/base.go` imports; it is `gopkg.in/yaml.v3`).
- Compare to the golden file read via `os.ReadFile("testdata/task_frontmatter_golden.yaml")` with `Expect(string(got)).To(Equal(string(want)))`.
- GENERATE the golden file from the NEW implementation's output, then MANUALLY INSPECT it before committing: every date field must appear as `YYYY-MM-DD` (e.g. `defer_date: 2026-12-01`), NOT as a quoted string and NOT as an RFC3339 datetime (since all fixtures are midnight UTC). If any date field is wrapped in quotes or has a `T00:00:00Z` suffix, the typed storage path is wrong — investigate before committing. The golden file is the recorded output of the new code, not a pre-change baseline.

### 7. Iterative verification

Run `make test` from the repo root after editing. Confirm all existing `pkg/domain/...` tests still pass unchanged. Do NOT run `make precommit` until the final step.

</requirements>

<constraints>
- Public setter and reader signatures MUST NOT change.
- The `last_completed_date` + `last_completed` dual-write window MUST be preserved — both keys written on set, both deleted on nil.
- On-disk YAML for tasks MUST stay byte-identical for the same input — this is guaranteed by `MarshalText` on `libtime.DateOrDateTime` matching the old `formatDateOrDateTime` output for midnight-UTC values. Do NOT add `MarshalYAML`/`UnmarshalYAML` to `bborbe/time`.
- Do NOT delete or modify `formatDateOrDateTime` or `formatTimeAsDate` in this prompt.
- Do NOT change `GetField` or `LastCompleted` in this prompt.
- Do NOT change Goal, Objective, or Theme — they are migrated in the next prompt.
- Do NOT add a feature flag / opt-out for the typed-storage path.
- Tests use Ginkgo v2 / Gomega in package `domain_test`. No Counterfeiter mocks needed.
- Use fixed `time.Time` literals in all tests — never the wall clock.
- Coverage for the changed setters MUST stay ≥80% per `docs/definition-of-done.md`.
- `make precommit` MUST stay green from the repo root.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. The five simple Task setters store the typed value, not the stringified one
grep -n 'f.Set("defer_date", \*d)\|f.Set("planned_date", \*d)\|f.Set("due_date", \*d)\|f.Set("completed_date", \*d)\|f.Set("created_date", \*d)' pkg/domain/task_frontmatter.go
# Expected: 5 matches

# 2. SetLastCompletedDate dual-writes the typed value to both keys
grep -n 'f.Set("last_completed_date", \*d)\|f.Set("last_completed", \*d)' pkg/domain/task_frontmatter.go
# Expected: 2 matches

# 3. formatDateOrDateTime still exists (NOT deleted in this prompt — Goal/Objective/Theme need it)
grep -n 'func formatDateOrDateTime' pkg/domain/task_frontmatter.go
# Expected: 1 match

# 4. formatTimeAsDate unchanged
grep -n 'func formatTimeAsDate' pkg/domain/task_frontmatter.go
# Expected: 1 match

# 5. AC #5: TypedDateRoundTrip test passes
go test -v ./pkg/domain/... -run TypedDateRoundTrip
# Expected: PASS

# 6. AC #10: golden file exists and date fields are YYYY-MM-DD (unquoted, no T-suffix)
grep -nE '^(defer_date|planned_date|due_date|completed_date|created_date|last_completed|last_completed_date): 2026-12-01$' pkg/domain/testdata/task_frontmatter_golden.yaml
# Expected: one line per date field; values are bare 2026-12-01 with no quotes and no T00:00:00Z

# 7. AC #4: all pkg/domain and pkg/ops tests still pass
go test ./pkg/domain/... ./pkg/ops/...
# Expected: ok

# 8. Coverage of changed setters
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/domain/... && go tool cover -func=/tmp/cover.out | grep -iE 'SetDeferDate|SetPlannedDate|SetDueDate|SetCompletedDate|SetCreatedDate|SetLastCompletedDate'
# Expected: each at 100%
```
</verification>

<!-- DARK-FACTORY-REPORT -->
