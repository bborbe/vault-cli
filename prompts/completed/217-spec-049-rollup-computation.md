---
status: completed
spec: [049-weekly-rollup]
summary: Added ListTasksStrict to the storage layer and built the weekly rollup computation (week resolution, three figures, family medians, no-data/no-counts states) in pkg/ops/rollup_weekly.go with a 13-spec Ginkgo suite at 94.1% lowest per-function coverage
execution_id: vault-cli-weekly-rollup-exec-217-spec-049-rollup-computation
dark-factory-version: v0.196.0
created: "2026-09-16T12:57:14Z"
queued: "2026-09-25T18:16:58Z"
started: "2026-09-25T19:03:53Z"
completed: "2026-09-25T19:10:25Z"
---

# Weekly rollup computation: week membership, the three figures, family medians (spec 049, prompt 1 of 2)

<summary>
- The library gains a weekly-rollup computation: given one vault and one week, it produces three figures and the per-family breakdown behind them.
- A week is named as a year and week number, or resolved from the clock when the caller names none — the last complete week.
- A task belongs to a week by the local date inside its own completion timestamp, so a task finishing just after local midnight lands in its own local week, not the previous UTC one.
- The three figures are: the week's total human interactions, its unattended deliveries, and the median interactions of a recurring task family.
- An unattended delivery is a completed task whose recorded interaction count is exactly zero; a task whose count was never recorded is neither a delivery nor a zero.
- A family with no recorded count reports "undefined" instead of zero and contributes nothing to the headline median — a missing measurement is never rendered as a measurement of nothing.
- A week with no completions reports "no data"; a week with completions but no recorded counts says "no recorded counts" — neither case ever prints a zero.
- Families are listed in a stable sorted order, so two runs over the same vault produce byte-identical output.
- A task file that cannot be read aborts the computation with an error naming the file, instead of silently reporting a smaller week.
- No command, output format, or existing behaviour changes yet — this prompt establishes the computation and its tests; the visible command lands in prompt 2.
</summary>

<objective>
Build the weekly rollup computation in the operations layer: resolve one ISO week, select the vault's task files whose recorded completion timestamp falls in that week by its own local offset, and derive the three figures — total human interactions, unattended deliveries, and the headline median of per-family medians — with the vault's written unattended rule and its written filename-stem grouping rule applied literally. This is the machine the spec's headline question needs; without it the vault records interaction counts that nothing reads, and every offload decision stays a feeling.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/development-patterns.md` (the layering: `pkg/domain` types, `pkg/storage` reads, `pkg/ops` operations, `pkg/cli` formatting) and `docs/dod.md` (the Definition of Done).

Read these files fully before making changes:

- `pkg/storage/storage.go` — the `TaskStorage` interface (`WriteTask`, `FindTaskByName`, `ListTasks`), the counterfeiter directives above each interface, and the constructor block (`NewTaskStorage`, `NewGoalStorage`, …). `storage.Config` is built from a vault by `NewConfigFromVault`, which is where `tasks_dir` enters.
- `pkg/storage/task.go` — `ListTasks` is the walk you extend. It calls `filepath.WalkDir(tasksDir, …)`, keeps only `*.md`, trims the `.md` suffix to get the task name, and — the one line that matters here — on a per-file read error it logs `slog.Debug("skipping unreadable task", "file", fileName, "error", err)` and returns `nil`, so the file silently leaves the result set. That is the behaviour this prompt must keep for `ListTasks` and invert for the new method.
- `pkg/storage/base.go` — `readEntityComponentsFromPath` is the shared read: it refuses a symlink resolving outside the vault, `os.ReadFile`s, and calls `parseToFrontmatterMap`, which runs `quoteBareWikilinks` before `yaml.Unmarshal`. The rollup must go through this path, never through a line grep — a bare `metrics_interaction_count: 0` inside a task *body* would match a grep and over-count.
- `pkg/domain/task_frontmatter_metrics.go` — the accessors you consume. Verified signatures:
  ```go
  // MetricsCompletedAt reads "metrics_completed_at" as *libtime.DateOrDateTime.
  // Returns nil when the key is absent or the value is unparseable.
  func (f TaskFrontmatter) MetricsCompletedAt() *libtime.DateOrDateTime

  // MetricsInteractionCount reads "metrics_interaction_count" as *int.
  // Returns nil when the key is absent or the value is not a number — "unknown"
  // must never be forged as 0.
  func (f TaskFrontmatter) MetricsInteractionCount() *int
  ```
  `MetricsInteractionCount()` returning a non-nil pointer whose value is `0` is a *recorded* zero; `nil` is an absent-or-malformed count. Reuse these two accessors — do NOT read `metrics_interaction_count` from `RawMap()` or `Get()` directly.
- `pkg/domain/task_frontmatter.go` — `func (f TaskFrontmatter) Status() TaskStatus` (the only frontmatter accessor you need beyond the two metrics ones). `pkg/domain/task_status.go` — `TaskStatusCompleted TaskStatus = "completed"`. `pkg/domain/file_metadata.go` — `type FileMetadata struct { Name string; FilePath string; ModifiedDate *time.Time }`, where `Name` is the filename without the `.md` extension. `domain.Task` embeds both `TaskFrontmatter` and `FileMetadata`, so a task's family stem is the **field** `task.Name` — there is no `Name()` method on `TaskFrontmatter`.
- `pkg/ops/complete.go` — the Interface → Constructor → Struct → Method shape every operation follows, and how `libtime.CurrentDateTime` is injected (`currentDateTime libtime.CurrentDateTime` field, `currentDateTime.Now()` at the point of use).
- `pkg/ops/errors.go` — where package-level sentinel errors live.
- `pkg/ops/ops_suite_test.go` — the package's ONLY `func TestSuite` calling `RunSpecs`. Do not add a second one; a second `RunSpecs` panics.
- `pkg/ops/export_test.go` — the test-only export file (`package ops`, `_test.go` suffix, visible to `ops_test`). `SessionTurnTimeout` and `DefaultSessionLockDir` are its existing entries.
- `pkg/ops/lint_test.go` — the fixture style for an operation that reads a whole directory: `os.MkdirTemp` for the vault, `os.MkdirAll(filepath.Join(vaultPath, tasksDir))`, `os.WriteFile(taskPath, []byte(content), 0600)` with a raw `---`-delimited frontmatter block, and an `AfterEach` that removes the temp tree.
- `pkg/ops/decision_ack_test.go` — the injected-clock style: `libtime.NewCurrentDateTime()` then `currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-16T12:00:00Z"))`, imported as `libtime "github.com/bborbe/time"` and `libtimetest "github.com/bborbe/time/test"`.
- `pkg/storage/task_test.go` — the storage-package suite shape (`package storage_test`, Ginkgo v2, temp vault fixtures). Read-only reference: you do not extend it, because the strict listing's behaviour is pinned through the operation's suite, which drives the real storage against a real temp vault.
- `specs/in-progress/049-weekly-rollup.md` — the spec. Read Desired Behavior, Constraints, Failure Modes, Security/Abuse and Non-goals; every requirement below comes from them.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, …)` / `errors.Wrapf(ctx, err, …)` / `errors.Errorf(ctx, …)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err` for a newly constructed error, never `context.Background()` in `pkg/`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external `_test` packages, coverage expectations.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — Interface → Constructor → Struct → Method, doc comments on every exported identifier.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — why the clock is injected and never read from the process.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits this change must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20, plus the BSD license header on every new Go file.

Two environment facts:

1. **Make no git calls anywhere, including in `<verification>`.** The daemon does not check `<verification>` exit codes, so a git command that dies (`fatal: not a git repository`) reports a false pass. Every check below is git-free.
2. **The two rule sentences are consumed, not authored.** The unattended rule lives in the vault's `Unattended Execution` topic page and the grouping rule in `Recurring Task Automation Ranking` § Method. This prompt implements them; it does not reword them and does not add a config key, flag, or environment variable that changes either.
</context>

<requirements>

## 0. Scope — exactly five files, no others

- `pkg/storage/storage.go` — one new method on the existing `TaskStorage` interface.
- `pkg/storage/task.go` — the new method's implementation.
- `pkg/ops/rollup_weekly.go` — the new operation, its result types, and its two internal helpers.
- `pkg/ops/rollup_weekly_test.go` — the new Ginkgo suite.
- `pkg/ops/export_test.go` — two test-only aliases.

Nothing else hand-authored: the five files above are the whole authored diff. `mocks/task-storage.go` and `mocks/storage.go` change as regenerated output of `make generate` (a widened interface makes the old mock fail its compile-time assertion), and `mocks/rollup-weekly-operation.go` is generated by the new counterfeiter directive — do not hand-edit anything under `mocks/`. `pkg/cli/` is untouched (prompt 2 wires the command), `CHANGELOG.md` and `README.md` are untouched (prompt 2 owns both), `pkg/domain/` is untouched (the accessors you need already exist), `go.mod` is untouched (no new dependency), and no scenario file is added or changed.

## 1. `pkg/storage` — a listing that fails on an unreadable file

`ListTasks` deliberately swallows a per-file read error so a listing still exits 0. The rollup cannot accept that: a skipped file would silently shrink the week, which is the exact failure the spec's Failure Modes table forbids.

Add one method to the `TaskStorage` interface in `pkg/storage/storage.go`, with a doc comment that states the difference from `ListTasks`:

```go
ListTasksStrict(ctx context.Context, vaultPath string) ([]*domain.Task, error)
```

Implement it on `taskStorage` in `pkg/storage/task.go`, receiver `t` like every sibling method in that file (the `<verification>` grep pins the declaration as `func (t *taskStorage) ListTasksStrict`). Contract:

- Same read set as `ListTasks`: `filepath.Join(vaultPath, t.config.TasksDir)`, walked recursively, `.md` files only, the `.md` suffix trimmed to become the task name, every file parsed through the existing `readTaskFromPath` (which is what routes it through `parseToFrontmatterMap` and the bare-wikilink quoting pass).
- On a per-file read error, return the error instead of skipping — wrapped so the message names the file path, e.g. `errors.Wrapf(ctx, err, "read task file %s", path)`. The caller must be able to see which file failed.
- On a walk error (a missing `tasks_dir`, a permission-denied directory), return the wrapped walk error naming the directory. `filepath.WalkDir` already reports a missing root through the callback, so no separate existence check is needed — do not add one.
- Honour `ctx` cancellation the way the sibling code does.
- `ListTasks` must stay behaviourally identical, including its `slog.Debug("skipping unreadable task", …)` line. Share the walk between the two methods behind a private helper rather than duplicating twenty lines; the two differ only in what they do with a per-file read error.
- No new interface, no new constructor, no new file. `NewTaskStorage` keeps returning `storage.TaskStorage` and now satisfies the widened interface.

The generated mock (`mocks/task-storage.go`) and the composed `mocks/storage.go` are regenerated by `make generate`; do not hand-edit anything under `mocks/`.

## 2. `pkg/ops/rollup_weekly.go` — the operation

New file, `package ops`, BSD license header matching its siblings. Interface → Constructor → Struct → Method, one counterfeiter directive above the interface following the sibling pattern:

```go
//counterfeiter:generate -o ../../mocks/rollup-weekly-operation.go --fake-name RollupWeeklyOperation . RollupWeeklyOperation
```

```go
// RollupWeeklyOperation computes the weekly unattended-delivery rollup for one vault.
type RollupWeeklyOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		week string,
	) (RollupWeeklyResult, error)
}

// NewRollupWeeklyOperation creates a new weekly rollup operation.
func NewRollupWeeklyOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) RollupWeeklyOperation
```

- `vaultName` is carried for error messages only — the failure modes require the vault to be named alongside the failing directory. It is not used to build any path.
- `week` is the raw `--week` value; the empty string means "the last complete ISO week". Resolving it here (rather than in the CLI) keeps every week rule in one place and makes the default testable against an injected clock.
- `taskStorage` is `storage.TaskStorage` — the same injection shape as every other operation. Call `ListTasksStrict`, never `ListTasks`.
- `currentDateTime` is the only source of "now". `time.Now()` must not appear in the file.

### Result types

```go
// RollupWeeklyResult is the computed weekly rollup for one vault.
type RollupWeeklyResult struct {
	Year                 int            `json:"year"`
	Week                 int            `json:"week"`
	WeekStart            string         `json:"week_start"`
	WeekEnd              string         `json:"week_end"`
	HumanInteractions    string         `json:"human_interactions"`
	UnattendedDeliveries string         `json:"unattended_deliveries"`
	PerFamilyMedian      string         `json:"per_family_median"`
	Families             []RollupFamily `json:"families"`
}

// RollupFamily is one task family's median within the week's task set.
type RollupFamily struct {
	Name   string `json:"name"`
	Median string `json:"median"`
}
```

- `Year` and `Week` are the resolved ISO week; `WeekStart` and `WeekEnd` are the Monday and Sunday of that week as `2006-01-02` strings, computed in UTC.
- The three figure fields are **strings**, pre-rendered, and `Families` is a **slice, never a map**. That is what makes the JSON output carry the same characters the plain output prints, and what makes two runs byte-identical: a map's iteration order would not be.
- `Families` must be initialised to an empty slice, not left nil, so the JSON carries `[]` rather than `null`.
- Do NOT add a task count, a vault name, a rule string, a raw-count field, a per-task list, or any other field. The four week fields plus the three figures plus the family list are the whole surface.
- No `encoding/json` import is needed for these tags; do not add one.

### Internal helper names (frozen — the tests pin them)

```go
// rollupFamilyName reduces a task filename stem to its family key.
func rollupFamilyName(stem string) string

// rollupMedian returns the median of values.
func rollupMedian(values []float64) float64
```

## 3. Resolving the week

`Execute` resolves `week` into a year and a week number before doing anything else.

- Non-empty token: it must match `YYYY-Wnn` — exactly four digits, a literal `-`, a literal uppercase `W`, exactly two digits (so `2026-W37` and `2026-W05` are valid, and `2026-W5`, `2026-w37`, `2026W37`, `W37`, `2026-37` and `2026-W370` are not). On a mismatch return `errors.Errorf(ctx, "invalid --week %q: expected YYYY-Wnn, e.g. 2026-W37", week)` — a usage error naming the expected format, and no figures are produced. The command exits non-zero on it; the CLI layer does nothing further.
- The week number must be within that year's ISO week count. Compute it as `_, weeks := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()` and reject anything outside `1..weeks` with the same shaped error. Verified: 2025 has 52 ISO weeks, **2026 has 53**, 2027 has 52 — so `2026-W53` is valid and must not be rejected.
- Empty token: the last complete ISO week, taken from the injected clock — `prev := o.currentDateTime.Now().Time().AddDate(0, 0, -7)`, then `year, week := prev.ISOWeek()`. Subtracting seven days always lands in the week before the current one, whatever day of the week "now" is.
- The week's Monday is derived from the ISO definition: `jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)`, `offset := (int(jan4.Weekday()) + 6) % 7`, `monday := jan4.AddDate(0, 0, -offset+(week-1)*7)`, `sunday := monday.AddDate(0, 0, 6)`. Verified: `2026-W37` is `2026-09-07` to `2026-09-13`; `2026-W20` is `2026-05-11` to `2026-05-17`.

## 4. Which tasks are in the week

Call `taskStorage.ListTasksStrict(ctx, vaultPath)` once. Wrap a failure as `errors.Wrapf(ctx, err, "list tasks of vault %s", vaultName)` so the resulting message names both the vault and the failing path the storage layer reported.

A task belongs to the week when `task.MetricsCompletedAt()` is non-nil **and** the timestamp's own `ISOWeek()` equals the resolved `(year, week)`:

```go
year, week := task.MetricsCompletedAt().Time().ISOWeek()
```

- Use the timestamp's own location — do not convert to UTC first. A task completing `2026-09-14T00:30:00+02:00` is in `2026-W38` (local Monday); the same instant is Sunday in UTC and would land in `2026-W37`. This is the rule the spec's Failure Modes table states for "two tasks complete either side of a local midnight within the same UTC week", and it is why the conversion must not happen.
- A task with an absent or unparseable `metrics_completed_at` is in no week at all. `MetricsCompletedAt()` already returns `nil` for both, and a malformed value must be treated as absent — no crash, no coercion.
- Membership is independent of `status`: an `in_progress` or `aborted` task with a completion timestamp inside the week is in the set.

## 5. The three figures

Over the week's task set:

**Human interactions** — the sum of `*task.MetricsInteractionCount()` across the tasks whose count is non-nil, rendered as a decimal integer.

**Unattended deliveries** — the count of tasks in the set that satisfy *both* conditions:

```go
task.Status() == domain.TaskStatusCompleted && task.MetricsInteractionCount() != nil && *task.MetricsInteractionCount() == 0
```

A completed task whose count key is absent is indeterminate and is counted in neither direction — it is neither a delivery nor a human interaction. Zero means a *recorded* zero. A malformed (non-numeric) count reads as absent through the accessor and is likewise neither.

**Per-family median** — two levels, never a cross-category average of raw task values:

- *Per family*: group the week's tasks by `rollupFamilyName(task.Name)`. For one family, take the recorded counts of its members (the non-nil ones) and report their median. A family with **no** recorded count among its members reports `undefined` and contributes no datum to the headline.
- *Headline*: the median of the per-family medians, taken over the families whose median is defined. The headline aggregates family medians, never raw per-task values.
- Every family present in the week's set appears in `Families`, including the `undefined` ones, sorted ascending by `Name`. Sort explicitly — the group list must be byte-identical across runs.

## 6. Family grouping — the filename-stem rule, stated concretely

The vault's rule (`Recurring Task Automation Ranking` § Method) reads: *"strip dates, week numbers, versions and month names from each filename, then group"*, and the ≥3-instance threshold governs whether a family counts as recurring *evidence* — never membership. A one- or two-member family stays in the median. Do not implement the threshold as a filter.

`rollupFamilyName` takes the filename stem (`FileMetadata.Name`, i.e. the basename without `.md`) and returns the family key. Exact steps, in this order:

1. Lowercase the stem — normalization is case-insensitive, so `Check Unassigned Tasks` and `check unassigned tasks` are one family.
2. Strip, each replaced with a single space, in this order: an ISO date (`\d{4}-\d{2}-\d{2}`), an ISO year-and-week (`\d{4}-w\d{1,2}`), a bare week number (`\bw\d{1,2}\b`), a version (`\bv?\d+(\.\d+)+\b`), and a month name (`\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b`). Dates and versions must be stripped before the separator collapse in step 3, or the digits that make them recognisable are gone.
3. Collapse every run of non-alphanumeric characters to a single space — one regexp, `[^a-z0-9]+`. This is what makes `Check Unassigned Tasks - 2026-09-16` and `Check Unassigned Tasks 2026-09-16` the same family, and it also guarantees the key contains no colon.
4. Trim leading and trailing spaces.
5. If steps 1–4 left the key empty (a filename that was nothing but a date), fall back to the lowercased stem with step 3's collapse applied and no stripping, so the key is never blank.

The returned key is the printed and JSON-reported family name. Do not title-case it, do not keep the original spelling, and do not add a display name alongside it.

## 7. The median

`rollupMedian(values []float64) float64` takes a non-empty slice and returns the middle value for an odd count and the arithmetic mean of the two middle values for an even count. It takes `[]float64` at both levels — the family level converts its recorded `int` counts — so the "headline aggregates family medians" rule is enforced by the type rather than by discipline. Sort a copy; do not mutate the caller's slice. The non-empty precondition is the caller's: guard the headline with the "no family's median is defined" check before calling, and guard the per-family level with the "no recorded count among its members" check. A zero-length slice must never reach it.

Render every median with `strconv.FormatFloat(m, 'f', -1, 64)`, so a whole value prints as `52` and a half value as `52.5`. The headline uses the same renderer.

## 8. The two degenerate weeks

Three states exist, and the difference between them is the whole point of the spec — a zero must never stand in for a missing measurement.

| Week's task set | `HumanInteractions` | `UnattendedDeliveries` | `PerFamilyMedian` |
|---|---|---|---|
| empty (no task carries a `metrics_completed_at` in that week) | `no data` | `no data` | `no data` |
| non-empty, no member carries a recorded count | `no recorded counts` | `no recorded counts` | `undefined` |
| otherwise | the sum | the count | the headline median, or `undefined` if no family's median is defined |

- The empty week's three fields carry the literal `no data`, which is the explicit no-data statement the spec requires. In the first two states no figure renders as `0` and none renders as an empty string — a zero must never stand in for a missing measurement. In the third state a `0` is a legitimate measurement and must print as `0`: a week whose recorded counts all happen to be `0` has a human-interaction sum of `0`, and a week with no recorded zero-count delivery has `UnattendedDeliveries` `0`. Do not add a guard that converts a measured `0` into `no recorded counts` — that would violate DB 3/DB 4.
- In the middle state every family line is `undefined` too — the headline and the family list agree.
- In the third state a family can still be `undefined` while the headline is defined; that is normal and is not an error.
- `WeekStart`/`WeekEnd`/`Year`/`Week` are populated in all three states — the week is known even when its data is not.

## 9. Tests — `pkg/ops/rollup_weekly_test.go`

New file, `package ops_test`, Ginkgo v2 + Gomega, mirroring `pkg/ops/lint_test.go`'s fixture style (temp vault directory, `Tasks/` subdirectory, raw `---`-delimited task files written with `0600`, an `AfterEach` that removes the tree). Build the operation with the **real** `storage.NewTaskStorage(storage.NewConfigFromVault(vault))`-equivalent — i.e. `storage.NewTaskStorage(&storage.Config{TasksDir: "Tasks"})` — against a temp vault, so every spec traverses the real frontmatter parse rather than a hand-built `*domain.Task`. Inject the clock with `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))`.

`pkg/ops/ops_suite_test.go` already owns the package's single `RunSpecs`. Do **not** add a second `func Test…` calling `RunSpecs` — it panics.

The suite must cover, at minimum, these behaviours. Use these spec names verbatim; they are grep targets in `<verification>`:

1. `"selects the tasks of a week by the local date of metrics_completed_at"` — a task completing `2026-09-14T00:30:00+02:00` (local Monday, week 38) is **not** in `2026-W37` while a task completing `2026-09-13T23:30:00Z` (Sunday, week 37) is. Assert both directions; this is the local-midnight rule and the one assertion that proves the timestamp's own offset was used.
2. `"sums human interactions over the week's task set"` — two tasks with recorded counts inside the week, one outside; the figure is the in-week sum only.
3. `"counts a completed task with a recorded zero as an unattended delivery"` — and asserts that task's count still contributes `0` to the interaction sum.
4. `"excludes a completed task with an absent count from unattended deliveries"` — a fixture week containing a `status: completed` task with **no** `metrics_interaction_count` key; assert the task is excluded from `UnattendedDeliveries` **and** that it does not raise the interaction sum. This is a body-level assertion, not a case name — write the assertion, not just the `It`.
5. `"reports no recorded counts for a week with no recorded counts"` — a non-empty fixture week in which no member carries a count; assert `HumanInteractions == "no recorded counts"`, `UnattendedDeliveries == "no recorded counts"` and `PerFamilyMedian == "undefined"`, and that every entry in `Families` has `Median == "undefined"`.
6. `"reports no data for a week with no completed task"` — a fixture vault whose tasks all lack `metrics_completed_at` (and one empty-directory case); assert all three figures are `"no data"`, that `Families` is empty, and that no figure string is `"0"`.
7. `"reports undefined for a family with no recorded count"` — two families, one with recorded counts and one without; the `undefined` one is present in `Families`, contributes nothing, and the headline is the median over the defined families only.
8. `"groups dated, versioned and month-named filenames into one family"` — fixture filenames such as `Check Unassigned Tasks - 2026-09-16.md`, `check unassigned tasks W37.md` and `Check Unassigned Tasks v2.1.md` collapse to one family, proving the date, week-number and version strip classes and the case-insensitive comparison through the real read path. (The month-name class is pinned by the `RollupFamilyName` table test below; do not add a fourth fixture file just to reach it.)
9. `"lists families sorted by name"` — insert the families out of order and assert the returned order is ascending, so two runs cannot differ.
10. `"rejects a malformed week token"` — a `DescribeTable` over `"2026-W5"`, `"2026-w37"`, `"2026W37"`, `"W37"`, `"2026-37"`, `"2026-W370"`, `"2026-W00"` and `"2026-W54"`, and assert each returns a non-nil error whose message contains `YYYY-Wnn` and produces no result. Do not put the empty token in that table — it is the valid "last complete ISO week" form, covered by spec 11. Include `"2026-W53"` in a separate spec asserting it is **accepted** (2026 has 53 ISO weeks) — this is the row the failure-modes table calls out by name.
11. `"defaults to the last complete ISO week"` — with the clock pinned to `2026-09-16T12:00:00Z` (a Wednesday in week 38) and an empty week token, the resolved week is `2026-W37` with range `2026-09-07`–`2026-09-13`. Also pin a Monday clock to prove the default is "the week before now", not "the week containing now" (W37, not W38), and a **Sunday** clock (`2026-09-13T12:00:00Z`) — the resolved week must be `2026-W36`, which is what distinguishes "the week before now" from "the week containing now minus one day".

Add one more spec, `"returns an error naming an unreadable task file"`, that makes a single task file unreadable (write it, then `os.Chmod(path, 0000)`; skip the spec when running as root, where the mode is ignored) and asserts `Execute` returns a non-nil error whose message contains the file's base name. Restore the mode in a `DeferCleanup` so the temp-tree removal can proceed.

**Helper table tests.** `pkg/ops/export_test.go` gains two test-only aliases so the external `ops_test` package can pin the two helpers directly:

```go
// RollupFamilyName exposes rollupFamilyName for the external ops_test package.
var RollupFamilyName = rollupFamilyName

// RollupMedian exposes rollupMedian for the external ops_test package.
var RollupMedian = rollupMedian
```

In `rollup_weekly_test.go`, add a `DescribeTable` over `RollupFamilyName` covering each strip class individually (a bare ISO date, a bare week number, a bare version, a bare month name, a mixed case-insensitive pair, and the all-date fallback from step 5), and a `DescribeTable` over `RollupMedian` covering odd count, even count with a whole mean, even count with a half mean, and the single-element case. These are cheap pins on the rule the operator will hand-compute against during the spec's probes — do not skip them.

**The JSON contract — the one boundary test.** Add one spec, `"renders the result under the frozen JSON keys"`, that marshals a `RollupWeeklyResult` with `encoding/json` and asserts on the **marshalled bytes**, not on the struct fields: every one of `year`, `week`, `week_start`, `week_end`, `human_interactions`, `unattended_deliveries`, `per_family_median` and `families` is a key in the output, and an empty `Families` marshals as `[]` and not `null`. The `encoding/json` import belongs in the `_test.go` file only — `pkg/ops/rollup_weekly.go` stays free of it. The tag greps in `<verification>` pin the tag text; this is the only check that pins what prompt 2's `--output json` consumer will actually read.

**Coverage.** The new `pkg/ops/rollup_weekly.go` must reach ≥80% coverage from this suite, measured as the **lowest per-function coverage** in the file. The `awk` assertion in the `<verification>` block is the authority and exits non-zero below 80.0%. This is deliberately stricter than file-level statement coverage: it stops one large well-covered function from carrying a small uncovered one. Do not add retroactive coverage to unrelated untested code, and do not add a test that exists only to raise the number.

## 10. Failure modes and security — what each carries

Map the spec's Failure Modes table onto the change and state the mapping in your completion report:

- **Malformed `--week`, or a week number outside the year's ISO week count** → non-zero error naming `YYYY-Wnn`, no figures. Covered by specs 10.
- **Week contains no task with `metrics_completed_at`** → the `no data` state, no error. Covered by spec 6.
- **Week's set non-empty but no member carries a count** → `no recorded counts` / `no recorded counts` / `undefined`. Covered by spec 5.
- **A `metrics_*` value is malformed (non-numeric)** → treated as absent by the existing accessor; no crash, no coercion to `0`. Add it to spec 4's fixture set (a task with `metrics_interaction_count: "unknown"` behaves exactly like an absent key) rather than giving it a spec of its own.
- **A task file is unreadable mid-scan** → the strict listing returns an error naming the file, `Execute` wraps it with the vault name, and no smaller week is reported. Covered by the unreadable-file spec.
- **The resolved vault's task directory is missing** → `ListTasksStrict` returns the wrapped walk error naming the directory; `Execute` names the vault. `tasks_dir` defaults to `Tasks` when unset, so an unset key is not itself an error — do not add a config check.
- **Two tasks completing either side of a local midnight in the same UTC week** → each lands in its own local week. Covered by spec 1.
- **A probe mutates a task count and is not restored** → impossible by construction; probes run against a scratch copy. This is an operator-side property; nothing to build here.

Security: the one untrusted-input surface is the task frontmatter itself. A malformed or hostile `metrics_*` value must read as absent, never be parsed into a number, and never panic the scan — the accessors already encode that contract, so consuming them is the control. The week token is validated before use and the vault path comes from config. Do not add path construction from user input, do not add a network call, and do not add a second data source.

## 11. Self-check before finishing

- Re-read the changed hunks and confirm: `ListTasks` is behaviourally identical, `ListTasksStrict` names the failing file, `pkg/ops/rollup_weekly.go` contains no `fmt.Print*`, no `os.Stdout`, no `time.Now()` and no `encoding/json` import, and `Execute` calls `ListTasksStrict`.
- Walk the spec's Desired Behaviors 2–6 and Acceptance Criteria 2, 3, 5 and 6 and state in your completion report which requirement and which spec satisfies each one.
- Confirm the state table in requirement 8 holds for all three rows — including that no figure in either of the first two rows renders as `0`, and that a measured `0` in the third row (an all-zero interaction sum, or no delivery at all) still prints as `0`.
- Run each check in `<verification>` and confirm it passes; do not report a check you did not run.

</requirements>

<constraints>
- **Copied from spec 049 — one vault, never an aggregate.** The figures are properties of one task population; summing Personal with Brogrammers would answer no question. The operation takes a single `vaultPath` and has no multi-vault path, no loop over vaults, and no aggregation across vaults.
- **Copied from spec 049 — read set is the resolved vault's configured `tasks_dir`, frontmatter only.** No data file, no cache, no sidecar. The directory comes from `storage.Config` (built from the vault's `tasks_dir`), never hardcoded — configured vaults use `tasks`, `24 Tasks` and `25 Tasks` variously.
- **Copied from spec 049 — parse frontmatter, never grep lines.** A line-grep for `metrics_interaction_count: 0` over the task directory over-counts: sample-YAML lines inside task bodies match too. Go through the storage read path, which is the only frontmatter-scoped extraction.
- **Copied from spec 049 — an absent count is never coerced to zero.** Reuse `TaskFrontmatter.MetricsInteractionCount()`; never read the key through `Get`/`RawMap` and never default a nil pointer to `0`.
- **Copied from spec 049 — the grouping rule is the filename-stem rule already documented in the vault**, and the ≥3-instance threshold governs whether a family counts as recurring evidence, never membership. A 1–2 member family stays in the median.
- **Copied from spec 049 — the unattended rule is the vault's written rule verbatim**: `status` completed and `metrics_interaction_count` exactly `0`. Zero means a *recorded* zero.
- **Copied from spec 049 — dates come from the injected clock**, never `time.Now()`. `libtime.CurrentDateTime` is the only source of "now", and the whole week rule must be exercisable with a pinned clock.
- **Copied from spec 049 — `pkg/ops/` never writes to stdout.** The operation returns a structured result; the CLI layer owns formatting. No `fmt.Print*`, no `os.Stdout`, no `log/slog` for the result.
- **Copied from spec 049 — Do NOT commit** — dark-factory handles git. Make no git calls at all, including in `<verification>`: the daemon does not check `<verification>` exit codes, so a git command that dies reports a false pass.
- **Copied from spec 049 — existing task/goal/theme commands and their output are unchanged**, and `make test` / `make precommit` stay green. `ListTasks`'s skip-on-error behaviour in particular is load-bearing for other callers and must not change.
- **Do NOT add a CHANGELOG entry, a README section, or a scenario file.** Prompt 2 owns the changelog, the README and the command surface. `make check-changelog` must pass with the file exactly as you found it.
- **Do NOT bump any version string** — not `CHANGELOG.md`, not `.claude-plugin/plugin.json`, not `.claude-plugin/marketplace.json`. `.maintainer.yaml` sets `release.autoRelease: true` and the post-merge releaser owns the bump.
- **Do NOT add a knob.** No opt-out flag, no config key, no environment variable, no tunable threshold, no `--include-*` filter. The rule, the grouping and the three figures are invariants of this feature.
- **Do NOT run `go mod vendor`** and never write `-mod=vendor` in a verification command. Do not add a dependency: everything needed is in the standard library plus `github.com/bborbe/errors`, `github.com/bborbe/time` and the repo's own packages.
- **Tests.** Ginkgo v2 + Gomega in external `_test` packages; counterfeiter mocks only where the repo already generates them. Every new Go file keeps its BSD license header. `pkg/ops/ops_suite_test.go` keeps the package's single `RunSpecs` — a second `func Test…` calling it panics.
- **Linter limits** the new file must respect: `funlen` 80 lines / 50 statements, `gocognit` 20, `nestif` 4, `maintidx` 20. Split `Execute` into small named helpers rather than growing it past the limit.
</constraints>

<verification>
Run everything from the repo root. `make precommit` is the full gate and must exit 0; it runs `ensure`, `format`, `generate`, the whole test suite, `check` (lint, vet, vulncheck, osv-scanner, trivy, check-changelog) and `addlicense`. If it fails, fix the cause, then re-run ONLY the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. If it still fails, report `"status":"failed"` naming the failing target — never rationalise a non-zero exit code as success.

The checks below are written as self-failing assertions on purpose: a bare `grep -c` exits 1 when the count is 0, so a check that silently stops matching still surfaces as a non-zero exit. The daemon does not check `<verification>` exit codes — you must read each line's exit status yourself and report `"status":"failed"` if any of them is non-zero.

**The suite, with its frozen spec names.** Capture the run, check its exit status separately from the name greps, and never pipe a test command. `-args -ginkgo.v -ginkgo.no-color` is mandatory, not cosmetic: Ginkgo v2's default reporter prints only dots on a green run, so without **`-ginkgo.v`** every spec-name grep below returns zero matches against a correct suite. `-ginkgo.no-color` is the second half of the guarantee: with colour on, Ginkgo injects ANSI escapes *between* the container and It texts, so keep both flags to ensure every target below matches as a contiguous substring. The name greps are the only thing that proves a spec is not an empty `It`, so the flags are what make this block a check rather than decoration:

```
go test ./pkg/ops/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/rollup-ops.log 2>&1; test "$?" = "0"
go test ./pkg/storage/... -v -count=1 -args -ginkgo.v -ginkgo.no-color > /tmp/rollup-storage.log 2>&1; test "$?" = "0"
grep -F -q -- 'selects the tasks of a week by the local date of metrics_completed_at' /tmp/rollup-ops.log
grep -F -q -- 'counts a completed task with a recorded zero as an unattended delivery' /tmp/rollup-ops.log
grep -F -q -- 'excludes a completed task with an absent count from unattended deliveries' /tmp/rollup-ops.log
grep -F -q -- 'reports no recorded counts for a week with no recorded counts' /tmp/rollup-ops.log
grep -F -q -- 'reports no data for a week with no completed task' /tmp/rollup-ops.log
grep -F -q -- 'reports undefined for a family with no recorded count' /tmp/rollup-ops.log
grep -F -q -- 'groups dated, versioned and month-named filenames into one family' /tmp/rollup-ops.log
grep -F -q -- 'lists families sorted by name' /tmp/rollup-ops.log
grep -F -q -- 'rejects a malformed week token' /tmp/rollup-ops.log
grep -F -q -- 'defaults to the last complete ISO week' /tmp/rollup-ops.log
grep -F -q -- 'returns an error naming an unreadable task file' /tmp/rollup-ops.log
grep -F -q -- 'sums human interactions over the week's task set' /tmp/rollup-ops.log
grep -F -q -- 'renders the result under the frozen JSON keys' /tmp/rollup-ops.log
```

The spec-name greps prove the specs exist *and* ran — they are only meaningful because the run above passes `-args -ginkgo.v -ginkgo.no-color`; on a green suite without those flags no spec name is printed at all. The two exit-status checks prove they passed. A spec that is only named — an empty `It`, or one whose body never calls `Execute` — does not count and fails the acceptance criteria.

**The Ginkgo suite exists and does not add a second `RunSpecs`:**

```
grep -nE 'Describe|It\(' pkg/ops/rollup_weekly_test.go
test "$(grep -c 'func Test' pkg/ops/rollup_weekly_test.go)" = "0"
test "$(grep -c 'RunSpecs' pkg/ops/rollup_weekly_test.go)" = "0"
```

**The storage layer gained the strict listing and left `ListTasks` alone:**

```
test "$(grep -c 'ListTasksStrict' pkg/storage/storage.go)" -ge 1
test "$(grep -c 'func (t \*taskStorage) ListTasksStrict' pkg/storage/task.go)" = "1"
grep -F -q -- 'skipping unreadable task' pkg/storage/task.go
test "$(grep -c 'func (t \*taskStorage) ListTasks(' pkg/storage/task.go)" = "1"
```

The third line is the guard that `ListTasks`'s skip-on-error behaviour survived; the fourth that a second, duplicated `ListTasks` was not introduced.

**The operation's shape — no stdout, no ambient clock, no hand-rolled JSON:**

```
test "$(grep -cE 'fmt\.Print|os\.Stdout' pkg/ops/rollup_weekly.go)" = "0"
test "$(grep -c 'time\.Now()' pkg/ops/rollup_weekly.go)" = "0"
test "$(grep -c '"encoding/json"' pkg/ops/rollup_weekly.go)" = "0"
grep -F -q 'ListTasksStrict' pkg/ops/rollup_weekly.go
test "$(grep -c 'ListTasks(' pkg/ops/rollup_weekly.go)" = "0"
test "$(grep -c 'MetricsInteractionCount()' pkg/ops/rollup_weekly.go)" -ge 1
test "$(grep -c 'MetricsCompletedAt()' pkg/ops/rollup_weekly.go)" -ge 1
```

The lenient-listing check is the one that matters most: a single call to `ListTasks` silently reintroduces the "unreadable file shrinks the week" failure the spec forbids. It is anchored on the call form, not the bare name, so a doc comment explaining why the lenient listing is avoided does not fail it.

**The frozen JSON tags, which prompt 2's `--output json` depends on:**

```
grep -F -q 'json:"human_interactions"' pkg/ops/rollup_weekly.go
grep -F -q 'json:"unattended_deliveries"' pkg/ops/rollup_weekly.go
grep -F -q 'json:"per_family_median"' pkg/ops/rollup_weekly.go
grep -F -q 'json:"families"' pkg/ops/rollup_weekly.go
```

**The helper exports the table tests need:**

```
grep -F -q 'var RollupFamilyName = rollupFamilyName' pkg/ops/export_test.go
grep -F -q 'var RollupMedian = rollupMedian' pkg/ops/export_test.go
```

**Coverage for the new operation file:**

```
go test -coverprofile=/tmp/rollup-cover.out ./pkg/ops/... > /dev/null 2>&1; test "$?" = "0"
awk '/rollup_weekly\.go/{gsub(/%/,"",$NF); if (v=="" || $NF+0 < v+0) v=$NF} END{printf "rollup_weekly.go lowest per-function coverage: %s%%\n", (v=="" ? "absent" : v); if (v+0 >= 80) exit 0; exit 1}' <(go tool cover -func=/tmp/rollup-cover.out)
```

The `awk` takes the **lowest** per-function coverage in `rollup_weekly.go` and fails below 80.0%. A bare `grep` for the file exits 0 at any percentage, which is why it is not used.

**Formatting:**

```
gofmt -e -l pkg/storage/storage.go pkg/storage/task.go pkg/ops/rollup_weekly.go pkg/ops/rollup_weekly_test.go pkg/ops/export_test.go > /tmp/rollup-gofmt.out 2>&1; test "$?" = "0"; test ! -s /tmp/rollup-gofmt.out
```

**Nothing outside the scope moved:**

```
test "$(ls scenarios/*.md | wc -l | tr -d ' ')" = "5"
test "$(grep -c '^## Unreleased' CHANGELOG.md)" = "0"
```

The second line asserts no `## Unreleased` section was added: prompt 2 owns that section, and one added here would be a scope violation, not a bonus.

Finally, run `make precommit` once more and confirm it exits 0, then walk spec 049's Desired Behaviors 2–6 and Acceptance Criteria 2, 3, 5 and 6 against the change and state in your completion report which requirement and which spec satisfies each one, plus which evidence covers each row of the spec's Failure Modes table.
</verification>
