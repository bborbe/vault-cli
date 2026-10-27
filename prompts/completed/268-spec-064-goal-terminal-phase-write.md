---
status: completed
spec: [064-bug-completed-goal-keeps-non-terminal-phase]
execution_id: vault-cli-goal-phase-exec-268-spec-064-goal-terminal-phase-write
dark-factory-version: v0.198.7
created: "2026-10-27T23:05:49Z"
queued: "2026-10-27T23:19:44Z"
started: "2026-10-27T23:19:46Z"
completed: "2026-10-27T23:23:21Z"
branch: dark-factory/bug-completed-goal-keeps-non-terminal-phase
---

# Write `phase: done` on both goal terminal transitions

<summary>
- Completing a goal through the CLI now lands `phase: done` in the same write that lands `status: completed`.
- Aborting a goal, or setting it to `completed` with `goal set`, lands `phase: done` in the same write that lands the terminal status.
- The phase never moves on any other command: a status change to a non-terminal value, a defer, an approve refusal, or a write to an unrelated field leaves it exactly as it was.
- There is still exactly one persistence call per command, so a goal file can never be seen with the new status and the old phase.
- The two terminal routes are covered by tests that count the writes and inspect the goal that was written, not just the command's printed output.
- The file on disk is verified end to end: the phase is read back from the written file and through the CLI's `goal get` verb.
- Existing behaviour is untouched: refusals still write nothing, close-out fields still land in the same write, and no new flag, config key, or storage method appears.
</summary>

<objective>
Make both goal terminal transitions write `phase: done` on the in-memory goal before the single existing `WriteGoal`, so a goal that reaches a terminal status reads `phase: done` on disk instead of keeping whatever phase it had when it closed. Tasks already hold this contract (`pkg/ops/complete.go`, `task.SetPhase(domain.TaskPhaseDone.Ptr())`); goals are the only entity kind whose lifecycle walk terminates through a CLI command and does not write its end state. This is spec 064 prompt 1 of 3: it covers Desired Behaviors 1 and 2 and Acceptance Criteria 1–7.
</objective>

<context>
Read `CLAUDE.md` first for project conventions, then `docs/dod.md` for the definition of done this change is graded against.

Read these files before changing anything:

- `pkg/ops/goal_complete.go` — `goalCompleteOperation.Execute` is the completion route. It finds the goal, refuses an already-completed goal, optionally checks open tasks, calls `g.setCompletedStatus(...)` (which writes `aborted_reason`, `gate_successor` and the completed status into the in-memory frontmatter), then `goal.SetCompleted(...)`, then one `g.goalStorage.WriteGoal(ctx, goal)`. There is no `phase` reference in this file today.
- `pkg/ops/frontmatter_entity.go` — `goalSetOperation.Execute` is the `goal set` route. It calls `writeGoalCloseOutFieldsIfCloseOut(ctx, goal, value, reason, gateSuccessor)` before `goal.SetField(ctx, key, value)` and before its single `o.goalStorage.WriteGoal(ctx, goal)`. `writeGoalCloseOutFieldsIfCloseOut` currently derives `target := domain.GoalStatus(value)` and, for `aborted` / `completed`, writes the close-out fields only.
- `pkg/ops/complete.go` — the task precedent. In `completeOperation.Execute`, immediately after `c.setCompletedStatus(...)` and before the single `WriteTask`, the code calls `task.SetPhase(domain.TaskPhaseDone.Ptr())`. Mirror that position and that call shape.
- `pkg/domain/goal_phase.go` — `domain.GoalPhaseDone` is `GoalPhase = "done"`, and `func (g GoalPhase) Ptr() *GoalPhase` returns a pointer to a copy.
- `pkg/domain/goal_frontmatter.go` — `func (f *GoalFrontmatter) SetPhase(p *GoalPhase)` stores the phase string and deletes the key when `p` is nil; `func (f GoalFrontmatter) Phase() *GoalPhase` reads it back (nil when absent); `func (f *GoalFrontmatter) SetStatus(s GoalStatus) error` validates and, for `aborted`, refuses unless `aborted_reason` and `gate_successor` are already non-empty in the map. `domain.Goal` embeds `GoalFrontmatter`, so `goal.SetPhase(...)` and `goal.Phase()` are available on the entity.
- `pkg/ops/goal_complete_test.go` — the existing Ginkgo suite for the completion route; `mockGoalStorage.WriteGoalCallCount()` and `mockGoalStorage.WriteGoalArgsForCall(0)` are the assertion primitives (precedent: `pkg/ops/complete_test.go` asserts `WriteTaskCallCount()` the same way).
- `pkg/ops/frontmatter_entity_test.go` — `Describe("NewGoalSetOperation")` holds the `goal set` cases, including `Context("one-step close-out via status set")`.
- `integration/cli_test.go` — `createTempVaultWithGoals(tasks map[string]string, goals map[string]string)` builds a temp vault with `Tasks/` and `Goals/` plus a config; `Describe("vault-cli goal close-out gating")` is the neighbouring goal suite. `binPath` is the binary built by `gexec.Build` in `integration/integration_suite_test.go`. The suite runs under `go test ./...`, so `make test`/`make precommit` execute it.
- `specs/in-progress/064-bug-completed-goal-keeps-non-terminal-phase.md` — read Desired Behavior 1–2, Acceptance Criteria 1–7, Constraints and Failure Modes.

Coding-plugin docs (in-container paths):

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — error wrapping with `github.com/bborbe/errors`, interface/constructor/struct shape.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite files, Counterfeiter mocks, external test packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap(ctx, err, "...")`, never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — linter limits (funlen 80, nestif 4, golines 100) and banned packages.
</context>

<requirements>
1. **Completion route — `pkg/ops/goal_complete.go`.** In `goalCompleteOperation.Execute`, after the successful `g.setCompletedStatus(ctx, goal, reason, gateSuccessor)` call and next to the existing `goal.SetCompleted(...)` call, add:

   `goal.SetPhase(domain.GoalPhaseDone.Ptr())`

   Position it exactly where `pkg/ops/complete.go` places `task.SetPhase(domain.TaskPhaseDone.Ptr())`: after the close-out status is set in memory, before the single `g.goalStorage.WriteGoal(ctx, goal)`. Do not add a second `WriteGoal`, a new storage method, or a new return field. The `domain` package is already imported in this file.

2. **`goal set` route — `pkg/ops/frontmatter_entity.go`.** Extend `writeGoalCloseOutFieldsIfCloseOut` so the terminal transition also writes the phase, and confine it to status writes:

   - Change its signature to `func writeGoalCloseOutFieldsIfCloseOut(ctx context.Context, goal *domain.Goal, key, value, reason, gateSuccessor string) error` and update its single call site in `goalSetOperation.Execute` to pass `key`.
   - First statement of the body: return `nil` when `key != "status"`. This guard is load-bearing: the function is called for every `goal set`, and without it a command such as `goal set "<name>" theme completed` — a non-status key whose value happens to be the word `completed` — would rewrite the phase and the close-out fields.
   - Keep the existing terminal test on `value` (`domain.GoalStatusAborted` / `domain.GoalStatusCompleted`) and the existing close-out field writes exactly as they are.
   - After the close-out field writes and before `return nil`, add `goal.SetPhase(domain.GoalPhaseDone.Ptr())`. The caller's single `o.goalStorage.WriteGoal(ctx, goal)` then persists status, phase and close-out fields together.
   - Extend the function's GoDoc: it now writes the terminal phase (`phase: done`) in the same in-memory mutation as the close-out fields, and it does nothing for a non-`status` key or a non-terminal status value.

   On the refusal path this is safe by construction: `goal.SetField(ctx, "status", "aborted")` still validates the close-out fields, and when it returns an error the caller returns before `WriteGoal`, so the in-memory phase change is discarded and the file stays byte-identical.

3. **Single-write contract.** Each route must still call its storage write exactly once. Do not write the status and the phase in two calls, even though the end state would be the same. The write count is asserted in requirement 4 and 5.

4. **Unit tests — `pkg/ops/goal_complete_test.go`.** Keep the existing cases; extend them and add the following. In every assertion read the goal handed to `WriteGoalArgsForCall(0)`, not the returned `MutationResult`:

   - In `Context("success")` (goal fixture has `status: active` and no `phase` key), assert `mockGoalStorage.WriteGoalCallCount()` equals 1 and the written goal's `Status()` is `domain.GoalStatusCompleted` and `*writtenGoal.Phase()` equals `domain.GoalPhaseDone`.
   - In `Context("goal completes without close-out fields")`, assert the same phase outcome.
   - In `Context("one-step close-out writes reason and successor with the status")`, assert all four values on the same written goal: `Status()` is `completed`, `GetString("aborted_reason")` is `"achieved"`, `GetString("gate_successor")` is `"none"`, and the phase is `done`, with `WriteGoalCallCount()` equal to 1. This is the atomicity evidence for Acceptance Criterion 2.
   - Add `Context("goal at phase execution completes to phase done")` with a fixture built from `map[string]any{"status": "in_progress", "phase": "execution"}`; assert exactly one write and that the written goal is `completed` with phase `done`. This is the regression case for the bug: before this prompt the phase stays `execution`.
   - Keep the refusal cases (`already completed`, open-task block) asserting `WriteGoalCallCount()` equals 0.

5. **Unit tests — `pkg/ops/frontmatter_entity_test.go`.** Keep the existing cases; extend them and add the following inside `Describe("NewGoalSetOperation")`, asserting on `mockGoalStorage.WriteGoalArgsForCall(0)`:

   - In `Context("setting a string field")` (`key = "status"`, `value = "completed"`), additionally assert the written goal's phase is `domain.GoalPhaseDone`.
   - In `Context("one-step close-out via status set")` (`status aborted` with reason and successor), additionally assert the written goal's phase is `domain.GoalPhaseDone` and `WriteGoalCallCount()` equals 1. This is Acceptance Criterion 7's evidence.
   - Add `Context("status aborted on a goal at phase execution")`: fixture `map[string]any{"status": "in_progress", "phase": "execution"}`, `key = "status"`, `value = "aborted"`, `reason = "probe"`, `gateSuccessor = "none"`; assert one write, status `aborted`, phase `done`.
   - Add `Context("status completed on a goal at phase execution")`: same fixture, `key = "status"`, `value = "completed"`; assert one write, status `completed`, phase `done`.
   - Add `Context("non-terminal status leaves the phase untouched")`: same fixture, `key = "status"`, `value = "in_progress"`; assert one write and that the written goal's phase is still `domain.GoalPhaseExecution`.
   - Add `Context("a non-status key whose value is a terminal status word leaves the phase untouched")`: same fixture, `key = "theme"`, `value = "completed"`; assert the write succeeded and the written goal's phase is still `domain.GoalPhaseExecution`. This locks the `key != "status"` guard from requirement 2.

6. **Integration tests — `integration/cli_test.go`.** Add a new `Describe("vault-cli goal terminal phase", func() { ... })` after the existing `Describe("vault-cli goal close-out gating")`. In `BeforeEach`, build the vault with `createTempVaultWithGoals(map[string]string{}, map[string]string{"my-goal": "---\nstatus: in_progress\nphase: execution\n---\n# My Goal\n"})`, and call `cleanup()` in `AfterEach`. Drive the binary through `gexec.Start(exec.Command(binPath, "--config", configPath, "--vault", "test", <args>...), GinkgoWriter, GinkgoWriter)` and `Eventually(session).Should(gexec.Exit(<code>))`, exactly like the neighbouring goal suite. Read the file at `filepath.Join(vaultPath, "Goals", "my-goal.md")` with `os.ReadFile` and the `//#nosec G304 -- test file` comment. Add these cases:

   - `goal complete my-goal` exits 0; the file contains `status: completed` and `phase: done`; a follow-up `goal get my-goal phase` exits 0 and its trimmed stdout equals `done`. (Acceptance Criterion 1.)
   - `goal complete my-goal --reason achieved --gate-successor none` exits 0; the file contains `status: completed`, `phase: done`, `aborted_reason: achieved` and `gate_successor: none`. (Acceptance Criterion 2.)
   - `goal set my-goal status aborted --reason probe --gate-successor none` exits 0; the file contains `status: aborted` and `phase: done`; `goal get my-goal phase` prints `done`. (Acceptance Criterion 4.)
   - `goal set my-goal status completed` exits 0; the file contains `status: completed` and `phase: done`; `goal get my-goal phase` prints `done`. (Acceptance Criterion 5.)
   - Non-terminal confinement (Acceptance Criterion 6), three separate assertions in one `It` or in three `It`s: `goal set my-goal status in_progress` exits 0 and the file still contains `phase: execution`; `goal defer my-goal +7d` exits 0 and the file still contains `phase: execution`; `goal approve my-goal` exits non-zero and the file still contains `phase: execution`. `goal approve` is refused on this fixture because the goal is at `phase: execution`, not in the approval inbox (`pkg/ops/goal_approve.go` refuses anything but `phase: todo`) — assert the non-zero exit, do not assert exit 0.

7. **Self-check.** Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above and Acceptance Criteria 1–7 of the spec against the change, and confirm no lint rule was touched (that is prompt 2's scope).
</requirements>

<constraints>
- `pkg/ops/` stays free of stdout and of `time.Now()`: the phase is set on the in-memory entity, and the operations keep returning `MutationResult`. No new print statement in the operations.
- Both operations keep their current `MutationResult` shape. Do NOT add `Status` or `Phase` fields to `MutationResult` — the acceptance criteria read the phase back from the file, not from the CLI's output.
- The single-`WriteGoal` contract is frozen for both routes. A change that writes the status and the phase in two calls does not satisfy this spec even if the end state is correct.
- Do NOT add a `--force` / `--skip-phase` flag or a config key that suppresses the phase write. The write is the fix; a bypass re-opens the hole this spec closes.
- Do NOT modify `pkg/ops/lint.go` in this prompt — widening lint Rule 2 is prompt 2's scope.
- Do NOT touch `writeTaskCloseOutFieldsIfCloseOut` in `pkg/ops/frontmatter.go`. The task-side `task set … status completed` gap is the same class of bug but out of scope for spec 064 (the spec's Goal is goal-scoped); leave it unchanged and do not "fix it while you are here".
- Do NOT change the `goal set` refusal for `todo → planning`, the `goal approve` approval record, entity-name resolution, file permissions (0600), or the symlink refusal in the write path.
- Tests follow repository convention: Ginkgo v2 / Gomega, Counterfeiter mocks, external test packages (`package ops_test`).
- Existing tests must still pass. Do NOT commit — dark-factory handles git.
</constraints>

<verification>
make precommit

# both terminal write paths set the phase
grep -q 'goal.SetPhase(domain.GoalPhaseDone.Ptr())' pkg/ops/goal_complete.go
grep -q 'goal.SetPhase(domain.GoalPhaseDone.Ptr())' pkg/ops/frontmatter_entity.go

# the completion route still performs exactly one write
test "$(grep -c 'g.goalStorage.WriteGoal(ctx, goal)' pkg/ops/goal_complete.go)" -eq 1

# the close-out hook is confined to status writes
grep -q 'key != "status"' pkg/ops/frontmatter_entity.go
</verification>

<!--
Notes for the human auditor — deviations from the spec's literal wording, resolved
against the source:

1. The spec's Acceptance Criterion 6 writes `goal defer Alpha --until <date>`. The CLI
   has no `--until` flag: `createGoalDeferCommand` declares `Use: "defer <goal-name>
   [date]"` with the date as a positional argument. Requirement 6 therefore uses
   `goal defer my-goal +7d`.

2. Acceptance Criterion 6 also lists `goal approve Alpha` among the commands that leave
   the phase untouched. On a goal at `phase: execution` the command is REFUSED
   (`goalApproveOperation.Execute` accepts only `phase: todo`), so the prompt asserts a
   non-zero exit plus an unchanged phase rather than exit 0. The criterion's own
   evidence is "phase unchanged", which the refusal satisfies.

3. `writeGoalCloseOutFieldsIfCloseOut` gains a `key` parameter. It is called for every
   `goal set`, and its terminal test looks only at `value`; without the guard a command
   like `goal set "<name>" theme completed` would rewrite the phase and the close-out
   fields. The guard also removes that pre-existing hazard. It is an unexported
   function with one call site.
-->

<!-- DARK-FACTORY-REPORT -->
