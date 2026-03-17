---
status: approved
spec: ["004"]
created: "2026-03-17T00:00:00Z"
queued: "2026-03-17T10:44:30Z"
branch: dark-factory/entity-complete-commands
---

<summary>
- `vault-cli goal complete "My Goal"` is now a valid command that marks a goal as completed
- `--force` flag on goal complete bypasses the open-task check
- `vault-cli objective complete "My Objective"` is now a valid command that marks an objective as completed
- Both commands dispatch across vaults using VaultDispatcher (first-success pattern)
- Both commands respect the global `--output` and `--vault` flags
- All existing goal and objective subcommands continue to work unchanged
</summary>

<objective>
Wire `goal complete` and `objective complete` subcommands into `pkg/cli/cli.go` by adding them to `createGoalCommands` and `createObjectiveCommands`. These are the final integration pieces for spec 004.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — read the ENTIRE file. Focus on:
  - `createGoalCommands` (~line 555) — add `goal complete` subcommand here
  - `createObjectiveCommands` (~line 641) — add `objective complete` subcommand here
  - `createCompleteCommand` (~line 129) — follow this pattern for vault dispatch, storage wiring, and op construction
- `pkg/ops/goal_complete.go` — `GoalCompleteOperation` interface and `NewGoalCompleteOperation` constructor (created in prompt 2)
- `pkg/ops/objective_complete.go` — `ObjectiveCompleteOperation` interface and `NewObjectiveCompleteOperation` constructor (created in prompt 2)
- `pkg/storage/storage.go` — `NewGoalStorage`, `NewTaskStorage`, `NewObjectiveStorage` constructors
</context>

<requirements>

## 1. Add `goal complete` subcommand inside `createGoalCommands`

Inside `createGoalCommands`, after the existing `cmd.AddCommand(...)` calls, add:

```go
cmd.AddCommand(createGoalCompleteCommand(ctx, configLoader, vaultName, outputFormat))
```

Create a new function:

```go
func createGoalCompleteCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    var force bool

    cmd := &cobra.Command{
        Use:   "complete <goal-name>",
        Short: "Mark a goal as complete",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            goalName := args[0]
            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            currentDateTime := libtime.NewCurrentDateTime()

            dispatcher := ops.NewVaultDispatcher()
            return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                goalStore := storage.NewGoalStorage(storageConfig)
                taskStore := storage.NewTaskStorage(storageConfig)
                completeOp := ops.NewGoalCompleteOperation(goalStore, taskStore, currentDateTime)
                return completeOp.Execute(ctx, vault.Path, goalName, vault.Name, *outputFormat, force)
            })
        },
    }

    cmd.Flags().BoolVar(&force, "force", false, "Complete even if open tasks are linked to this goal")
    return cmd
}
```

## 2. Add `objective complete` subcommand inside `createObjectiveCommands`

Inside `createObjectiveCommands`, after existing `cmd.AddCommand(...)` calls, add:

```go
cmd.AddCommand(createObjectiveCompleteCommand(ctx, configLoader, vaultName, outputFormat))
```

Create a new function:

```go
func createObjectiveCompleteCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    return &cobra.Command{
        Use:   "complete <objective-name>",
        Short: "Mark an objective as complete",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            objectiveName := args[0]
            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            currentDateTime := libtime.NewCurrentDateTime()

            dispatcher := ops.NewVaultDispatcher()
            return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                objectiveStore := storage.NewObjectiveStorage(storageConfig)
                completeOp := ops.NewObjectiveCompleteOperation(objectiveStore, currentDateTime)
                return completeOp.Execute(ctx, vault.Path, objectiveName, vault.Name, *outputFormat)
            })
        },
    }
}
```

## 3. Update CHANGELOG.md

Add under `## Unreleased` (create the section if it does not exist, append if it does):

```
- feat: add `goal complete` command with open-task validation and --force flag
- feat: add `objective complete` command
```

</requirements>

<constraints>
- The `--force` flag must be a local flag on the `goal complete` command only (not persistent)
- `getVaults`, `ops.NewVaultDispatcher`, and `storage.NewConfigFromVault` are already imported — do not add duplicate imports
- Do NOT change any existing command functions — only add new functions and wire them in
- Existing tests must still pass
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
make precommit

# Verify commands are registered:
go run . goal complete --help
go run . objective complete --help

# Verify --force flag appears:
go run . goal complete --help | grep force
</verification>
