---
status: completed
spec: [018-typed-date-storage]
summary: Migrated Goal/Objective/Theme date setters to typed storage, deleted both formatDateOrDateTime helpers (domain + ops), updated all GetField date arms and ops JSON projections to use d.String() inline via dateFieldString helper, audited Decision frontmatter (no migration needed), captured v0.80.0-baseline testdata from feature binary, and verified scenarios 002/003/004 produce consistent output.
container: vault-cli-date-storage-exec-144-goal-objective-theme-and-baseline
dark-factory-version: v0.182.0
created: "2026-06-20T13:48:15Z"
queued: "2026-06-20T13:48:15Z"
started: "2026-06-20T13:55:05Z"
completed: "2026-06-20T14:05:36Z"
branch: dark-factory/typed-date-storage
---

<summary>
- Goal, Objective, and Theme date fields now store typed date values directly, matching the change already made for tasks
- The duplicated date-to-string helper is removed from both the domain layer and the ops layer — there is now a single source of truth for how a date looks on disk and in JSON (the date type itself)
- All read paths that previously called the helper now ask the date value for its own string form
- Decision frontmatter is audited and confirmed to need no migration (it has no map-backed date setter)
- A frozen v0.80.0 output baseline is captured so we can prove the on-disk and JSON output did not change byte-for-byte
- Scenarios 002, 003, and 004 are replayed against a freshly built binary and their output is checked against the v0.80.0 baseline
</summary>

<objective>
Complete the typed-date migration: migrate the Goal, Objective, and Theme `Set*Date` setters to store typed values; delete BOTH `formatDateOrDateTime` helpers (`pkg/domain/task_frontmatter.go` and `pkg/ops/frontmatter.go`); update every `GetField` date arm and `TaskFrontmatter.LastCompleted()` and the `pkg/ops/` JSON projection in `show.go`/`list.go` to use `libtime.DateOrDateTime`'s own `String()` inline with a nil-check; audit Decision frontmatter (confirm no map-backed `*DateOrDateTime` setter); and capture a v0.80.0 byte-identical baseline for scenarios 002/003/004 and task list/show JSON.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` for the `## Unreleased` entry conventions in step 9.
Read `docs/releasing-vault-cli.md` for the scenario-replay procedure (build `/tmp/new-vault-cli`, never test against installed `vault-cli`).

PRECONDITION: prompts `1-reader-side-date-multi-shape` and `2-task-setter-typed-storage` must be shipped. Verify both:
```bash
grep -n 'case libtime.DateOrDateTime:' pkg/domain/frontmatter_map.go   # must match (prompt 1)
grep -n 'f.Set("defer_date", \*d)' pkg/domain/task_frontmatter.go       # must match (prompt 2)
```
If either returns no match, STOP and report `status: failed` with message `"prerequisite typed-storage prompts not yet deployed (prompts 1-2)"`.

Read these files in full before changing anything:
- `/workspace/pkg/domain/goal_frontmatter.go` — `SetStartDate` (152-158), `SetTargetDate` (161-167), `SetDeferDate` (188-194), `GetField` date arms (214, 216, 226), getters `StartDate` (69), `TargetDate` (81), `DeferDate` (115).
- `/workspace/pkg/domain/objective_frontmatter.go` — `SetStartDate` (136-142), `SetTargetDate` (145-150), `GetField` date arms (187, 189), getters `StartDate` (66), `TargetDate` (78).
- `/workspace/pkg/domain/theme_frontmatter.go` — `SetStartDate` (114-120), `SetTargetDate` (123-128), `GetField` date arms (156, 158), getters `StartDate` (65), `TargetDate` (77).
- `/workspace/pkg/domain/task_frontmatter.go` — `formatDateOrDateTime` (488-495, TO DELETE), `formatTimeAsDate` (478-486, KEEP), `LastCompleted` (121-123), `GetField` date arms (340, 356-364).
- `/workspace/pkg/ops/frontmatter.go` — `formatDateOrDateTime` (118-129, TO DELETE).
- `/workspace/pkg/ops/show.go` — JSON projection (100-103).
- `/workspace/pkg/ops/list.go` — JSON projection (116-122).
- `/workspace/pkg/domain/decision.go` — `Decision` struct; field `ReviewedDate *libtime.DateOrDateTime` is tagged `yaml:"-"` (storage-managed, NOT written through a FrontmatterMap setter). Confirm there is no `Set*Date` method that calls `formatDateOrDateTime` on a `Decision`.

KEY FACTS (verified in source):
- `libtime.DateOrDateTime` has value-receiver `String() string` returning `YYYY-MM-DD` for midnight-UTC values, else RFC3339Nano (`github.com/bborbe/time@v1.27.1/time_date-or-date-time.go:132`). For all practical vault data (zero nanoseconds), this matches the old `formatDateOrDateTime` output byte-for-byte.
- The `GetField` arms each currently call `formatDateOrDateTime(f.SomeDate())` where `SomeDate()` returns `*libtime.DateOrDateTime` (may be nil). The replacement is an inline nil-check returning `.String()`.
- Current vault data version is v0.80.0 (`CHANGELOG.md` top entry, `.claude-plugin/plugin.json`).
</context>

<requirements>

### 1. Migrate Goal date setters (`pkg/domain/goal_frontmatter.go`)

- `SetStartDate` (152-158): `f.Set("start_date", formatDateOrDateTime(d))` → `f.Set("start_date", *d)`
- `SetTargetDate` (161-167): `f.Set("target_date", formatDateOrDateTime(d))` → `f.Set("target_date", *d)`
- `SetDeferDate` (188-194): `f.Set("defer_date", formatDateOrDateTime(d))` → `f.Set("defer_date", *d)`

The nil branches (Delete) are unchanged.

### 2. Migrate Objective date setters (`pkg/domain/objective_frontmatter.go`)

- `SetStartDate` (136-142): `f.Set("start_date", formatDateOrDateTime(d))` → `f.Set("start_date", *d)`
- `SetTargetDate` (145-150): `f.Set("target_date", formatDateOrDateTime(d))` → `f.Set("target_date", *d)`

### 3. Migrate Theme date setters (`pkg/domain/theme_frontmatter.go`)

- `SetStartDate` (114-120): `f.Set("start_date", formatDateOrDateTime(d))` → `f.Set("start_date", *d)`
- `SetTargetDate` (123-128): `f.Set("target_date", formatDateOrDateTime(d))` → `f.Set("target_date", *d)`

### 4. Replace every `GetField` date arm with inline nil-check + `.String()`

Across `goal_frontmatter.go`, `objective_frontmatter.go`, `theme_frontmatter.go`, AND `task_frontmatter.go`, replace each arm of the form `return formatDateOrDateTime(f.SomeDate())` with the inline pattern. Example for one arm:

```go
case "defer_date":
	if d := f.DeferDate(); d != nil {
		return d.String()
	}
	return ""
```

Apply to all of these arms:
- Goal `GetField`: `start_date` (214), `target_date` (216), `defer_date` (226)
- Objective `GetField`: `start_date` (187), `target_date` (189)
- Theme `GetField`: `start_date` (156), `target_date` (158)
- Task `GetField`: `defer_date` (340), `last_completed_date` (356), `completed_date` (358), `created_date` (360), `planned_date` (362), `due_date` (364)

Note: the Goal/Objective/Theme `completed` arms call `d.String()` on a `*libtime.Date` already — do NOT touch those; only the `*DateOrDateTime` arms listed above change.

### 5. Fix `TaskFrontmatter.LastCompleted()` (`pkg/domain/task_frontmatter.go` 121-123)

The Task `GetField` arm for `last_completed` (353-354) calls `f.LastCompleted()`, which currently calls `formatDateOrDateTime`. After deletion that will not compile, so rewrite `LastCompleted`:

```go
func (f TaskFrontmatter) LastCompleted() string {
	d := f.LastCompletedDate()
	if d == nil {
		return ""
	}
	return d.String()
}
```

### 6. Delete the domain `formatDateOrDateTime` helper (`pkg/domain/task_frontmatter.go` 488-495)

After steps 4 and 5 there are no remaining callers in `pkg/domain/`. Delete the function. KEEP `formatTimeAsDate` (478-486) exactly as is.

### 7. Migrate ops JSON projection and delete the ops `formatDateOrDateTime` helper

In `pkg/ops/show.go` (100-103), replace:

```go
detail.DeferDate = formatDateOrDateTime(task.DeferDate())
detail.PlannedDate = formatDateOrDateTime(task.PlannedDate())
detail.DueDate = formatDateOrDateTime(task.DueDate())
detail.CompletedDate = formatDateOrDateTime(task.CompletedDate())
```

with inline nil-check + `.String()` for each field:

```go
if d := task.DeferDate(); d != nil {
	detail.DeferDate = d.String()
}
if d := task.PlannedDate(); d != nil {
	detail.PlannedDate = d.String()
}
if d := task.DueDate(); d != nil {
	detail.DueDate = d.String()
}
if d := task.CompletedDate(); d != nil {
	detail.CompletedDate = d.String()
}
```

In `pkg/ops/list.go` (116-122), apply the same inline nil-check + `.String()` transformation to `items[i].DeferDate`, `items[i].PlannedDate`, `items[i].DueDate`, `items[i].CompletedDate`.

**Also migrate `pkg/ops/decision_list.go:73`** (third caller — easy to miss):

```go
// Before
items[i].ReviewedDate = formatDateOrDateTime(dec.ReviewedDate)
// After
if d := dec.ReviewedDate; d != nil {
    items[i].ReviewedDate = d.String()
}
```

This file is the third caller of the ops `formatDateOrDateTime`. If you do NOT migrate it before deleting the helper, the build fails on undefined identifier. The defensive grep below catches it, but better to migrate proactively.

Then delete the ops `formatDateOrDateTime` helper (`pkg/ops/frontmatter.go` 118-129). Verify no other caller in `pkg/ops/` remains (`grep -rn 'formatDateOrDateTime' pkg/ops/`). If the `time` import in `frontmatter.go` becomes unused after deletion, remove it (otherwise the build fails on an unused import).

NOTE on format equivalence: the deleted ops helper formatted midnight-UTC as `time.DateOnly` and non-midnight as RFC3339. `DateOrDateTime.String()` formats midnight-UTC as `DateOnly` and non-midnight as RFC3339Nano. For zero-nanosecond timestamps (all real vault data) these are identical. The v0.80.0 baseline check in step 10 is the guard.

### 8. Audit Decision frontmatter (no migration)

Read `pkg/domain/decision.go`. Confirm there is no `Set*Date` method that stores a `*libtime.DateOrDateTime` through a `FrontmatterMap` (the `ReviewedDate` field is tagged `yaml:"-"` and managed by the storage layer, not a map setter). In the completion report's `## Notes` or `## Improvements` section, record the finding: "Decision frontmatter has no map-backed *DateOrDateTime setter; field ReviewedDate is yaml:\"-\" storage-managed. No migration needed." Do NOT add a Decision setter.

### 9. Update CHANGELOG and verify helpers are gone

Add a `## Unreleased` entry (or append if it exists) following `changelog-guide.md`:
```
- refactor: Store *DateOrDateTime date fields as typed values at the setter boundary and emit via the type's own MarshalText/String; remove both formatDateOrDateTime helpers (domain + ops)
```

Confirm AC #1: `grep -rn 'formatDateOrDateTime' pkg/` returns ZERO matches.

### 10. Capture v0.80.0 baseline and run scenarios 002/003/004 (AC #11–#15)

There is no `make scenarios` target — scenarios are markdown docs replayed by building a binary. The current released version IS v0.80.0, so the baseline is captured from the v0.80.0 source.

Procedure (run inside the container, which is a git repo):

a. Determine the v0.80.0 commit SHA: `git rev-list -n 1 v0.80.0` (the tag should exist; if not, use the commit of the `## v0.80.0` CHANGELOG entry on the base branch — record whichever you used).

b. Build a v0.80.0 binary into `/tmp/baseline-vault-cli` from a clean worktree of that commit:
```bash
git worktree add /tmp/v0800-src v0.80.0
go build -C /tmp/v0800-src -o /tmp/baseline-vault-cli .
```

c. Build the feature-branch binary into `/tmp/new-vault-cli`:
```bash
go build -C /workspace -o /tmp/new-vault-cli .
```

d. Create the baseline directory `pkg/ops/testdata/v0.80.0-baseline/` with subdirectories `scenario-002/`, `scenario-003/`, `scenario-004/`. For each scenario (`scenarios/002-task-lifecycle.md`, `scenarios/003-task-recurring-completion.md`, `scenarios/004-decision-list-ack.md`), replay the scenario's documented commands using `/tmp/baseline-vault-cli`, then capture the resulting on-disk markdown files (the task/decision files the scenario mutates) into the matching `scenario-NNN/` directory. Also capture:
   - `task-list.json` — output of `<binary> --config <cfg> task list --output json` (AC #13)
   - `task-show.json` — output of `<binary> --config <cfg> task show <task> --output json` (AC #14)
   using `/tmp/baseline-vault-cli`.

e. Re-run the identical replay using `/tmp/new-vault-cli` against a fresh copy of the same fixtures, and assert the captured output (markdown files + the two JSON files) is BYTE-IDENTICAL to the v0.80.0 baseline. Use `diff` — any difference is a failure (AC #12, #13, #14). If a diff appears, do NOT commit; investigate (likely an RFC3339 vs RFC3339Nano or quoting difference) and report `status: failed`.

f. Write `pkg/ops/testdata/v0.80.0-baseline/README.md` (AC #15) containing, verbatim:
   - the exact v0.80.0 commit SHA in the form `commit: <40-hex-sha>`
   - the verbatim replay command(s) used to capture the baseline
   - the capture date

g. Clean up the worktree: `git worktree remove /tmp/v0800-src`.

The committed baseline directory + README is the durable AC #12–#15 artifact. The diff in step (e) is the gate.

### 11. Iterative verification

Run `make test` after each group of edits. Do NOT run `make precommit` until the final step.

</requirements>

<constraints>
- Public setter and reader signatures MUST NOT change.
- BOTH `formatDateOrDateTime` helpers (domain + ops) MUST be deleted — neither survives. AC #1: `grep -rn 'formatDateOrDateTime' pkg/` returns zero matches.
- `formatTimeAsDate(time.Time) string` MUST remain in `pkg/domain/task_frontmatter.go`, unchanged.
- On-disk YAML and CLI JSON output MUST be byte-identical to v0.80.0 for the same input — this is the gate in step 10(e).
- The `last_completed_date` + `last_completed` dual-write window MUST remain intact (already implemented in prompt 2; do not regress it).
- Do NOT add `MarshalYAML`/`UnmarshalYAML` to `bborbe/time`. Do NOT add a feature flag / opt-out.
- Do NOT add a Decision setter — the audit confirms none is needed.
- `ParseDateOrDateTime` / `ParseDateOrDateTimeDefault` already exist in `bborbe/time@v1.27.1` — do NOT add local copies.
- Tests use Ginkgo v2 / Gomega. Use fixed `time.Time` literals — never the wall clock.
- Coverage for changed setters and `GetField`/`LastCompleted`/ops projection MUST stay ≥80%.
- Do NOT release a new binary version or bump the four version strings — this prompt is a refactor with no behavior change; the daemon's autoRelease handles versioning. If `make precommit`'s `check-versions` is the only failure, that is a separate release step — report it in `## Improvements`, do NOT bump versions yourself unless the prompt's verification explicitly fails on it.
- `make precommit` MUST stay green from the repo root.
- Do NOT commit — dark-factory handles git. Use `git worktree` only as a read-only build helper, and remove it before finishing. Do NOT change the git remote.
</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. AC #1: both formatDateOrDateTime helpers gone
grep -rn 'formatDateOrDateTime' pkg/
# Expected: zero matches (exit code 1 from grep)

# 2. AC #2: formatTimeAsDate still present
grep -n 'func formatTimeAsDate' pkg/domain/task_frontmatter.go
# Expected: 1 match

# 3. Goal/Objective/Theme setters store typed values
grep -n 'f.Set("start_date", \*d)\|f.Set("target_date", \*d)\|f.Set("defer_date", \*d)' pkg/domain/goal_frontmatter.go pkg/domain/objective_frontmatter.go pkg/domain/theme_frontmatter.go
# Expected: 8 matches total (Goal: start/target/defer, Objective: start/target, Theme: start/target)

# 4. AC #4: all domain + ops tests pass unchanged
go test ./pkg/domain/... ./pkg/ops/...
# Expected: ok

# 5. AC #5: TypedDateRoundTrip still passes (from prompt 2)
go test -v ./pkg/domain/... -run TypedDateRoundTrip
# Expected: PASS

# 6. AC #15: baseline README has commit SHA + replay command
test -f pkg/ops/testdata/v0.80.0-baseline/README.md && grep -E '^commit: [0-9a-f]{40}$' pkg/ops/testdata/v0.80.0-baseline/README.md
# Expected: README exists; one line matching commit: <40-hex>

# 7. AC #11/#12/#13/#14: scenario + JSON outputs byte-identical to baseline (run in step 10e)
# Expected: diff produced no output for every captured file
```

Scenario replay (AC #11–#14) — build the binary, never use installed vault-cli:

```bash
go build -C /workspace -o /tmp/new-vault-cli .
# Replay scenarios/002, 003, 004 per their documented commands using /tmp/new-vault-cli
# and diff captured markdown + task-list.json + task-show.json against
# pkg/ops/testdata/v0.80.0-baseline/  — every diff MUST be empty.
```
</verification>

<!-- DARK-FACTORY-REPORT -->
