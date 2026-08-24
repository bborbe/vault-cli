---
status: completed
spec: [036-passive-per-task-metrics]
summary: 'Verified and completed the passive metrics frontmatter contract: MetricsSession/MetricsCycle types and all eleven accessors (sessions, completed_at, interaction_count, cycles) with multi-shape coercion, accumulate-never-overwrite appends, and unknown-vs-zero distinction; added the missing fresh-frontmatter nil-assertions spec; boundary check confirmed the round-trip spec fails on a broken yaml tag; make precommit exits 0'
execution_id: vault-cli-exec-188-spec-036-metrics-frontmatter
dark-factory-version: dev
created: "2026-08-24T18:30:00Z"
queued: "2026-08-24T18:31:32Z"
started: "2026-08-24T19:45:14Z"
completed: "2026-08-24T19:49:43Z"
branch: dark-factory/passive-per-task-metrics
---

<summary>
- Task frontmatter gains four passive metrics fields: `metrics_sessions` (a list), `metrics_completed_at` (a timestamp), `metrics_interaction_count` (an integer), and `metrics_cycles` (a list of archived recurring-cycle aggregates).
- Each `metrics_sessions` entry records one work-on run: a session id and a start timestamp.
- Each `metrics_cycles` entry records a finished recurring cycle: start timestamp, end timestamp, and interaction count.
- Appending a session entry keeps every prior entry — accumulation, never overwrite.
- Values already present in a task file survive every read-write round-trip through vault-cli's YAML frontmatter serializer.
- An absent `metrics_interaction_count` or `metrics_completed_at` reads back distinctly from a stored value — "unknown" is never forged as zero.
- Malformed or hand-edited `metrics_*` values are treated as absent rather than causing a crash or a wrong number.
- No CLI, ops, or storage behavior changes yet — this prompt only establishes the data contract the next three prompts build on.
</summary>

<objective>
Add typed frontmatter accessors to `TaskFrontmatter` for the four metrics fields — `metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles` — so the work-on and complete hooks (subsequent prompts) can read and write them with the same round-trip safety, multi-shape tolerance, and unknown-vs-zero distinction every other frontmatter field already has.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done (coverage rules, changelog rules), and `/workspace/docs/development-patterns.md` for the entity/frontmatter layering.

Read these files fully before making changes:

- `/workspace/pkg/domain/frontmatter_map.go` — the `FrontmatterMap` raw accessor family (`Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap`). The metrics accessors build on `Get`/`Set`/`Delete` and reuse `GetTime`'s multi-shape coercion pattern (handles `time.Time`, `libtime.DateOrDateTime`, and string). Verified current shapes:
  ```go
  type FrontmatterMap struct {
      data map[string]any
  }
  func (f FrontmatterMap) Get(key string) any
  func (f *FrontmatterMap) Set(key string, value any)
  func (f *FrontmatterMap) Delete(key string)
  func (f FrontmatterMap) RawMap() map[string]any
  func (f FrontmatterMap) GetTime(key string) *time.Time
  ```
- `/workspace/pkg/domain/task_frontmatter.go` — `TaskFrontmatter` embeds `FrontmatterMap`; the accessor convention is value receivers for readers (`func (f TaskFrontmatter) ...`), pointer receivers for writers (`func (f *TaskFrontmatter) ...`), nil deletes the key. Verified exemplars to mirror:
  ```go
  func (f TaskFrontmatter) ClaudeSessionID() string { return f.GetString("claude_session_id") }
  func (f *TaskFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }
  func (f TaskFrontmatter) CompletedDate() *libtime.DateOrDateTime {
      t := f.GetTime("completed_date")
      if t == nil { return nil }
      d := libtime.DateOrDateTime(*t)
      return &d
  }
  func (f *TaskFrontmatter) SetCompletedDate(d *libtime.DateOrDateTime) {
      if d == nil { f.Delete("completed_date"); return }
      f.Set("completed_date", *d)
  }
  ```
- `/workspace/pkg/storage/base.go` — how frontmatter actually round-trips. `parseToFrontmatterMap` runs `yaml.Unmarshal` over the frontmatter block into `map[string]any`; `serializeMapAsFrontmatter` runs `yaml.Marshal(task.RawMap())`. Confirmed: any value stored in the map (including a `[]MetricsSession` or `[]MetricsCycle`) is marshaled as YAML and re-parsed as `[]any` of `map[string]any` — so every metrics reader must coerce both the in-memory typed shape and the on-disk generic shape.
- `/workspace/pkg/domain/frontmatter_map_test.go` and `/workspace/pkg/domain/task_frontmatter_test.go` — Ginkgo v2 / Gomega suites, `package domain_test`, for the new tests.
- `/workspace/pkg/domain/domain_suite_test.go` — exists; new specs in the package run automatically.

Verified library facts (do not re-derive):
- `libtime.DateOrDateTime` (`github.com/bborbe/time`, `gopkg.in/yaml.v3` handles it via `encoding.TextMarshaler`/`TextUnmarshaler`): `func (d DateOrDateTime) String() string`, `func (d DateOrDateTime) Time() time.Time`. It is a `stdtime.Time` defined type with `MarshalText`/`UnmarshalText`, so yaml.v3 serializes a `DateOrDateTime` struct field as a string and parses it back — this is exactly how `completed_date` already round-trips.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external `_test` packages, coverage.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface/constructor/struct conventions.
</context>

<requirements>

## 1. New domain types

Create `/workspace/pkg/domain/metrics.go`:

```go
package domain

import (
	libtime "github.com/bborbe/time"
)

// MetricsSession records one work-on run: the Claude session it started and when.
type MetricsSession struct {
	SessionID string                 `yaml:"session_id" json:"session_id"`
	StartedAt libtime.DateOrDateTime `yaml:"started_at" json:"started_at"`
}

// MetricsCycle archives one finished cycle of a recurring task: when it ran,
// when it ended, and how many user interactions it had.
type MetricsCycle struct {
	StartedAt        libtime.DateOrDateTime `yaml:"started_at" json:"started_at"`
	CompletedAt      libtime.DateOrDateTime `yaml:"completed_at" json:"completed_at"`
	InteractionCount int                    `yaml:"interaction_count" json:"interaction_count"`
}
```

Write it with a license header matching the existing files, GoDoc comments on both types (start with the type name), and the exact yaml tags shown. Do NOT add a `Validate` method, a `Ptr` helper, or any other method — these are passive derived-data records.

## 2. New accessor file

Create `/workspace/pkg/domain/task_frontmatter_metrics.go` in `package domain`. It defines the metrics accessors on `TaskFrontmatter` with the exact signatures below. Follow the repo conventions: value receivers for readers, pointer receivers for writers, `f.Delete(key)` (not `Set(key, "")`) when clearing, `errors.Wrapf(ctx, validation.Error, ...)` only where an error is actually produced (the readers below produce none).

```go
// MetricsSessions reads the "metrics_sessions" field as a slice of MetricsSession.
// Returns nil when the key is absent, the value is not a list, or every element is malformed.
func (f TaskFrontmatter) MetricsSessions() []MetricsSession

// AppendMetricsSession appends one entry to "metrics_sessions", preserving every prior entry.
// A nil or empty SessionID is ignored (not appended).
func (f *TaskFrontmatter) AppendMetricsSession(entry MetricsSession)

// ClearMetricsSessions removes the "metrics_sessions" key entirely.
func (f *TaskFrontmatter) ClearMetricsSessions()

// MetricsCompletedAt reads "metrics_completed_at" as *libtime.DateOrDateTime.
// Returns nil when the key is absent or the value is unparseable.
func (f TaskFrontmatter) MetricsCompletedAt() *libtime.DateOrDateTime

// SetMetricsCompletedAt stores "metrics_completed_at". A nil argument deletes the key.
func (f *TaskFrontmatter) SetMetricsCompletedAt(d *libtime.DateOrDateTime)

// ClearMetricsCompletedAt removes the "metrics_completed_at" key entirely.
func (f *TaskFrontmatter) ClearMetricsCompletedAt()

// MetricsInteractionCount reads "metrics_interaction_count" as *int.
// Returns nil when the key is absent or the value is not a number — "unknown" must never be forged as 0.
func (f TaskFrontmatter) MetricsInteractionCount() *int

// SetMetricsInteractionCount stores "metrics_interaction_count".
func (f *TaskFrontmatter) SetMetricsInteractionCount(count int)

// ClearMetricsInteractionCount removes the "metrics_interaction_count" key entirely.
func (f *TaskFrontmatter) ClearMetricsInteractionCount()

// MetricsCycles reads "metrics_cycles" as a slice of MetricsCycle.
// Returns nil when the key is absent, the value is not a list, or every element is malformed.
func (f TaskFrontmatter) MetricsCycles() []MetricsCycle

// AppendMetricsCycle appends one archived cycle to "metrics_cycles", preserving every prior entry.
func (f *TaskFrontmatter) AppendMetricsCycle(cycle MetricsCycle)
```

### Coercion rules (the shared helpers inside this file)

**Reading a session/cycle list** — `f.Get(key)` returns:
- `[]MetricsSession` / `[]MetricsCycle` (our own in-memory set) → return directly.
- `[]any` (re-parsed from a YAML file) → coerce element by element:
  - non-`map[string]any` element → skip.
  - `session_id` (or `interaction_count`/`started_at`/`completed_at` as applicable): `session_id` uses stringification like `GetString` (string passes through, otherwise `fmt.Sprintf`); an empty `session_id` → skip the whole entry. `started_at`/`completed_at` reuse the `GetTime` coercion shape (`time.Time` → `DateOrDateTime(t.Time())`, `libtime.DateOrDateTime` → as-is, string → `libtime.ParseTime`, anything else / parse failure → zero `DateOrDateTime` — do not drop the entry, a zero time is a data-quality matter, not malformed). `interaction_count` accepts `int`, `int64`, `float64`, and numeric strings; anything unparseable reads as `0`.
- anything else (string, map, number, nil) → `nil` (treated as absent — never a panic, never a fabricated value).

**Writing** — `AppendMetricsSession`/`AppendMetricsCycle` read the current list with the reader above, append the new entry, and `f.Set(key, slice)`. An `AppendMetricsSession` with empty `SessionID` must not append. `ClearMetricsSessions`/`ClearMetricsCompletedAt`/`ClearMetricsInteractionCount` call `f.Delete(key)`.

**`MetricsInteractionCount`** — raw value via `f.Get("metrics_interaction_count")`; `nil` → nil; `int`/`int64`/`float64` → pointer to the int value; numeric `string` → `strconv.Atoi` (failure → nil); anything else → nil.

**`MetricsCompletedAt`** — exactly the `CompletedDate()` shape: `t := f.GetTime("metrics_completed_at"); if t == nil { return nil }; d := libtime.DateOrDateTime(*t); return &d`.

Do NOT add `metrics_*` cases to `GetField`/`SetField` in `/workspace/pkg/domain/task_frontmatter.go` — the metrics fields are written passively by vault-cli and are deliberately not hand-editable via `task set`; they fall through the default branch of both methods today and must keep doing so. Do NOT add any other method to the file.

## 3. Tests

Create `/workspace/pkg/domain/task_frontmatter_metrics_test.go`, `package domain_test`, Ginkgo v2 / Gomega, mirroring the existing accessor suites. Cover at minimum:

1. **Append preserves prior entries**: two `AppendMetricsSession` calls yield two entries; entry 0's `SessionID`/`StartedAt` are byte-identical after the second append.
2. **YAML round-trip through the real serializer shape** (the serialization boundary): build a `TaskFrontmatter`, `AppendMetricsSession` twice, `SetMetricsCompletedAt`, `SetMetricsInteractionCount`, `AppendMetricsCycle`; marshal `yaml.Marshal(f.RawMap())`; unmarshal back into `map[string]any`; `NewTaskFrontmatter`; assert every accessor reads back the original values, and assert the marshaled YAML contains the expected keys (`metrics_sessions:`, `session_id:`, `started_at:`, `metrics_completed_at:`, `metrics_interaction_count:`, `metrics_cycles:`, `completed_at:`, `interaction_count:`).
3. **On-disk generic shape reads**: feed `map[string]any{"metrics_sessions": []any{map[string]any{"session_id": "s1", "started_at": <time.Time>}, map[string]any{"session_id": "s2", "started_at": "2026-08-24T18:14:35Z"}}}` and assert the typed `[]MetricsSession` result.
4. **Absent semantics**: a fresh `NewTaskFrontmatter(nil)` returns nil for all four readers; `MetricsInteractionCount()` nil; `MetricsCompletedAt()` nil.
5. **Malformed treated as absent**: `metrics_sessions: "not-a-list"` → nil; `metrics_interaction_count: "abc"` → nil; an entry with an empty `session_id` → skipped; a `metrics_sessions` value that is a single `map[string]any` (not a list) → nil. An entry with `started_at: "not-a-time"` is **retained** with a zero `DateOrDateTime` (parse failure → zero time, entry preserved — the deliberate coercion rule in requirements step 2, not "absent").
6. **Unknown vs zero**: `SetMetricsInteractionCount(0)` → `MetricsInteractionCount()` returns a non-nil pointer whose value is 0; absent key → nil.
7. **Clearers delete**: after `SetMetricsCompletedAt(&d)` then `ClearMetricsCompletedAt()`, `MetricsCompletedAt()` is nil and `f.Get("metrics_completed_at")` is nil; same for `ClearMetricsSessions` and `ClearMetricsInteractionCount`.

`pkg/domain` is the changed package. The new file must reach ≥80% statement coverage from these tests (run `go test -coverprofile=/tmp/cover.out ./pkg/domain/ && go tool cover -func=/tmp/cover.out` to check the `task_frontmatter_metrics.go` lines). Do NOT add retroactive coverage to unrelated untested `pkg/domain` code.

## 4. Boundary check — every new value crosses the YAML serializer

Requirement 2's test (YAML round-trip through `RawMap()`) is the serialization-boundary test: it proves a `MetricsSession`/`MetricsCycle`/`metrics_completed_at`/`metrics_interaction_count` written by the accessors survives the exact marshal→unmarshal path vault-cli's storage layer runs on every task write. It must FAIL (lose the values) if the yaml tags or the multi-shape readers are wrong — a shape-only assertion (e.g. `Expect(f.MetricsSessions()).To(HaveLen(2))`) does not satisfy this. Verify by hand once: temporarily break a yaml tag (e.g. `yaml:"session_id"` → `yaml:"session"`), run the round-trip spec, confirm it fails, restore the tag.

## 5. Changelog — NOT this prompt

Do NOT add a CHANGELOG entry. Prompt `4-spec-036-plugin-docs-release.md` owns the single `## Unreleased` bullet for this feature. Do NOT touch `/workspace/CHANGELOG.md`, `/workspace/pkg/ops/`, `/workspace/pkg/cli/`, `/workspace/pkg/storage/`, `/workspace/commands/`, or `/workspace/mocks/` in this prompt.

</requirements>

<constraints>
- Metrics land only in the task frontmatter fields `metrics_sessions`, `metrics_completed_at`, `metrics_interaction_count`, and `metrics_cycles`. No new service, no new datastore, no Prometheus sink.
- All timestamps are `libtime.DateOrDateTime` (ISO 8601 with timezone offset), consistent with the existing date-time fields; the injected clock (`libtime.CurrentDateTime`) is owned by the ops layer, NOT this prompt.
- Accumulate, never overwrite: `AppendMetricsSession`/`AppendMetricsCycle` must not truncate prior entries. The only path that clears the active accumulator is recurring-task completion, which is a later prompt — do not build a clear-all helper here beyond the per-key `Clear*` methods.
- Do NOT add an opt-out flag, config key, or environment variable for metrics recording — passive recording is the invariant.
- Do NOT change existing accessors (`Get`, `GetString`, `GetTime`, `GetStringSlice`, `Set`, `Delete`, `Keys`, `RawMap`) or any existing `TaskFrontmatter` method.
- Do NOT wire the accessors into `GetField`/`SetField` — see requirement 2. Hand-editing the metrics fields via `task set` must stay impossible.
- Tests use Ginkgo v2 / Gomega in the external `domain_test` package. `pkg/domain/domain_suite_test.go` already exists.
- `pkg/` is a library layer — no `fmt.Print*`, no `os.Stdout` writes. `fmt.Sprintf` (for stringification) is fine.
- Errors are wrapped with `github.com/bborbe/errors`; never `fmt.Errorf` for new code. The readers in this file produce no errors.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string, do NOT create a git tag (`.dark-factory.yaml` has `autoRelease: false`; `.maintainer.yaml` has `autoRelease: true` — the github-releaser-agent owns version bumps on merge, never this prompt).
- Existing tests must still pass, unmodified.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**
```
make test
```
Must exit 0.

**2. The types and accessors exist** — each command must print exactly one line:
```
grep -n 'type MetricsSession struct' pkg/domain/metrics.go
grep -n 'type MetricsCycle struct' pkg/domain/metrics.go
grep -n 'func (f TaskFrontmatter) MetricsSessions() \[\]MetricsSession' pkg/domain/task_frontmatter_metrics.go
grep -n 'func (f \*TaskFrontmatter) AppendMetricsSession(entry MetricsSession)' pkg/domain/task_frontmatter_metrics.go
grep -n 'func (f TaskFrontmatter) MetricsInteractionCount() \*int' pkg/domain/task_frontmatter_metrics.go
grep -n 'func (f TaskFrontmatter) MetricsCycles() \[\]MetricsCycle' pkg/domain/task_frontmatter_metrics.go
```

**3. Named test specs are present and pass** — each command must print a number `>= 1`:
```
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "YAML round-trip"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "preserves prior entries"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "unknown.*never forged"
go test -count=1 ./pkg/domain/... -v -ginkgo.v 2>&1 | grep -c "malformed.*absent"
```
The `-ginkgo.v` flag is required — plain `go test -v` does not enable Ginkgo's verbose reporter. Pick spec `It`/`Describe` descriptions that match these patterns (adjust the grep patterns to your chosen descriptions and state them in the test file so the verification is reproducible).

**4. GetField/SetField are untouched** — the default-branch fall-through is still there:
```
grep -c 'f.Set(key, value)' pkg/domain/task_frontmatter.go
grep -c 'return f.GetString(key)' pkg/domain/task_frontmatter.go
```
Each must print exactly `1` (the pre-existing default branches).

**5. No ops/cli/storage/commands change** — each must print `0`:
```
grep -rc 'metrics_' pkg/ops/ pkg/cli/ pkg/storage/ commands/ | grep -v ':0' | wc -l
```
Must print `0` lines (no file under those trees references `metrics_` yet).

**6. Coverage for the new accessor file:**
```
go test -coverprofile=/tmp/cover.out ./pkg/domain/... && go tool cover -func=/tmp/cover.out | grep task_frontmatter_metrics.go
```
Must report `>= 80.0%`.

**7. Full gate, once, at the end:**
```
make precommit
```
Must exit 0. If it fails, fix the issue and re-run only the failing target (`make lint`, `make format`, etc.) until it passes, then run `make precommit` one final time.
</verification>
