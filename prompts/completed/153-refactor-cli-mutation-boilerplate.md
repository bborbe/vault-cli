---
status: completed
summary: Extracted runMutation helper in pkg/cli/cli.go and refactored 7 mutation commands (complete, defer, update, goal-complete, goal-defer, objective-complete, decision-ack) to use it, eliminating ~100 lines of duplicated boilerplate and nolint:dupl suppressions
execution_id: vault-cli-exec-153-refactor-cli-mutation-boilerplate
dark-factory-version: v0.188.1
created: "2026-06-28T22:00:00Z"
queued: "2026-06-28T20:59:31Z"
started: "2026-06-28T21:18:51Z"
completed: "2026-06-28T21:24:48Z"
---

<summary>
- `cli.go` has 15 `//nolint:dupl` suppressions — mutation commands (complete, defer, update) repeat the same vault-iteration + output-formatting boilerplate
- Each ~50-line RunE block does: loop vaults → create dispatcher → call FirstSuccess → create stores → create op → execute → format JSON/plain output → handle warnings
- Extract a `runMutation` helper accepting an `op func(vault) (result, error)` to collapse the pattern
- Entity commands already use a factory pattern — mutation commands should too
- Cuts ~120 lines of duplication
</summary>

<objective>
Reduce duplication in `pkg/cli/cli.go` by extracting a shared `runMutation` helper for the complete/defer/update command pattern, eliminating the repeated vault-iteration + output-formatting boilerplate.
</objective>

<context>
Read:
- `pkg/cli/cli.go` — especially:
  - `createCompleteCommand` at line 112 (~55 lines) — vault loop → dispatcher → store creation → op execution → output formatting
  - `createDeferCommand` at line 167 (~65 lines) — same pattern
  - `createUpdateCommand` at line 233 (~45 lines) — same pattern, simpler output
  - `createGoalCompleteCommand` at line 1216 (~53 lines) — same pattern with `force` flag
  - `createGoalDeferCommand` at line 1270 (~60 lines) — same pattern
  - `createObjectiveCompleteCommand` at line 1486 (~40 lines) — same pattern
  - `createDecisionAckCommand` at line 1670 (~50 lines) — same pattern with `statusOverride` flag
  - Entity commands at line 788-1093 already use a factory pattern (`newXOp func(cfg)`) — good template
- `pkg/ops/complete.go` — `MutationResult` struct at line 54
</context>

<requirements>
1. **Extract a generic `runMutation` function** in `pkg/cli/cli.go`:
   ```go
   type mutationRunner func(ctx, vault *config.Vault) (ops.MutationResult, error)
   
   func runMutation(
       ctx context.Context,
       vaults []*config.Vault,
       outputFormat string,
       runner mutationRunner,
   ) error {
       dispatcher := ops.NewVaultDispatcher()
       return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
           result, err := runner(ctx, vault)
           if err != nil {
               if outputFormat == OutputFormatJSON {
                   _ = PrintJSON(result)
               }
               return err
           }
           if outputFormat == OutputFormatJSON {
               return PrintJSON(result)
           }
           return nil
       })
   }
   ```

2. **Refactor `createCompleteCommand`**: replace the vault-iteration + dispatcher + formatting body with a call to `runMutation` passing a closure that creates stores and calls the operation.

3. **Refactor `createDeferCommand`**: same pattern.

4. **Refactor `createUpdateCommand`**: same pattern — though its output is simpler (just warnings vs success message), the closure handles that.

5. **Refactor `createGoalCompleteCommand`** (~line 1216): same pattern but includes the `force` flag — capture it in the closure.

6. **Refactor `createGoalDeferCommand`** (~line 1270): same pattern.

7. **Refactor `createObjectiveCompleteCommand`** (~line 1486): same pattern.

8. **Refactor `createDecisionAckCommand`** (~line 1670): same pattern with `statusOverride` flag — capture in the closure.

9. **KISS**: For commands with extra flags (force, statusOverride), capture them in the closure. Do NOT try to parameterize output formatting at the `runMutation` level — let each closure handle its own warning/success output after `PrintJSON` check.

10. **Do NOT refactor** the entity commands (get/set/clear/add/remove/show) — they already use good factory patterns. Do NOT refactor `createWorkOnCommand` — it has a fundamentally different output pattern (formatWorkOnResult with session output).

11. **Remove `//nolint:dupl` annotations** on refactored commands if the duplication is eliminated.

12. **Existing tests must still pass** — run `make precommit`
</requirements>

<constraints>
- `runMutation` is a simple extraction — no new interfaces, no new files
- The helper does NOT handle `fmt.Printf` for success/warning messages — each closure does that AFTER `runMutation` returns (or inside it with access to the result)
- Keep the existing `dispatcher.FirstSuccess` pattern for multi-vault first-match semantics
- No behavior change — same vault iteration, same output formatting, just less code
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
