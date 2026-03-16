---
status: approved
created: "2026-03-16T15:19:37Z"
queued: "2026-03-16T15:19:37Z"
---

<summary>
- Extract duplicated multi-vault try-each-until-success loop into a reusable dispatcher
- Replace all 10 duplicated vault loops in CLI commands with dispatcher calls
- Add tests for the dispatcher covering empty, single, and multi-vault scenarios
</summary>

<objective>
Extract the duplicated multi-vault first-success dispatch loop from pkg/cli/cli.go into a reusable VaultDispatcher in pkg/ops/. The same try-each-vault pattern is copy-pasted across 10 command functions, violating DRY and making the dispatch strategy untestable.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — the full file. Search for `var lastErr error` to find all 10 vault-loop instances
- `pkg/ops/complete.go` — example of how ops are constructed and executed
- `pkg/config/config.go` — `Vault` struct definition
- `pkg/storage/storage.go` — `NewConfigFromVault` and storage constructors

The duplicated pattern appears in these functions (search for `var lastErr error`):
1. `createCompleteCommand` (~line 154)
2. `createDeferCommand` (~line 229)
3. `createUpdateCommand` (~line 276)
4. `createWorkOnCommand` (~line 326)
5. `createDecisionAckCommand` (~line 851)
6. `createTaskGetCommand` (~line 1013)
7. `createTaskSetCommand` (~line 1100)
8. `createTaskClearCommand` (~line 1185)
9. `createTaskShowCommand` (~line 1245)
10. `createValidateCommand` — has the loop with slightly different error message

Each loop follows this pattern:
```go
if len(vaults) == 1 {
    vault := vaults[0]
    // construct storage + op
    return op.Execute(ctx, vault.Path, ...)
}
var lastErr error
for _, vault := range vaults {
    // construct storage + op
    if err := op.Execute(ctx, vault.Path, ...); err == nil {
        return nil
    }
    lastErr = err
}
return fmt.Errorf("task not found in any vault: %w", lastErr)
```
</context>

<constraints>
- Create `pkg/ops/vault_dispatcher.go` with `VaultDispatcher` type
- The dispatcher takes a callback `func(vault *config.Vault) error` — each command provides its own closure
- Single-vault optimization: if only 1 vault, call directly (no loop, better error messages)
- Use `github.com/bborbe/errors` for error wrapping
- Add counterfeiter directive for the interface
- Add tests in `pkg/ops/vault_dispatcher_test.go` using Ginkgo/Gomega
- Do NOT change any command's functional behavior — only extract the loop
- Keep the RunE closures thin: get vaults, build closure, call dispatcher
- Test coverage for new package must be at least 80%
</constraints>

<requirements>

## 1. Create `pkg/ops/vault_dispatcher.go`

New file with:
- Interface `VaultDispatcher` with `FirstSuccess(ctx, vaults, fn) error`
- Constructor `NewVaultDispatcher() VaultDispatcher`
- Implementation: empty vaults → error, single vault → direct call, multi vault → loop with first-success

## 2. Add tests in `pkg/ops/vault_dispatcher_test.go`

Test cases using Ginkgo/Gomega:
- No vaults → returns error
- Single vault, fn succeeds → returns nil
- Single vault, fn fails → returns the error directly (not wrapped with "not found in any vault")
- Multiple vaults, first succeeds → returns nil, only first vault's fn called
- Multiple vaults, second succeeds → returns nil
- Multiple vaults, all fail → returns wrapped last error containing "not found in any vault"

## 3. Update `pkg/cli/cli.go` — Replace all 10 vault loops

For each function listed in context, replace the if-single/loop pattern with:

```go
dispatcher := ops.NewVaultDispatcher()
return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
    storageConfig := storage.NewConfigFromVault(vault)
    // construct op with storage from vault
    return op.Execute(ctx, vault.Path, ...)
})
```

Note: some commands use "decision not found in any vault" vs "task not found in any vault" — the dispatcher should use a generic message like "not found in any vault". If the exact error message matters for tests, verify no test assertions break.

## 4. Generate mock

Add counterfeiter generate directive and run `go generate ./pkg/ops/...` to create the mock.

</requirements>

<verification>
make precommit
</verification>
