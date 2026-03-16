---
status: queued
---

<summary>
- Extract multi-vault first-success dispatch loop from cli.go into `pkg/ops/vault_dispatcher.go`
- Create `VaultDispatcher` with `FirstSuccess` method
- Replace duplicated loops in 7+ `create*Command` functions with `VaultDispatcher.FirstSuccess`
- Add tests for VaultDispatcher
</summary>

<objective>
Extract the duplicated multi-vault try-each-until-success loop from `pkg/cli/cli.go` into a reusable `VaultDispatcher` in `pkg/ops/`. The same pattern is copy-pasted across createCompleteCommand, createDeferCommand, createUpdateCommand, createWorkOnCommand, createTaskGetCommand, createTaskSetCommand, createTaskClearCommand, createTaskShowCommand, and createValidateCommand.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — the full file, focus on the duplicated vault-loop pattern in RunE closures
- `pkg/ops/complete.go` — example of how ops are constructed and executed
- `pkg/config/config.go` — `Vault` struct definition
- `pkg/storage/storage.go` — `NewConfigFromVault` and storage constructors

The duplicated pattern (appears 7+ times):
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

<rules>
- Create `pkg/ops/vault_dispatcher.go` with `VaultDispatcher` type
- The dispatcher takes a callback `func(vault *config.Vault) error` — each command provides its own closure
- Single-vault optimization: if only 1 vault, call directly (no loop overhead, better error messages)
- Use `github.com/bborbe/errors` for error wrapping
- Add counterfeiter directive for the interface
- Add tests in `pkg/ops/vault_dispatcher_test.go` using Ginkgo/Gomega
- Do NOT change any command's functional behavior — only extract the loop
- Keep the `RunE` closures thin: get vaults, build closure, call dispatcher
</rules>

<changes>

## 1. Create `pkg/ops/vault_dispatcher.go`

```go
package ops

import (
    "context"
    "fmt"

    "github.com/bborbe/errors"

    "github.com/bborbe/vault-cli/pkg/config"
)

//counterfeiter:generate -o ../../mocks/vault-dispatcher.go --fake-name VaultDispatcher . VaultDispatcher

// VaultDispatcher tries an operation across multiple vaults, returning on first success.
type VaultDispatcher interface {
    FirstSuccess(ctx context.Context, vaults []*config.Vault, fn func(*config.Vault) error) error
}

// NewVaultDispatcher creates a new VaultDispatcher.
func NewVaultDispatcher() VaultDispatcher {
    return &vaultDispatcher{}
}

type vaultDispatcher struct{}

func (d *vaultDispatcher) FirstSuccess(
    ctx context.Context,
    vaults []*config.Vault,
    fn func(*config.Vault) error,
) error {
    if len(vaults) == 0 {
        return errors.Errorf(ctx, "no vaults configured")
    }

    if len(vaults) == 1 {
        return fn(vaults[0])
    }

    var lastErr error
    for _, vault := range vaults {
        if err := fn(vault); err == nil {
            return nil
        } else {
            lastErr = err
        }
    }
    return errors.Wrap(ctx, lastErr, "not found in any vault")
}
```

## 2. Add tests in `pkg/ops/vault_dispatcher_test.go`

Test cases:
- No vaults → returns error
- Single vault, success → returns nil
- Single vault, failure → returns the error directly (not wrapped with "not found in any vault")
- Multiple vaults, first succeeds → returns nil, only first called
- Multiple vaults, second succeeds → returns nil
- Multiple vaults, all fail → returns wrapped last error

## 3. Update `pkg/cli/cli.go` — Replace all vault loops

For each `create*Command` that has the duplicated pattern, replace with:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    taskName := args[0]
    vaults, err := getVaults(ctx, configLoader, vaultName)
    if err != nil {
        return errors.Wrap(ctx, err, "get vaults")
    }
    currentDateTime := libtime.NewCurrentDateTime()
    dispatcher := ops.NewVaultDispatcher()
    return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
        storageConfig := storage.NewConfigFromVault(vault)
        completeOp := ops.NewCompleteOperation(
            storage.NewTaskStorage(storageConfig),
            storage.NewGoalStorage(storageConfig),
            storage.NewDailyNoteStorage(storageConfig),
            currentDateTime,
        )
        return completeOp.Execute(ctx, vault.Path, taskName, vault.Name, *outputFormat)
    })
},
```

Apply this pattern to ALL commands that currently have the multi-vault loop.

## 4. Generate mock

Run `go generate ./pkg/ops/...` to generate the VaultDispatcher mock.

</changes>

<verification>
make precommit
</verification>
