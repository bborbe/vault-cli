---
status: completed
spec: [058-bug-approve-status-removes-row-from-spawn-offer]
summary: 'Changed vault-cli task approve to write status: next instead of in_progress, moved the unit and integration assertions that pinned the old value, added spec 058 AC 2''s in_progress fixture and spec, and corrected the approval-status claim in docs/task-writing.md and commands/plan-task.md.'
execution_id: vault-cli-approve-next-exec-245-spec-058-approve-writes-next-status
dark-factory-version: v0.196.0
created: "2026-09-30T07:10:08Z"
queued: "2026-09-30T07:20:47Z"
started: "2026-09-30T07:26:56Z"
completed: "2026-09-30T07:31:05Z"
---

# Approve leaves the row queued, not owned (spec 058, prompt 1 of 2)

<summary>
- Approving a task no longer removes it from the fleet's list of work it may start.
- Before this change, approving a row was the same act that made every manager skip it: the row was recorded as "someone is already on this", so no manager ever offered it.
- An approved row now reads as queued, which is what it is — nobody has picked it up yet.
- The row therefore reaches the manager's spawn offer, and the manager's own readiness checks decide whether to open it or hold it.
- Nothing else about approval changes: the same four fields are written together in the same single write, and the owner is resolved exactly as before.
- Every refusal is unchanged — a row past the inbox, a row that already carries an approval record, an empty approver, and a row with no resolvable owner all still fail without touching the file.
- The command's output is byte-identical, in both the plain and the JSON form.
- The repository's own task-writing documentation states the value the command now writes.
- Rows approved before this change keep their old value; this changes what the command writes from now on, not existing files.
</summary>

<objective>
Make `vault-cli task approve` leave the task in the state the fleet acts on: `phase: planning`, `status: next`, `approved_by`, and an RFC3339 `approved_at`, in the same single write and with every other key untouched. This prompt covers spec 058 Desired Behaviors 1, 3, 4, 5 and 7 and Acceptance Criteria 1, 2, 3, 4, 5, 6 and 8 (Desired Behavior 5 is enforced by `<verification>`; Desired Behavior 2 is the operator-side Post-Deploy Acceptance Criterion and is not a prompt concern). It is the precondition for prompt 2, which records the change in the changelog.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

The bug in one paragraph, because it is what makes requirement 1 the whole fix: the manager sweep's `ready-to-start` bucket requires `status: next` — both renderers gate on it, the vault's model-free sweep gate returning `🚀 ready-to-start` only inside `if status == "next"` and rendering `🔄 progressing` for `status == "in_progress"`, and the claude-supervisor plugin's manager-sweep reader stating the same rule in words. The approval write produced `status: in_progress`, so the act of approving a row was the same act that disqualified it from ever being offered. Nothing in this repository reads that classification; the fix is one value in one write, and the file's own frontmatter is the observable.

Read fully before changing anything:

- `pkg/ops/task_approve.go` — the whole file. This is the file you are changing. Note `Execute`'s guard order (`approved_by` empty → `FindTaskByName` → clock → phase-`todo` → existing approval record → `resolveOwner` → compose the keys → exactly one `WriteTask`), and the compose block that calls `SetStatus` / `SetPhase` / `Set("approved_by", …)` / `Set("approved_at", …)` / the conditional `SetAssignee` before that single write. Also read `resolveOwner` and `refuseExistingApprovalRecord`: both are frozen (spec Constraints) and must be byte-identical when you finish.
- `pkg/domain/task_status.go` — the status constants. `TaskStatusNext TaskStatus = "next"` is the canonical queued value ("means the task is queued for action but not yet started"); `TaskStatusInProgress TaskStatus = "in_progress"` means "someone is actively working on the task"; `TaskStatusTodo` is a legacy read alias and must never be written.
- `pkg/domain/task_frontmatter.go` — `TaskFrontmatter.SetStatus(s TaskStatus) error` (validates the value against `AvailableTaskStatuses` and stores it) and `Status()`. `domain.Task` embeds `TaskFrontmatter`, so both are promoted onto the task returned by `FindTaskByName`.
- `pkg/ops/task_approve_test.go` — the whole file. Copy its idiom: external `ops_test` package, `mockStorage := &mocks.Storage{}`, the local `seedTask(fields map[string]any)` helper, and a pinned clock via `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))`. Exactly one line in this file pins the value you are changing.
- `integration/cli_test.go` — the `Describe("task approve", …)` block in full, plus `createTempVault` and `createTempVaultWithCurrentUser` near the top of the file. `createTempVault` writes a config with **no** `current_user`; `createTempVaultWithCurrentUser` writes `current_user: tester@example.com`. Which helper each fixture uses is load-bearing here (see requirement 4). Note the block's helpers — `readFile`, `valueOf`, `sha256OfFile`, `approvedKeys`, `withoutApprovalKeys`, `frontmatterOf` — and the comment above the fixtures recording that every fixture's keys are authored in **alphabetical** order, because the storage writer re-serializes the whole frontmatter alphabetically on every write.
- `docs/task-writing.md` — the `### Phase transitions` section only (search for `**The approval is`). One sentence there states the value the command writes; that sentence is the whole doc change.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — read only to confirm that no bullet is required in this prompt (see requirement 7).

**Scope note.** A repo-wide search returns exactly one further site stating the superseded value: `commands/plan-task.md`'s `phase`-`planning` bullet reads *"the approval already wrote `status: in_progress`"*, a claim this change makes false. It is corrected by requirement 5, so this change leaves no repo doc contradicting the shipped behaviour. (`CHANGELOG.md`'s occurrence is released history and `specs/completed/*` are immutable — both correctly untouched.)
</context>

<requirements>
1. **Change the approval write's status value in `pkg/ops/task_approve.go`.** In `taskApproveOperation.Execute`, the compose block currently opens with:

   ```go
   if err := task.SetStatus(domain.TaskStatusInProgress); err != nil {
       return MutationResult{
           Success: false,
           Error:   err.Error(),
       }, errors.Wrap(ctx, err, "set status")
   }
   ```

   Change the constant to `domain.TaskStatusNext`:

   ```go
   if err := task.SetStatus(domain.TaskStatusNext); err != nil {
       return MutationResult{
           Success: false,
           Error:   err.Error(),
       }, errors.Wrap(ctx, err, "set status")
   }
   ```

   Add a short comment above the call recording why the value is `next` and not `in_progress`: an approved row with no live session is *queued to be started*, not *active*, and the manager sweep's `ready-to-start` bucket requires `status: next` in both renderers — so writing `in_progress` was the write that removed the row from the spawn offer. **Do not write the literal string `SetStatus(domain.TaskStatusInProgress)` anywhere in this file, comments included** — the verification greps the whole file for it, and a comment quoting the old call would fail that check. Writing the bare word `in_progress` in prose is fine; the literal call form is not.

   Nothing else in `Execute` moves. The guard order, all four refusals, `task.SetPhase(domain.TaskPhasePlanning.Ptr())`, `task.Set("approved_by", approvedBy)`, `task.Set("approved_at", now.Time())`, the conditional `task.SetAssignee(owner)` and the single `taskStorage.WriteTask(ctx, task)` call are all unchanged. There is still exactly one `SetStatus(` call and exactly one `WriteTask(` call in the file.

2. **Move the unit assertion that pins the old value.** In `pkg/ops/task_approve_test.go`, in the spec `It("writes all four approval keys in exactly one write", …)`, change:

   ```go
   Expect(written.Status()).To(Equal(domain.TaskStatusInProgress))
   ```

   to:

   ```go
   Expect(written.Status()).To(Equal(domain.TaskStatusNext))
   ```

   This is the only occurrence of `TaskStatusInProgress` in the file. Change nothing else in `pkg/ops/task_approve_test.go`: every other spec — the approval-key assertions, the four ownership specs, the two `DescribeTable`s, the find/write/not-found error specs — stays byte-identical and green. This assertion is the file's record of what the write produces, so it moves with the write.

3. **Move the integration assertion that pins the old value.** In `integration/cli_test.go`, inside `Describe("task approve", …)`, in the spec `It("AC1: task approve records the four-key transition in one write", …)`, change:

   ```go
   Expect(valueOf(after, "status")).To(Equal("in_progress"))
   ```

   to:

   ```go
   Expect(valueOf(after, "status")).To(Equal("next"))
   ```

   Leave the rest of that spec intact: it must keep asserting `approvedKeys(after)` has length 4, `phase == "planning"`, `approved_by == "operator"`, `assignee == "tester@example.com"`, and that `approved_at` parses as RFC3339 and is not the zero time. This line is the only `Equal("in_progress")` in the file. Spec 058 AC 1 supersedes spec 056's assertion here, whose `in_progress` expectation is the behaviour being corrected — the fixture (`todoFrontmatter`) and the helper (`createTempVaultWithCurrentUser`) are already correct and do not change.

4. **Add spec 058 AC 2's integration spec and fixture** — the write must not depend on the pre-approval status.

   **Fixture.** Beside the block's other fixtures (after `todoFrontmatter`), add one whose keys are authored in **alphabetical** order — the storage writer re-serializes frontmatter alphabetically, so a non-alphabetical fixture makes the assertion meaningless:

   ```go
   // AC 2 (058)'s fixture: a row that is already in_progress while still sitting in
   // the approval inbox. The pre-fix command always wrote in_progress, so this arm
   // distinguishes "always writes next" from "keeps whatever was there".
   inProgressFrontmatter := `---
   page_type: task
   phase: todo
   priority: 1
   status: in_progress
   task_identifier: 11111111-1111-4111-8111-111111111111
   ---
   body line
   `
   ```

   **Spec.** Add it inside `Describe("task approve", …)`, directly after the AC1 spec:

   ```go
   It("AC2 (058): the write does not depend on the pre-approval status", func() {
       vaultPath, configPath, cleanup = createTempVaultWithCurrentUser(map[string]string{
           "Alpha": inProgressFrontmatter,
       })
       taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")

       session := runEntityCommand("task", "approve", "Alpha")
       Eventually(session).Should(gexec.Exit(0))

       after := readFile(taskFile)
       Expect(valueOf(after, "status")).To(Equal("next"))
   })
   ```

   The `(058)` qualifier is deliberate and is not cosmetic: the block already carries an `AC2:` spec from spec 057 (`AC2: an existing assignee is kept and no warning is printed`) and an `AC2a:` / `AC2b:` pair from spec 056, so an unqualified `AC2:` would make the label ambiguous for a reader grepping the block. The parenthetical form matches the block's existing `AC4 (nil phase)` / `AC4 (unowned)` convention.

   Three things about the fixture choice are load-bearing:
   - **`createTempVaultWithCurrentUser`, not `createTempVault`.** The row carries no `assignee`, so against `createTempVault`'s config (no `current_user`) the owner guard refuses it and the command exits 1 — a correct implementation would fail this spec. The `current_user: tester@example.com` in this helper's config is what supplies the owner. Spec 058's fixture convention states this for every AC that exercises the write.
   - **The spec asserts the `status` line only.** Do not add an `assignee` assertion, a hash comparison, or an `approvedKeys` length assertion here — AC 3's key-preservation evidence is already pinned by the existing `AC3: task approve preserves every unrelated key` spec, and duplicating it here would give the block two specs whose failure modes overlap.
   - **The fixture is `status: in_progress`, and that is the whole point of the arm.** AC 2 exists to prove the command overwrites a pre-existing `in_progress` rather than preserving it — that value is the one that removes the row from the offer. Do not change it to `status: next` (which would make the arm duplicate AC 1) and do not delete it.

   Do not modify any other spec in the block. In particular the six refusal specs — `AC2b` (empty `--by`), `AC4` (a `planning` row), `AC4 (nil phase)`, `AC5` (an `execution` row), `AC6` (the three approval-record fixtures) and `AC4 (unowned)` — already assert exactly the evidence spec 058 AC 4 requires (six exit codes plus six unchanged hashes). Leave them byte-identical and let them stay green; do not switch their helper, add a `current_user` to them, or reword them.

5. **Correct the documentation.** In `docs/task-writing.md`, in the `### Phase transitions` section, in the paragraph beginning `**The approval is `vault-cli task approve`.**`, replace the clause

   ```
   The approval writes `status: in_progress`, `phase: planning`, `approved_by` and `approved_at` together in one write, so a row can never sit at `planning` without a record beside it.
   ```

   with

   ```
   The approval writes `status: next`, `phase: planning`, `approved_by` and `approved_at` together in one write, so a row can never sit at `planning` without a record beside it.
   ```

   That substitution is the **entire** documentation change: do not reword the rest of the sentence, do not add a rationale clause, do not touch the paragraph's other sentences, the `### Phase transitions` table, or any other part of the file. In particular **do not write the string `in_progress` anywhere in that sentence** — the verification asserts that the approval sentence names `next` and carries no `in_progress` claim, and a rationale clause would trip the second half of that check. The one other `status: in_progress` occurrence in the file (`status: in_progress walks an inner phase lifecycle…`, the generic lifecycle sentence) is not an approval claim and stays exactly as it is.

   **Then correct the second site that states the superseded value.** In `commands/plan-task.md`, the `phase`-`planning` bullet opens with the claim that the approval already wrote `status: in_progress`. Replace that clause with the claim that the approval leaves `status: next`, and change nothing else in the bullet — in particular leave the following promotion sentence intact, because that promotion is plan-task's own behaviour and is unchanged; it now simply applies to every freshly-approved row rather than only to rows still at `next`/`backlog`. Touch no other part of `commands/plan-task.md`.

6. **Change nothing about the command's surface.** `pkg/cli/cli.go` is untouched by this prompt: `createTaskApproveCommand` keeps its `--by` and `--assignee` flags, its `currentUser` resolution, its plain output (`✅ Approved %s: phase %s`), its JSON object (`{"success": true, "name": …, "phase": "planning"}`) and its non-zero exit on a refusal. Spec 058 AC 5 is satisfied by construction, and the existing `AC9a`, `AC9b`, `AC9c` specs pin it.

   The four refusals are frozen (spec Constraints): an empty `--by`; a `phase` that is not `todo` (including a row with no `phase` key); a row already carrying `approved_by` or `approved_at`; and a row with no resolvable owner (`resolveOwner`). Do not add, remove, reorder, relax or reword a guard, and do not touch `resolveOwner` or `refuseExistingApprovalRecord`. The `in_progress` write was never a reason for any of them, and fixing it must not loosen any of them. No new flag, no `--force`, no bypass.

7. **Do not touch `CHANGELOG.md` in this prompt.** Spec 058's decomposition assigns the `## Unreleased` bullet to prompt 2. `make precommit` does not require one — its `check-changelog` target verifies the file's structure (no `## ` section above the preamble), not the presence of a bullet — so the tree is green without it. Do not add a version bump either: `.maintainer.yaml` sets `release.autoRelease: true`, so the releaser owns the version strings and the tag.

8. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, then walk spec 058's AC 1, 2, 3, 4, 5, 6 and 8 against the change: the four key lines read from the file; the second arm's `status` line; the stripped-slice equality spec still green; the six refusal exit codes and six unchanged hashes; the plain and JSON stdout specs still green; `make precommit` green; and the corrected approval sentence with no `in_progress` claim beside it. Confirm `pkg/ops/task_approve.go` still contains exactly one `SetStatus(` call and exactly one `WriteTask(` call, and that `resolveOwner` and `refuseExistingApprovalRecord` are unchanged.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit. Note that in-container `git` does not work in this project directory (it is a git worktree, so `.git` is a `gitdir:` pointer file): any `git` command fails with `fatal: not a git repository`, and the daemon records a pass regardless of exit code. Use the non-git checks in `<verification>`.
- **The change is one frontmatter value in one write.** Spec Non-goal: "Changing the `ready-to-start` predicate" is out of scope — the bucket's requirement of `status: next` is correct and is shared by both renderers; the write is what is wrong. Do not widen the predicate anywhere.
- **`phase: planning`, `approved_by` and `approved_at` keep their exact spellings and formats** — `approved_at` stays a bare `time.Time` (RFC3339 on the wire) and the vault's frontmatter re-serialization order is unchanged. The four keys travel in a single `WriteTask`, so a sweep never observes a half-written row.
- **The refusal conditions are frozen — all four**: a row whose `phase` is not `todo`, a row already carrying `approved_by` or `approved_at`, an empty `--by`, and a row with **no resolvable owner** (`resolveOwner`, `pkg/ops/task_approve.go`; added in v0.155.0 and asserted by the `AC4 (unowned)` integration spec). The owner guard is named explicitly because this prompt's own fixtures must satisfy it rather than trip it — which is why requirement 4 uses `createTempVaultWithCurrentUser`.
- **No new flag, no `--force`, no bypass.** Re-opening a closed transition remains `task set --force phase <earlier>`.
- **The command's stdout and its `--output json` shape are frozen.** `pkg/cli/cli.go` is not in this prompt's scope.
- **Do not repair existing data.** Spec Non-goal: rows already carrying `status: in_progress` are a data condition, not a code path. Nothing in this prompt walks a vault or rewrites an existing file — only the one task being approved is written.
- **Do not change the resume gate, its clause 11, or any phase/approval-key/refusal semantics.** Spec Non-goals: the resume gate's refusal is correct and this spec does not touch it; `phase` semantics, the approval keys and the refusal conditions are frozen by spec 056.
- **Tests follow repository convention:** Ginkgo v2 / Gomega, against the `integration/` harness's temp vault. The fixture-authoring rule from spec 056 holds — the storage writer re-serializes frontmatter in alphabetical key order, so every fixture is authored alphabetically or byte comparisons become meaningless.
- **`pkg/ops/` is a library layer: no stdout.** Operations return structured results; the CLI layer owns all output formatting. Do not add a print, a log line or a `MutationResult.Warnings` entry in this change.
- **Scope is `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `integration/cli_test.go`, `docs/task-writing.md` and `commands/plan-task.md`.** `CHANGELOG.md` is prompt 2's. `pkg/cli/cli.go`, `pkg/ops/frontmatter.go`, `pkg/ops/workon.go`, `pkg/domain/`, `mocks/`, the rest of `commands/`, `agents/`, `scenarios/` and `.dark-factory.yaml` are nobody's scope in this prompt. `commands/plan-task.md` is in scope for exactly one clause in one bullet — nothing else in it moves.
- **Do not hand-bump a version or tag.** This repo is `autoRelease: true` via `.maintainer.yaml`; the releaser owns the bump and the tag. `make precommit` does not run `check-versions` (that is `make release-check`, at release time), so the four version strings stay untouched here.
- **Do not hunt for artifacts outside this repository.** Spec 058 cites `sweep-gate.py` (the Personal vault's model-free sweep gate) and `agents/manager-sweep-reader.md` (the claude-supervisor plugin) as evidence for the bug, not as modification targets. They are not in this repository and must not be searched for, opened or edited.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…`, which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every spec in `pkg/ops/task_approve_test.go` and the whole `Describe("task approve", …)` block in `integration/cli_test.go`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. Absence assertions are written as `! grep -q` because `grep -c` exits 1 when it prints `0`.

**PRIMARY GATE — the write moved, and the old value is gone.**

```
grep -n 'SetStatus(domain.TaskStatusNext)' pkg/ops/task_approve.go          # exactly 1 line
! grep -q 'SetStatus(domain.TaskStatusInProgress)' pkg/ops/task_approve.go  # no match, anywhere in the file — comments included
grep -c 'SetStatus(' pkg/ops/task_approve.go                               # exactly 1 — the compose block still sets exactly one status
grep -c 'WriteTask(' pkg/ops/task_approve.go                               # exactly 1 — the single-write property is unchanged
```

**The two assertions that pinned the old value have moved.**

```
grep -c 'domain.TaskStatusNext' pkg/ops/task_approve_test.go               # exactly 1
! grep -q 'domain.TaskStatusInProgress' pkg/ops/task_approve_test.go       # no match

grep -c 'Expect(valueOf(after, "status")).To(Equal("next"))' integration/cli_test.go   # exactly 2 — AC 1's moved assertion plus AC 2's new one
! grep -q 'Equal("in_progress")' integration/cli_test.go                   # no match — the old assertion is gone
grep -c 'inProgressFrontmatter' integration/cli_test.go                    # exactly 2 — the fixture constant and its single use
grep -c 'createTempVaultWithCurrentUser' integration/cli_test.go            # 12 — 11 at HEAD plus AC 2's new spec
```

**The refusals are intact.** Each of these must still be present exactly once; the specs themselves are unchanged:

```
grep -c 'AC2b: an empty --by is refused and nothing is written' integration/cli_test.go     # 1
grep -c 'AC4: a task past the inbox is refused and nothing is written' integration/cli_test.go  # 1
grep -c 'AC4 (nil phase): a row with no phase key is refused and nothing is written' integration/cli_test.go  # 1
grep -c 'AC5: a task further along is refused and nothing is written' integration/cli_test.go  # 1
grep -c 'AC6: a task carrying any approval record is refused, per fixture' integration/cli_test.go  # 1
grep -c 'AC4 (unowned): a resolvable-owner-free approve is refused and nothing is written' integration/cli_test.go  # 1
```

**The stdout contract is intact** (`pkg/cli/cli.go` untouched):

```
grep -c 'AC9a: the default plain output names the task and the new phase' integration/cli_test.go  # 1
grep -c 'AC9b: --output json carries the task name and the new phase' integration/cli_test.go      # 1
```

**The documentation states the value the command writes.**

```
grep -c 'status: next' docs/task-writing.md                                # exactly 2 — line ~83's example plus the corrected approval sentence
grep -c 'approval writes' docs/task-writing.md                             # exactly 1
! grep -n 'approval writes' docs/task-writing.md | grep -q 'in_progress'   # the approval sentence names next, not in_progress
! grep -n 'status: in_progress' docs/task-writing.md | grep -qi approval   # zero lines — no approval-related in_progress claim remains
! grep -n 'already wrote' commands/plan-task.md | grep -q 'in_progress'    # zero lines — plan-task no longer claims the approval wrote in_progress
grep -c 'the approval leaves' commands/plan-task.md                        # exactly 1 — the corrected clause is present
```

**TESTS.**

```
go test ./pkg/ops/... -count=1
make test
make precommit
```

All three must exit 0. `make test` runs the full suite including `integration/`, which builds the real binary and asserts against a real task file; `make precommit` runs `ensure format generate test check addlicense`.

⚠️ **Guard the unit run against a false pass.** A focus that matches nothing exits 0 while running nothing, so confirm the changed spec actually ran:

```
go test ./pkg/ops/... -count=1 -ginkgo.focus='TaskApproveOperation' -ginkgo.v 2>&1 | grep -c 'writes all four approval keys in exactly one write'   # >= 1
```

⚠️ **Guard the integration run against a false pass.** `make test` runs without `-ginkgo.v`, so a suite that was skipped rather than run also looks green. Confirm the new spec ran, and that the focus matched something:

```
go test ./integration/... -count=1 -ginkgo.focus='task approve' -ginkgo.v 2>&1 | grep -c 'AC2 (058): the write does not depend on the pre-approval status'   # >= 1
```

**SYNTAX.**

```
gofmt -e -l pkg/ops/task_approve.go pkg/ops/task_approve_test.go integration/cli_test.go   # must list NO files
```

**SELF-CHECK before finishing:** re-run the PRIMARY GATE and the TESTS, and walk spec 058's AC 1 (the four key lines read from the file, `approved_at` RFC3339 and non-zero), AC 2 (the `in_progress` fixture lands at `next`), AC 3 (the stripped-slice equality spec still green), AC 4 (the six refusal specs green with their exit codes and unchanged hashes), AC 5 (the plain and JSON stdout specs green), AC 6 (`make precommit` exit 0) and AC 8 (the corrected approval sentence, no `in_progress` claim beside it) against the change. Confirm `resolveOwner` and `refuseExistingApprovalRecord` are byte-identical to their committed state.
</verification>
