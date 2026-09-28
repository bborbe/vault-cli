---
spec: ["056-task-approve-command"]
status: draft
created: "2026-09-28T07:54:18Z"
---

# Add the `task approve` operation (spec 056, prompt 1 of 5)

<summary>
- Adds the missing approving half of the task approval gate: one operation that moves a filed task out of the operator's inbox and records, in the same write, that it was approved and when.
- The four facts of the transition travel in a single storage write, so a row can never end up planned without an approval record beside it.
- Refuses any task that is not waiting for approval — one already past the inbox, or one that already carries an approval record — and writes nothing at all when it refuses.
- Refuses an empty approver, so a record can never be present-but-blank.
- The approver is `operator` unless a caller names one explicitly.
- Every other frontmatter key, known or unknown, survives untouched; only four keys change.
- The instant is read from an injected clock, never from the system directly, so the recorded timestamp is testable.
- Ships with unit coverage that pins the single-write property, the zero-write refusals, and the injected instant.
- This prompt is the operation only — the `vault-cli task approve` verb and its integration coverage are prompt 2, the docs and CHANGELOG are prompt 3, and the `work-on` bypass is prompt 4.
- Scope note for the reviewer: `vault-cli task work-on` also advances a `todo` row to `planning` with no approval record. Prompt 4 closes that path; this prompt records the approval but does not by itself close the gate.
</summary>

<objective>
Ship the `pkg/ops` operation behind `vault-cli task approve`: it validates that the named task is waiting in the approval inbox, composes `status: in_progress`, `phase: planning`, `approved_by` and `approved_at` onto the existing frontmatter map, and hands the whole map to the storage layer in exactly one write. Every refusal path writes nothing. This prompt covers spec 056 ACs 7 and 8 plus the operation-level substance of spec 056 ACs 1–6 and Desired Behaviors 1–5 and 7. It is the precondition for prompt 2, which exposes the operation as a Cobra verb.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**This operation is a plain task mutation with two refusals.** Read the closest siblings before writing anything; the new code should be recognisably the same code with a different field set and a stricter precondition.

Read fully (in this order):
- `pkg/ops/update.go` — the whole file. It is the closest shape: a task-mutating operation with `//counterfeiter:generate`, a pure-composition factory, `FindTaskByName` → mutate the entity → one `WriteTask`, `MutationResult` on every return, and `errors.Wrap(ctx, err, "find task")` / `"write task"` on the two storage calls. Copy that skeleton.
- `pkg/ops/complete.go` — lines 1-81 only. The `CompleteOperation` interface and the `MutationResult` struct definition. Note `MutationResult` has **no** phase field; do not add one (prompt 2 renders the phase itself).
- `pkg/ops/frontmatter.go` — lines 47-135 and 213-264. This is where the phase-regression guard lives (`checkPhaseRegression`) and it is called **only** from `frontmatterSetOperation.Execute`. Your operation does not call it: its own precondition is stricter than the guard (it accepts only `phase: todo`), and a forward `todo → planning` move is never a regression. Do not modify this file.
- `pkg/domain/task_frontmatter.go` — the whole file. `Status()`, `Phase()`, `SetStatus(TaskStatus) error`, `SetPhase(*TaskPhase)`, `Get(key)`, `GetString(key)`, and the promoted `Set(key, value any)` from `FrontmatterMap`. Note that `SetStatus` validates and that `SetPhase(nil)` deletes the key.
- `pkg/domain/frontmatter_map.go` — lines 19-50 and 193-227 only. `Get`, `GetString`, `Set`, `RawMap`. `Set` with a nil value deletes the key, so a refusal that never calls `Set` leaves the map untouched.
- `pkg/domain/task_phase.go` — the `TaskPhase` constants and `Ptr()`. `domain.TaskPhaseTodo`, `domain.TaskPhasePlanning`.
- `pkg/domain/task_status.go` — the `TaskStatus` constants. `domain.TaskStatusInProgress`.
- `pkg/storage/storage.go` — lines 53-66 only, the `TaskStorage` interface. `FindTaskByName(ctx, vaultPath, name)` and `WriteTask(ctx, task)`.
- `pkg/ops/metrics_session_append_test.go` — the whole file. The ops unit-spec idiom to copy: external `ops_test` package, `mocks.TaskStorage`, a local `seedTask` helper that re-stubs `FindTaskByNameReturns`, `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))` for a pinned clock, `WriteTaskCallCount()`, and `WriteTaskArgsForCall(0)` to inspect what was written. Note its `assertRefused` closure pattern for the "nothing was written" cases.
- `mocks/storage.go` — lines 1-60 and 1736-1800 only, to confirm the fake you will use: `*mocks.Storage` carries `FindTaskByNameReturns`, `FindTaskByNameCallCount`, `WriteTaskReturns`, `WriteTaskCallCount` and `WriteTaskArgsForCall`, and satisfies `storage.TaskStorage` (spec 056 AC 7 names a counterfeiter `Storage` mock).
- `docs/development-patterns.md` § Testability and § Mocks — injected `libtime.CurrentDateTime`, injected `storage.Storage`, pure-composition factories, counterfeiter in `mocks/`.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct, the counterfeiter annotation form.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` / `errors.Wrap(ctx, ...)` / `errors.Wrapf(ctx, ...)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, `DescribeTable`/`Entry`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-time-injection.md` — injected `libtime.CurrentDateTime`; never `time.Now()`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-composition.md` — no test-only package-level mutable state; pure-composition factories.

NOTE: `.dark-factory.yaml` is `workflow: direct` with no `hideGit`, so the container is expected to expose the real `.git/` and the `git diff --exit-code` / `git status --porcelain` guards below should run here. This repo's own specs disagree on the observed runtime (`specs/in-progress/047` states `hideGit=true`; `specs/in-progress/048` says it may or may not), and a run started with `--set hideGit=true` masks `.git/`, so every git command here would fail with `fatal: not a git repository` — which the daemon reports as a pass, not a failure. The PRIMARY GATE and TESTS rows are git-free and non-vacuous either way; for the frozen-neighbour guarantee, also pin the two files by content — `sha256sum pkg/ops/frontmatter.go pkg/ops/frontmatter_entity.go` must print `7f9c990c10ef43fe1315381f6cdd95769b94537de636256d90fd3ae693f35d83` and `687afeb23ce605678cf39879b4e693fb0de1adb11a8929b5768d57f84efd5965`.
</context>

<requirements>
1. **Add the operation.** New file `pkg/ops/task_approve.go`, following the `pkg/ops/update.go` skeleton:
   Both new files carry the repository's three-line copyright header above `package ops` / `package ops_test` (copy from `pkg/ops/update.go` lines 1-3).
   ```go
   //counterfeiter:generate -o ../../mocks/task-approve-operation.go --fake-name TaskApproveOperation . TaskApproveOperation
   type TaskApproveOperation interface {
       // Execute approves the named task: it moves a task waiting in the approval
       // inbox (phase todo) to phase planning and records, in the same write, who
       // approved it and the instant of the transition. Any task that is not
       // waiting in the inbox, and any task that already carries an approval
       // record, is refused with nothing written.
       Execute(
           ctx context.Context,
           vaultPath string,
           taskName string,
           vaultName string,
           approvedBy string,
       ) (MutationResult, error)
   }

   // NewTaskApproveOperation creates a new task approve operation.
   func NewTaskApproveOperation(
       taskStorage storage.TaskStorage,
       currentDateTime libtime.CurrentDateTime,
   ) TaskApproveOperation {
       return &taskApproveOperation{
           taskStorage:     taskStorage,
           currentDateTime: currentDateTime,
       }
   }

   type taskApproveOperation struct {
       taskStorage     storage.TaskStorage
       currentDateTime libtime.CurrentDateTime
   }
   ```
   The factory is pure composition — no conditionals, no I/O, no `context.Background()`.

2. **`Execute` runs exactly these five steps, in this order.** Every refusal returns a non-nil error and a `MutationResult{Success: false, Error: err.Error()}` alongside it, and every refusal happens **before** `WriteTask` is ever reached — the byte-identical guarantee is structural, not a rewrite that happens to be stable.

   **Step 1 — empty approver, before any read.** An empty `approvedBy` is refused before `FindTaskByName` runs:
   ```go
   if approvedBy == "" {
       err := errors.Errorf(ctx, "approved_by must not be empty: refusing to record a blank approver on task %q", taskName)
       return MutationResult{Success: false, Error: err.Error()}, err
   }
   ```
   The literal string `approved_by` in the message is load-bearing: spec 056 AC 2 asserts the refusal's stderr names it. This is also what makes `--by ""` refuse rather than write a present-but-blank record.

   **Step 2 — find the task.** `task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)`; on error return the `MutationResult` plus `errors.Wrap(ctx, err, "find task")`. Leave the storage layer's own `ErrNotFound` wrapping intact — the CLI's multi-vault lookup depends on it.

   **Step 3 — read and validate the clock, before any guard.** `now := o.currentDateTime.Now()`; when `now.Time().IsZero()` refuse with a message naming `approved_at`:
   ```go
   err := errors.Errorf(ctx, "approved_at is not a valid instant: refusing to record a zero timestamp on task %q", taskName)
   ```
   Nothing is written on this path. This is the spec's Failure Modes row "Clock source returns a zero or malformed instant", and the message must contain the literal `approved_at`.

   **Step 4a — the inbox guard.** The task must be at `phase: todo`, and nothing else. Read `phase := task.Phase()`; refuse unless `phase != nil && *phase == domain.TaskPhaseTodo`. The message must contain both the task's current phase and the literal `todo`, so an operator reading stderr can see what was expected and what was found. Render an absent `phase` key as `(none)` rather than as an empty string:
   ```go
   current := "(none)"
   if phase != nil {
       current = string(*phase)
   }
   err := errors.Errorf(
       ctx,
       "refusing to approve %q: task is at phase %q, not %q; only a task in the approval inbox (phase %q) can be approved",
       taskName, current, string(domain.TaskPhaseTodo), string(domain.TaskPhaseTodo),
   )
   ```
   Spec 056 AC 4 asserts this message contains `planning` and `todo` for a `planning` row; AC 5 asserts the same refusal for an `execution` row; DB 4 additionally requires the refusal for `ai_review`, `human_review`, `done`, and a row with no `phase` key at all.

   **Step 4b — the existing-record guard.** Refuse a `todo` row that already carries an approval record, with the same nothing-written guarantee. **Both** keys must be checked independently — a guard that inspects only `approved_by` passes a both-keys fixture and silently re-approves a row someone already approved:
   ```go
   if task.Get("approved_by") != nil || task.Get("approved_at") != nil {
       // name every key that is present, not just the first one
   }
   ```
   The message must name the key(s) actually present (`approved_by`, `approved_at`, or both) and say that re-approving is refused. Spec 056 AC 6 runs three separate fixtures — only `approved_by`, only `approved_at`, and both — so the message must be built from what is present rather than hardcoding one key name.

   **Step 5 — compose all four keys, then one write.** Only after all four guards pass:
   ```go
   if err := task.SetStatus(domain.TaskStatusInProgress); err != nil {
       return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
   }
   task.SetPhase(domain.TaskPhasePlanning.Ptr())
   task.Set("approved_by", approvedBy)
   task.Set("approved_at", now.Time())

   if err := o.taskStorage.WriteTask(ctx, task); err != nil {
       return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write task")
   }

   return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
   ```
   `WriteTask` is called **exactly once**, after all four keys are on the map. No intermediate write, no second write, no write on a refusal path. Only these four keys are touched — every other key, known or unknown, is preserved because the map itself is preserved.

3. **`approved_at` must be stored as a bare `time.Time`, via `now.Time()`.** This is the one non-obvious line in the prompt and it is deliberate, so do not substitute a nicer-looking type:

   `libtime.DateTime` (what `Now()` returns), `libtime.DateOrDateTime`, and a plain Go `string` all implement `encoding.TextMarshaler`, so yaml.v3 quotes them. Measured against `gopkg.in/yaml.v3 v3.0.1` in this repo's module graph:

   ```
   libtime.DateTime(ts)        →  approved_at: "2026-03-03T12:00:00Z"   (quoted)
   libtime.DateOrDateTime(ts)  →  approved_at: "2026-03-03T12:00:00Z"   (quoted)
   string("2026-03-03T12:00:00Z") → approved_at: "2026-03-03T12:00:00Z" (quoted)
   ts  (a bare time.Time)      →  approved_at: 2026-03-03T12:00:00Z     (unquoted)
   ```

   Spec 056 DB 3 requires the value to be written **unquoted** and AC 1 requires the value read straight off the file to parse as RFC3339, so `now.Time()` — a `time.Time` — is the required value. It also round-trips stably: `parseToFrontmatterMap` reads the unquoted scalar back as a `time.Time` and re-serializes it byte-identically. `docs/development-patterns.md` documents exactly this rendering for date keys ("parses to `time.Time` and re-serializes as RFC3339").

   The consequence is recorded deliberately: `approved_at` is therefore written by a different mechanism than the repository's libtime-typed date keys. A second consequence follows from the unquoted form: `parseToFrontmatterMap` reads it back as a `time.Time`, and `TaskFrontmatter.GetField` carries no `approved_at` case, so `task get <name> approved_at` renders it in Go's default format (`2026-03-03 12:00:00 +0000 UTC`) rather than RFC3339 — every other date key has a `dateFieldString` case and does not. Nothing in this spec reads the key back as a string, so this is accepted, not fixed; do not add a reader that expects RFC3339 from `task get`.

4. **Add the unit specs.** New file `pkg/ops/task_approve_test.go`, external `ops_test` package, Ginkgo v2 / Gomega, modelled on `metrics_session_append_test.go`. Use `mockStorage := &mocks.Storage{}` (spec 056 AC 7 names a counterfeiter `Storage` mock), stub `FindTaskByNameReturns` and `WriteTaskReturns(nil)` in `BeforeEach`, and pin the clock:
   ```go
   pinned := libtime.NewCurrentDateTime()
   pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
   approveOp := ops.NewTaskApproveOperation(mockStorage, pinned)
   ```
   Cover, each as its own spec or `DescribeTable` row:
   - **The single-write property (AC 7).** A `todo` row: `err` is nil, `mockStorage.WriteTaskCallCount()` is exactly `1`, and the task handed to `WriteTaskArgsForCall(0)` carries `status: in_progress`, `phase: planning`, `approved_by: operator` and an `approved_at`. A three-write implementation must fail here.
   - **The injected instant (AC 8).** Assert the written `approved_at` equals the pinned instant. Read it as a raw type assertion — `raw, ok := written.Get("approved_at").(time.Time)`; `Expect(ok).To(BeTrue())`, then `Expect(raw.Equal(pinned.Now().Time())).To(BeTrue())`. Do **not** read it back with `written.GetTime("approved_at")`: `GetTime` accepts `time.Time`, `libtime.DateOrDateTime` and `string` alike, so it cannot tell the three apart and would pass with the wrong type. The equality assertion already fails if the implementation reaches for `time.Now()`.
   - **The serialization contract (DB 3 — the boundary this prompt's new value crosses).** Marshal the map handed to `WriteTask` with `yaml.Marshal(written.RawMap())` (`gopkg.in/yaml.v3` is already a direct dependency) and assert the rendered bytes carry the value unquoted:
     ```go
     out, marshalErr := yaml.Marshal(written.RawMap())
     Expect(marshalErr).NotTo(HaveOccurred())
     Expect(string(out)).To(ContainSubstring("approved_at: 2026-03-03T12:00:00Z"))
     Expect(string(out)).NotTo(ContainSubstring(`approved_at: "2026-03-03T12:00:00Z"`))
     ```
     Then unmarshal `out` back into a `map[string]any` and assert the round-tripped value is a `time.Time` equal to the pinned instant. This is the test that fails if the implementation substitutes `libtime.DateTime`, `libtime.DateOrDateTime` or a `string` — the substitution requirement 3 exists to forbid. Add `time` and `gopkg.in/yaml.v3` to the test file's imports.
   - **The named approver.** `Execute(..., "Manager Layer")` writes `approved_by: Manager Layer`.
   - **The empty approver is refused with zero writes (AC 2).** `Execute(..., "")` returns an error whose message contains `approved_by`, and both `mockStorage.FindTaskByNameCallCount()` and `mockStorage.WriteTaskCallCount()` are `0` — the check runs before the read.
   - **The zero clock is refused with zero writes.** Pin `SetNow(libtimetest.ParseDateTime("0001-01-01T00:00:00Z"))` (or `libtime.DateTime(time.Time{})`) on a valid `todo` row: error containing `approved_at`, `WriteTaskCallCount()` equal to `0`.
   - **The phase refusal (AC 4, AC 5, DB 4).** A `DescribeTable` over the non-inbox phases — `planning`, `execution`, `ai_review`, `human_review`, `done`, plus one `Entry` for a task whose map has no `phase` key at all — each asserting: an error occurs, its message contains the row's own current phase string (and `todo`), and `WriteTaskCallCount()` is `0`. The per-phase rows are load-bearing: the refusal must be pinned to "not `todo`", not to one specific wrong phase.
   - **The existing-record refusal (AC 6).** A `DescribeTable` over three `todo` fixtures — only `approved_by` set, only `approved_at` set, both set — each asserting: an error occurs, its message names the key(s) present, and `WriteTaskCallCount()` is `0`. The one-key-each rows are load-bearing; a combined fixture alone cannot distinguish "refuses either key" from "refuses one key".
   - **Key preservation.** Seed a `todo` row carrying `assignee`, `priority`, `page_type`, `task_identifier` and an unknown `custom_key`, approve it, and assert every one of those five values is identical on the task handed to `WriteTask` and that `custom_key` is still present. This is the spec-056 AC 3 property at unit level — it fails if the implementation rebuilds the frontmatter from a literal instead of composing onto the existing map.
   - **Wrapped failures.** `FindTaskByNameReturns(nil, errors.New(ctx, "boom"))` → the error's message contains `find task` and `WriteTaskCallCount()` is `0`; `WriteTaskReturns(errors.New(ctx, "boom"))` → the error's message contains `write task`.
   - **The not-found class survives.** `FindTaskByNameReturns(nil, storage.ErrNotFound)` → `errors.Is(err, storage.ErrNotFound)` is true and `WriteTaskCallCount()` is `0`. Prompt 2's multi-vault dispatch branches on exactly that class, and a substring-only assertion would still pass if the class were lost. Add the `storage` import to the test file.

5. **Generate the mock and restore `mocks/mocks.go`.** The counterfeiter annotation from requirement 1 produces `mocks/task-approve-operation.go`; run `make generate` (do not hand-write the mock). ⚠️ `make generate` does `rm -rf mocks` and then `echo "package mocks" > mocks/mocks.go`, which **strips the four-line copyright header** from `mocks/mocks.go` — a pre-existing generator defect, not yours to fix. Restore it with `git checkout -- mocks/mocks.go` as the **last** action, after the final `make precommit` (see `<verification>`), and do not carry the deletion.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` / `git diff --exit-code` / `git checkout --` only read or restore; never stage or commit.
- **No `time.Now()`.** The instant comes only from the injected `libtime.CurrentDateTime`. This is a repository-wide rule (`docs/development-patterns.md` § Testability) and spec 056 AC 8 fails if it is broken.
- **No `fmt.Errorf`.** Use `github.com/bborbe/errors` (`errors.Errorf`, `errors.Wrap`, `errors.Wrapf`) with a `ctx`. Never a bare `return err`.
- **No stdout in `pkg/ops/`.** `pkg/ops` is a library layer: operations return `MutationResult`, the CLI formats output. No `fmt.Print*`, no `os.Stdout`.
- **`MutationResult` is not extended.** Do not add a `phase` field (or any field) to it — spec 056 renders the phase in the CLI layer (prompt 2), and a new field on a shared struct widens every other mutation verb's JSON payload.
- **`task set` and its guard are frozen.** `pkg/ops/frontmatter.go` — including `checkPhaseRegression` and `frontmatterSetOperation.Execute` — must be byte-identical to its committed state. This spec adds a verb; it does not change an existing one. Verify with `git diff --exit-code -- pkg/ops/frontmatter.go`. Prompt 5 later narrows this file deliberately (it adds the `task set` guard for a `todo → planning` move), so this freeze holds for this prompt's own execution order only — do not treat it as permanent.
- **No `--force`, no bypass.** The refusal has no escape hatch; re-opening a closed transition is `task set --force phase <earlier>`, which already exists. Do not add a `force` parameter to the operation or to `Execute`.
- **Only four keys are written.** `status`, `phase`, `approved_by`, `approved_at`. No other key is added, deleted, renamed or reordered in the map, and `approved_by`/`approved_at` stay plain scalar frontmatter values — no new field type, no nested map, no schema object.
- **`approved_at` is a bare `time.Time` (`now.Time()`), not `libtime.DateTime`, not `libtime.DateOrDateTime`, not a string** — see requirement 3 for the measured reason. A quoted rendering fails spec 056 DB 3.
- **Do not touch the CLI.** `pkg/cli/` is prompt 2's scope; `docs/` and `CHANGELOG.md` are prompt 3's. This prompt changes `pkg/ops/`, `mocks/` and nothing else.
- **Tests use Ginkgo v2 / Gomega with a counterfeiter mock**, in the external `ops_test` package, following the `pkg/ops/metrics_session_append_test.go` idiom. Do not add a new suite file — `pkg/ops/ops_suite_test.go` already exists.
- Existing tests must still pass, including every spec in `pkg/ops/frontmatter_test.go` that pins the phase-regression guard.
</constraints>

<verification>
PRIMARY GATE — evidence greps. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'TaskApproveOperation' pkg/ops/task_approve.go        # >= 3 (annotation, interface, constructor)
grep -c 'NewTaskApproveOperation' pkg/ops/task_approve.go     # >= 1
grep -c 'WriteTaskCallCount' pkg/ops/task_approve_test.go     # >= 4 (the one-write pin plus the zero-write refusals)
grep -c 'libtimetest.ParseDateTime' pkg/ops/task_approve_test.go   # >= 1 (the injected clock, AC 8)
grep -c 'yaml.Marshal' pkg/ops/task_approve_test.go                      # >= 1 (the DB 3 serialization contract)
grep -c 'approved_at: 2026-03-03T12:00:00Z' pkg/ops/task_approve_test.go # >= 1 (the unquoted form is asserted)
grep -c 'time.Now()' pkg/ops/task_approve.go                  # 0 (write as: ! grep -q 'time.Now()' pkg/ops/task_approve.go)
grep -c 'fmt.Errorf' pkg/ops/task_approve.go                  # 0 (write as: ! grep -q 'fmt.Errorf' pkg/ops/task_approve.go)
grep -c 'approved_by' pkg/ops/task_approve.go                 # >= 2 (the write and the empty-approver refusal message)
grep -c 'approved_at' pkg/ops/task_approve.go                 # >= 2 (the write and the clock refusal message)
grep -c 'TaskApprove' mocks/task-approve-operation.go         # >= 1 (the generated mock exists)
ls mocks/task-approve-operation.go                            # exists
```

FROZEN-NEIGHBOUR GUARD — these must exit 0 with empty output:
```
git diff --exit-code -- pkg/ops/frontmatter.go
git diff --exit-code -- pkg/ops/frontmatter_entity.go
git diff --exit-code -- mocks/mocks.go      # run AFTER the final make precommit + restore below
git status --porcelain -- pkg mocks         # must list exactly these three, each untracked: ?? pkg/ops/task_approve.go, ?? pkg/ops/task_approve_test.go, ?? mocks/task-approve-operation.go
# (git diff --name-only cannot see them: the constraint forbids staging, so all three are untracked and git diff prints an empty list — which would read as a pass.)
```

TESTS:
```
go test ./pkg/ops/...        # exits 0
go test ./pkg/domain/...     # exits 0 — unchanged, but this prompt must not break it
make test                    # exits 0
```

⚠️ **Guard the ops run against a false pass.** After `go test ./pkg/ops/...`, confirm the new specs actually ran — `go test ./pkg/ops/ -ginkgo.focus="TaskApproveOperation" -ginkgo.v` and grep the log for the new spec names (`WriteTaskCallCount`, `approved_at`, the refusal rows). A focus run that matches nothing exits 0 while running nothing.

SYNTAX:
```
gofmt -e -l pkg/ops/task_approve.go pkg/ops/task_approve_test.go    # must list NO files
```

FULL GATE — `make precommit` at the repo root must exit 0. If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make vet`, `make vulncheck`, `make check-changelog`, ...), then `make precommit` once more. `make precommit` does **not** run `check-versions` (that is release-time only, `make release-check`), so the four version strings stay untouched: this prompt adds no release, and the `## Unreleased` CHANGELOG bullet is prompt 3's job.

⚠️ **`make precommit`'s `generate` target re-runs `rm -rf mocks` + `echo "package mocks" > mocks/mocks.go`, which strips the copyright header.** The prerequisite list is `ensure format generate test check addlicense`, so `addlicense` runs after `generate` and normally re-adds the header — but the strip has reached a commit before (`fa1e596` → `42cd4ff` on `mocks/mocks.go`), so do not rely on it. Make the restoration the **last** action, after the final `make precommit`:
```
git checkout -- mocks/mocks.go
git diff --exit-code -- mocks/mocks.go   # the generator's header strip is not carried
```
</verification>
