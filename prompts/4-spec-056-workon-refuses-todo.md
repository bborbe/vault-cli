---
spec: ["056-task-approve-command"]
status: draft
created: "2026-09-28T08:18:48Z"
---

# Refuse a `todo` row in `task work-on` (spec 056, prompt 4 of 5)

<summary>
- `vault-cli task work-on` no longer moves a task out of the operator's approval inbox.
- A task still waiting at `phase: todo` is refused: the command exits non-zero and its message names the approval command the operator must run instead.
- The refusal writes nothing at all, so the task file is byte-identical after the failed attempt.
- A task that has already been approved — `planning` or any later phase — keeps working exactly as it does today.
- A task that carries no phase value at all keeps entering the workflow as it does today, so nothing that worked before this change breaks.
- The status-only promotion (`next` → `in_progress`) is untouched.
- This closes the second unrecorded route into `planning`: the agent-facing `plan-task` bypass is closed by prompt 3 and the `work-on` path behind the Vault UI Start button by this one. `vault-cli task set <name> phase planning` is the third and is closed by prompt 5.
- Covered at two levels: a unit case over the operation, and an end-to-end case that builds the real binary and asserts the exit code plus the untouched file.
- This prompt changes only the `work-on` operation and its tests; the approve operation, its CLI surface, the docs and the CHANGELOG are prompts 1–3.
</summary>

<objective>
Stop `pkg/ops/workon.go`'s `advancePhaseIfEntering` from advancing a task that is still waiting in the approval inbox. Today it writes `phase: planning` (and, through `Execute`, `status: in_progress`) on any row whose phase is nil or `todo`, with no `approved_by` and no `approved_at` — so `vault-cli task work-on`, the path behind the Vault UI Start button, is a second way to reach `planning` unrecorded. After this change a `todo` row is refused with an error naming `vault-cli task approve` and nothing is written; a row already at `planning` or later, and a row with no phase at all, are unaffected. This prompt covers spec 056 Desired Behavior 10 and the two Acceptance Criteria that assert it.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully (in this order):
- `pkg/ops/workon.go` — the whole file. Three regions matter: the `Execute` doc comment near line 63 and its body from `_ = task.SetStatus(domain.TaskStatusInProgress)` near line 90 through the `WriteTask` call near line 98; and `advancePhaseIfEntering` near line 179, the function this prompt changes. Note the file already imports `context`, `github.com/bborbe/errors` and `libtime` — no import changes are needed.
- `pkg/ops/workon_test.go` — the `Context("phase advancement")` block near line 803. Its four cases pin the current behaviour: `when phase is missing (nil)`, `when phase is empty string` and `when phase is todo` all assert the written phase is `planning`, and `when phase is in_progress (resume case)` asserts it is left alone. Read the block's shared `BeforeEach` (near line 43) and `JustBeforeEach` (near line 80) too: three of the four cases re-stub `mockTaskStorage.FindTaskByNameReturns(task, nil)` with their own `task` (the nil case uses the shared default task set in the outer `BeforeEach`), then the shared `JustBeforeEach` calls `Execute` and assigns `result` and `err`.
- `pkg/ops/metrics_session_append_test.go` — the whole file. The ops unit-spec idiom: external `ops_test` package, `mocks.TaskStorage`, a local `seedTask` helper, `WriteTaskCallCount()` / `WriteTaskArgsForCall(0)` to inspect what was written, and an `assertRefused` closure for the "nothing was written" cases.
- `pkg/domain/task_frontmatter.go` — `Phase()` near line 99 and `SetPhase` near line 339. **`Phase()` returns `nil` for a missing `phase` key AND for an empty one** (`GetString` returns `""`, and `""` short-circuits to `nil`), which is why the existing nil and empty-string cases behave identically. `SetPhase(nil)` deletes the key.
- `pkg/domain/task_phase.go` — `TaskPhase`, `TaskPhaseTodo` (`"todo"`), `TaskPhasePlanning` (`"planning"`), `Ptr()`.
- `pkg/domain/task_status.go` — `TaskStatusInProgress`.
- `pkg/cli/cli.go` — `createWorkOnCommand` near line 410 and `formatWorkOnResult` near line 481. **Read only, do not change.** `formatWorkOnResult` returns the error it is handed, and `cli.Execute` near line 2880 prints it to stderr and calls `os.Exit(1)`. That is how the refusal becomes a non-zero exit; no CLI change is needed or wanted.
- `pkg/ops/vault_dispatcher.go` — the whole file. **Read only, do not change.** In the multi-vault case, a non-`storage.ErrNotFound` error (which is what the refusal is) is returned immediately rather than continuing to the next vault, so the refusal reaches the user intact.
- `integration/cli_test.go` — lines 85-135 (`createTempVaultWithCurrentUser`) and lines 2589-2630 (a `Describe` whose local `runEntityCommand` / `sha256OfFile` / `readFile` closures you will copy). Read those two regions, not the whole 4000-line file.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` / `errors.Wrap(ctx, ...)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-state-machine-pattern.md` — phase transitions as an explicit guard rather than an incidental branch.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct; unexported helpers.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so the `git diff --exit-code` / `git diff --name-only` / `git checkout --` guards in `<verification>` genuinely run here. Prompts 1–3 carry this same note.
</context>

<requirements>
1. **Make `advancePhaseIfEntering` refuse a `todo` row.** It is an unexported function with exactly one caller (`Execute`, near line 96 — verified with `grep -rn 'advancePhaseIfEntering' pkg/`), so its signature may change freely. Replace the body near line 179 verbatim from this old code:

   ```go
   // advancePhaseIfEntering moves a task into the planning phase only when entering
   // the workflow (current phase nil or "todo"). Resuming a mid-flight task
   // (in_progress, ai_review, human_review, done, ...) must not reset progress backward.
   func advancePhaseIfEntering(task *domain.Task) {
   	if currentPhase := task.Phase(); currentPhase == nil || *currentPhase == domain.TaskPhaseTodo {
   		task.SetPhase(domain.TaskPhasePlanning.Ptr())
   	}
   }
   ```

   to this new code:

   ```go
   // advancePhaseIfEntering moves a task into the planning phase when it enters the
   // workflow, and refuses a task that is still waiting in the approval inbox.
   //
   // Entering means the task carries no phase at all (a missing "phase" key or an
   // empty one): such a row predates the phase lifecycle or was filed before the
   // field was written, and it is advanced to planning exactly as it always has been.
   //
   // A row at "todo" is the operator's approval inbox. Leaving it is the operator's
   // approval and is performed by `vault-cli task approve`, which records approved_by
   // and approved_at in the same write. Advancing it here would produce a planning row
   // with no approval record — the state the approval gate exists to prevent — so this
   // refuses and the caller writes nothing.
   //
   // Resuming a mid-flight task (planning, execution, in_progress, ai_review,
   // human_review, done, ...) must not reset progress backward.
   func advancePhaseIfEntering(ctx context.Context, task *domain.Task, taskName string) error {
   	currentPhase := task.Phase()
   	if currentPhase != nil && *currentPhase == domain.TaskPhaseTodo {
   		return errors.Errorf(
   			ctx,
   			"refusing to work on %q: task is at phase %q, not yet approved; run `vault-cli task approve %q` first",
   			taskName,
   			string(domain.TaskPhaseTodo),
   			taskName,
   		)
   	}
   	if currentPhase == nil {
   		task.SetPhase(domain.TaskPhasePlanning.Ptr())
   	}
   	return nil
   }
   ```

   Two things about this shape are load-bearing:
   - The literal substring `vault-cli task approve` appears in the source. Spec 056's work-on grep AC asserts `grep -cE 'task approve' pkg/ops/workon.go` is ≥ 1, and the unit case in requirement 4 asserts the returned error contains it. Do not paraphrase the command into words like "the approve command" or "task approval" — the command name itself must be there.
   - The comparison names `domain.TaskPhaseTodo` rather than the string literal `"todo"` — a readability choice, not a spec requirement. (The spec dropped the `TaskPhaseTodo`-grep half of that AC as vacuous: the string already occurs at HEAD, so the expectation would pass with no change at all.)

2. **Hoist the call in `Execute` above the two mutations, and return the refusal early.** In `Execute` (the body near lines 88-98), replace this old region:

   ```go
   	_ = task.SetStatus(domain.TaskStatusInProgress)

   	if w := applyAssigneeMatrix(task, assignee); w != "" {
   		warnings = append(warnings, w)
   	}

   	advancePhaseIfEntering(task)

   	if err := w.taskStorage.WriteTask(ctx, task); err != nil {
   ```

   with this new region:

   ```go
   	if err := advancePhaseIfEntering(ctx, task, taskName); err != nil {
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, err
   	}

   	_ = task.SetStatus(domain.TaskStatusInProgress)

   	if w := applyAssigneeMatrix(task, assignee); w != "" {
   		warnings = append(warnings, w)
   	}

   	if err := w.taskStorage.WriteTask(ctx, task); err != nil {
   ```

   Consequences that make this the required ordering, not a style choice:
   - The refusal happens before `SetStatus` and before `applyAssigneeMatrix`, so on the refusal path the task object is not mutated at all, and `WriteTask` is never reached — the no-write guarantee is structural rather than a rewrite that happens to be stable.
   - Returning the `MutationResult{Success: false, Error: err.Error()}` alongside the raw error matches every other early return in this function. The error is created by `errors.Errorf` inside `advancePhaseIfEntering`, so there is no underlying error to wrap: return it as-is, never re-wrapped in a way that would drop the message (the CLI prints exactly this string and exits 1).
   - `FindTaskByName` still runs first, so a task that resolves in no vault keeps its existing not-found behaviour and `VaultDispatcher.FirstSuccess`'s `storage.ErrNotFound` handling is untouched.

3. **Update the `Execute` doc comment** near line 63, which currently claims the operation "advances phase to planning when entering the workflow (current phase nil/empty/\"todo\")". Replace it with a comment that states the new contract, in the file's own voice:

   ```go
   // Execute marks a task as in_progress, assigns it, and starts or resumes a Claude
   // session. A task that enters the workflow (no phase at all) is advanced to planning;
   // a task still waiting in the approval inbox (phase "todo") is refused and nothing is
   // written — leaving the inbox is the operator's approval and is performed by
   // `vault-cli task approve`. A mid-flight phase (planning, execution, in_progress,
   // ai_review, human_review, done, ...) is preserved.
   ```

   A stale comment that still advertises the `todo` advance is the exact defect this prompt removes; do not leave it behind.

4. **Update the unit specs in `pkg/ops/workon_test.go`.** The existing `Context("when phase is todo")` case (near line 834) asserts the written phase becomes `planning`, which the new behaviour contradicts — it must be replaced, not left to fail.

   a. Replace that block with this refusal block:

   ```go
   		Context("when phase is todo (the approval inbox)", func() {
   			BeforeEach(func() {
   				task = domain.NewTask(
   					map[string]any{"status": "todo", "phase": "todo"},
   					domain.FileMetadata{
   						Name:     taskName,
   						FilePath: "/path/to/vault/tasks/my-task.md",
   					},
   					domain.Content(""),
   				)
   				mockTaskStorage.FindTaskByNameReturns(task, nil)
   			})

   			It("refuses with an error naming the approve command", func() {
   				Expect(err).To(HaveOccurred())
   				Expect(err.Error()).To(ContainSubstring("vault-cli task approve"))
   			})

   			It("returns Success=false", func() {
   				Expect(result.Success).To(BeFalse())
   			})

   			It("leaves the phase at todo", func() {
   				Expect(task.Phase()).NotTo(BeNil())
   				Expect(*task.Phase()).To(Equal(domain.TaskPhaseTodo))
   			})

   			It("writes nothing", func() {
   				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
   			})
   		})
   ```

   b. Add this second block, so the change is pinned to the `todo` case rather than to "work-on stopped working" (spec 056's second work-on AC case (the `phase: planning` row must still succeed)):

   ```go
   		Context("when phase is planning (already past the gate)", func() {
   			BeforeEach(func() {
   				task = domain.NewTask(
   					map[string]any{"status": "next", "phase": "planning"},
   					domain.FileMetadata{
   						Name:     taskName,
   						FilePath: "/path/to/vault/tasks/my-task.md",
   					},
   					domain.Content(""),
   				)
   				mockTaskStorage.FindTaskByNameReturns(task, nil)
   			})

   			It("returns no error", func() {
   				Expect(err).To(BeNil())
   			})

   			It("leaves the phase at planning", func() {
   				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
   				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
   				Expect(writtenTask.Phase()).NotTo(BeNil())
   				Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
   			})

   			It("still promotes status to in_progress", func() {
   				Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
   				_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
   				Expect(writtenTask.Status()).To(Equal(domain.TaskStatusInProgress))
   			})
   		})
   ```

   `WriteTaskArgsForCall(0)` is `Execute`'s own write; the second write on the fresh-start path is the pre-spawn session persist (see the `Fresh run records one entry` case near line 147), so call 0 is the one that carries the phase.

   c. **Leave the other two cases exactly as they are.** `when phase is missing (nil)` and `when phase is empty string` both assert the written phase becomes `planning`, and both must keep passing: `Phase()` returns `nil` for a missing key and for an empty one, and requirement 1 advances only the `nil` case. `when phase is in_progress (resume case)` is unchanged. This is not a judgement call about completeness — refusing a nil-phase row would break the existing integration specs and the shipped example vault, which is why the refusal is pinned to `todo` (see the reviewer note after `<verification>`).

5. **Add an end-to-end case through the real binary in `integration/cli_test.go`.** The unit case in requirement 4 proves the operation refuses; it cannot prove the CLI exits non-zero and leaves the file untouched, and the user-visible contract is the whole point of DB 10 ("the path behind the Vault UI Start button"). Spec 056's scenario-coverage note names this harness as the place where exit codes and file content are asserted against a temp vault.

   Add a new `Describe` block inside the top-level `Describe("vault-cli integration tests", ...)` (near line 435), immediately after the `Describe("vault-cli task JSON schema", ...)` block closes and immediately above `Describe("vault-cli blocked_by JSON surface", ...)`. Model the local closures on the `Describe("task append-metrics-session", ...)` block near line 2589 — copy its `runEntityCommand`, `sha256OfFile` and `readFile` helpers and its `AfterEach(func() { cleanup() })` verbatim, including the `//#nosec G304 -- test file` comments. The two specs:

   ```go
   	Describe("task work-on approval gate", func() {
   		var vaultPath, configPath string
   		var cleanup func()

   		AfterEach(func() {
   			cleanup()
   		})

   		runEntityCommand := func(args ...string) *gexec.Session {
   			fullArgs := append(
   				[]string{"--config", configPath, "--vault", "test"},
   				args...,
   			)
   			session, err := gexec.Start(exec.Command(binPath, fullArgs...), GinkgoWriter, GinkgoWriter)
   			Expect(err).NotTo(HaveOccurred())
   			return session
   		}

   		sha256OfFile := func(path string) string {
   			data, err := os.ReadFile(path) //#nosec G304 -- test file
   			Expect(err).NotTo(HaveOccurred())
   			sum := sha256.Sum256(data)
   			return fmt.Sprintf("%x", sum)
   		}

   		readFile := func(path string) string {
   			content, err := os.ReadFile(path) //#nosec G304 -- test file
   			Expect(err).NotTo(HaveOccurred())
   			return string(content)
   		}

   		It("refuses a todo row, names the approve command, and writes nothing", func() {
   			vaultPath, configPath, cleanup = createTempVaultWithCurrentUser(map[string]string{
   				"Alpha": `---
   page_type: task
   phase: todo
   priority: 1
   status: next
   task_identifier: 11111111-1111-4111-8111-111111111111
   ---
   body line
   `,
   			})
   			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")
   			before := sha256OfFile(taskFile)

   			session := runEntityCommand("task", "work-on", "Alpha", "--mode", "headless")
   			Eventually(session, 30*time.Second).Should(gexec.Exit(1))
   			Expect(string(session.Err.Contents())).To(ContainSubstring("vault-cli task approve"))
   			Expect(sha256OfFile(taskFile)).To(Equal(before))
   		})

   		It("still works on a row already at phase planning", func() {
   			vaultPath, configPath, cleanup = createTempVaultWithCurrentUser(map[string]string{
   				"Alpha": `---
   page_type: task
   phase: planning
   priority: 1
   status: next
   task_identifier: 11111111-1111-4111-8111-111111111111
   ---
   body line
   `,
   			})
   			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")

   			session := runEntityCommand("task", "work-on", "Alpha", "--mode", "headless")
   			Eventually(session, 30*time.Second).Should(gexec.Exit(0))
   			after := readFile(taskFile)
   			Expect(after).To(ContainSubstring("phase: planning"))
   			Expect(after).To(ContainSubstring("status: in_progress"))
   		})
   	})
   ```

   Notes on why this shape works, so you do not "fix" it: `createTempVaultWithCurrentUser` supplies the `current_user` that `task work-on` requires, and its `claude_script` is deliberately not installed, so the starter is nil. On the refusal path the session is never reached at all; on the `planning` path the nil starter produces the soft `ErrStarterUnavailable` warning and the command exits 0 after writing the status. Both specs therefore finish in well under a second and neither spawns a process. `binPath`, `exec`, `gexec`, `Eventually`, `time`, `filepath`, `os`, `fmt`, `sha256` and `GinkgoWriter` are all already in the file's import set.

6. **Change nothing else.** Specifically, and verified against the current tree:
   - `pkg/cli/cli.go` — no change. `formatWorkOnResult` already returns the error and `cli.Execute` already exits 1.
   - `pkg/ops/vault_dispatcher.go` — no change. A non-`storage.ErrNotFound` error is already returned immediately.
   - `pkg/ops/frontmatter.go` — frozen by spec 056's constraints (`task set` and its `checkPhaseRegression` guard). This prompt adds no verb and no flag.
   - `docs/`, `CHANGELOG.md`, `commands/plan-task.md` — prompt 3's scope.
   - `pkg/domain/`, `pkg/storage/`, `mocks/` — no interface changes here, so no mock regeneration is needed and `make generate` must produce no new file.
   - `scenarios/` — no scenario changes. Verified: `scenarios/002-task-lifecycle.md` works on `example/vault/24 Tasks/Simple Task.md`, which has `status: todo` and **no** `phase` key, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` uses `phase: execution` and is unaffected either way, so it is not evidence for the nil-phase carve-out; both keep passing under a `todo`-only refusal.
   - The two existing integration specs that actually invoke `task work-on` (`integration/cli_test.go` line 2775 on `Beta` and line 2862 on `Alpha`) both use fixtures with **no** `phase` key, so they keep passing unchanged. The `--help` registration entry at line 464 only runs `task work-on --help`. Do not edit any of them.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` / `git diff --exit-code` / `git checkout --` only read or restore; never stage or commit.
- **The refusal is pinned to `todo`, and only to `todo`.** A row with no `phase` key, or an empty one, keeps advancing to planning; a row at `planning` or later is untouched. Widening the refusal to a nil phase would break the existing unit cases, the two existing integration specs that invoke `task work-on` and the shipped example vault, and spec 056 DB 10 does not ask for it.
- **The refusal writes nothing.** It returns before `SetStatus`, before `applyAssigneeMatrix` and before `WriteTask`. No partial frontmatter, no key reordering, no `approved_at`.
- **No `fmt.Errorf`.** Use `github.com/bborbe/errors` (`errors.Errorf`, `errors.Wrap`) with the `ctx` already in scope. Never a bare `return err`, and never `errors.New` from the standard library.
- **No `time.Now()`.** The file's clock is the injected `libtime.CurrentDateTime`; do not add a direct clock read. (The refusal reads no clock at all.)
- **No stdout in `pkg/ops/`.** `pkg/ops` is a library layer: the operation returns a `MutationResult` and an error, and the CLI formats output. No `fmt.Print*`, no `os.Stdout`.
- **Do not change `MutationResult`.** Do not add a field to it; the CLI renders the phase.
- **No bypass flag.** There is no `--force` on `work-on` and none is added. Re-opening a closed transition is `task set --force phase <earlier>`, which already exists and already requires the flag.
- **The refusal error must contain the literal substring `vault-cli task approve`.** Both the spec's work-on grep AC and the unit case assert it; a message that describes the command without naming it fails both.
- **`pkg/cli/` and `pkg/ops/frontmatter.go` must be byte-identical to their committed state.** This prompt changes exactly three files: `pkg/ops/workon.go`, `pkg/ops/workon_test.go` and `integration/cli_test.go`. Prompt 5 later narrows this file deliberately (it adds the `task set` guard for a `todo → planning` move), so this freeze holds for this prompt's own execution order only — do not treat it as permanent.
- Existing tests must still pass. The one existing case this prompt is allowed to change is `Context("when phase is todo")` in `pkg/ops/workon_test.go`; every other spec, including the two existing integration specs that invoke `task work-on`, plus the `--help` registration entry, and the scenario files, stays as it is.
- Tests follow repository convention: Ginkgo v2 / Gomega in the external `ops_test` / `integration_test` packages, counterfeiter mocks from `mocks/`, `Eventually(...).Should(gexec.Exit(n))` for the binary cases. Do not add a new suite file — `pkg/ops/ops_suite_test.go` and `integration/integration_suite_test.go` already exist.
- No `go mod vendor`.
</constraints>

<verification>
PRIMARY GATE — spec 056's work-on evidence greps plus the supporting ones. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'TaskPhaseTodo' pkg/ops/workon.go                  # >= 1 (supporting check only — not a spec AC; the string occurs once at HEAD either way)
grep -c 'task approve' pkg/ops/workon.go                   # >= 1 (spec 056's work-on grep AC; 0 at HEAD)
grep -c 'advancePhaseIfEntering' pkg/ops/workon.go         # >= 2 (the declaration and the call)
grep -c 'vault-cli task approve' pkg/ops/workon_test.go    # >= 1 (the unit case asserts the message)
grep -c 'phase is todo' pkg/ops/workon_test.go             # >= 1 (the refusal case exists)
grep -c 'phase is planning' pkg/ops/workon_test.go         # >= 1 (the still-succeeds case exists)
grep -c 'work-on approval gate' integration/cli_test.go    # >= 1 (the end-to-end block exists)
! grep -q 'time.Now()' pkg/ops/workon.go                   # 0
! grep -q 'fmt.Errorf' pkg/ops/workon.go                   # 0
```

⚠️ The second row is the spec's own AC and it is weak on its own — `task approve` occurs zero times at HEAD, and the first row (`TaskPhaseTodo`) is a supporting check only, not a spec AC (`TaskPhaseTodo` already occurs once in `workon.go` at HEAD inside `advancePhaseIfEntering`). The counts are the spec's evidence, not proof the refusal works; the tests below are the proof.

TESTS:
```
go test ./pkg/ops/... -count=1        # exits 0
go test ./pkg/domain/... -count=1     # exits 0 — untouched, but this prompt must not break it
make test                             # exits 0
```

⚠️ **Guard the focused runs against a false pass.** After `go test ./pkg/ops/...`, confirm the new cases actually ran — a focus expression that matches nothing exits 0 while running nothing:
```
go test ./pkg/ops/ -count=1 -ginkgo.focus='phase is todo' -ginkgo.v      # the refusal case runs and passes
go test ./pkg/ops/ -count=1 -ginkgo.focus='phase is planning' -ginkgo.v  # the still-succeeds case runs and passes
```
Read the log: each run must report the focused spec names, not `0 Passed | 0 Failed`.

```
go test ./integration/ -count=1 -ginkgo.focus='work-on approval gate' -ginkgo.v   # both new specs run and pass
```
This one builds the binary in `BeforeSuite` and takes a couple of minutes; it is worth the wait because it is the only evidence for the exit code. If the integration package reports the known pre-existing flake documented in prompts 2 and 3 (the `topic defer writes defer_date for a relative and an absolute date` spec, which fails for about two hours a day when the local date leads the UTC date) in a spec outside your focus, that is not this prompt's failure — do not "fix" it opportunistically, and do not let it block this prompt.

FROZEN NEIGHBOURS — every one of these must exit 0 with empty output:
```
git diff --exit-code -- pkg/cli/
git diff --exit-code -- pkg/ops/frontmatter.go
git diff --exit-code -- pkg/ops/vault_dispatcher.go
git diff --exit-code -- pkg/domain/ pkg/storage/ mocks/
git diff --exit-code -- docs/ CHANGELOG.md commands/
git diff --exit-code -- scenarios/
```

SCOPE — run this **before** the final `make precommit` (which regenerates `mocks/` and would otherwise muddy the list):
```
git rev-parse --is-inside-work-tree   # must print `true`. If this fails, `.git` is masked (daemon started with --set hideGit=true) and EVERY git row below reports a false pass — fall back to the PRIMARY GATE content greps and report the masked run instead of trusting the git rows
git diff --name-only    # must list exactly: pkg/ops/workon.go, pkg/ops/workon_test.go, integration/cli_test.go
git status --porcelain  # nothing untracked
```
If a file from prompts 1, 2 or 3 appears in that list, those prompts were not committed before this one ran — stop and report it rather than reverting anything.

SYNTAX:
```
gofmt -e -l pkg/ops/workon.go pkg/ops/workon_test.go integration/cli_test.go    # must list NO files
```

FULL GATE — `make precommit` at the repo root must exit 0 (its prerequisites are `ensure format generate test check addlicense`, so it runs the whole suite including your new integration specs). If it fails on something this prompt introduced, fix it and re-run only the failing target (`make lint`, `make vet`, `make vulncheck`, `make check-changelog`, ...), then `make precommit` once more.

⚠️ **`make precommit` runs `generate`, which does `rm -rf mocks` + `echo "package mocks" > mocks/mocks.go` and strips the four-line copyright header from `mocks/mocks.go` — a pre-existing generator defect, not yours to fix.** `addlicense` normally re-adds it, but the strip has reached a commit before (`fa1e596` → `42cd4ff` on `mocks/mocks.go`), so do not rely on it. Make the restoration the **last** action, after the final `make precommit`:
```
git checkout -- mocks/mocks.go
git diff --exit-code -- mocks/mocks.go
```
</verification>

<!--
REVIEWER NOTES — decisions already made. The executing agent reads this file top-down, so
these restate the decision rather than reopen it; do not treat them as open instructions.

1. Nil phase is NOT refused. Spec 056 DB 10 says `advancePhaseIfEntering` "refuses a `todo`
   row" and names the nil-or-`todo` condition only as the *current* behaviour being narrowed.
   `Phase()` returns nil for both a missing and an empty `phase` key, and two existing
   integration specs (`integration/cli_test.go` lines 2775 and 2862) plus the shipped
   `example/vault/24 Tasks/Simple Task.md` carry no `phase` key at all — refusing nil would
   turn those green specs red and is not asked for. The residual gap is deliberate and worth
   the operator's awareness: a nil-phase row is neither approvable (spec 056 DB 4 refuses it
   in `approve`) nor refused by `work-on`, so it can still reach `planning` unrecorded. Rows
   at `todo` are the normal filing state, so this is an edge case rather than the main path.

2. `pkg/ops/workon.go` IS named in prompt 3's CHANGELOG `Change set:` clause, which now lists
   `pkg/ops/task_approve.go`, `pkg/ops/workon.go`, `pkg/ops/frontmatter.go`, `pkg/cli/cli.go`,
   `docs/task-writing.md` and `commands/plan-task.md` — the bullet is complete. The file change itself still lands here,
   in prompt 4, so if this prompt never runs the CHANGELOG over-claims one file.

3. Requirement 5 (the end-to-end integration block) goes one step beyond the two ACs the spec
   attaches to DB 10, both of which are `pkg/ops`-level. It is included because DB 10's stated
   stake is the user-visible path ("the path behind the Vault UI Start button") and a mocked
   storage cannot prove exit 1 plus an untouched file; spec 056's scenario-coverage note names
   this harness for exactly those two assertions. Drop it if the reviewer judges the two
   `pkg/ops` cases sufficient.
-->
