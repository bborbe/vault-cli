---
status: approved
created: "2026-03-16T11:03:00Z"
queued: "2026-03-16T12:09:47Z"
---

<summary>
- Each CLI command creates only the storage instances it needs instead of a full-access storage object
- The temporary repeated-store pattern from prompt 2 is replaced with clean per-domain constructors
- Single-domain commands create one storage; multi-domain commands create each needed storage
- No behavioral changes -- all CLI commands work identically
</summary>

<objective>
Update `pkg/cli/cli.go` to construct per-domain storage instances and pass them as narrow interfaces to ops constructors, replacing the temporary `store, store, store` pattern with clean, explicit wiring.
</objective>

<context>
DEPENDENCY: This prompt requires prompts 1, 2, and 3 to have completed successfully. Prompt 1 created per-domain storage constructors. Prompt 2 changed ops to accept narrow interfaces. Prompt 3 regenerated mocks.

Read CLAUDE.md for project conventions.
Read `pkg/cli/cli.go` -- current wiring that uses the temporary `store, store, store` pattern from prompt 2.
Read `pkg/storage/storage.go` -- the per-domain constructors: `NewTaskStorage`, `NewGoalStorage`, `NewDailyNoteStorage`, `NewPageStorage`, `NewDecisionStorage`.
Read the ops constructor signatures (updated by prompt 2):
- `ops.NewCompleteOperation(taskStorage, goalStorage, dailyNoteStorage, currentDateTime)`
- `ops.NewDeferOperation(taskStorage, dailyNoteStorage, currentDateTime)`
- `ops.NewUpdateOperation(taskStorage, goalStorage)`
- `ops.NewWorkOnOperation(taskStorage, dailyNoteStorage, currentDateTime, starter, resumer)`
- `ops.NewListOperation(pageStorage)`
- `ops.NewFrontmatterGetOperation(taskStorage)`, Set, Clear same
- `ops.NewShowOperation(taskStorage)`
- `ops.NewDecisionListOperation(decisionStorage)`
- `ops.NewDecisionAckOperation(decisionStorage, currentDateTime)`
Read `docs/development-patterns.md` -- "Multi-Vault Pattern" section.
</context>

<requirements>
1. Update `createCompleteCommand` in `pkg/cli/cli.go`:
   - Replace the single `store := storage.NewStorage(storageConfig)` with:

   ```go
   storageConfig := storage.NewConfigFromVault(vault)
   taskStore := storage.NewTaskStorage(storageConfig)
   goalStore := storage.NewGoalStorage(storageConfig)
   dailyStore := storage.NewDailyNoteStorage(storageConfig)
   completeOp := ops.NewCompleteOperation(taskStore, goalStore, dailyStore, currentDateTime)
   ```

   - Apply this to BOTH the single-vault path and the multi-vault loop

2. Update `createDeferCommand`:
   - Create `taskStore` and `dailyStore`, pass to `ops.NewDeferOperation(taskStore, dailyStore, currentDateTime)`

3. Update `createUpdateCommand`:
   - Create `taskStore` and `goalStore`, pass to `ops.NewUpdateOperation(taskStore, goalStore)`

4. Update `createWorkOnCommand`:
   - Create `taskStore` and `dailyStore`, pass to `ops.NewWorkOnOperation(taskStore, dailyStore, currentDateTime, starter, resumer)`

5. Update `createTaskListCommand`:
   - Create `pageStore := storage.NewPageStorage(storageConfig)`, pass to `ops.NewListOperation(pageStore)`

6. Update `createGenericListCommand`:
   - Same as task list: create `pageStore`, pass to `ops.NewListOperation(pageStore)`

7. Update `createValidateCommand`:
   - Replace `store.FindTaskByName(...)` with `taskStore := storage.NewTaskStorage(storageConfig)` then `taskStore.FindTaskByName(...)`

8. Update `createTaskGetCommand`, `createTaskSetCommand`, `createTaskClearCommand`:
   - Create `taskStore := storage.NewTaskStorage(storageConfig)`, pass to the frontmatter ops

9. Update `createTaskShowCommand`:
   - Create `taskStore := storage.NewTaskStorage(storageConfig)`, pass to `ops.NewShowOperation(taskStore)`

10. Update `createDecisionListCommand`:
    - Create `decisionStore := storage.NewDecisionStorage(storageConfig)`, pass to `ops.NewDecisionListOperation(decisionStore)`

11. Update `createDecisionAckCommand`:
    - Create `decisionStore := storage.NewDecisionStorage(storageConfig)`, pass to `ops.NewDecisionAckOperation(decisionStore, currentDateTime)`

12. Remove unused import of `storage.NewStorage` if no callers remain. The `storage.Config` and `storage.NewConfigFromVault` are still used. Check if any callers of `storage.NewStorage` remain -- if the generic search/lint commands use it, those can stay using the full `Storage` or be updated to use narrow interfaces.

13. For functions that use `storage.Config` accessors (like `getDirFunc(storageConfig)` in generic commands), the `storageConfig` variable creation stays the same -- only the store construction changes.

IMPORTANT: The `createGenericSearchCommand` does NOT use storage at all (it calls `ops.NewSearchOperation()` with no storage argument). Leave it unchanged.

IMPORTANT: The `createGenericLintCommand` does NOT use storage either (it calls `ops.NewLintOperation()` with no storage argument). Leave it unchanged.
</requirements>

<constraints>
- Each vault iteration creates its own storage instances -- do NOT share storage instances across vaults
- `storageConfig := storage.NewConfigFromVault(vault)` stays as the first line in each vault processing block
- The per-domain constructor names are: `storage.NewTaskStorage`, `storage.NewGoalStorage`, `storage.NewDailyNoteStorage`, `storage.NewPageStorage`, `storage.NewDecisionStorage`
- If `storage.NewStorage` has no remaining callers in cli.go, it's OK to leave the function defined (it may be used by integration tests or external consumers) -- just remove the import/usage from cli.go if unused
- Do NOT change ops files or test files -- those were completed in prompts 2 and 3
- Do NOT commit -- dark-factory handles git
- All tests must pass
</constraints>

<verification>
Run `make precommit` -- must pass.

Specifically verify:
- `go build ./...` compiles without unused import warnings
- `go test ./...` -- full test suite passes
- `go vet ./...` passes

Manual smoke test (if binary is available):
```
vault-cli task list
vault-cli task show <any-task>
vault-cli decision list
vault-cli goal list
```
</verification>
