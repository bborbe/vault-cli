---
status: approved
spec: [056-task-approve-command]
created: "2026-09-28T08:38:32Z"
queued: "2026-09-28T09:12:46Z"
---

# Refuse `task set <name> phase planning` on a `todo` row (spec 056, prompt 5 of 5)

<summary>
- The command that moves a task out of the operator's approval inbox by setting its phase directly is now refused.
- A task still waiting for approval is refused: the command exits non-zero and its message names the approval command the operator must run instead.
- The refusal writes nothing at all, so the task file is byte-identical after the failed attempt.
- The override flag does not get past the refusal; it keeps the narrower meaning it already has, overriding a backward phase move on a task that has moved on.
- A task at any other phase behaves exactly as it does today — the command is otherwise untouched, including its pass-through of unrecognised fields.
- The forward move that already works, from a task in execution back to planning, still succeeds, so this is one pinned combination rather than a general phase gate.
- The task's status is never consulted, so the refusal is about the approval inbox alone.
- This closes the third and last unrecorded route into planning — and the one the board's phase drag reaches — so the approval record can no longer be walked around.
- Covered at two levels: unit cases over the operation, and end-to-end cases that build the real binary and assert the exit codes plus the untouched file.
- Nothing else changes: no new verb, no flag, no documentation and no other command.
</summary>

<objective>
`vault-cli task set <name> phase planning` no longer moves a `todo` row out of the operator's approval inbox: the command is refused with an error naming `vault-cli task approve` and writes nothing, so an unrecorded `planning` row can no longer be produced from a shell or from vault-ui's `PATCH /api/tasks/{id}/phase`, which shells out to the same command. Every other phase, every other key, and the `--force` phase-regression flag keep their existing behaviour. Covers spec 056 DB 11 and AC 14.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**Sequencing note, read before you plan.** Prompts 1 and 4 both carry a constraint that `pkg/ops/frontmatter.go` must be byte-identical to its committed state. That freeze was scoped to those prompts — each changes a different file, and prompt 1's constraint says so in as many words ("This spec adds a verb; it does not change an existing one"). Spec 056's Constraints now narrow it: `task set` keeps every behaviour **except the single guarded combination DB 11 names**. This prompt is that sanctioned change and the only one. Run it after prompts 1–4. If `pkg/ops/workon.go` or `pkg/cli/cli.go` appears in `git status` as uncommitted when you start, prompts 1–4 were not committed first — stop and report that rather than reverting anything.

Read fully (in this order):
- `pkg/ops/frontmatter.go` — the whole file. Four regions matter: the refusal cluster at the top of `frontmatterSetOperation.Execute` (the `blockedBySetRefusal` call near line 88 and the `metricsSessionsWriteRefusal` call near line 95 — both return before anything is mutated); `checkPhaseRegression` near line 229, the existing phase guard, whose doc comment records why a forward move always passes it and which must keep its behaviour; the `SetField` call near line 118 and the `WriteTask` call near line 127; and `writeTaskCloseOutFieldsIfCloseOut` near line 140, which mutates the in-memory task. Note the file already imports `context`, `strings`, `github.com/bborbe/errors`, `github.com/bborbe/validation`, `domain` and `storage` — no import changes are needed.
- `pkg/ops/blocked_by_write.go` — `blockedBySetRefusal` near line 64. This is the shape to copy for a `set`-verb refusal: an unexported helper, `errors.Errorf` (not `validation.Error`), a doc comment that states the no-write property and the absence of a bypass, and a guard that returns `nil` for every key it does not own.
- `pkg/ops/metrics_session_write.go` — `metricsSessionsWriteRefusal` near line 25. The closest analogue of all: a one-key guard on a generic write verb, whose doc comment ends "There is no --force bypass — --force is scoped to the phase-regression guard, and this refusal is deliberately absolute."
- `pkg/ops/frontmatter_test.go` — the `FrontmatterSetOperation` `Describe` near line 331: its shared `BeforeEach` near line 347 (default task has neither `status` nor `phase`), its `JustBeforeEach` near line 369 (which calls `Execute` and assigns `err`), the six phase-regression contexts from near line 404, the two must-still-pass cases and the comment above them near line 531, and the `Context("allowing todo -> planning on in_progress task")` case near line 534 that this prompt must replace. `force` is a shared variable, so a `--force` case only sets `force = true`.
- `pkg/domain/task_phase.go` — `TaskPhase`, `TaskPhaseTodo` (`"todo"`), `TaskPhasePlanning` (`"planning"`), `NormalizeTaskPhase`. **Read only, do not change.**
- `pkg/domain/task_frontmatter.go` — `Phase()` near line 99 (**returns `nil` for a missing `phase` key AND for an empty one**, because `GetString` returns `""` and `""` short-circuits to `nil`) and `SetField` near line 504, whose `case "phase"` branch delegates to `setPhaseField` near line 489 (which normalizes through `NormalizeTaskPhase` and refuses an unknown value with `unknown task phase`). **Read only, do not change.**
- `pkg/cli/cli.go` — `createTaskSetCommand` near line 2511, and the `--force` flag's own help text near line 2573. **Read only, do not change.** The refusal becomes exit 1 because `setOp.Execute`'s error is returned from the `RunE` closure and `cli.Execute` prints it to stderr and calls `os.Exit(1)` — the same path the `blocked_by` and `metrics_sessions` refusals already take.
- `integration/cli_test.go` — the `Describe("vault-cli blocked_by set refusal", ...)` block near line 2439: the closest analogue, a `task set` refusal asserted with exit 1, a stderr substring match and an unchanged sha256, with `runEntityCommand` and `sha256OfFile` declared locally inside the Describe. Also read its `It("AC4: ...")` sibling in `Describe("task append-metrics-session", ...)` near line 2810, which runs the same command with `--force` appended and asserts the refusal still fires (near line 2830) — that is the precedent for this prompt's `--force` spec. Also read `createTempVault` near line 27. Read those three regions, not the whole 4000-line file.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Errorf(ctx, ...)` / `errors.Wrap(ctx, ...)` from `github.com/bborbe/errors`; never `fmt.Errorf`, never a bare `return err`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, `DescribeTableSubtree` with an inner `BeforeEach`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-state-machine-pattern.md` — a lifecycle transition guarded by an explicit check rather than an incidental branch.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — interface → constructor → private struct; unexported helpers beside the type they serve.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so the `git diff --exit-code` / `git diff --name-only` / `git checkout --` guards in `<verification>` genuinely run here. Prompts 1–4 carry this same note.
</context>

<requirements>
1. **Add the refusal helper.** In `pkg/ops/frontmatter.go`, immediately after `checkPhaseRegression`, add this unexported function:

   ```go
   // todoPlanningSetRefusal returns an error when a `set` invocation would move a task
   // out of the operator's approval inbox by setting the phase to planning.
   //
   // "todo" is the operator's approval inbox. Leaving it is the operator's approval and
   // is performed by `vault-cli task approve`, which records approved_by and approved_at
   // in the same write. `task set <name> phase planning` writes the field and nothing
   // else, and checkPhaseRegression guards only backward moves, so this forward move
   // passes it — leaving the row at planning with no evidence that anyone approved it.
   // The refusal closes that route.
   //
   // The guard is deliberately one (field, value, current-phase) combination: key
   // "phase", the canonical value "planning", and a current phase of "todo". It is not a
   // general phase gate — a row at any other phase, a missing or empty phase key, an
   // unknown phase value and every non-phase key keep their existing behaviour, and
   // status is never consulted. --force does not bypass it: --force is scoped to a
   // backward phase move on a row already past planning, and this refusal exists because
   // the unrecorded row is the defect rather than a warning to be overridden.
   //
   // Nothing is written on the refusal path: the check runs before any mutation, so the
   // file on disk is byte-identical.
   func todoPlanningSetRefusal(ctx context.Context, task *domain.Task, taskName, key, value string) error {
   	if key != "phase" {
   		return nil
   	}
   	canonical, ok := domain.NormalizeTaskPhase(value)
   	if !ok || canonical != domain.TaskPhasePlanning {
   		return nil
   	}
   	current := task.Phase()
   	if current == nil || *current != domain.TaskPhaseTodo {
   		return nil
   	}
   	return errors.Errorf(ctx,
   		"refusing to set phase %q on %q: the task is at phase %q, the operator's approval inbox. Leaving it is the operator's approval and is performed by `vault-cli task approve %q`, which records approved_by and approved_at in the same write",
   		canonical, taskName, string(domain.TaskPhaseTodo), taskName,
   	)
   }
   ```

   Five things about this shape are load-bearing:
   - The literal substring `vault-cli task approve` appears in the source. Spec 056 AC 14 asserts the refusal's stderr contains it, and the unit cases assert the same on the returned error. Do not paraphrase the command into words like "the approve command" or "task approval" — the command name itself must be there.
   - The comparison names the constants `domain.TaskPhasePlanning` and `domain.TaskPhaseTodo`, not the string literals `"planning"` and `"todo"`. The value is normalized through `domain.NormalizeTaskPhase` first, exactly as `checkPhaseRegression` does, so an aliased spelling cannot slip past the guard.
   - The `!ok` branch returns `nil`. An unknown phase value is not this guard's business — `SetField`'s `setPhaseField` already refuses it with `unknown task phase`, and that message must survive unchanged.
   - The helper takes no `force` argument, so it cannot consult `force` even by accident. `--force` does not bypass this refusal (requirement 2's comment says so, and `<constraints>` records why). `metricsSessionsWriteRefusal` and `blockedBySetRefusal` are the precedents for a refusal with no override.
   - The error is built with `errors.Errorf`, not `errors.Wrapf(ctx, validation.Error, ...)`. This is a policy refusal on a generic write verb, not a validation failure, so it takes the `metricsSessionsWriteRefusal` shape. The CLI turns any returned error into stderr plus exit 1 either way.

2. **Call it in `Execute`, before anything is mutated.** In `frontmatterSetOperation.Execute`, replace this region verbatim:

   ```go
   	// Refuse metrics_sessions before anything is mutated — see
   	// metricsSessionsWriteRefusal. Nothing below runs on this path, so the task
   	// file stays byte-identical, and --force does not bypass the refusal.
   	if err := metricsSessionsWriteRefusal(ctx, taskName, key); err != nil {
   		return err
   	}
   ```

   with this new region:

   ```go
   	// Refuse metrics_sessions before anything is mutated — see
   	// metricsSessionsWriteRefusal. Nothing below runs on this path, so the task
   	// file stays byte-identical, and --force does not bypass the refusal.
   	if err := metricsSessionsWriteRefusal(ctx, taskName, key); err != nil {
   		return err
   	}

   	// Refuse the unrecorded exit from the approval inbox before anything is
   	// mutated — see todoPlanningSetRefusal. Nothing below runs on this path, so
   	// the task file stays byte-identical, and --force does not bypass the
   	// refusal.
   	if err := todoPlanningSetRefusal(ctx, task, taskName, key, value); err != nil {
   		return err
   	}
   ```

   Consequences that make this the required position, not a style choice:
   - This is the file's refusal cluster. Every helper there returns before `previousAssignee` is read, before `writeTaskCloseOutFieldsIfCloseOut` can mutate the task, before `checkPhaseRegression` and before `SetField`, so the no-write guarantee is structural rather than a rewrite that happens to be stable.
   - `FindTaskByName` still runs first, so a task that resolves in no vault keeps its existing not-found behaviour and `VaultDispatcher.FirstSuccess`'s `storage.ErrNotFound` handling is untouched.
   - The `unknown field` pass-through is unaffected: the guard returns on its first line for any key other than `phase`, so `task set <name> some_unknown_key value` still stores the string unvalidated.

3. **Point the neighbouring comment at the new guard.** `checkPhaseRegression`'s doc comment currently ends with "A forward move (`todo` -> `planning`, `planning` -> `execution`) is never a regression and always passes." That remains true of `checkPhaseRegression` itself and must stay true — do not change its behaviour. But the comment now reads as a promise that the whole `todo` -> `planning` move is allowed, which this prompt makes false, so make this edit:

   ```
Old (lines 227-228):
// A deliberate reset must pass force=true. A forward move (`todo` -> `planning`,
// `planning` -> `execution`) is never a regression and always passes.

New:
// A deliberate reset must pass force=true. A forward move (`planning` -> `execution`) is
// never a regression and always passes.
// `todo` -> `planning` is likewise not a regression and passes here; it is refused
// earlier, by todoPlanningSetRefusal, because leaving the approval inbox without a
// record is a different defect from a backward move.
   ```

   Keep the two-line "A deliberate reset must pass force=true." sentence above it unchanged and leave the rest of the comment intact. A comment that still advertises the `todo` -> `planning` move as unconditionally allowed is the same stale-documentation defect this prompt exists to remove.

4. **Replace the unit case that now contradicts the guard.** In `pkg/ops/frontmatter_test.go`, the `Context("allowing todo -> planning on in_progress task")` block near line 534 asserts that `task set phase planning` writes on a row at `phase: todo`. That is exactly the combination the guard now refuses, so it must be replaced, not left to fail. The comment above it near line 531 currently calls `todo` -> `planning` one of "the two must-still-pass cases"; after this change the surviving must-still-pass case is `planning` -> `execution`, so reword that comment accordingly rather than leaving it claiming the opposite. Replace it verbatim with:

   ```go
   // The must-still-pass case. Broadening the guard must not break a forward
   // move out of planning. The `todo` -> `planning` entry contract is no longer a
   // legal `task set`: it is refused below and performed by `vault-cli task approve`.
   ```

   a. Replace the `Context("allowing todo -> planning on in_progress task")` block with this table. **Use `DescribeTableSubtree`, not `DescribeTable`:** a plain `DescribeTable` body runs *after* the shared `JustBeforeEach`, so a fixture assigned there would be assigned after `Execute` had already run — the same reason `pkg/ops/frontmatter_entity_test.go` near line 1162 uses the subtree form with an inner `BeforeEach`.

   ```go
   	// The `todo` -> `planning` forward move is no longer a legal `task set`: it leaves
   	// the operator's approval inbox without a record. It is refused for every status —
   	// including `in_progress`, where the only guard that could otherwise have fired,
   	// checkPhaseRegression, deliberately allows this direction.
   	DescribeTableSubtree("refusing todo -> planning on a task in the approval inbox",
   		func(status string) {
   			BeforeEach(func() {
   				task = domain.NewTask(
   					map[string]any{"status": status, "phase": "todo"},
   					domain.FileMetadata{Name: taskName},
   					domain.Content(""),
   				)
   				mockTaskStorage.FindTaskByNameReturns(task, nil)
   				key = "phase"
   				value = "planning"
   			})

   			It("refuses with an error naming the approve command", func() {
   				Expect(err).To(HaveOccurred())
   				Expect(err.Error()).To(ContainSubstring("vault-cli task approve"))
   				Expect(err.Error()).To(ContainSubstring("todo"))
   			})

   			It("does not write the task", func() {
   				Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
   			})

   			It("leaves the phase at todo", func() {
   				Expect(task.Phase()).NotTo(BeNil())
   				Expect(*task.Phase()).To(Equal(domain.TaskPhaseTodo))
   			})
   		},
   		Entry("next", "next"),
   		Entry("in_progress", "in_progress"),
   	)
   ```

   The two entries are load-bearing, not symmetry: the `next` entry proves the refusal fires with no other guard in play at all, and the `in_progress` entry is the direct inversion of the case being removed — it pins the refusal to the new guard rather than to `checkPhaseRegression`, which allows `todo` -> `planning` for every status.

   b. Add the `--force` non-bypass case, so the narrowed constraint is asserted rather than merely stated:

   ```go
   	Context("refusing todo -> planning even when force is set", func() {
   		BeforeEach(func() {
   			task = domain.NewTask(
   				map[string]any{"status": "next", "phase": "todo"},
   				domain.FileMetadata{Name: taskName},
   				domain.Content(""),
   			)
   			mockTaskStorage.FindTaskByNameReturns(task, nil)
   			key = "phase"
   			value = "planning"
   			force = true
   		})

   		It("returns the refusal error", func() {
   			Expect(err).NotTo(BeNil())
   			Expect(err.Error()).To(ContainSubstring("vault-cli task approve"))
   		})

   		It("does not write the task", func() {
   			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
   		})
   	})
   ```

   c. Add the regression lock, so the change is pinned to one combination rather than to "`task set` stopped moving phases":

   ```go
   	Context("allowing execution -> planning on a next-status task", func() {
   		BeforeEach(func() {
   			task = domain.NewTask(
   				map[string]any{"status": "next", "phase": "execution"},
   				domain.FileMetadata{Name: taskName},
   				domain.Content(""),
   			)
   			mockTaskStorage.FindTaskByNameReturns(task, nil)
   			key = "phase"
   			value = "planning"
   		})

   		It("writes the phase", func() {
   			Expect(err).To(BeNil())
   			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
   			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
   			Expect(writtenTask.Phase()).NotTo(BeNil())
   			Expect(*writtenTask.Phase()).To(Equal(domain.TaskPhasePlanning))
   		})
   	})
   ```

   ⚠️ **The fixture status must be `next`, not `in_progress`.** At `status: in_progress` with `phase: execution`, `checkPhaseRegression` refuses `execution` -> `planning` — that is the 2026-09-19 vault-ui board-drag guard, and the existing `Context("allowing execution -> planning with --force on in_progress task")` case near line 570 exists precisely because `--force` is needed there. A regression-lock fixture at `in_progress` would therefore fail for a reason that has nothing to do with this prompt, and would prove nothing about the new guard.

   d. **Leave every other case in the file exactly as it is.** Verified against the current tree, each of these keeps passing because the guard returns `nil` on one of its three conditions:
   - `Context("setting phase field")` near line 373 — the default task has no `phase` key, so `Phase()` is `nil`.
   - `Context("setting invalid phase field")` near line 388 — `NormalizeTaskPhase` fails, so the guard returns before the phase check and `SetField` still reports `unknown task phase`.
   - the six phase-regression contexts near lines 404, 427, 450, 473, 491 and 509 — the current phase is `execution`, `ai_review`, `human_review` or `done`, never `todo`.
   - `Context("allowing planning -> execution on in_progress task")` near line 552 — the value is `execution`, not `planning`.
   - `Context("allowing execution -> planning with --force on in_progress task")` near line 570 — the current phase is `execution`, not `todo`.
   - `Context("allowing execution -> todo with --force on in_progress task")` near line 589 and `Context("allowing planning -> todo on in_progress task")` near line 610 — the value is `todo`, not `planning`.
   - `Context("task not found")` near line 1018 and `Context("write error")` near line 1031 — the default task has no `phase` key.
   - every non-`phase` setter context in the Describe (status, priority, flag, assignee, page_type, dates, goals, tags, unknown key, close-out fields) — the guard returns on its first line.

5. **Add the end-to-end block through the real binary in `integration/cli_test.go`.** The unit cases prove the operation refuses; they cannot prove the CLI exits non-zero, prints the command name on stderr, and leaves the file untouched — and spec 056's scenario-coverage note names this harness as the place where exit codes and file content are asserted against a temp vault.

   Add a new `Describe("task set approval guard", func() { … })` **immediately above** `Describe("task append-metrics-session", ...)` (near line 2589). That anchor is deliberate: prompt 2 inserts its block above `Describe("vault-cli defer", ...)` and prompt 4 inserts above `Describe("vault-cli blocked_by JSON surface", ...)`, so this region is untouched by both and the anchor survives them. Declare `runEntityCommand` and `sha256OfFile` locally inside the new Describe — the sibling blocks' copies are closure-scoped and not visible — copying them verbatim from the `Describe("vault-cli blocked_by set refusal", ...)` block near line 2447, including the `//#nosec G304 -- test file` comment. Use `createTempVault(map[string]string{"Alpha": …})` for the fixtures and `AfterEach(func() { cleanup() })`. The three specs:

   ```go
   		var vaultPath, configPath string
   		var cleanup func()

   		It("refuses todo -> planning, names the approve command, and writes nothing", func() {
   			vaultPath, configPath, cleanup = createTempVault(map[string]string{
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

   			session := runEntityCommand("task", "set", "Alpha", "phase", "planning")
   			Eventually(session).Should(gexec.Exit(1))
   			Expect(string(session.Err.Contents())).To(ContainSubstring("vault-cli task approve"))
   			Expect(sha256OfFile(taskFile)).To(Equal(before))
   		})

   		It("refuses todo -> planning with --force and writes nothing", func() {
   			vaultPath, configPath, cleanup = createTempVault(map[string]string{
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

   			session := runEntityCommand("task", "set", "Alpha", "phase", "planning", "--force")
   			Eventually(session).Should(gexec.Exit(1))
   			Expect(string(session.Err.Contents())).To(ContainSubstring("vault-cli task approve"))
   			Expect(sha256OfFile(taskFile)).To(Equal(before))
   		})

   		It("still allows execution -> planning on a next-status task", func() {
   			vaultPath, configPath, cleanup = createTempVault(map[string]string{
   				"Alpha": `---
   page_type: task
   phase: execution
   priority: 1
   status: next
   task_identifier: 11111111-1111-4111-8111-111111111111
   ---
   body line
   `,
   			})
   			taskFile := filepath.Join(vaultPath, "Tasks", "Alpha.md")

   			session := runEntityCommand("task", "set", "Alpha", "phase", "planning")
   			Eventually(session).Should(gexec.Exit(0))

   			content, readErr := os.ReadFile(taskFile) //#nosec G304 -- test file
   			Expect(readErr).NotTo(HaveOccurred())
   			Expect(string(content)).To(ContainSubstring("phase: planning"))
   		})
   ```

   Notes on why this shape works, so you do not "fix" it:
   - Both refusal fixtures are at `status: next`, so `checkPhaseRegression` cannot be the refusing party: for a task that is not `in_progress` it returns `nil` for any phase target. That is what makes the stderr assertion evidence for the new guard rather than for an older one.
   - The third fixture is at `phase: execution` with `status: next` for the same reason, and **must not** be `in_progress` — at `in_progress`, `checkPhaseRegression` refuses `execution` -> `planning` and the spec would fail for an unrelated reason.
   - `task set` needs no `current_user` and no `claude_script`, so `createTempVault` is the right fixture helper (unlike prompt 4's `work-on` block, which needed `createTempVaultWithCurrentUser`).
   - `binPath`, `exec`, `gexec`, `Eventually`, `filepath`, `os` and `sha256` are all already in the file's import set; `fmt` is imported for the `sha256OfFile` helper.

6. **Change nothing else.** Specifically, and verified against the current tree:
   - `pkg/cli/cli.go` — no change. `createTaskSetCommand` already returns the operation's error and `cli.Execute` already prints it and exits 1.
   - `pkg/ops/workon.go` — prompt 4's file. Byte-identical here.
   - `pkg/domain/`, `pkg/storage/`, `mocks/` — no interface or signature changes, so no mock regeneration is needed and `make generate` must produce no new file. In particular `FrontmatterSetOperation`'s `Execute` keeps its eight-parameter signature; `mocks/frontmatter-set-operation.go` is generated from it and `pkg/cli/cli.go` calls it, so adding a parameter breaks both.
   - `docs/`, `CHANGELOG.md`, `commands/` — prompt 3's scope. This prompt touches no documentation, no CHANGELOG bullet and no command file.
   - `scenarios/` — no scenario changes. Verified: `scenarios/002-task-lifecycle.md` works on `example/vault/24 Tasks/Simple Task.md`, which has no `phase` key at all, and `scenarios/005-work-on-resume-auto-invokes-subtask.md` uses `phase: execution`; neither runs `task set … phase planning` on a `todo` row.
   - The existing integration specs that invoke `task set` — the `frontmatter round-trip` block near line 786, the `task get/set` block near line 909, the close-out gating blocks near lines 1565 and 1779, and the `blocked_by`/`metrics_sessions` refusals near lines 2484 and 2810 — set `status`, `priority`, `blocked_by` and `metrics_sessions`, never `phase`. They keep passing unchanged. Do not edit any of them.
   - The three other suites that construct this same operation (`pkg/ops/metrics_session_write_test.go`, `pkg/ops/blocked_by_set_test.go`, `pkg/ops/wikilink_roundtrip_test.go`) never set `phase` — `grep -n 'phase' ` on all three returns no matches — so they keep passing unchanged.

7. **Self-check before finishing.** Re-run `<verification>` and confirm each count against its expectation, including the two absence assertions. Walk spec 056 AC 14 arm by arm against the change: the `todo` fixture exits non-zero with `vault-cli task approve` on stderr and an unchanged hash; the `--force` fixture does the same; the `execution` fixture at `status: next` exits 0 and reads `phase: planning`. Then confirm `git diff --name-only` lists exactly the three files this prompt owns. If any arm is not evidenced, fix it before declaring done.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` / `git diff --exit-code` / `git checkout --` only read or restore; never stage or commit.
- **The guard is pinned to one combination, and only that one.** `key == "phase"` **and** the canonical value `planning` **and** a current phase of `todo`. A row at any other phase, a missing or empty `phase` key, an unknown phase value, and every non-`phase` key keep their existing behaviour. Widening this into a general phase gate would break the six existing phase-regression cases, the `planning` -> `execution` allowance, the two `--force` allowances, and the shipped example vault — and spec 056 DB 11 does not ask for it.
- **`status` is never consulted.** The refusal fires for a `todo` row whatever its status. The fixture statuses in the tests exist only to keep `checkPhaseRegression` out of the picture, never as a condition of the guard.
- **`--force` does not bypass the refusal, and the helper must not accept a `force` argument.** `--force` keeps its documented meaning — the flag's own help text reads "Allow a phase regression (e.g. execution -> todo) on an in-progress task for a deliberate reset" — and is scoped to a backward move on a row already past `planning`. `todo` -> `planning` is a forward move that `--force` has never governed, and the unrecorded row is the defect rather than a warning to be overridden. `metricsSessionsWriteRefusal` and `blockedBySetRefusal` are the precedents, and the integration spec near line 2830 already asserts the same non-bypass for `metrics_sessions`. Do not add a `force` parameter and do not consult it in `Execute`.
- **The refusal writes nothing.** It returns before `previousAssignee` is read, before `writeTaskCloseOutFieldsIfCloseOut` can mutate the task, before `checkPhaseRegression`, before `SetField` and before `WriteTask`. No partial frontmatter, no key reordering, no new key.
- **The refusal message must contain the literal substring `vault-cli task approve`.** Spec 056 AC 14's stderr assertion and the unit cases both assert it; a message that describes the command without naming it fails both.
- **No `fmt.Errorf`.** Use `github.com/bborbe/errors` (`errors.Errorf`, `errors.Wrap`) with the `ctx` already in scope. Never a bare `return err`, and never `errors.New` from the standard library.
- **No `time.Now()`** and no clock read at all — this refusal reads no clock, and `pkg/ops/frontmatter.go` has neither `time.Now()` nor a `libtime` import today. Do not add one.
- **No stdout in `pkg/ops/`.** `pkg/ops` is a library layer: the operation returns an error and the CLI formats output. No `fmt.Print*`, no `os.Stdout`.
- **Do not change `FrontmatterSetOperation`'s interface or its `Execute` signature**, and do not change `checkPhaseRegression`'s behaviour. Requirement 3 is a comment-only edit to that function. The only functional change in `pkg/ops/frontmatter.go` is the new guard and its call site.
- **`pkg/ops/frontmatter.go` is this prompt's one sanctioned edit to a file prompts 1 and 4 declared frozen.** Their freeze was scoped to their own execution order; spec 056's Constraints narrow it to "every behaviour except the single guarded combination DB 11 names". Do not take this as licence to touch any other refusal, the close-out path, the escalation publish, or `SetField`'s error handling.
- **Existing tests must still pass.** The one existing case this prompt is allowed to change is `Context("allowing todo -> planning on in_progress task")` in `pkg/ops/frontmatter_test.go`; every other spec, including the six phase-regression contexts, the `planning` -> `execution` and `--force` allowances, the integration `task set` blocks and the scenario files, stays as it is.
- **Do not touch `docs/task-writing.md` or `CHANGELOG.md`.** Prompt 3 documents this surface; its `docs/task-writing.md` prose already states that a phase set without an approval record is not an approval, which this prompt makes enforceable rather than advisory.
- Tests follow repository convention: Ginkgo v2 / Gomega in the external `ops_test` / `integration_test` packages, counterfeiter mocks from `mocks/`, `DescribeTableSubtree` with an inner `BeforeEach` wherever a case needs a per-entry fixture, and `Eventually(...).Should(gexec.Exit(n))` for the binary cases. Do not add a new suite file — `pkg/ops/ops_suite_test.go` and `integration/integration_suite_test.go` already exist.
- No `go mod vendor`.
</constraints>

<verification>
PRIMARY GATE — spec 056 AC 14's evidence greps plus the supporting ones. Run each, record the count, and confirm it against the expectation. Rows expecting 0 are written as `! grep -q` because `grep -c` exits 1 when it prints 0:

```
grep -c 'todoPlanningSetRefusal' pkg/ops/frontmatter.go            # >= 2 (the declaration and the call; 0 at HEAD)
grep -c 'vault-cli task approve' pkg/ops/frontmatter.go            # >= 1 (the refusal names the verb; 0 at HEAD)
grep -c 'task approve' pkg/ops/frontmatter.go                      # >= 1 (spec 056 AC 14; 0 at HEAD)
grep -c 'TaskPhaseTodo' pkg/ops/frontmatter.go                     # >= 2 (1 at HEAD, inside checkPhaseRegression — the guard adds at least one more)
grep -c 'approval inbox' pkg/ops/frontmatter.go                    # >= 1 (0 at HEAD)
grep -c 'vault-cli task approve' pkg/ops/frontmatter_test.go       # >= 1 (the unit cases assert the message; 0 at HEAD)
grep -c 'DescribeTableSubtree' pkg/ops/frontmatter_test.go         # >= 1 (the per-status refusal table; 0 at HEAD)
grep -c 'even when force is set' pkg/ops/frontmatter_test.go       # >= 1 (the --force non-bypass case; 0 at HEAD)
grep -c 'next-status' pkg/ops/frontmatter_test.go                  # >= 1 (the regression lock; 0 at HEAD)
grep -c 'task set approval guard' integration/cli_test.go          # >= 1 (the end-to-end block exists; 0 at HEAD)
! grep -q 'allowing todo -> planning' pkg/ops/frontmatter_test.go  # 0 (the contradicting case is gone, not left to fail; 1 at HEAD)
! grep -q 'fmt.Errorf' pkg/ops/frontmatter.go                      # 0
! grep -q 'time.Now()' pkg/ops/frontmatter.go                      # 0
```

⚠️ **Two of these rows are weak on their own and the expectations are set accordingly.** `TaskPhaseTodo` already occurs once in `frontmatter.go` at HEAD (inside `checkPhaseRegression`), so a `>= 1` expectation there would pass with no change at all — hence `>= 2`. The `todoPlanningSetRefusal`, `vault-cli task approve`, `approval inbox`, `DescribeTableSubtree`, `even when force is set`, `next-status` and `task set approval guard` rows are all 0 at HEAD, so those are the non-vacuous ones. The counts are the spec's evidence, not proof the refusal works; the tests below are the proof.

TESTS:
```
go test ./pkg/ops/... -count=1        # exits 0
go test ./pkg/domain/... -count=1     # exits 0 — untouched, but this prompt must not break it
make test                             # exits 0
```

⚠️ **Guard the focused runs against a false pass.** After `go test ./pkg/ops/...`, confirm the new cases actually ran — a focus expression that matches nothing exits 0 while running nothing:
```
go test ./pkg/ops/ -count=1 -ginkgo.focus='refusing todo' -ginkgo.v             # the two-entry refusal table runs and passes
go test ./pkg/ops/ -count=1 -ginkgo.focus='even when force is set' -ginkgo.v    # the --force non-bypass case runs and passes
go test ./pkg/ops/ -count=1 -ginkgo.focus='next-status' -ginkgo.v               # the regression lock runs and passes
```
Read the log: each run must report the focused spec names, not `0 Passed | 0 Failed`.

```
go test ./integration/ -count=1 -ginkgo.focus='task set approval guard' -ginkgo.v   # all three new specs run and pass
```
This one builds the binary in `BeforeSuite` and takes a couple of minutes; it is worth the wait because it is the only evidence for the exit codes. If the integration package reports the known pre-existing flake documented in prompts 2 and 3 (the `topic defer writes defer_date for a relative and an absolute date` spec, which fails for about two hours a day when the local date leads the UTC date) in a spec outside your focus, that is not this prompt's failure — do not "fix" it opportunistically, and do not let it block this prompt.

FROZEN NEIGHBOURS — every one of these must exit 0 with empty output:
```
git diff --exit-code -- pkg/cli/
git diff --exit-code -- pkg/ops/workon.go
git diff --exit-code -- pkg/ops/vault_dispatcher.go
git diff --exit-code -- pkg/domain/ pkg/storage/
git diff --exit-code -- docs/ CHANGELOG.md commands/
git diff --exit-code -- scenarios/
git status --porcelain -- mocks/    # empty: this prompt changes no mock
```
If `mocks/mocks.go` reports a diff here, a previous prompt's `make precommit` left the generator-stripped copyright header behind — restore it with `git checkout -- mocks/mocks.go` and re-check rather than carrying the deletion.

SCOPE — run this **before** the final `make precommit` (which regenerates `mocks/` and would otherwise muddy the list):
```
git diff --name-only    # must list exactly: pkg/ops/frontmatter.go, pkg/ops/frontmatter_test.go, integration/cli_test.go
git status --porcelain  # nothing untracked
```
If a file from prompts 1, 2, 3 or 4 appears in that list, those prompts were not committed before this one ran — stop and report it rather than reverting anything.

SYNTAX:
```
gofmt -e -l pkg/ops/frontmatter.go pkg/ops/frontmatter_test.go integration/cli_test.go    # must list NO files
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

1. `--force` does NOT bypass the new guard, and that is the deliberate choice. The flag's own
   help text scopes it to a phase regression ("Allow a phase regression (e.g. execution -> todo)
   on an in-progress task for a deliberate reset"), and `todo` -> `planning` is a forward move
   that has never been a regression. The alternative — letting `--force` write the unrecorded
   `planning` row — would mean the one documented escape hatch from every other `set` refusal
   doubles as an approval bypass, so a deliberate regression override would stand in for an
   approval record. That is the exact state spec 056 exists to make impossible. The precedents
   agree: `metricsSessionsWriteRefusal` and `blockedBySetRefusal` both document "no --force
   bypass", and the integration spec near line 2830 asserts it for `metrics_sessions`. The cost
   is recorded honestly: an operator who genuinely wants a `todo` row at `planning` with no
   record has no command for it. That is the point rather than a gap — the row is already at the
   earliest phase, so there is nothing to reset to, and `vault-cli task approve` is the verb.

2. The refusal message deliberately does not mention `--force`. Every other refusal in this file
   that `--force` can override says so ("Pass --force to override"); this one cannot be
   overridden, so naming the flag would invite the reader to try it. The message names the task,
   the current phase, and `vault-cli task approve` with its two recorded fields — nothing else.

3. The refusal fixtures are at `next` and the refusal table carries one deliberate `in_progress`
   entry (requirement 4a); the regression-lock fixture is at `next` and that is load-bearing.
   `checkPhaseRegression` refuses `execution` -> `planning` (and `ai_review`/`human_review` ->
   `planning`) only when `status: in_progress`, and it allows `todo` -> `planning` for every
   status, because `todo` is not in its refusal switch. So a refusal fixture at `in_progress`
   still has exactly one candidate refuser — the new guard — which is precisely why the table
   includes that entry: it proves the guard fires where `checkPhaseRegression` deliberately
   allows the direction. A regression-lock fixture at `in_progress` would instead fail for the
   2026-09-19 board-drag guard rather than for this prompt's change. At `next`,
   `checkPhaseRegression` returns `nil` for every target, so the new guard is provably the only
   thing that can refuse.

4. Residual gap, recorded for the operator's awareness rather than fixed here. A row with no
   `phase` key at all (or an empty one) is still moved to `planning` by `task set … phase
   planning` with no record: `Phase()` returns `nil` for both shapes, so the guard's third
   condition does not hold. Spec 056 DB 11 is scoped to a row "at `phase: todo`", and prompt 4's
   reviewer note 1 records the mirror-image gap on the `work-on` side — such a row is neither
   approvable (DB 4 refuses it in `approve`) nor refused by either guard. Rows at `todo` are the
   normal filing state, so this is an edge case, and widening the guard to a nil phase would
   break the existing `setting phase field`, `task not found` and `write error` cases plus the
   shipped example vault. Not this prompt's scope; worth a follow-up spec if it bites.

5. Open questions surfaced rather than silently decided:
   - Should `task set <name> phase planning` on a `todo` row that already carries `approved_by`
     or `approved_at` be refused for a different reason (the row has a record but never left the
     inbox)? Left unchanged here: DB 11 says nothing about it, the new guard fires on the phase
     pair alone, and DB 5's existing-record refusal belongs to `approve`. If the operator wants
     that combination handled, it is a DB 11 amendment rather than an implementation detail.
   - Should the guard live in `frontmatter.go` beside `checkPhaseRegression`, or in its own file
     beside `blocked_by_write.go` and `metrics_session_write.go`? Decided: `frontmatter.go`,
     matching the spec's Suggested Decomposition row 5 and keeping the two phase guards
     adjacent. A future refusal of this kind may want the sibling-file shape instead.
   - `pkg/ops/frontmatter.go` is the only file this prompt changes functionally, but it is a
     file prompts 1 and 4 both declared frozen. The sequencing note in `<context>` and the
     constraint in `<constraints>` resolve that for the executing agent; if the prompts are ever
     re-ordered so that this one runs before prompt 4, the freeze in prompt 4's constraints must
     be narrowed the same way spec 056's own Constraints now are.
-->
