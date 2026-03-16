---
status: completed
summary: Updated all ops constructors and structs to accept narrow per-domain storage interfaces instead of monolithic Storage, with temporary multi-arg wiring in cli.go and test files.
container: vault-cli-058-narrow-ops-interfaces
dark-factory-version: v0.54.0
created: "2026-03-16T11:01:00Z"
queued: "2026-03-16T12:09:47Z"
started: "2026-03-16T12:16:21Z"
completed: "2026-03-16T12:24:56Z"
---

<summary>
- Each operation declares only the storage capabilities it actually uses instead of the full interface
- Operations needing multiple domains (complete, defer, update, workon) accept one parameter per domain
- Single-domain operations (list, show, frontmatter, decisions) accept one narrow interface
- All constructors and struct fields change; method bodies update field references accordingly
- CLI wiring temporarily passes the composed store for each parameter to keep compilation working
- No behavioral changes -- all operations work identically
</summary>

<objective>
Update all ops constructors and structs to accept narrow per-domain storage interfaces instead of the monolithic `storage.Storage`, reducing coupling and enabling focused mocking in tests.
</objective>

<context>
DEPENDENCY: This prompt requires prompt 1 (split-storage-interfaces-and-files) to have completed successfully. The per-domain interfaces referenced below are created by prompt 1.

Read CLAUDE.md for project conventions.
Read `pkg/storage/storage.go` -- the per-domain interfaces: `TaskStorage`, `GoalStorage`, `DailyNoteStorage`, `PageStorage`, `DecisionStorage`.
Read each ops file to understand which storage methods each operation actually calls:
- `pkg/ops/complete.go` -- calls FindTaskByName, WriteTask, FindGoalByName, WriteGoal, ReadDailyNote, WriteDailyNote
- `pkg/ops/defer.go` -- calls FindTaskByName, WriteTask, ReadDailyNote, WriteDailyNote
- `pkg/ops/update.go` -- calls FindTaskByName, WriteTask, FindGoalByName, WriteGoal
- `pkg/ops/workon.go` -- calls FindTaskByName, WriteTask, ReadDailyNote, WriteDailyNote
- `pkg/ops/list.go` -- calls ListPages
- `pkg/ops/frontmatter.go` -- calls FindTaskByName, WriteTask (all three operations)
- `pkg/ops/show.go` -- calls FindTaskByName
- `pkg/ops/decision_list.go` -- calls ListDecisions
- `pkg/ops/decision_ack.go` -- calls FindDecisionByName, WriteDecision
Read `pkg/cli/cli.go` -- understand how ops are constructed (will be updated in prompt 4).
Read `docs/development-patterns.md` -- naming conventions.
</context>

<requirements>
1. Update `pkg/ops/complete.go`:
   - Change the struct fields and constructor to accept narrow interfaces:

   ```go
   // Old:
   func NewCompleteOperation(
       storage storage.Storage,
       currentDateTime libtime.CurrentDateTime,
   ) CompleteOperation

   // New:
   func NewCompleteOperation(
       taskStorage storage.TaskStorage,
       goalStorage storage.GoalStorage,
       dailyNoteStorage storage.DailyNoteStorage,
       currentDateTime libtime.CurrentDateTime,
   ) CompleteOperation
   ```

   - Update the `completeOperation` struct:

   ```go
   type completeOperation struct {
       taskStorage      storage.TaskStorage
       goalStorage      storage.GoalStorage
       dailyNoteStorage storage.DailyNoteStorage
       currentDateTime  libtime.CurrentDateTime
   }
   ```

   - Update all method bodies: replace `c.storage.FindTaskByName` with `c.taskStorage.FindTaskByName`, `c.storage.WriteTask` with `c.taskStorage.WriteTask`, `c.storage.FindGoalByName` with `c.goalStorage.FindGoalByName`, `c.storage.WriteGoal` with `c.goalStorage.WriteGoal`, `c.storage.ReadDailyNote` with `c.dailyNoteStorage.ReadDailyNote`, `c.storage.WriteDailyNote` with `c.dailyNoteStorage.WriteDailyNote`

2. Update `pkg/ops/defer.go`:
   - Constructor: `NewDeferOperation(taskStorage storage.TaskStorage, dailyNoteStorage storage.DailyNoteStorage, currentDateTime libtime.CurrentDateTime) DeferOperation`
   - Struct fields: `taskStorage storage.TaskStorage`, `dailyNoteStorage storage.DailyNoteStorage`, `currentDateTime libtime.CurrentDateTime`
   - Replace `d.storage.FindTaskByName` -> `d.taskStorage.FindTaskByName`, `d.storage.WriteTask` -> `d.taskStorage.WriteTask`, `d.storage.ReadDailyNote` -> `d.dailyNoteStorage.ReadDailyNote`, `d.storage.WriteDailyNote` -> `d.dailyNoteStorage.WriteDailyNote`

3. Update `pkg/ops/update.go`:
   - Constructor: `NewUpdateOperation(taskStorage storage.TaskStorage, goalStorage storage.GoalStorage) UpdateOperation`
   - Struct fields: `taskStorage storage.TaskStorage`, `goalStorage storage.GoalStorage`
   - Replace `u.storage.FindTaskByName` -> `u.taskStorage.FindTaskByName`, `u.storage.WriteTask` -> `u.taskStorage.WriteTask`, `u.storage.FindGoalByName` -> `u.goalStorage.FindGoalByName`, `u.storage.WriteGoal` -> `u.goalStorage.WriteGoal`

4. Update `pkg/ops/workon.go`:
   - Constructor: `NewWorkOnOperation(taskStorage storage.TaskStorage, dailyNoteStorage storage.DailyNoteStorage, currentDateTime libtime.CurrentDateTime, starter ClaudeSessionStarter, resumer ClaudeResumer) WorkOnOperation`
   - Struct fields: `taskStorage storage.TaskStorage`, `dailyNoteStorage storage.DailyNoteStorage`, `currentDateTime libtime.CurrentDateTime`, `starter ClaudeSessionStarter`, `resumer ClaudeResumer`
   - Replace all `w.storage.` calls with the appropriate narrow storage: `w.taskStorage.FindTaskByName`, `w.taskStorage.WriteTask`, `w.dailyNoteStorage.ReadDailyNote`, `w.dailyNoteStorage.WriteDailyNote`. NOTE: `workon.go` has a `handleClaudeSession` method that also calls `w.storage.WriteTask` -- update that too.

5. Update `pkg/ops/list.go`:
   - Constructor: `NewListOperation(pageStorage storage.PageStorage) ListOperation`
   - Struct field: `pageStorage storage.PageStorage`
   - Replace `l.storage.ListPages` -> `l.pageStorage.ListPages`

6. Update `pkg/ops/frontmatter.go` -- all three operations:
   - `NewFrontmatterGetOperation(taskStorage storage.TaskStorage) FrontmatterGetOperation`
   - `NewFrontmatterSetOperation(taskStorage storage.TaskStorage) FrontmatterSetOperation`
   - `NewFrontmatterClearOperation(taskStorage storage.TaskStorage) FrontmatterClearOperation`
   - Each struct gets `taskStorage storage.TaskStorage` field
   - Replace `o.storage.FindTaskByName` -> `o.taskStorage.FindTaskByName`, `o.storage.WriteTask` -> `o.taskStorage.WriteTask`

7. Update `pkg/ops/show.go`:
   - Constructor: `NewShowOperation(taskStorage storage.TaskStorage) ShowOperation`
   - Struct field: `taskStorage storage.TaskStorage`
   - Replace `o.storage.FindTaskByName` -> `o.taskStorage.FindTaskByName`

8. Update `pkg/ops/decision_list.go`:
   - Constructor: `NewDecisionListOperation(decisionStorage storage.DecisionStorage) DecisionListOperation`
   - Struct field: `decisionStorage storage.DecisionStorage`
   - Replace `d.storage.ListDecisions` -> `d.decisionStorage.ListDecisions`

9. Update `pkg/ops/decision_ack.go`:
   - Constructor: `NewDecisionAckOperation(decisionStorage storage.DecisionStorage, currentDateTime libtime.CurrentDateTime) DecisionAckOperation`
   - Struct field: `decisionStorage storage.DecisionStorage`
   - Replace `d.storage.FindDecisionByName` -> `d.decisionStorage.FindDecisionByName`, `d.storage.WriteDecision` -> `d.decisionStorage.WriteDecision`

10. IMPORTANT: Do NOT update `pkg/cli/cli.go` yet -- it will fail to compile temporarily. That's OK because prompt 3 (mocks) and prompt 4 (CLI wiring) follow immediately. To keep compilation working during this prompt, temporarily update `pkg/cli/cli.go` callers to pass the same `store` variable for each narrow interface parameter. For example:

    ```go
    // Old:
    completeOp := ops.NewCompleteOperation(store, currentDateTime)

    // Temporary (passes store which satisfies all interfaces via embedding):
    completeOp := ops.NewCompleteOperation(store, store, store, currentDateTime)
    ```

    This works because `store` is of type `storage.Storage` which embeds all narrow interfaces. Apply this pattern to ALL constructor calls in `pkg/cli/cli.go`.
</requirements>

<constraints>
- Do NOT change the `Execute` method signatures on any operation -- only constructors and struct internals change
- Do NOT change any test files in this prompt -- tests are updated in prompt 3
- The `import "github.com/bborbe/vault-cli/pkg/storage"` stays in all ops files -- the narrow interfaces are in the storage package
- Every ops file that currently imports `storage` must still import it (the types just change from `storage.Storage` to `storage.TaskStorage` etc.)
- The `MutationResult` and other result types in complete.go stay unchanged
- Do NOT commit -- dark-factory handles git
- Existing tests must still pass (they construct ops with `mocks.Storage` which still satisfies the narrow interfaces since it implements all methods)
</constraints>

<verification>
Run `make precommit` -- must pass.

Specifically verify:
- `go build ./...` compiles (cli.go uses the temporary `store, store, store` pattern)
- `go test ./pkg/ops/...` passes (tests use `mocks.Storage` which satisfies all narrow interfaces)
- `go vet ./...` passes
</verification>
