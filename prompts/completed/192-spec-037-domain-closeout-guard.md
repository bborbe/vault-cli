---
status: completed
spec: [037-mandatory-abort-reason-and-gate-successor]
summary: Implemented the domain close-out guard at the TaskFrontmatter/GoalFrontmatter SetStatus chokepoints (reject aborted/completed without non-empty aborted_reason + gate_successor, naming missing fields, leaving frontmatter unchanged), added 16 new domain guard tests, mechanically updated all collateral test fixtures (including three the prompt's list missed), and fixed a pre-existing make format/precommit tooling breakage (golines v0.13.0 / goimports-reviser v3.12.6 / Go 1.27 gofmt convert return X{...}, fn(...) to a form golangci-lint v2.13.1's bundled gofmt rejects) so make precommit exits 0
execution_id: vault-cli-exec-192-spec-037-domain-closeout-guard
dark-factory-version: dev
created: "2026-08-24T19:00:00Z"
queued: "2026-08-24T19:00:06Z"
started: "2026-08-24T19:00:08Z"
completed: "2026-08-24T19:17:25Z"
---

# Domain close-out guard at the SetStatus chokepoints

<summary>
- Closing out a task or goal (target status `aborted` or `completed`) now requires the frontmatter to already hold a non-empty `aborted_reason` AND a non-empty `gate_successor` at the moment of transition.
- A close-out attempted without both fields is rejected as a validation error before anything is written — the frontmatter stays byte-for-byte as it was; there is no partial state.
- The error names exactly which field(s) are missing (`aborted_reason`, `gate_successor`, or both), states what the work may own (trigger / gate / threshold / recurring check), and asks where that risk moves or to record `none`.
- The rule lives at the single shared status-change point on both tasks and goals, so every route that can set the status (set, complete, checkbox sync) inherits the guard with no route-specific code.
- Non-close-out transitions (`next`, `in_progress`, `backlog`, `hold`) never require the fields and behave exactly as today.
- Whitespace-only field values count as missing — a blank reason or a blank successor still blocks the close-out.
- Pre-existing aborted/completed tasks and goals are untouched: nothing is migrated or backfilled.
- Existing tests that close out without the fields are mechanically updated to carry them, because the guard now rejects their setups.
</summary>

<objective>
`TaskFrontmatter.SetStatus` and `GoalFrontmatter.SetStatus` must reject a transition to `aborted` or `completed` unless the frontmatter at that moment holds a non-empty `aborted_reason` AND a non-empty `gate_successor`, raising the rejection in the existing validation style (`errors.Wrapf(ctx, validation.Error, ...)`) and leaving the frontmatter unchanged on rejection. No other transition is affected.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, `/workspace/docs/development-patterns.md` for the domain/ops/CLI layering, and `/workspace/docs/task-writing.md` § Lifecycle → "Close-out fields (`aborted` / `completed`)" — this is the convention the guard enforces: `aborted_reason` is the free-text why (used for BOTH `aborted` and `completed`; there is no `completion_reason` field), `gate_successor` names where any risk gate moves or the literal `none`.

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-validation-framework-guide.md` — the `github.com/bborbe/validation` style used for validation errors
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrapf(ctx, cause, ...)` with `github.com/bborbe/errors`, never `fmt.Errorf`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite style, `package domain_test`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — doc-comment style for the new helper file

Read these files fully before making changes:

- `/workspace/pkg/domain/task_frontmatter.go` — the file being changed. Verified current `SetStatus` (lines 179-186):

  ```go
  // SetStatus validates and stores the status in the map.
  func (f *TaskFrontmatter) SetStatus(s TaskStatus) error {
  	if err := s.Validate(context.Background()); err != nil {
  		return err
  	}
  	f.Set("status", string(s))
  	return nil
  }
  ```

  It already imports `strings`, `github.com/bborbe/errors`, and `github.com/bborbe/validation`. `TaskFrontmatter` embeds `FrontmatterMap`, so inside a method the map is reachable as `f.FrontmatterMap`.

- `/workspace/pkg/domain/goal_frontmatter.go` — same `SetStatus` shape (lines 140-147) with `GoalStatus`. Also already imports `strings`, `errors`, `validation`.

- `/workspace/pkg/domain/frontmatter_map.go` — `GetString` (line 39) returns `""` for an absent key, and `fmt.Sprintf("%v", v)` for non-string values; a stored empty string round-trips as `""`.

- `/workspace/pkg/domain/task_status.go` — `TaskStatusAborted` / `TaskStatusCompleted` constants and the `Validate` pattern this guard mirrors. `/workspace/pkg/domain/goal.go` — `GoalStatusAborted` / `GoalStatusCompleted` (lines 53-55) and `GoalStatus.Validate`.

- `/workspace/pkg/domain/task_frontmatter_test.go` — existing `Describe("SetStatus", ...)` block (line 69) to mirror; the `fm` fixture is `domain.NewTaskFrontmatter(nil)` from the enclosing `BeforeEach`.
- `/workspace/pkg/domain/goal_frontmatter_test.go` — existing `Describe("SetStatus", ...)` block (line 40).

**Collateral breakage you MUST repair (this is why the fixture updates are requirements below):** the guard fires the moment it lands, and `make test` runs the whole module. Verified exact call sites that now fail and are fixed by the mechanical updates in requirement 6:

- `/workspace/pkg/storage/markdown_test.go:156` — `_ = task.SetStatus(domain.TaskStatusCompleted)` inside `It("writes a task successfully", ...)`. `ctx` is in scope in that `It` (enclosing `BeforeEach` sets it at line 68).
- `/workspace/pkg/ops/complete_test.go` — default `task` fixture (lines 54-57, `map[string]any{"status": "todo"}`) used by every success context.
- `/workspace/pkg/ops/goal_complete_test.go` — default `goal` fixture (lines 48-51, `map[string]any{"status": "active"}`) used by every success context AND by the `Context("goal already completed", ...)` at line 74 whose `_ = goal.SetStatus(domain.GoalStatusCompleted)` (line 78) must actually mark the goal completed for the "already completed" branch to fire.
- `/workspace/pkg/ops/update_test.go` — the shared default task fixture in the enclosing `BeforeEach` (line 40, `map[string]any{"status": "todo"}` — the `Context("with all checkboxes checked", ...)` at lines 62-73 overrides only `task.Content`, not the frontmatter map) whose computed target is `TaskStatusCompleted`.
- `/workspace/pkg/ops/frontmatter_test.go` — default `FrontmatterSetOperation` fixture (lines 327-331, `map[string]any{}` at line 328) used by `Context("setting status field", ...)` (line 399; `key = "status"` / `value = "completed"` at 401-402).
- `/workspace/integration/cli_test.go` — `Describe("vault-cli complete", ...)` `Context("when task exists", ...)` fixture (lines 651-659): `task complete my-task` without fields now exits non-zero, so the `It("exits 0 and updates status to completed", ...)` (line 666) fails.

The ops tests use `github.com/bborbe/vault-cli/mocks` storage fakes with real `domain.NewTask` / `domain.NewGoal` frontmatter, so the real domain guard is exercised; the integration suite builds the real binary (`gexec.Build` in `/workspace/integration/integration_suite_test.go`).
</context>

<requirements>
1. Create `/workspace/pkg/domain/closeout.go` — a new file, `package domain`, with the BSD license header used by the other files in that package (copy it from `task_frontmatter.go`), containing exactly:

   ```go
   package domain

   import "strings"

   // closeOutFields are the frontmatter keys that must both be present and
   // non-empty before a task or goal may transition to status aborted or
   // completed (a "close-out"). aborted_reason is the free-text why; gate_successor
   // names where any risk gate the entity owned moves, or the literal "none".
   // Both names are frozen by spec 037 and documented in docs/task-writing.md § Lifecycle.
   var closeOutFields = []string{"aborted_reason", "gate_successor"}

   // missingCloseOutFields returns the subset of close-out fields that are absent
   // or whitespace-only on the frontmatter. An empty slice means a close-out
   // transition is allowed.
   func missingCloseOutFields(f FrontmatterMap) []string {
   	var missing []string
   	for _, field := range closeOutFields {
   		if strings.TrimSpace(f.GetString(field)) == "" {
   			missing = append(missing, field)
   		}
   	}
   	return missing
   }
   ```

2. In `/workspace/pkg/domain/task_frontmatter.go`, replace the whole `SetStatus` method (lines 179-186) with:

   ```go
   // SetStatus validates and stores the status in the map.
   // Close-out transitions (aborted, completed) are rejected unless the frontmatter
   // already holds a non-empty aborted_reason AND a non-empty gate_successor; the
   // rejection is raised before any write, so the frontmatter is left unchanged.
   func (f *TaskFrontmatter) SetStatus(s TaskStatus) error {
   	if err := s.Validate(context.Background()); err != nil {
   		return err
   	}
   	if s == TaskStatusAborted || s == TaskStatusCompleted {
   		if missing := missingCloseOutFields(f.FrontmatterMap); len(missing) > 0 {
   			return errors.Wrapf(
   				context.Background(),
   				validation.Error,
   				"cannot set status %q: missing close-out field(s) %s; a close-out must record why the work is being closed out (aborted_reason), consider what it owns (trigger / gate / threshold / recurring check), and name where that risk moves (gate_successor, or %q when nothing is inherited)",
   				s,
   				strings.Join(missing, ", "),
   				"none",
   			)
   		}
   	}
   	f.Set("status", string(s))
   	return nil
   }
   ```

   Do NOT change the `SetStatus` signature, and do NOT touch any other method in the file.

3. In `/workspace/pkg/domain/goal_frontmatter.go`, replace the whole `SetStatus` method (lines 140-147) with the mirror:

   ```go
   // SetStatus validates and stores the status in the map.
   // Close-out transitions (aborted, completed) are rejected unless the frontmatter
   // already holds a non-empty aborted_reason AND a non-empty gate_successor; the
   // rejection is raised before any write, so the frontmatter is left unchanged.
   func (f *GoalFrontmatter) SetStatus(s GoalStatus) error {
   	if err := s.Validate(context.Background()); err != nil {
   		return err
   	}
   	if s == GoalStatusAborted || s == GoalStatusCompleted {
   		if missing := missingCloseOutFields(f.FrontmatterMap); len(missing) > 0 {
   			return errors.Wrapf(
   				context.Background(),
   				validation.Error,
   				"cannot set status %q: missing close-out field(s) %s; a close-out must record why the work is being closed out (aborted_reason), consider what it owns (trigger / gate / threshold / recurring check), and name where that risk moves (gate_successor, or %q when nothing is inherited)",
   				s,
   				strings.Join(missing, ", "),
   				"none",
   			)
   		}
   	}
   	f.Set("status", string(s))
   	return nil
   }
   ```

   Do NOT change the `SetStatus` signature, and do NOT touch any other method in the file.

4. Add a new `Describe("SetStatus close-out guard", ...)` block to `/workspace/pkg/domain/task_frontmatter_test.go`, placed directly after the existing `Describe("SetStatus", ...)` block, reusing the enclosing `fm` fixture. Use exactly these `It` texts so the verification gates can find them. Every rejection `It` below must ALSO assert `errors.Is(err, validation.Error)` is true (mirroring the existing convention at `task_frontmatter_test.go:596`), in addition to the non-nil + message assertions listed:

   - `"rejects aborted without aborted_reason and gate_successor and leaves the frontmatter unchanged"` — `err := fm.SetStatus(domain.TaskStatusAborted)`; assert `err` is non-nil, `err.Error()` contains `"missing close-out field(s) aborted_reason, gate_successor"`, and `fm.Status()` is still `domain.TaskStatus("")`. (Spec AC 1.)
   - `"rejects completed without close-out fields and leaves the frontmatter unchanged"` — same for `domain.TaskStatusCompleted`; assert `fm.Status()` is `domain.TaskStatus("")`. (Spec AC 1, completed arm.)
   - `"accepts aborted when aborted_reason and gate_successor are both present"` — `fm = domain.NewTaskFrontmatter(map[string]any{"aborted_reason": "no longer needed", "gate_successor": "none"})`; `SetStatus(domain.TaskStatusAborted)` succeeds and `fm.Status()` equals `domain.TaskStatusAborted`. (Spec AC 1 success arm.)
   - `"accepts completed when aborted_reason and gate_successor are both present"` — same with `"all criteria met"` and `domain.TaskStatusCompleted`.
   - `"rejects aborted when only aborted_reason is present"` — `map[string]any{"aborted_reason": "no longer needed"}`; error non-nil and `err.Error()` contains `"missing close-out field(s) gate_successor"` and NOT `"missing close-out field(s) aborted_reason, gate_successor"`. (Spec Desired Behavior 1 — names the missing field.)
   - `"rejects aborted when only gate_successor is present"` — `map[string]any{"gate_successor": "none"}`; error non-nil and `err.Error()` contains `"missing close-out field(s) aborted_reason"`.
   - `"treats whitespace-only close-out fields as missing"` — `map[string]any{"aborted_reason": "   ", "gate_successor": "none"}`; error non-nil, message names `aborted_reason`. (Security input rule — empty-after-trim is missing.)
   - `"does not require close-out fields for non-close-out statuses"` — for each of `domain.TaskStatusNext`, `domain.TaskStatusInProgress`, `domain.TaskStatusBacklog`, `domain.TaskStatusHold` build a fresh `domain.NewTaskFrontmatter(nil)`, assert `SetStatus` succeeds and the status is stored. (Spec AC 7 — regression.)

5. Add the mirror `Describe("SetStatus close-out guard", ...)` block to `/workspace/pkg/domain/goal_frontmatter_test.go` after the existing `Describe("SetStatus", ...)` block, with `GoalStatusAborted` / `GoalStatusCompleted` and the same four non-close-out statuses (`GoalStatusNext`, `GoalStatusInProgress`, `GoalStatusBacklog`, `GoalStatusHold`), same `It` texts with `goal` wording where natural, and the same `errors.Is(err, validation.Error)` assertion on every rejection `It`. (Spec AC 2.)

6. Mechanical collateral updates — the guard now rejects close-outs without fields, so update exactly these fixtures to carry the two fields. Do NOT weaken, delete, or re-target any other assertion in these files, and do NOT change any other fixture content:

   - `/workspace/pkg/storage/markdown_test.go` — inside `It("writes a task successfully", ...)`, replace
     ```go
     				_ = task.SetStatus(domain.TaskStatusCompleted)
     				Expect(store.WriteTask(ctx, task)).To(Succeed())
     ```
     with
     ```go
     				Expect(task.SetField(ctx, "aborted_reason", "test reason")).To(Succeed())
     				Expect(task.SetField(ctx, "gate_successor", "none")).To(Succeed())
     				Expect(task.SetStatus(domain.TaskStatusCompleted)).To(Succeed())
     				Expect(store.WriteTask(ctx, task)).To(Succeed())
     ```
   - `/workspace/pkg/ops/complete_test.go` — default `task` fixture: change `map[string]any{"status": "todo"}` to `map[string]any{"status": "todo", "aborted_reason": "test reason", "gate_successor": "none"}`.
   - `/workspace/pkg/ops/goal_complete_test.go` — default `goal` fixture: change `map[string]any{"status": "active"}` to `map[string]any{"status": "active", "aborted_reason": "test reason", "gate_successor": "none"}`.
   - `/workspace/pkg/ops/update_test.go` — the `Context("with all checkboxes checked", ...)` task fixture: change `map[string]any{"status": "todo"}` to `map[string]any{"status": "todo", "aborted_reason": "test reason", "gate_successor": "none"}`.
   - `/workspace/pkg/ops/frontmatter_test.go` — default `FrontmatterSetOperation` fixture: change `map[string]any{}` to `map[string]any{"aborted_reason": "test reason", "gate_successor": "none"}`.
   - `/workspace/integration/cli_test.go` — the `Context("when task exists", ...)` fixture: add `aborted_reason: test reason` and `gate_successor: none` to the `my-task` frontmatter (between `priority: 2` and the closing `---`), so `task complete my-task` without flags succeeds via the two-step close-out (fields already in the file).

7. Run `make format` (golines wraps at 100 columns) and then run `make test`; iterate until green, then `make precommit` once. If `make precommit` fails on a target, re-run only the failing target (`make lint`, `make vet`, `make generate`, `make test`, ...) until it passes, then `make precommit` once more.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Frozen field names: `aborted_reason` (free text, used for BOTH `aborted` and `completed` transitions — there is NO `completion_reason` field) and `gate_successor` (a successor name or the literal `none`). Convention documented in `docs/task-writing.md` § Lifecycle (Close-out fields).
- `gate_successor` is required at close-out time with `none` allowed as the explicit no-gate disposition. "Optional" means the VALUE may be `none`; a missing field is indistinguishable from "did not think about the gate".
- Validation style is frozen: errors raised via `errors.Wrapf(ctx, validation.Error, ...)`, matching the existing `TaskStatus.Validate` / `GoalStatus.Validate` pattern. Do NOT introduce a new error-raise mechanism, do NOT add a custom error type or sentinel for this.
- The `SetStatus` chokepoints are frozen enforcement points: `TaskFrontmatter.SetStatus` and `GoalFrontmatter.SetStatus`. Do NOT change either signature and do NOT add a guard anywhere else (no route-level guard, no ops-level re-check in this prompt).
- Gate ONLY `aborted` and `completed`. Transitions to `in_progress`, `next`, `backlog`, `hold`, and all non-status fields must behave exactly as today.
- No backfill: do NOT touch any existing aborted/completed task or goal file in any vault. This prompt changes code and tests only.
- No bypass/opt-out: do NOT add a flag, env var, or config key that disables the guard.
- `aborted_reason` / `gate_successor` are plain Go strings stored through the existing YAML serializer; never string-concatenate them into file text (no file writes happen in this prompt anyway).
- The fixture updates in requirement 6 are mechanical and additive only: adding the two fields. Do not otherwise modify the existing test bodies or fixtures.
- Existing tests must still pass; `make precommit` must exit 0.
</constraints>

<verification>
Run all of the following from `/workspace`.

The shared helper exists exactly once and the guard uses it in both frontmatter files:

```
grep -c 'func missingCloseOutFields' pkg/domain/closeout.go
grep -c 'missingCloseOutFields(f.FrontmatterMap)' pkg/domain/task_frontmatter.go
grep -c 'missingCloseOutFields(f.FrontmatterMap)' pkg/domain/goal_frontmatter.go
grep -c 'closeOutFields = \[\]string{"aborted_reason", "gate_successor"}' pkg/domain/closeout.go
```

Each must print exactly `1`.

The frozen validation-style message is raised from both chokepoints, once each (the `missing close-out field(s)` message is the binding gate):

```
grep -c 'missing close-out field(s)' pkg/domain/task_frontmatter.go
grep -c 'missing close-out field(s)' pkg/domain/goal_frontmatter.go
grep -c 'trigger / gate / threshold / recurring check' pkg/domain/task_frontmatter.go
grep -c 'trigger / gate / threshold / recurring check' pkg/domain/goal_frontmatter.go
```

Each must print exactly `1`.

The new domain specs exist:

```
grep -c 'rejects aborted without aborted_reason and gate_successor and leaves the frontmatter unchanged' pkg/domain/task_frontmatter_test.go
grep -c 'does not require close-out fields for non-close-out statuses' pkg/domain/task_frontmatter_test.go
grep -c 'does not require close-out fields for non-close-out statuses' pkg/domain/goal_frontmatter_test.go
grep -c 'rejects aborted when only gate_successor is present' pkg/domain/task_frontmatter_test.go
grep -c 'treats whitespace-only close-out fields as missing' pkg/domain/task_frontmatter_test.go
```

Each must print a number `>= 1`.

The collateral fixtures carry the fields:

```
grep -c '"aborted_reason": "test reason"' pkg/ops/complete_test.go
grep -c '"aborted_reason": "test reason"' pkg/ops/goal_complete_test.go
grep -c '"aborted_reason": "test reason"' pkg/ops/update_test.go
grep -c '"aborted_reason": "test reason"' pkg/ops/frontmatter_test.go
grep -c 'aborted_reason: test reason' integration/cli_test.go
grep -c 'SetField(ctx, "aborted_reason", "test reason")' pkg/storage/markdown_test.go
```

Each must print a number `>= 1`.

No close-out logic landed anywhere but the two chokepoints — the ops and CLI layers are untouched by the guard in this prompt:

```
grep -c 'missingCloseOutFields' pkg/ops/complete.go pkg/ops/update.go pkg/ops/goal_complete.go pkg/ops/frontmatter.go pkg/cli/cli.go
```

The combined count must print exactly `0`.

The packages are green:

```
go test -mod=mod -count=1 ./pkg/domain/... ./pkg/ops/... ./pkg/storage/... ./integration/...
```

Must exit 0.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</verification>
