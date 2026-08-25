---
status: completed
spec: [039-bug-completed-requires-closeout-fields]
summary: 'Split the spec-037 close-out guard by status: TaskFrontmatter/GoalFrontmatter SetStatus now consult aborted_reason+gate_successor for aborted only, completed always succeeds, dead Try-decoration and unused name params removed from the completion ops, all domain/ops/integration tests flipped to assert field-less completion succeeds, five completed routes covered in-container, docs and CHANGELOG updated'
execution_id: vault-cli-aborted-reason-exec-196-spec-039-status-split-guard
dark-factory-version: dev
created: "2026-08-25T07:35:47Z"
queued: "2026-08-25T08:09:37Z"
started: "2026-08-25T08:09:38Z"
completed: "2026-08-25T08:13:46Z"
branch: dark-factory/bug-completed-requires-closeout-fields
---

<summary>
- Splits the spec-037 close-out guard by status at the two `SetStatus` chokepoints: `completed` transitions stop consulting the close-out fields entirely, while `aborted` keeps the guard exactly as spec 037 locked it (both fields required, same actionable error, rejection before any write).
- After this prompt every completion route — `task complete`, `goal complete`, `task set <name> status completed`, `goal set <name> status completed`, and the `task update` checkbox sync — succeeds on a field-less task/goal and persists `status: completed` with no `aborted_reason` / `gate_successor` added.
- `--reason` / `--gate-successor` stay accepted on completion and are still persisted in the same single write; they just stop being required.
- The now-unreachable "missing close-out field(s) ... Try: vault-cli task complete" error decoration is removed from the two completion operations; the `task set` / `goal set` Try-decoration stays because it still serves the `aborted` path.
- Domain unit tests flip the completed-arm of the close-out guard (completed succeeds without fields, asserting the two keys are absent) while every aborted-arm test stays green as the regression lock.
- The three ops integration tests that asserted a no-field completion errors are flipped to assert success; in-container (real-binary) integration coverage is added for all five completed routes so `make test` proves the fix end-to-end in the container.
- `make precommit` passes.
</summary>

<objective>
Make `TaskFrontmatter.SetStatus` and `GoalFrontmatter.SetStatus` status-aware: only the `aborted` transition consults the close-out fields (`aborted_reason` + `gate_successor`, both required, unchanged from spec 037); a `completed` transition always succeeds after status validation and never reads or writes those fields. All five completion routes inherit the fix from the single domain seam, and every test that asserted the old completed-gate is updated to assert success.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, `/workspace/docs/development-patterns.md` for the domain/ops/CLI layering, and `/workspace/docs/task-writing.md` § Lifecycle → "Close-out fields (`aborted` / `completed`)" for the convention being re-scoped.

Relevant coding-plugin guides:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-validation-framework-guide.md` — `github.com/bborbe/validation` style for validation errors (`validation.Error`)
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` (`errors.Wrapf`, `errors.Wrap`, `errors.Errorf`), never `fmt.Errorf`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega suite style, external `package *_test`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — doc-comment style for the comment updates

Read these files fully before making changes:

- `/workspace/pkg/domain/closeout.go` — `closeOutFields` and `missingCloseOutFields` stay exactly as-is (frozen by spec 037); only the package-level comment is updated to state the guard applies to `aborted` only.
- `/workspace/pkg/domain/task_frontmatter.go` — `TaskFrontmatter.SetStatus` (lines 179-201) is the task chokepoint; the only behavior change is the guard condition `if s == TaskStatusAborted || s == TaskStatusCompleted` → `if s == TaskStatusAborted`. `TaskStatusAborted` / `TaskStatusCompleted` come from `task_status.go`. The file already imports `strings`, `github.com/bborbe/errors`, `github.com/bborbe/validation`.
- `/workspace/pkg/domain/goal_frontmatter.go` — `GoalFrontmatter.SetStatus` (lines 144-162), mirror, with `GoalStatusAborted` / `GoalStatusCompleted` from `goal.go`.
- `/workspace/pkg/domain/task_frontmatter_test.go` — the `Describe("SetStatus close-out guard", ...)` block (lines 80-146). The `fm` fixture in the enclosing `BeforeEach` is `domain.NewTaskFrontmatter(nil)` (no close-out fields). The `It` at lines 89-95 is the completed-rejection arm that must flip. The file imports `errors "github.com/bborbe/errors"` and `github.com/bborbe/validation`.
- `/workspace/pkg/domain/goal_frontmatter_test.go` — the mirror `Describe("SetStatus close-out guard", ...)` block (lines 53-119); the `It` at lines 62-68 must flip.
- `/workspace/pkg/ops/complete.go` — `setCompletedStatus` (lines 191-234) and its single call site at line 122. After the domain fix, `task.SetStatus(domain.TaskStatusCompleted)` can never return the "missing close-out field(s)" error, so the `strings.Contains(msg, "missing close-out field(s)")` Try-decoration block (lines 220-227) is unreachable dead code and its comment (lines 191-194) is now false; both are removed and the `taskName` parameter becomes unused and is removed from the signature (the `unparam` linter is enabled in `.golangci.yml` — an unused parameter would fail `make precommit`).
- `/workspace/pkg/ops/goal_complete.go` — the mirror `setCompletedStatus` (lines 107-144) and its single call site at line 88.
- `/workspace/pkg/ops/update.go` — the checkbox-sync path (`Execute`, lines 79-88) calls `task.SetStatus(target)`; no code change, it inherits the fix.
- `/workspace/pkg/ops/complete_test.go` — `Context("task without close-out fields", ...)` (lines 215-240) must flip; `Context("one-step close-out writes reason and successor with the status", ...)` (lines 242-269) stays (fields-supplied completion still succeeds).
- `/workspace/pkg/ops/goal_complete_test.go` — `Context("goal without close-out fields", ...)` (lines 260-281) must flip; `Context("one-step close-out writes reason and successor with the status", ...)` (lines 283-310) stays.
- `/workspace/pkg/ops/update_test.go` — `Context("all checkboxes checked but no close-out fields", ...)` (lines 194-224) must flip.
- `/workspace/integration/cli_test.go` — three gating tests assert the old completed-gate and WILL fail until flipped: `It("rejects complete without reason", ...)` (line 815, inside `Describe("vault-cli task complete gating", ...)` → `Context("field-less task with no incomplete subtasks", ...)`), `It("rejects complete with --force but without reason", ...)` (line 895, inside `Context("field-less task with an incomplete subtask", ...)`), and `It("rejects checkbox-sync completion without close-out fields", ...)` (line 929, inside `Describe("vault-cli task update close-out gating", ...)`). The `Describe("vault-cli goal close-out gating", ...)` block (lines 976-1052) has no completed-rejection test to flip, but gains two positive in-container cases. All other gating tests (aborted reject/accept, JSON error output, non-close-out transitions, `accepts complete with --reason...`, `persists YAML-special characters...`) stay exactly as they are.

**Why no scenario prompt:** the spec declares "Scenario coverage: none" — this is a local CLI behavior change reachable by unit + integration tests and replayed on a scratch vault at the operator rung; no Docker/cluster/external service is involved.

**Boundary coverage intent:** the ops tests use real `domain.NewTask` / `domain.NewGoal` against mock storage, so they traverse the real domain chokepoint through the ops path; the integration suite builds the real binary (`gexec.Build` in `/workspace/integration/integration_suite_test.go`) and traverses the full CLI→ops→domain→serializer path. Together they cover every boundary the fix crosses; the host-side operator ACs (6-8) stay on the spec's Verification ladder and are NOT duplicated here.
</context>

<requirements>
1. In `/workspace/pkg/domain/closeout.go`, replace the package-level comment on `closeOutFields` (lines 9-13) so it states the guard applies to `aborted` only. Keep `closeOutFields`, `missingCloseOutFields`, and all imports byte-for-byte. New comment:

   ```go
   // closeOutFields are the frontmatter keys that must both be present and
   // non-empty before a task or goal may transition to status aborted (a
   // "close-out"). aborted_reason is the free-text why; gate_successor names
   // where any risk gate the entity owned moves, or the literal "none".
   // Both names are frozen by spec 037; the guard is consulted for aborted
   // only — completed never reads these fields (spec 039).
   ```

2. In `/workspace/pkg/domain/task_frontmatter.go`, replace the whole `SetStatus` method (lines 179-201) with:

   ```go
   // SetStatus validates and stores the status in the map.
   // The aborted transition is a close-out and is rejected unless the frontmatter
   // already holds a non-empty aborted_reason AND a non-empty gate_successor;
   // completed never consults the close-out fields (spec 039). The rejection is
   // raised before any write, so the frontmatter is left unchanged.
   func (f *TaskFrontmatter) SetStatus(s TaskStatus) error {
   	if err := s.Validate(context.Background()); err != nil {
   		return err
   	}
   	if s == TaskStatusAborted {
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

   The ONLY behavioral change is the guard condition (`s == TaskStatusAborted || s == TaskStatusCompleted` becomes `s == TaskStatusAborted`) plus the doc comment. Do NOT change the signature, the error message text, or any other method.

3. In `/workspace/pkg/domain/goal_frontmatter.go`, replace the whole `SetStatus` method (lines 144-162) with the mirror (same shape, `GoalStatus` and `GoalStatusAborted`):

   ```go
   // SetStatus validates and stores the status in the map.
   // The aborted transition is a close-out and is rejected unless the frontmatter
   // already holds a non-empty aborted_reason AND a non-empty gate_successor;
   // completed never consults the close-out fields (spec 039). The rejection is
   // raised before any write, so the frontmatter is left unchanged.
   func (f *GoalFrontmatter) SetStatus(s GoalStatus) error {
   	if err := s.Validate(context.Background()); err != nil {
   		return err
   	}
   	if s == GoalStatusAborted {
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

   Do NOT change the signature, the error message text, or any other method.

4. In `/workspace/pkg/ops/complete.go`, remove the now-unreachable Try-decoration from `setCompletedStatus` and drop the now-unused `taskName` parameter (the `unparam` linter is enabled, so an unused parameter fails `make precommit`).

   a. Replace the call site at line 122:
   ```go
   if result, err := c.setCompletedStatus(ctx, task, taskName, reason, gateSuccessor); err != nil {
   ```
   with:
   ```go
   if result, err := c.setCompletedStatus(ctx, task, reason, gateSuccessor); err != nil {
   ```

   b. Replace the whole `setCompletedStatus` function (lines 191-234) with:

   ```go
   // setCompletedStatus persists the one-step close-out fields (when provided)
   // and transitions the task to completed. completed never consults the
   // close-out fields (spec 039), so this cannot fail on the guard; the error
   // return is kept for status-set failure propagation. Recurring tasks are
   // handled before this is reached, so the status always changes here.
   //
   //nolint:dupl // Structurally parallel to the goal variant; frozen field names prevent dedup
   func (c *completeOperation) setCompletedStatus(
   	ctx context.Context,
   	task *domain.Task,
   	reason, gateSuccessor string,
   ) (MutationResult, error) {
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
   		return MutationResult{
   			Success: false,
   			Error:   err.Error(),
   		}, errors.Wrap(ctx, err, "set status")
   	}
   	return MutationResult{}, nil
   }
   ```

   Do NOT change anything else in the file. `fmt` and `strings` remain imported and used elsewhere in the file — do not touch imports.

5. In `/workspace/pkg/ops/goal_complete.go`, mirror requirement 4:

   a. Replace the call site at line 88:
   ```go
   if result, err := g.setCompletedStatus(ctx, goal, goalName, reason, gateSuccessor); err != nil {
   ```
   with:
   ```go
   if result, err := g.setCompletedStatus(ctx, goal, reason, gateSuccessor); err != nil {
   ```

   b. Replace the whole `setCompletedStatus` function (lines 107-144) with the mirror (same shape, `goal *domain.Goal`, `goal.SetField`, `goal.SetStatus(domain.GoalStatusCompleted)`):

   ```go
   // setCompletedStatus persists the one-step close-out fields (when provided)
   // and transitions the goal to completed. completed never consults the
   // close-out fields (spec 039), so this cannot fail on the guard; the error
   // return is kept for status-set failure propagation.
   //
   //nolint:dupl // Structurally parallel to the task variant; frozen field names prevent dedup
   func (g *goalCompleteOperation) setCompletedStatus(
   	ctx context.Context,
   	goal *domain.Goal,
   	reason, gateSuccessor string,
   ) (MutationResult, error) {
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
   	if err := goal.SetStatus(domain.GoalStatusCompleted); err != nil {
   		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
   	}
   	return MutationResult{}, nil
   }
   ```

   Do NOT change anything else in the file.

6. Domain unit tests — task. In `/workspace/pkg/domain/task_frontmatter_test.go`, within the `Describe("SetStatus close-out guard", ...)` block, replace the completed-rejection `It` (lines 89-95) with a completed-acceptance `It` that asserts the two keys are absent after the transition (the enclosing `fm` fixture is `domain.NewTaskFrontmatter(nil)`, so no fields are present):

   ```go
   		It("accepts completed without close-out fields", func() {
   			Expect(fm.SetStatus(domain.TaskStatusCompleted)).To(Succeed())
   			Expect(fm.Status()).To(Equal(domain.TaskStatusCompleted))
   			Expect(fm.GetString("aborted_reason")).To(Equal(""))
   			Expect(fm.GetString("gate_successor")).To(Equal(""))
   		})
   ```

   Keep every other `It` in the block byte-for-byte unchanged — the aborted-rejection arms (with and without each field, whitespace-only), the both-fields-acceptance arms (including `"accepts completed when aborted_reason and gate_successor are both present"`), and the non-close-out loop are the regression lock for spec 037 and must stay green.

7. Domain unit tests — goal. In `/workspace/pkg/domain/goal_frontmatter_test.go`, mirror requirement 6: replace the completed-rejection `It` (lines 62-68) with:

   ```go
   		It("accepts completed without close-out fields", func() {
   			Expect(fm.SetStatus(domain.GoalStatusCompleted)).To(Succeed())
   			Expect(fm.Status()).To(Equal(domain.GoalStatusCompleted))
   			Expect(fm.GetString("aborted_reason")).To(Equal(""))
   			Expect(fm.GetString("gate_successor")).To(Equal(""))
   		})
   ```

   Keep every other `It` in the block unchanged.

8. Ops integration test — `complete_test.go`. In `/workspace/pkg/ops/complete_test.go`, replace the whole `Context("task without close-out fields", ...)` block (lines 215-240) with a success context. The task fixture is `map[string]any{"status": "todo"}` (no close-out fields), and `mockTaskStorage.WriteTaskReturns(nil)` is set in the enclosing `BeforeEach`. `writtenTask` is retrieved via `mockTaskStorage.WriteTaskArgsForCall(0)`, and `GetString` returns `""` for an absent key:

   ```go
   	Context("task completes without close-out fields", func() {
   		BeforeEach(func() {
   			task = domain.NewTask(
   				map[string]any{"status": "todo"},
   				domain.FileMetadata{Name: taskName},
   				domain.Content(""),
   			)
   			mockTaskStorage.FindTaskByNameReturns(task, nil)
   		})

   		It("returns no error", func() {
   			Expect(err).To(BeNil())
   		})

   		It("writes the task in a single write", func() {
   			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
   		})

   		It("persists status completed without close-out fields", func() {
   			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
   			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusCompleted))
   			Expect(writtenTask.GetString("aborted_reason")).To(Equal(""))
   			Expect(writtenTask.GetString("gate_successor")).To(Equal(""))
   		})
   	})
   ```

9. Ops integration test — `goal_complete_test.go`. In `/workspace/pkg/ops/goal_complete_test.go`, replace the whole `Context("goal without close-out fields", ...)` block (lines 260-281) with the mirror success context (`mockGoalStorage.WriteGoalReturns(nil)` is set in the enclosing `BeforeEach`; `writtenGoal` via `mockGoalStorage.WriteGoalArgsForCall(0)`):

   ```go
   	Context("goal completes without close-out fields", func() {
   		BeforeEach(func() {
   			goal = domain.NewGoal(
   				map[string]any{"status": "active"},
   				domain.FileMetadata{Name: goalName},
   				domain.Content(""),
   			)
   			mockGoalStorage.FindGoalByNameReturns(goal, nil)
   		})

   		It("returns no error", func() {
   			Expect(err).To(BeNil())
   		})

   		It("writes the goal in a single write", func() {
   			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(1))
   		})

   		It("persists status completed without close-out fields", func() {
   			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
   			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusCompleted))
   			Expect(writtenGoal.GetString("aborted_reason")).To(Equal(""))
   			Expect(writtenGoal.GetString("gate_successor")).To(Equal(""))
   		})
   	})
   ```

10. Ops integration test — `update_test.go`. In `/workspace/pkg/ops/update_test.go`, replace the whole `Context("all checkboxes checked but no close-out fields", ...)` block (lines 194-224) with a success context. The fixture is `map[string]any{"status": "todo"}` with all three checkboxes `[x]`, so `statusFromProgress` computes `TaskStatusCompleted`; `mockTaskStorage.WriteTaskReturns(nil)` is set in the enclosing `BeforeEach`:

    ```go
    	Context("all checkboxes checked without close-out fields", func() {
    		BeforeEach(func() {
    			task = domain.NewTask(
    				map[string]any{"status": "todo"},
    				domain.FileMetadata{Name: taskName},
    				domain.Content(`---
    status: todo
    ---

    # My Task

    - [x] First item
    - [x] Second item
    - [x] Third item
    `),
    			)
    			mockTaskStorage.FindTaskByNameReturns(task, nil)
    		})

    		It("returns no error", func() {
    			Expect(err).To(BeNil())
    		})

    		It("writes the task in a single write", func() {
    			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
    		})

    		It("persists status completed without close-out fields", func() {
    			_, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
    			Expect(writtenTask.Status()).To(Equal(domain.TaskStatusCompleted))
    			Expect(writtenTask.GetString("aborted_reason")).To(Equal(""))
    			Expect(writtenTask.GetString("gate_successor")).To(Equal(""))
    		})
    	})
    ```

11. Integration suite — flip the three now-false rejection tests in `/workspace/integration/cli_test.go`:

    a. In `Describe("vault-cli task complete gating", ...)` → `Context("field-less task with no incomplete subtasks", ...)`, replace `It("rejects complete without reason", ...)` (lines 815-831) with:

    ```go
    			It("completes without close-out fields", func() {
    				cmd := exec.Command(
    					binPath,
    					"--config", configPath,
    					"--vault", "test",
    					"task", "complete", "my-task",
    				)
    				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    				Expect(err).NotTo(HaveOccurred())
    				Eventually(session).Should(gexec.Exit(0))

    				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
    				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
    				Expect(err).NotTo(HaveOccurred())
    				Expect(string(content)).To(ContainSubstring("status: completed"))
    				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    			})
    ```

    b. In the same `Describe`, `Context("field-less task with an incomplete subtask", ...)`, replace `It("rejects complete with --force but without reason", ...)` (lines 895-911) with (the fixture has `- [ ] not done`, so `--force` is what bypasses the incomplete-subtask check — and since `completed` no longer consults the guard, the field-less completion now succeeds):

    ```go
    			It("completes with --force despite incomplete subtasks and without close-out fields", func() {
    				cmd := exec.Command(
    					binPath,
    					"--config", configPath,
    					"--vault", "test",
    					"task", "complete", "my-task", "--force",
    				)
    				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    				Expect(err).NotTo(HaveOccurred())
    				Eventually(session).Should(gexec.Exit(0))

    				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
    				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
    				Expect(err).NotTo(HaveOccurred())
    				Expect(string(content)).To(ContainSubstring("status: completed"))
    				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    			})
    ```

    c. In `Describe("vault-cli task update close-out gating", ...)`, replace `It("rejects checkbox-sync completion without close-out fields", ...)` (lines 929-945) with (the fixture has two `[x]` checkboxes and no close-out fields):

    ```go
    		It("completes via checkbox sync without close-out fields", func() {
    			cmd := exec.Command(
    				binPath,
    				"--config", configPath,
    				"--vault", "test",
    				"task", "update", "my-task",
    			)
    			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    			Expect(err).NotTo(HaveOccurred())
    			Eventually(session).Should(gexec.Exit(0))

    			taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
    			content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
    			Expect(err).NotTo(HaveOccurred())
    			Expect(string(content)).To(ContainSubstring("status: completed"))
    			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    		})
    ```

12. Integration suite — add in-container positive cases for the remaining completed routes in `/workspace/integration/cli_test.go`:

    a. In `Describe("vault-cli task complete gating", ...)` → `Context("field-less task with no incomplete subtasks", ...)` (same fixture as 11a), add this `It` after the flipped one:

    ```go
    			It("completes via task set status completed without close-out fields", func() {
    				cmd := exec.Command(
    					binPath,
    					"--config", configPath,
    					"--vault", "test",
    					"task", "set", "my-task", "status", "completed",
    				)
    				session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    				Expect(err).NotTo(HaveOccurred())
    				Eventually(session).Should(gexec.Exit(0))

    				taskPath := filepath.Join(vaultPath, "Tasks", "my-task.md")
    				content, err := os.ReadFile(taskPath) //#nosec G304 -- test file
    				Expect(err).NotTo(HaveOccurred())
    				Expect(string(content)).To(ContainSubstring("status: completed"))
    				Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    				Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    			})
    ```

    b. In `Describe("vault-cli goal close-out gating", ...)` (the `BeforeEach` fixture at lines 980-986 is `createTempVaultWithGoals(map[string]string{}, map[string]string{"my-goal": "---\nstatus: in_progress\n---\n# My Goal\n"})` — keep it as-is), add these two `It`s after the existing `"accepts goal complete with --reason and --gate-successor"`:

    ```go
    		It("accepts goal set status completed without close-out fields", func() {
    			cmd := exec.Command(
    				binPath,
    				"--config", configPath,
    				"--vault", "test",
    				"goal", "set", "my-goal", "status", "completed",
    			)
    			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    			Expect(err).NotTo(HaveOccurred())
    			Eventually(session).Should(gexec.Exit(0))

    			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
    			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
    			Expect(err).NotTo(HaveOccurred())
    			Expect(string(content)).To(ContainSubstring("status: completed"))
    			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    		})

    		It("accepts goal complete without close-out fields", func() {
    			cmd := exec.Command(
    				binPath,
    				"--config", configPath,
    				"--vault", "test",
    				"goal", "complete", "my-goal",
    			)
    			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
    			Expect(err).NotTo(HaveOccurred())
    			Eventually(session).Should(gexec.Exit(0))

    			goalPath := filepath.Join(vaultPath, "Goals", "my-goal.md")
    			content, err := os.ReadFile(goalPath) //#nosec G304 -- test file
    			Expect(err).NotTo(HaveOccurred())
    			Expect(string(content)).To(ContainSubstring("status: completed"))
    			Expect(string(content)).NotTo(ContainSubstring("aborted_reason:"))
    			Expect(string(content)).NotTo(ContainSubstring("gate_successor:"))
    		})
    ```

    Do NOT modify any other integration test. The aborted gating tests, the JSON-error-output test, the `accepts ... with --reason` tests, the YAML-special-characters test, and the non-close-out transitions test all stay byte-for-byte.

13. Run `make format` (golines wraps at 100 columns), then `make test`; iterate until green, then `make precommit` once. If `make precommit` fails on a target, re-run only the failing target (`make lint`, `make vet`, `make generate`, `make test`, ...) until it passes, then `make precommit` once more.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Frozen field names: `aborted_reason` (free text) and `gate_successor` (successor name or literal `none`). Both remain REQUIRED for `aborted` only; neither may ever gate a `completed` transition.
- The two `SetStatus` chokepoints (`pkg/domain/task_frontmatter.go`, `pkg/domain/goal_frontmatter.go`) are the sole enforcement points; the status split lives there. No route may special-case around them and no route may re-introduce a `completed` guard (e.g. an update-path or set-path check that demands the fields before writing `completed`).
- Spec-037 `aborted` semantics are frozen and unchanged: both fields required, rejection raised before any write via `errors.Wrapf(ctx, validation.Error, ...)`, same actionable error text (owned-risk categories + command form). The `task set` / `goal set` "Try: vault-cli ... status ..." error decoration in `pkg/ops/frontmatter.go` and `pkg/ops/frontmatter_entity.go` is NOT touched — it still serves the `aborted` path.
- `--reason` / `--gate-successor` remain user-supplied Go strings persisted through the existing YAML serializer; on completion they stay optional and are still written when supplied (the `writeTaskCloseOutFieldsIfCloseOut` / `writeGoalCloseOutFieldsIfCloseOut` / `setCompletedStatus` field-writing paths are unchanged except for the dead-branch removal in requirements 4-5).
- `task complete --force` semantics unchanged (bypasses only the incomplete-subtask check, never the `aborted` guard); non-close-out transitions (`in_progress`, `next`, `backlog`, `hold`) unchanged; objectives / themes / visions untouched (the guard never applied there).
- No migration, no backfill: completed-without-fields is the new contract; existing task/goal files are untouched.
- The `completed` status value and `TaskStatusCompleted` / `GoalStatusCompleted` constants are unchanged.
- Existing tests must still pass; `make precommit` must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</constraints>

<verification>
Run all of the following from `/workspace`.

The guard condition is split by status in both chokepoints — the combined `|| ... Completed` condition is gone and the `aborted`-only condition is present exactly once per file:

```
grep -c 's == TaskStatusAborted || s == TaskStatusCompleted' pkg/domain/task_frontmatter.go
grep -c 's == GoalStatusAborted || s == GoalStatusCompleted' pkg/domain/goal_frontmatter.go
```

Both must print `0`.

```
grep -c 'if s == TaskStatusAborted {' pkg/domain/task_frontmatter.go
grep -c 'if s == GoalStatusAborted {' pkg/domain/goal_frontmatter.go
```

Both must print `1`.

The completion operations no longer carry the dead "missing close-out field(s)" Try-decoration, while the `aborted`-serving set paths still do:

```
grep -c 'missing close-out field(s)' pkg/ops/complete.go
grep -c 'missing close-out field(s)' pkg/ops/goal_complete.go
```

Both must print `0`.

```
grep -c 'missing close-out field(s)' pkg/ops/frontmatter.go
grep -c 'missing close-out field(s)' pkg/ops/frontmatter_entity.go
```

Both must print `1` (the `aborted` path keeps its actionable Try-form).

The domain test blocks still match the AC evidence command and carry the new completed-acceptance arm:

```
grep -c 'Describe("SetStatus close-out guard"' pkg/domain/task_frontmatter_test.go
grep -c 'Describe("SetStatus close-out guard"' pkg/domain/goal_frontmatter_test.go
grep -c 'accepts completed without close-out fields' pkg/domain/task_frontmatter_test.go
grep -c 'accepts completed without close-out fields' pkg/domain/goal_frontmatter_test.go
```

The first two must print `1`; the last two must print `>= 1`.

The ops and integration flips landed:

```
grep -c 'task completes without close-out fields' pkg/ops/complete_test.go
grep -c 'goal completes without close-out fields' pkg/ops/goal_complete_test.go
grep -c 'all checkboxes checked without close-out fields' pkg/ops/update_test.go
grep -c 'completes without close-out fields' integration/cli_test.go
grep -c 'completes via checkbox sync without close-out fields' integration/cli_test.go
grep -c 'accepts goal complete without close-out fields' integration/cli_test.go
```

Each must print `>= 1`.

The aborted regression lock is intact:

```
grep -c 'rejects aborted without aborted_reason and gate_successor and leaves the frontmatter unchanged' pkg/domain/task_frontmatter_test.go
grep -c 'rejects aborted when only gate_successor is present' pkg/domain/goal_frontmatter_test.go
grep -c 'accepts aborted when aborted_reason and gate_successor are both present' pkg/domain/task_frontmatter_test.go
```

Each must print `>= 1`.

The AC evidence command exits 0:

```
go test -mod=mod -count=1 ./pkg/domain/ -run "SetStatus close-out" -v
```

Must exit 0 and show the new completed-acceptance specs running.

The packages are green:

```
go test -mod=mod -count=1 ./pkg/domain/... ./pkg/ops/... ./integration/...
```

Must exit 0.

Finally, run the full gate once:

```
make precommit
```

Must exit 0. A non-zero exit code means `"status":"failed"` — no exceptions.
</verification>
