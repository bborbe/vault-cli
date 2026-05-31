---
status: completed
summary: Add ClearClaudeSessionID() to TaskFrontmatter and call it in handleRecurringTask so recurring task completion clears the session ID key
container: vault-cli-clear-session-exec-136-recurring-task-clear-claude-session-id
dark-factory-version: v0.173.0
created: "2026-05-31T11:58:47Z"
queued: "2026-05-31T11:58:47Z"
started: "2026-05-31T11:58:49Z"
completed: "2026-05-31T12:00:56Z"
branch: dark-factory/recurring-task-clears-claude-session-id
---

<summary>
- Spec 015 bug fix: recurring task completion currently inherits the previous run's `claude_session_id`
- Add `ClearClaudeSessionID()` method to `TaskFrontmatter` (delegates to `FrontmatterMap.Delete`)
- Document the method with a GoDoc comment placed near the existing `SetClaudeSessionID` setter
- Call `ClearClaudeSessionID()` in `handleRecurringTask` before `WriteTask` so the next occurrence has no session ID
- Add a Ginkgo test under the existing "recurring daily task" context that asserts the key is ABSENT from the written task (not merely empty-string)
- Test assertion must use `Get("claude_session_id")` returning `nil` — `ClaudeSessionID() == ""` does NOT distinguish absent from empty
- Non-recurring task completion is unaffected — explicitly verified by leaving existing non-recurring tests untouched and passing
</summary>

<objective>
When `vault-cli task complete` runs on a recurring task, the rewritten file must not contain a `claude_session_id` key. The next `workon` on that task sets a fresh ID. Non-recurring tasks are unchanged.
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Read `pkg/domain/task_frontmatter.go` for these snippets:

**Set method (lines 199-200):**
```go
// SetClaudeSessionID stores the claude_session_id in the map.
func (f *TaskFrontmatter) SetClaudeSessionID(v string) { f.Set("claude_session_id", v) }
```

**Delete method — lives in `pkg/domain/frontmatter_map.go` (line 122), inherited by `TaskFrontmatter` via embedding:**
```go
// Delete removes key from the map. No-op if key is absent.
func (f *FrontmatterMap) Delete(key string) {
    delete(f.data, key)
}
```
`TaskFrontmatter` embeds `FrontmatterMap` (see `pkg/domain/task_frontmatter.go:20-22`), so `(*TaskFrontmatter).Delete(key)` works through the embedded receiver.

**ClearField method (lines 467-469):**
```go
// ClearField removes a frontmatter field by key.
// Works for both known and unknown fields.
func (f *TaskFrontmatter) ClearField(key string) {
	f.Delete(key)
}
```

Read `pkg/ops/complete.go` for `handleRecurringTask` (lines 168-230). Key insertion point is after step 4 (clearing planned_date) and before step 5 / write (line 202):
```go
// 4. If planned_date exists and < new defer_date, clear it
if task.PlannedDate() != nil && task.PlannedDate().Before(newDeferDate) {
    task.SetPlannedDate(nil)
}

// 5. Status remains as-is (do NOT set to completed)

// +++ INSERT ClearClaudeSessionID HERE +++

// Write updated task
if err := c.taskStorage.WriteTask(ctx, task); err != nil {
```

Read `pkg/ops/complete_test.go` for the "recurring daily task" context (lines 272-317) — this is where the new test should be added:
```go
Context("recurring daily task", func() {
    BeforeEach(func() {
        task.SetRecurring("daily")
        _ = task.SetStatus(domain.TaskStatusInProgress)
        task.Content = `---
status: in_progress
recurring: daily
---
# My Task

## Checklist
- [x] Item 1
- [x] Item 2
`
    })

    It("returns no error", func() {
        Expect(err).To(BeNil())
    })

    It("resets checkboxes in content", func() {
        Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
        _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
        Expect(string(writtenTask.Content)).To(ContainSubstring("- [ ] Item 1"))
        Expect(string(writtenTask.Content)).To(ContainSubstring("- [ ] Item 2"))
        Expect(string(writtenTask.Content)).NotTo(ContainSubstring("- [x]"))
    })
    // ... more assertions
})
```

The test must set `task.SetClaudeSessionID("some-uuid")` in BeforeEach and assert that the key is **absent** from the written task — not merely empty. Use `Expect(writtenTask.Get("claude_session_id")).To(BeNil())` because `Get` returns `nil` for absent keys and the actual value (including `""`) for present keys. `BeEmpty()` on `ClaudeSessionID()` would NOT distinguish "key deleted" from "key set to empty string" — a lazy `SetClaudeSessionID("")` would pass it.
</context>

<requirements>
1. Add `ClearClaudeSessionID()` to `pkg/domain/task_frontmatter.go`:
   ```go
   // ClearClaudeSessionID removes the claude_session_id key from the map.
   func (f *TaskFrontmatter) ClearClaudeSessionID() {
       f.Delete("claude_session_id")
   }
   ```
   Place it near `SetClaudeSessionID` (after line 200).

2. In `pkg/ops/complete.go`, add `task.ClearClaudeSessionID()` in `handleRecurringTask` after the planned_date clearing block (after line 197) and before the write call (line 202).
   ```go
   // 4. If planned_date exists and < new defer_date, clear it
   if task.PlannedDate() != nil && task.PlannedDate().Before(newDeferDate) {
       task.SetPlannedDate(nil)
   }

   // 5. Clear claude_session_id so next occurrence starts fresh
   task.ClearClaudeSessionID()

   // 6. Status remains as-is (do NOT set to completed)

   // Write updated task
   ```
   Update the comment numbering to reflect the new step 5.

3. In `pkg/ops/complete_test.go`, add a test under the existing "recurring daily task" context. Before the `JustBeforeEach` that calls `Execute`, set a session ID on the task:
   ```go
   Context("recurring daily task", func() {
       BeforeEach(func() {
           task.SetRecurring("daily")
           _ = task.SetStatus(domain.TaskStatusInProgress)
           task.SetClaudeSessionID("test-session-uuid")
           task.Content = `---
   status: in_progress
   recurring: daily
   ---
   # My Task

   ## Checklist
   - [x] Item 1
   - [x] Item 2
   `
       })
       // existing tests...

       It("clears claude_session_id after completion", func() {
           Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(1))
           _, writtenTask := mockTaskStorage.WriteTaskArgsForCall(0)
           // Assert key is ABSENT (Get returns nil), not merely empty-string —
           // this kills the lazy `SetClaudeSessionID("")` shortcut.
           Expect(writtenTask.Get("claude_session_id")).To(BeNil())
       })
   })
   ```

4. Verify `make test` passes for the ops package.

5. Verify `make precommit` exits 0.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- The clearing must remove the key (not set it to empty string) — routes through `Delete`, not `Set`
- Only recurring task completion clears the field; non-recurring is unchanged
</constraints>

<verification>
Run: `make test`
Run: `go test ./pkg/domain/ -run TaskFrontmatter -v`
Run: `go test ./pkg/ops/ -run TestComplete -v`
Run: `make precommit`
</verification>

<open_questions>
None — the spec fully specifies the fix location, the method signature, and the test placement.
</open_questions>