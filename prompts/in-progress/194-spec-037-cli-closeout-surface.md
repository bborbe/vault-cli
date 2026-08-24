---
status: approved
spec: [037-mandatory-abort-reason-and-gate-successor]
created: "2026-08-24T19:00:00Z"
queued: "2026-08-24T19:00:06Z"
---

# CLI surface: --reason / --gate-successor flags, one-step close-out, and the risk-naming error

<summary>
- All four close-out routes — task set status, task complete, goal set status, goal complete — accept `--reason <text>` and `--gate-successor <name|none>` flags.
- When a close-out is attempted with the flags, the reason and successor are persisted together with the status in a single file write — no partial state, no intermediate status-only file.
- A close-out rejected by the domain guard now returns an error that names the missing field(s), lists what the task may own (trigger / gate / threshold / recurring check), asks where the risk moves, and shows the exact command form that succeeds — in both plain text and the `error` field of JSON output.
- Supplying the flags for a non-close-out transition (e.g. `status in_progress --reason x`) does nothing to the fields — those transitions work exactly as today.
- The two-step close-out (set the fields directly, then set the status) still works, because the guard reads the fields at transition time.
- `task complete --force` still bypasses only the incomplete-subtask check — never the reason guard.
- The existing binary-level integration test for `task complete` is updated to pass the flags, and new end-to-end tests cover the gated rejections and acceptances for tasks and goals, checkbox-sync completion, YAML-special-character reasons, and the non-close-out regression.
- The task-writing convention doc gains one sentence naming the flags, and the CHANGELOG records the intended breaking change for scripts that close out without the fields.
</summary>

<objective>
Wire `--reason` and `--gate-successor` flags through the four close-out-capable CLI commands (`task set status aborted|completed`, `task complete`, `goal set status aborted|completed`, `goal complete`) into the ops layer, persist both fields with the status in one write when supplied, and return the full risk-naming error (missing fields + owned-risk categories + succeeding command form) on a guard rejection in both plain and JSON output. No bypass or opt-out of the guard.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, `/workspace/docs/development-patterns.md` for the ops/CLI layering, and `/workspace/docs/task-writing.md` § Lifecycle → "Close-out fields" (kept in sync by requirement 22).

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra flag declaration
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Errorf` with `github.com/bborbe/errors`, never `fmt.Errorf`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md` — `make generate` / counterfeiter mock regeneration
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, integration suite conventions
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` entry format

**Preconditions — prompts 1 and 2 must have shipped.** Verify before writing anything:

```
grep -c 'missingCloseOutFields(f.FrontmatterMap)' pkg/domain/task_frontmatter.go
grep -c 'if err := task.SetStatus(domain.TaskStatusCompleted); err != nil' pkg/ops/complete.go
```

If either prints `0`, STOP and report `"status":"failed"` with the message `domain close-out guard (prompt 1) or ops propagation (prompt 2) not yet deployed`, and do NOT re-implement either yourself.

Read these files fully before making changes:

- `/workspace/pkg/ops/complete.go` — `CompleteOperation` interface (lines 22-32) and `completeOperation.Execute` (lines 74-158). The prompt-2 error block now sits at the `task.SetStatus(domain.TaskStatusCompleted)` call in the non-recurring path. `handleRecurringTask` returns before it and must stay untouched.
- `/workspace/pkg/ops/goal_complete.go` — `GoalCompleteOperation` interface (lines 20-28) and `goalCompleteOperation.Execute` (lines 50-95), including the already-completed short-circuit (lines 69-72).
- `/workspace/pkg/ops/frontmatter.go` — `FrontmatterSetOperation` interface (lines 44-48) and `frontmatterSetOperation.Execute` (lines 61-79). Currently imports only `context`, `errors`, `storage` — you will add `strings` and `github.com/bborbe/vault-cli/pkg/domain`.
- `/workspace/pkg/ops/frontmatter_entity.go` — `EntitySetOperation` interface (lines 89-95) and the four set operations `goalSetOperation` / `themeSetOperation` / `objectiveSetOperation` / `visionSetOperation` (each `Execute(ctx, vaultPath, entityName, key, value) error`). Currently imports `context`, `fmt`, `errors`, `domain`, `storage` — you will add `strings`.
- `/workspace/pkg/cli/cli.go` — the four command builders: `createCompleteCommand` (line 145, flags at 200-201), `createGoalCompleteCommand` (line 1316, flags at 1369-1370), `createTaskSetCommand` (line 2039), `createEntitySetCommand` (line 894, shared by goal/theme/objective/vision set). `runMutation` (line 53) prints `result.Error` in JSON mode. In JSON mode these commands print the error body and return `PrintJSON(...)` (exit 0) — a pre-existing JSON contract; exit codes are asserted in plain mode.
- `/workspace/pkg/ops/complete_test.go` (JustBeforeEach at line 65), `/workspace/pkg/ops/goal_complete_test.go` (line 59), `/workspace/pkg/ops/frontmatter_test.go` (line 337), `/workspace/pkg/ops/frontmatter_entity_test.go` (line 175) — the single `Execute` call sites whose arity changes.
- `/workspace/integration/cli_test.go` — `createTempVault` / `createTempVaultWithGoals` helpers (top of file), the existing `Describe("vault-cli complete", ...)` block (line 645, fixture at 651-659 already carries `aborted_reason` / `gate_successor` from prompt 1), and the gexec-based assertion style. `task get <name> <key> --output json` prints `{"key":..., "value":..., "name":...}` — used by the YAML-special-characters spec.

Design decisions already made (follow them, do not revisit):

1. The four ops `Execute` signatures gain `reason, gateSuccessor string` as the last two parameters. This keeps the one-step write atomic at the ops layer — fields and status are set on the in-memory entity and persisted by the single existing `WriteTask` / `WriteGoal` call; a guard rejection still returns before any write.
2. Fields are written via `SetField` (unknown keys → stored as Go strings through the YAML serializer, so newlines/colons are quoted — never string-concatenated). They are written only when the flag is non-empty AND the target status is a close-out. The status set happens through the same `SetField` / `SetStatus` the route already uses, so the guard is always the final arbiter.
3. On a guard rejection (`err.Error()` contains the frozen marker `"missing close-out field(s)"`), the ops layer composes the final message as the guard message plus `"\nTry: vault-cli ... --reason \"<text>\" --gate-successor \"<successor|none>\""` with the route's own command form and entity name. This text is what lands in both the returned error (plain output) and `MutationResult.Error` / the JSON `error` field.
4. `UpdateOperation.Execute` does NOT change signature (the `task update` checkbox-sync has no flags; it is gated by the guard alone, which prompt 2 already honors).
5. `ObjectiveCompleteOperation` and the theme/objective/vision set operations are untouched functionally — only their `EntitySetOperation` signatures gain the two ignored parameters.
</context>

<requirements>

**Part A — ops layer: one-step close-out and the risk-naming error**

1. In `/workspace/pkg/ops/complete.go`, change the `CompleteOperation` interface to:
   ```go
   //counterfeiter:generate -o ../../mocks/complete-operation.go --fake-name CompleteOperation . CompleteOperation
   type CompleteOperation interface {
   	// Execute marks a task as complete. When force is true the incomplete-subtask
   	// guard is bypassed, mirroring the --force flag on goal complete.
   	// reason and gateSuccessor are the one-step close-out fields (aborted_reason /
   	// gate_successor) and are persisted with the status in a single write when
   	// non-empty; they are ignored for recurring tasks, whose status never changes.
   	Execute(
   		ctx context.Context,
   		vaultPath string,
   		taskName string,
   		vaultName string,
   		force bool,
   		reason string,
   		gateSuccessor string,
   	) (MutationResult, error)
   }
   ```
   Update the `completeOperation.Execute` method signature to match. Do NOT change `NewCompleteOperation`.

2. In `/workspace/pkg/ops/complete.go`, replace the prompt-2 `SetStatus` error block in the non-recurring path (currently `if err := task.SetStatus(domain.TaskStatusCompleted); err != nil { ... }`) with the one-step block:

   ```go
   	// One-step close-out: persist reason and successor before the status
   	// transition so both land in a single WriteTask. Fields are written only
   	// when provided — the two-step close-out (fields already in the file) works
   	// unchanged, and recurring tasks (status never changes) are untouched.
   	if reason != "" {
   		if err := task.SetField(ctx, "aborted_reason", reason); err != nil {
   			return MutationResult{
   				Success: false,
   				Error:   err.Error(),
   			}, errors.Wrap(ctx, err, "set aborted_reason")
   		}
   	}
   	if gateSuccessor != "" {
   		if err := task.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
   			return MutationResult{
   				Success: false,
   				Error:   err.Error(),
   			}, errors.Wrap(ctx, err, "set gate_successor")
   		}
   	}
   	if err := task.SetStatus(domain.TaskStatusCompleted); err != nil {
   		msg := err.Error()
   		if strings.Contains(msg, "missing close-out field(s)") {
   			msg = fmt.Sprintf(
   				"%s\nTry: vault-cli task complete \"%s\" --reason \"<text>\" --gate-successor \"<successor|none>\"",
   				msg,
   				taskName,
   			)
   		}
   		return MutationResult{
   			Success: false,
   			Error:   msg,
   		}, errors.Wrap(ctx, err, "set status")
   	}
   ```

3. In `/workspace/pkg/ops/goal_complete.go`, change the `GoalCompleteOperation` interface and `goalCompleteOperation.Execute` signature to add `reason, gateSuccessor string` after `force`. In `Execute`, keep the already-completed short-circuit and the open-tasks check as-is; between them and the existing `goal.SetStatus(domain.GoalStatusCompleted)` error block, insert the field writes, and extend the rejection branch with the goal command form:

   ```go
   	if reason != "" {
   		if err := goal.SetField(ctx, "aborted_reason", reason); err != nil {
   			return MutationResult{
   				Success: false,
   				Error:   err.Error(),
   			}, errors.Wrap(ctx, err, "set aborted_reason")
   		}
   	}
   	if gateSuccessor != "" {
   		if err := goal.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
   			return MutationResult{
   				Success: false,
   				Error:   err.Error(),
   			}, errors.Wrap(ctx, err, "set gate_successor")
   		}
   	}
   ```
   and change the `goal.SetStatus` rejection branch's returned message to:
   ```go
   		msg := err.Error()
   		if strings.Contains(msg, "missing close-out field(s)") {
   			msg = fmt.Sprintf(
   				"%s\nTry: vault-cli goal complete \"%s\" --reason \"<text>\" --gate-successor \"<successor|none>\"",
   				msg,
   				goalName,
   			)
   		}
   		return MutationResult{Success: false, Error: msg}, errors.Wrap(ctx, err, "set status")
   ```

4. In `/workspace/pkg/ops/frontmatter.go`, change the `FrontmatterSetOperation` interface and `frontmatterSetOperation.Execute` signature to:
   ```go
   type FrontmatterSetOperation interface {
   	Execute(ctx context.Context, vaultPath, taskName, key, value, reason, gateSuccessor string) error
   }
   ```
   In `Execute`, after the `FindTaskByName` block and before `task.SetField(ctx, key, value)`, insert the one-step close-out write (task variant):

   ```go
   	// One-step close-out: when this invocation sets a close-out status, persist
   	// reason and successor first so both land in a single WriteTask. Fields are
   	// written only when provided; non-close-out targets never receive them.
   	if key == "status" {
   		if target, ok := domain.NormalizeTaskStatus(value); ok &&
   			(target == domain.TaskStatusAborted || target == domain.TaskStatusCompleted) {
   			if reason != "" {
   				if err := task.SetField(ctx, "aborted_reason", reason); err != nil {
   					return errors.Wrap(ctx, err, "set aborted_reason")
   				}
   			}
   			if gateSuccessor != "" {
   				if err := task.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
   					return errors.Wrap(ctx, err, "set gate_successor")
   				}
   			}
   		}
   	}
   ```
   And wrap the existing `task.SetField(ctx, key, value)` error so a guard rejection names the succeeding command form:

   ```go
   	if err := task.SetField(ctx, key, value); err != nil {
   		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
   			err = errors.Errorf(ctx,
   				"%s\nTry: vault-cli task set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
   				err.Error(), taskName, value)
   		}
   		return errors.Wrap(ctx, err, "set field")
   	}
   ```
   Add `"strings"` and `"github.com/bborbe/vault-cli/pkg/domain"` to the imports.

5. In `/workspace/pkg/ops/frontmatter_entity.go`, change the `EntitySetOperation` interface to:
   ```go
   type EntitySetOperation interface {
   	Execute(ctx context.Context, vaultPath, entityName, key, value, reason, gateSuccessor string) error
   }
   ```
   Update all four set operations' method signatures to match (extra params ignored). Only in `goalSetOperation.Execute`, after `FindGoalByName` and before `goal.SetField(ctx, key, value)`, add the goal one-step block and the rejection wrap:

   ```go
   	// One-step close-out: when this invocation sets a close-out goal status,
   	// persist reason and successor first so both land in a single WriteGoal.
   	if key == "status" {
   		target := domain.GoalStatus(value)
   		if target == domain.GoalStatusAborted || target == domain.GoalStatusCompleted {
   			if reason != "" {
   				if err := goal.SetField(ctx, "aborted_reason", reason); err != nil {
   					return errors.Wrap(ctx, err, "set aborted_reason")
   				}
   			}
   			if gateSuccessor != "" {
   				if err := goal.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
   					return errors.Wrap(ctx, err, "set gate_successor")
   				}
   			}
   		}
   	}
   	if err := goal.SetField(ctx, key, value); err != nil {
   		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
   			err = errors.Errorf(ctx,
   				"%s\nTry: vault-cli goal set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
   				err.Error(), entityName, value)
   		}
   		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
   	}
   ```
   Add `"strings"` to the imports.

**Part B — CLI flag wiring**

6. In `/workspace/pkg/cli/cli.go`, `createCompleteCommand` (line 145): add `var reason, gateSuccessor string`, register the flags next to the existing `--force` flag, and pass them to `completeOp.Execute`:
   ```go
   	cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required to complete")
   	cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required to complete")
   ```
   `completeOp.Execute(ctx, vault.Path, taskName, vault.Name, force, reason, gateSuccessor)`.

7. In `/workspace/pkg/cli/cli.go`, `createGoalCompleteCommand` (line 1316): add the two flags with goal-specific help text distinct from the task-complete and goal-set variants — `cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required to complete a goal")` and `cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required to complete a goal")` — and pass them to `completeOp.Execute(ctx, vault.Path, goalName, vault.Name, force, reason, gateSuccessor)`. (Each of the four flag-verification greps must match exactly one declaration, so the goal-complete help text must not collide with the task-complete "required to complete" or goal-set "required for goal close-out" texts.)

8. In `/workspace/pkg/cli/cli.go`, `createTaskSetCommand` (line 2039): add `var reason, gateSuccessor string`, register the two flags, and call `setOp.Execute(ctx, vault.Path, taskName, key, value, reason, gateSuccessor)`.

9. In `/workspace/pkg/cli/cli.go`, `createEntitySetCommand` (line 894): restructure so the command is built into a named `cmd` variable and the two flags are registered ONLY when `entityType == "goal"`:
   ```go
   	var reason, gateSuccessor string
   	cmd := &cobra.Command{
   		Use:   "set <name> <key> <value>",
   		Short: fmt.Sprintf("Set a frontmatter field value on a %s", entityType),
   		Args:  cobra.ExactArgs(3),
   		RunE: func(cmd *cobra.Command, args []string) error {
   			// ... existing body unchanged, except the Execute call becomes:
   			if err := setOp.Execute(ctx, vault.Path, entityName, key, value, reason, gateSuccessor); err != nil {
   				return err
   			}
   		},
   	}
   	if entityType == "goal" {
   		cmd.Flags().StringVar(&reason, "reason", "", "Close-out reason (aborted_reason); required for goal close-out")
   		cmd.Flags().StringVar(&gateSuccessor, "gate-successor", "", "Where any risk gate moves, or 'none' (gate_successor); required for goal close-out")
   	}
   	return cmd
   ```
   The theme / objective / vision registrations are untouched (they pass `""` for both).

10. Regenerate the mocks against the new interfaces:
    ```
    make generate
    ```
    This rewrites `mocks/complete-operation.go`, `mocks/goal-complete-operation.go`, `mocks/frontmatter-set-operation.go`, `mocks/entity-set-operation.go`. Do not hand-edit them.

11. Update the four existing ops-test `Execute` call sites mechanically — arity only, pass `, "", ""`; do not weaken or re-fixture any existing assertion:
    - `/workspace/pkg/ops/complete_test.go:65` — `completeOp.Execute(ctx, vaultPath, taskName, "test-vault", force, "", "")`
    - `/workspace/pkg/ops/goal_complete_test.go:59` — `op.Execute(ctx, vaultPath, goalName, vaultName, force, "", "")`
    - `/workspace/pkg/ops/frontmatter_test.go:337` — `setOp.Execute(ctx, vaultPath, taskName, key, value, "", "")`
    - `/workspace/pkg/ops/frontmatter_entity_test.go:175` — `setOp.Execute(ctx, vaultPath, goalName, key, value, "", "")`
    - `/workspace/pkg/ops/wikilink_roundtrip_test.go` — the `exec` helper closures (~lines 115-140) call `.Execute(ctx, vaultPath, name, key, value)` on `NewFrontmatterSetOperation` and the four `NewGoal/Theme/Objective/VisionSetOperation` constructors; append `, "", ""` to all five calls (`Execute(ctx, vaultPath, name, key, value, "", "")`).
    Run `make format` afterwards so golines re-wraps any over-100-column line.

**Part C — ops-level one-step tests**

12. In `/workspace/pkg/ops/complete_test.go`, add `Context("one-step close-out writes reason and successor with the status", ...)` that overrides the fixture with a field-less task (`map[string]any{"status": "todo"}`, empty content) and asserts that calling `completeOp.Execute(ctx, vaultPath, taskName, "test-vault", false, "gate moved to X", "none")` returns a nil error, `mockTaskStorage.WriteTaskCallCount()` is `1`, and the written task (via `mockTaskStorage.WriteTaskArgsForCall(0)`) has `GetString("aborted_reason") == "gate moved to X"`, `GetString("gate_successor") == "none"`, and `Status() == domain.TaskStatusCompleted`. (Spec Desired Behavior 4 — one invocation.)

13. In `/workspace/pkg/ops/goal_complete_test.go`, add the goal mirror: field-less goal (`map[string]any{"status": "active"}`, empty content), `op.Execute(ctx, vaultPath, goalName, vaultName, false, "achieved", "none")` → nil error, `WriteGoalCallCount()` `1`, written goal carries both fields and `Status() == domain.GoalStatusCompleted`.

14. In `/workspace/pkg/ops/frontmatter_test.go`, add a `Context("one-step close-out via status set", ...)` on `FrontmatterSetOperation`: field-less task, call `setOp.Execute(ctx, vaultPath, taskName, "status", "aborted", "reason text", "none")` → nil error, written task (via `WriteTaskArgsForCall(0)`) has `Status() == domain.TaskStatusAborted` and both fields set. Add a second `It` showing a non-close-out target with flags writes NO fields: `setOp.Execute(ctx, vaultPath, taskName, "status", "in_progress", "reason text", "none")` → nil error, written task has `Status() == domain.TaskStatusInProgress` and `GetString("aborted_reason") == ""`. (Spec Invariant 6.)

15. In `/workspace/pkg/ops/frontmatter_entity_test.go`, add the goal mirror on `goalSetOperation`: field-less goal, `setOp.Execute(ctx, vaultPath, goalName, "status", "aborted", "reason text", "none")` → nil error, written goal has `Status() == domain.GoalStatusAborted` and both fields set. Mirror the file's existing assertion style for the written goal.

**Part D — end-to-end integration tests**

16. In `/workspace/integration/cli_test.go`, update the existing `Describe("vault-cli complete", ...)` `It("exits 0 and updates status to completed", ...)` (line 666) to pass `--reason "test reason"` and `--gate-successor none` after `"complete"` / `"my-task"` in the `exec.Command` args, so the one-step path is exercised (the prompt-1 fixture fields stay; the flags overwrite them with the same values). Keep the exit-0 assertion and the `status: completed` file assertion.

17. Add a new `Describe("vault-cli task set status aborted gating", ...)` block using a field-less task fixture via `createTempVault(map[string]string{"my-task": "---\nstatus: in_progress\n---\n# My Task\n"})`. Plain-mode (default output format) specs, using `session.Err` (stderr) for the message:
    - `"rejects aborted without reason and gate-successor"` — `task set my-task status aborted` → `Eventually(session).Should(gexec.Exit(1))`; `string(session.Err.Contents())` contains `aborted_reason`; the file content does NOT contain `status: aborted`. (Spec AC 3.)
    - `"accepts aborted with --reason and --gate-successor"` — `task set my-task status aborted --reason "gate moved to X" --gate-successor none` → exit 0; file contains `status: aborted`, `aborted_reason: gate moved to X`, `gate_successor: none`. (Spec AC 3.)
    - `"names the missing field and the succeeding command form in JSON error output"` — `task set my-task status aborted --output json` → the parsed JSON (see the JSON-schema Describe blocks for the `json.Unmarshal` + `HaveKeyWithValue` style) has `success == false` and its `error` field contains `aborted_reason`, `trigger / gate / threshold / recurring check`, and `vault-cli task set "my-task" status aborted --reason`. Do NOT assert the exit code here — JSON-mode mutation commands return the printed body (exit 0 by pre-existing design). (Spec DB 3 — same text in JSON.)

18. Add a new `Describe("vault-cli task complete gating", ...)` block with a field-less task fixture (`status: in_progress`):
    - `"rejects complete without reason"` — `task complete my-task` → exit non-zero; stderr contains `aborted_reason`; file does NOT contain `status: completed`. (Spec AC 4.)
    - `"rejects complete with --force but without reason"` — `task complete my-task --force` on a task whose content has an incomplete `- [ ]` subtask → exit non-zero; stderr contains `aborted_reason`; file does NOT contain `status: completed`. (Spec Invariant 6 — `--force` bypasses only the incomplete-subtask check, never the reason guard.)
    - `"accepts complete with --reason and --gate-successor"` — `task complete my-task --reason "all done" --gate-successor none` → exit 0; file contains `status: completed`, `aborted_reason: all done`, `gate_successor: none`. (Spec AC 4.)
    - `"persists YAML-special characters in reason via the serializer"` — `task complete my-task --reason "multi\nline: quoted" --gate-successor none` → exit 0; then `task get my-task aborted_reason --output json` → exit 0 and the parsed JSON `value` equals the literal two-line string `multi\nline: quoted` (the newline and colon must round-trip through the YAML serializer, never break the frontmatter parse). (Spec Failure Modes row 5 — YAML-special characters must be covered by a test.)

19. Add a new `Describe("vault-cli task update close-out gating", ...)` block with a task whose content is all checked checkboxes and no close-out fields (`status: in_progress`, `- [x]` items):
    - `"rejects checkbox-sync completion without close-out fields"` — `task update my-task` → exit non-zero; stderr contains `aborted_reason`; file does NOT contain `status: completed`. (Spec AC 5.)
    - `"accepts checkbox-sync completion once the fields are present"` — first `task set my-task aborted_reason "all done"` and `task set my-task gate_successor "none"` (two-step close-out), then `task update my-task` → exit 0; file contains `status: completed`. (Spec AC 5 — checkbox sync still completes when the fields exist.)

20. Add a new `Describe("vault-cli goal close-out gating", ...)` block using `createTempVaultWithGoals(map[string]string{}, map[string]string{"my-goal": "---\nstatus: in_progress\n---\n# My Goal\n"})`:
    - `"rejects goal aborted without reason"` — `goal set my-goal status aborted` → exit non-zero; stderr contains `aborted_reason`; goal file does NOT contain `status: aborted`. (Spec AC 6.)
    - `"accepts goal aborted with --reason and --gate-successor"` — `goal set my-goal status aborted --reason "no longer needed" --gate-successor none` → exit 0; goal file contains `status: aborted`, `aborted_reason: no longer needed`, `gate_successor: none`. (Spec AC 6.)
    - `"accepts goal complete with --reason and --gate-successor"` — `goal complete my-goal --reason "achieved" --gate-successor none` → exit 0; goal file contains `status: completed`, `aborted_reason: achieved`, `gate_successor: none`. (Spec AC 6.)

21. Add a new `Describe("vault-cli non-close-out transitions unaffected", ...)` block (regression, Spec AC 7) with a field-less task fixture:
    - `"task set status in_progress works without flags"` — `task set my-task status in_progress` → exit 0; file contains `status: in_progress` and does NOT contain `aborted_reason:`.
    - `"task set status hold works without flags"` — `task set my-task status hold` → exit 0; file contains `status: hold`.

22. In `/workspace/docs/task-writing.md` § Lifecycle, in the "Close-out fields" paragraph (after "Pre-existing reason-less close-outs are not backfilled."), add one sentence: "`task set`, `task complete`, `goal set`, and `goal complete` accept `--reason <text>` and `--gate-successor <successor|none>` to record both fields in the same invocation; a close-out attempted without them is rejected with an error that names the missing fields and the succeeding command form."

22b. In `/workspace/README.md`, the two usage examples at lines 77 and 85 (`vault-cli task complete "..."` and `vault-cli task set "..." status done`) show close-out commands that now fail without the fields — append `--reason "..." --gate-successor none` to both so the documented commands still work after this change. (docs/dod.md: "README.md is updated if the change affects usage".)

23. Add a `## Unreleased` section to `/workspace/CHANGELOG.md` immediately below the "All notable changes…" preamble and immediately above `## v0.115.0`, containing exactly one bullet. Use a `feat:` prefix and call out the breaking change (spec Constraint):

    ```
    ## Unreleased

    - feat: closing out a task or goal (`status: aborted` or `status: completed` via `task set`, `task complete`, `goal set`, `goal complete`, or the `task update` checkbox sync) now requires an `aborted_reason` and a `gate_successor` frontmatter field — the literal `none` when no risk gate is owned — and is rejected otherwise with an error naming the missing fields, the owned-risk categories (trigger / gate / threshold / recurring check), and the succeeding command form. `--reason <text>` and `--gate-successor <successor|none>` on the four close-out commands record both fields with the status in one step. Breaking change: existing scripts that close out without the fields now fail. Pre-existing reason-less close-outs are not backfilled.
    ```

    Do NOT create a `## vX.Y.Z` section and do NOT touch `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`.

24. Run `make format`, then `make test`; iterate until green, then `make precommit` once. If `make precommit` fails on a target, re-run only the failing target until it passes, then `make precommit` once more.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Frozen field names: `aborted_reason` (free text, used for BOTH `aborted` and `completed` — there is NO `completion_reason` field) and `gate_successor` (a successor name or the literal `none`).
- `gate_successor` is required at close-out time with `none` allowed as the explicit no-gate disposition. A missing value is indistinguishable from "did not think about the gate".
- The `SetStatus` chokepoints remain the only enforcement points — do NOT add a guard at the ops or CLI layer. This prompt only enriches the error text and wires the one-step flags.
- No bypass/opt-out: do NOT add a flag, env var, or config key that disables the reason guard. `task complete --force` bypasses ONLY the incomplete-subtask check, never the reason guard (a `--force` close-out without the fields still fails).
- The one-step write must be a single `WriteTask` / `WriteGoal`: fields and status land together, or (on a guard rejection) nothing is written. Do NOT introduce a two-write sequence in the ops layer.
- `aborted_reason` / `gate_successor` values are user-supplied strings stored as Go strings through the existing YAML serializer (special characters quoted) — never string-concatenated into file text.
- Flags on a non-close-out target must not persist the fields (verified in requirement 14).
- The two-step close-out (`task set <name> aborted_reason "<text>"`, then set status) must keep working — the guard reads the fields at transition time; do not break it.
- `task set <name> <key> <value>` remains a valid 3-argument command for every non-close-out key.
- Non-close-out transitions (`in_progress`, `next`, `backlog`, `hold`) and all non-status fields behave exactly as today.
- `pkg/ops` stays a library layer: it returns structured results and error text, never writes to stdout.
- Do NOT touch `objective_complete.go` (objectives are out of scope) or the theme/objective/vision set-operation logic beyond the ignored signature parameters.
- Existing tests must still pass; `make precommit` must exit 0. The JSON-mode exit-0-with-error-body behavior of `task set` / `goal set` is pre-existing — do not change it.
</constraints>

<verification>
Run all of the following from `/workspace`.

The four flags are declared:

```
grep -c '"reason", "", "Close-out reason (aborted_reason); required to complete"' pkg/cli/cli.go
grep -c '"gate-successor", "", "Where any risk gate moves, or .none. (gate_successor); required to complete"' pkg/cli/cli.go
grep -c '"reason", "", "Close-out reason (aborted_reason); required for goal close-out"' pkg/cli/cli.go
grep -c '"gate-successor", "", "Where any risk gate moves, or .none. (gate_successor); required for goal close-out"' pkg/cli/cli.go
```

Each must print exactly `1` (the goal variants are conditional on `entityType == "goal"`).

The one-step write and the command-form enrichment are wired in all four ops paths:

```
grep -c 'task.SetField(ctx, "aborted_reason", reason)' pkg/ops/complete.go pkg/ops/frontmatter.go
grep -c 'goal.SetField(ctx, "aborted_reason", reason)' pkg/ops/goal_complete.go pkg/ops/frontmatter_entity.go
grep -c 'missing close-out field(s)' pkg/ops/complete.go pkg/ops/goal_complete.go pkg/ops/frontmatter.go pkg/ops/frontmatter_entity.go
grep -c 'vault-cli task complete' pkg/ops/complete.go
grep -c 'vault-cli goal complete' pkg/ops/goal_complete.go
grep -c 'vault-cli task set' pkg/ops/frontmatter.go
grep -c 'vault-cli goal set' pkg/ops/frontmatter_entity.go
```

Each must print a number `>= 1`. The theme/objective/vision set operations must NOT reference the close-out fields:

```
grep -c 'aborted_reason' pkg/ops/frontmatter_entity.go
```

The count must be exactly `2` (the goalSetOperation field-write line and its import reference — if golines splits the line, accept `>= 1` but confirm the goal set op is the only consumer).

The regenerated mocks match the new arity (counterfeiter fake signatures; the gofmt-aligned multi-space padding between the field name and type is expected, so match on the signature fragment):

```
grep -c 'func(context.Context, string, string, string, bool, string, string) (ops.MutationResult, error)' mocks/complete-operation.go
grep -c 'func(context.Context, string, string, string, string, string, string) error' mocks/frontmatter-set-operation.go
grep -c 'func(context.Context, string, string, string, string, string, string) error' mocks/entity-set-operation.go
grep -c 'func(context.Context, string, string, string, bool, string, string) (ops.MutationResult, error)' mocks/goal-complete-operation.go
```

Each must print exactly `1`. The parameter arity must be 7 for all four ops (set ops: `context.Context` + 6 strings; complete ops: `context.Context` + 3 strings + `bool` + 2 strings).

The integration specs exist:

```
grep -c 'rejects aborted without reason and gate-successor' integration/cli_test.go
grep -c 'accepts aborted with --reason and --gate-successor' integration/cli_test.go
grep -c 'rejects complete without reason' integration/cli_test.go
grep -c 'rejects complete with --force but without reason' integration/cli_test.go
grep -c 'rejects checkbox-sync completion without close-out fields' integration/cli_test.go
grep -c 'accepts checkbox-sync completion once the fields are present' integration/cli_test.go
grep -c 'rejects goal aborted without reason' integration/cli_test.go
grep -c 'accepts goal complete with --reason and --gate-successor' integration/cli_test.go
grep -c 'task set status in_progress works without flags' integration/cli_test.go
grep -c 'persists YAML-special characters in reason via the serializer' integration/cli_test.go
grep -c '--reason' integration/cli_test.go
```

Each must print a number `>= 1`.

The ops-level one-step specs exist:

```
grep -c 'one-step close-out writes reason and successor with the status' pkg/ops/complete_test.go
grep -c 'one-step close-out via status set' pkg/ops/frontmatter_test.go
```

Each must print a number `>= 1`.

The docs and changelog are in sync:

```
grep -c '--reason <text> and --gate-successor' docs/task-writing.md
grep -c '^## Unreleased$' CHANGELOG.md
grep -c 'gate_successor' CHANGELOG.md
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

The first three must each print `>= 1`; the fourth must still print `## v0.115.0`; the fifth must still show the existing version string.

The full module is green:

```
go test -mod=mod -count=1 ./pkg/... ./integration/...
```

Must exit 0.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</verification>
