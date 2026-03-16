---
status: completed
summary: Regenerated per-domain counterfeiter mocks (TaskStorage, GoalStorage, DailyNoteStorage, PageStorage, DecisionStorage) and updated all ops test files to use narrow per-domain mock types instead of monolithic Storage mock
container: vault-cli-059-generate-mocks-update-tests
dark-factory-version: v0.54.0
created: "2026-03-16T11:02:00Z"
queued: "2026-03-16T12:09:47Z"
started: "2026-03-16T12:25:00Z"
completed: "2026-03-16T12:36:24Z"
---

<summary>
- One mock per domain replaces the single all-methods mock
- All ops tests switch to domain-specific mock variables
- Tests for cross-domain operations get one mock per domain they use
- Test logic and assertions are unchanged -- only mock types and variable names change
- The mocks directory is cleaned and regenerated from scratch
</summary>

<objective>
Regenerate all counterfeiter mocks from the new per-domain `//counterfeiter:generate` directives, then update all ops test files to use narrow per-domain mocks instead of the monolithic `mocks.Storage`.
</objective>

<context>
DEPENDENCY: This prompt requires prompts 1 and 2 to have completed successfully. Prompt 1 created per-domain interfaces with `//counterfeiter:generate` directives. Prompt 2 changed ops constructor signatures to accept narrow interfaces.

Read CLAUDE.md for project conventions.
Read `pkg/storage/storage.go` -- the `//counterfeiter:generate` directives for each narrow interface and the composed Storage interface.
Read `mocks/storage.go` -- current monolithic mock, will be regenerated.
Read all ops test files to understand mock usage patterns:
- `pkg/ops/complete_test.go` -- uses `mocks.Storage` for FindTaskByName, WriteTask, FindGoalByName, WriteGoal, ReadDailyNote, WriteDailyNote
- `pkg/ops/defer_test.go` -- uses `mocks.Storage` for FindTaskByName, WriteTask, ReadDailyNote, WriteDailyNote
- `pkg/ops/update_test.go` -- uses `mocks.Storage` for FindTaskByName, WriteTask, FindGoalByName, WriteGoal
- `pkg/ops/workon_test.go` -- uses `mocks.Storage` for FindTaskByName, WriteTask, ReadDailyNote, WriteDailyNote
- `pkg/ops/list_test.go` -- uses `mocks.Storage` for ListPages
- `pkg/ops/frontmatter_test.go` -- uses `mocks.Storage` for FindTaskByName, WriteTask
- `pkg/ops/show_test.go` -- uses `mocks.Storage` for FindTaskByName
- `pkg/ops/decision_list_test.go` -- uses `mocks.Storage` for ListDecisions
- `pkg/ops/decision_ack_test.go` -- uses `mocks.Storage` for FindDecisionByName, WriteDecision
Read `docs/development-patterns.md` -- "Mocks" section.
</context>

<requirements>
1. Run `go generate ./...` to regenerate all mocks. This will:
   - Create `mocks/task-storage.go` with `TaskStorage` fake
   - Create `mocks/goal-storage.go` with `GoalStorage` fake
   - Create `mocks/daily-note-storage.go` with `DailyNoteStorage` fake
   - Create `mocks/page-storage.go` with `PageStorage` fake
   - Create `mocks/decision-storage.go` with `DecisionStorage` fake
   - Regenerate `mocks/storage.go` with the composed `Storage` fake (now embeds all narrow interface methods)

   NOTE: Before running `go generate`, first run `rm -rf mocks` to clear stale mock files. Then run `go generate ./...`.

2. Update `pkg/ops/complete_test.go`:
   - Replace `mockStorage *mocks.Storage` with three separate mocks:

   ```go
   mockTaskStorage      *mocks.TaskStorage
   mockGoalStorage      *mocks.GoalStorage
   mockDailyNoteStorage *mocks.DailyNoteStorage
   ```

   - In BeforeEach:

   ```go
   mockTaskStorage = &mocks.TaskStorage{}
   mockGoalStorage = &mocks.GoalStorage{}
   mockDailyNoteStorage = &mocks.DailyNoteStorage{}
   completeOp = ops.NewCompleteOperation(mockTaskStorage, mockGoalStorage, mockDailyNoteStorage, currentDateTime)
   ```

   - Replace all `mockStorage.FindTaskByNameReturns(...)` with `mockTaskStorage.FindTaskByNameReturns(...)`
   - Replace all `mockStorage.WriteTaskReturns(...)` with `mockTaskStorage.WriteTaskReturns(...)`
   - Replace all `mockStorage.FindGoalByNameReturns(...)` with `mockGoalStorage.FindGoalByNameReturns(...)`
   - Replace all `mockStorage.WriteGoalReturns(...)` with `mockGoalStorage.WriteGoalReturns(...)`
   - Replace all `mockStorage.ReadDailyNoteReturns(...)` with `mockDailyNoteStorage.ReadDailyNoteReturns(...)`
   - Replace all `mockStorage.WriteDailyNoteReturns(...)` with `mockDailyNoteStorage.WriteDailyNoteReturns(...)`
   - Replace all call count and args assertions similarly (e.g., `mockStorage.FindTaskByNameCallCount()` -> `mockTaskStorage.FindTaskByNameCallCount()`)

3. Update `pkg/ops/defer_test.go`:
   - Two mocks: `mockTaskStorage *mocks.TaskStorage`, `mockDailyNoteStorage *mocks.DailyNoteStorage`
   - Constructor: `ops.NewDeferOperation(mockTaskStorage, mockDailyNoteStorage, currentDateTime)`
   - Replace all `mockStorage.` references with the appropriate narrow mock

4. Update `pkg/ops/update_test.go`:
   - Two mocks: `mockTaskStorage *mocks.TaskStorage`, `mockGoalStorage *mocks.GoalStorage`
   - Constructor: `ops.NewUpdateOperation(mockTaskStorage, mockGoalStorage)`
   - Replace all `mockStorage.` references accordingly

5. Update `pkg/ops/workon_test.go`:
   - Two mocks: `mockTaskStorage *mocks.TaskStorage`, `mockDailyNoteStorage *mocks.DailyNoteStorage`
   - Constructor: `ops.NewWorkOnOperation(mockTaskStorage, mockDailyNoteStorage, currentDateTime, starter, resumer)`
   - Replace all `mockStorage.` references accordingly

6. Update `pkg/ops/list_test.go`:
   - One mock: `mockPageStorage *mocks.PageStorage`
   - Constructor: `ops.NewListOperation(mockPageStorage)`
   - Replace `mockStorage.ListPagesReturns(...)` -> `mockPageStorage.ListPagesReturns(...)`

7. Update `pkg/ops/frontmatter_test.go`:
   - One mock: `mockTaskStorage *mocks.TaskStorage`
   - Constructor: `ops.NewFrontmatterGetOperation(mockTaskStorage)`, same for Set and Clear
   - Replace all `mockStorage.` references accordingly

8. Update `pkg/ops/show_test.go`:
   - One mock: `mockTaskStorage *mocks.TaskStorage`
   - Constructor: `ops.NewShowOperation(mockTaskStorage)`
   - Replace all `mockStorage.` references accordingly

9. Update `pkg/ops/decision_list_test.go`:
   - One mock: `mockDecisionStorage *mocks.DecisionStorage`
   - Constructor: `ops.NewDecisionListOperation(mockDecisionStorage)`
   - Replace `mockStorage.ListDecisionsReturns(...)` -> `mockDecisionStorage.ListDecisionsReturns(...)`

10. Update `pkg/ops/decision_ack_test.go`:
    - One mock: `mockDecisionStorage *mocks.DecisionStorage`
    - Constructor: `ops.NewDecisionAckOperation(mockDecisionStorage, currentDateTime)`
    - Replace `mockStorage.FindDecisionByNameReturns(...)` -> `mockDecisionStorage.FindDecisionByNameReturns(...)`, etc.
</requirements>

<constraints>
- Each test file must import the specific mock type it uses from `github.com/bborbe/vault-cli/mocks`
- Mock type names match the counterfeiter --fake-name: `TaskStorage`, `GoalStorage`, `DailyNoteStorage`, `PageStorage`, `DecisionStorage`
- The composed `mocks.Storage` mock still exists (for any integration tests or CLI tests that use it) -- do NOT delete it
- Test assertions on call counts and arguments must be updated to use the new mock variable names
- Do NOT change any test logic, assertions, or test structure -- only the mock types and variable names change
- Do NOT change any ops implementation files -- those were updated in prompt 2
- Do NOT commit -- dark-factory handles git
- All tests must pass after these changes
- License headers on test files must be preserved
</constraints>

<verification>
Run `make precommit` -- must pass.

Specifically verify:
- `go generate ./...` succeeds and creates all expected mock files in `mocks/`
- `go build ./...` compiles
- `go test ./pkg/ops/...` passes with all tests using narrow mocks
- `go test ./pkg/storage/...` passes
- `go vet ./...` passes
</verification>
