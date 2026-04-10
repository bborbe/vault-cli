---
status: completed
spec: [003-list-field-add-remove]
summary: Wired EntityListAddOperation and EntityListRemoveOperation into the CLI by adding createEntityListAddCommand and createEntityListRemoveCommand helpers and add/remove subcommands to task, goal, theme, objective, and vision command groups; also extracted createTaskCommands helper to fix funlen linter violation.
container: vault-cli-073-spec-003-cli-wiring
dark-factory-version: v0.57.5
created: "2026-03-17T10:34:50Z"
queued: "2026-03-17T10:44:29Z"
started: "2026-03-17T11:09:11Z"
completed: "2026-03-17T11:16:34Z"
branch: dark-factory/list-field-add-remove
---

<summary>
- All five entity types (task, goal, theme, objective, vision) gain `add` and `remove` subcommands
- `vault-cli task add "My Task" goals "My Goal"` appends a value to a list field
- `vault-cli task remove "My Task" goals "My Goal"` removes a value from a list field
- Attempting add/remove on a scalar field returns a non-zero exit with "not a list field" error
- Adding a duplicate or removing a non-existent value returns a descriptive error, no file is written
- JSON output is supported for add and remove commands via `--output json`
- Multi-vault dispatch works identically to existing mutation commands
</summary>

<objective>
Wire the `EntityListAddOperation` and `EntityListRemoveOperation` (created in prompt 1) into the CLI by adding `add` and `remove` subcommands to the task, goal, theme, objective, and vision command groups. Follow the exact same `VaultDispatcher` pattern used by `createTaskSetCommand`.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read these files before making changes (read ALL of them first):

- `pkg/cli/cli.go` — full file; focus on:
  - `createTaskSetCommand` — exact pattern for mutation commands with vault dispatch and JSON output
  - `createEntitySetCommand` (added by spec 002 prompt 3) — helper pattern to replicate for list ops
  - `createTaskCommands` — where to add `add`/`remove` to the task subcommand group
  - `createGoalCommands` — where to add goal subcommands
  - `createThemeCommands` — where to add theme subcommands
  - `createObjectiveCommands` — where to add objective subcommands
  - `createVisionCommands` — where to add vision subcommands
- `pkg/ops/frontmatter_entity.go` — `EntityListAddOperation`, `EntityListRemoveOperation` and their constructors (`NewTaskListAddOperation`, `NewGoalListAddOperation`, etc.)
- `pkg/storage/storage.go` — `NewTaskStorage`, `NewGoalStorage`, `NewThemeStorage`, `NewObjectiveStorage`, `NewVisionStorage` constructor functions
- `pkg/cli/output.go` — `OutputFormatJSON` constant and `PrintJSON` helper
</context>

<requirements>

## 1. Add `createEntityListAddCommand` helper to `pkg/cli/cli.go`

Add a new private helper function that creates the `add` subcommand for a generic entity. Follow the exact shape of `createEntitySetCommand`:

```go
func createEntityListAddCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
    entityType string,
    newAddOp func(cfg *storage.Config) ops.EntityListAddOperation,
) *cobra.Command {
    return &cobra.Command{
        Use:   "add <name> <field> <value>",
        Short: fmt.Sprintf("Add a value to a list field on a %s", entityType),
        Args:  cobra.ExactArgs(3),
        RunE: func(cmd *cobra.Command, args []string) error {
            entityName := args[0]
            field := args[1]
            value := args[2]

            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            dispatcher := ops.NewVaultDispatcher()
            err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                addOp := newAddOp(storageConfig)
                return addOp.Execute(ctx, vault.Path, entityName, field, value)
            })
            if err != nil {
                if *outputFormat == OutputFormatJSON {
                    return PrintJSON(map[string]any{
                        "success": false,
                        "error":   err.Error(),
                    })
                }
                return err
            }
            if *outputFormat == OutputFormatJSON {
                return PrintJSON(map[string]any{
                    "success": true,
                    "field":   field,
                    "value":   value,
                    "name":    entityName,
                })
            }
            fmt.Printf("✅ Added %s to %s on: %s\n", value, field, entityName)
            return nil
        },
    }
}
```

## 2. Add `createEntityListRemoveCommand` helper to `pkg/cli/cli.go`

Same shape as `createEntityListAddCommand`, but calls `removeOp.Execute(...)`:

- `Use: "remove <name> <field> <value>"`
- `Short: fmt.Sprintf("Remove a value from a list field on a %s", entityType)`
- Success plain output: `fmt.Printf("✅ Removed %s from %s on: %s\n", value, field, entityName)`
- Success JSON: `{"success": true, "field": field, "value": value, "name": entityName}`
- Error JSON: `{"success": false, "error": err.Error()}`

## 3. Update `createTaskCommands` — add `add` and `remove`

Inside `createTaskCommands`, after the existing `set`/`clear`/`show` subcommands, add:

```go
cmd.AddCommand(createEntityListAddCommand(
    ctx, configLoader, vaultName, outputFormat,
    "task",
    func(cfg *storage.Config) ops.EntityListAddOperation {
        return ops.NewTaskListAddOperation(storage.NewTaskStorage(cfg))
    },
))
cmd.AddCommand(createEntityListRemoveCommand(
    ctx, configLoader, vaultName, outputFormat,
    "task",
    func(cfg *storage.Config) ops.EntityListRemoveOperation {
        return ops.NewTaskListRemoveOperation(storage.NewTaskStorage(cfg))
    },
))
```

## 4. Update `createGoalCommands` — add `add` and `remove`

Same pattern using `storage.NewGoalStorage(cfg)` and `ops.NewGoalListAddOperation` / `ops.NewGoalListRemoveOperation`.

## 5. Update `createThemeCommands` — add `add` and `remove`

Same pattern using `storage.NewThemeStorage(cfg)` and `ops.NewThemeListAddOperation` / `ops.NewThemeListRemoveOperation`.

## 6. Update `createObjectiveCommands` — add `add` and `remove`

Same pattern using `storage.NewObjectiveStorage(cfg)` and `ops.NewObjectiveListAddOperation` / `ops.NewObjectiveListRemoveOperation`.

## 7. Update `createVisionCommands` — add `add` and `remove`

Same pattern using `storage.NewVisionStorage(cfg)` and `ops.NewVisionListAddOperation` / `ops.NewVisionListRemoveOperation`.

## 8. Address linter warnings if needed

The two helper functions `createEntityListAddCommand` and `createEntityListRemoveCommand` are structurally similar to each other and to existing entity command helpers. If `make lint` reports `dupl` violations, add `//nolint:dupl` to each function. If `funlen` or `nestif` violations appear, extract the inner JSON-output logic into a small private helper. Check `make lint` output and fix before declaring done.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- All existing `get`, `set`, `clear`, `show` commands must not change behavior — all current tests must pass
- `set` on a list field continues to replace the whole list (backward compatible)
- The `add` command signature is `add <entity-name> <field> <value>` — exactly 3 positional args
- The `remove` command signature is `remove <entity-name> <field> <value>` — exactly 3 positional args
- Multi-vault dispatch: use `ops.NewVaultDispatcher().FirstSuccess(...)` — same pattern as `createTaskSetCommand`
- JSON output format: use `OutputFormatJSON` constant and `PrintJSON` helper from `pkg/cli/output.go`
- Error output in JSON mode: `{"success": false, "error": "..."}` — same as task set/clear commands
- Plain success output uses ✅ prefix (matching existing task mutation commands)
- Error propagation: "not a list field", "already exists", "not found", "unknown field" errors all propagate as non-zero exit codes automatically (returned from RunE)
- Factory closures receive `*storage.Config` (not `*config.Vault`) — construct narrow storage from config inside the closure
- Use `github.com/bborbe/errors` for error wrapping — never `fmt.Errorf` for wrapping
- `make precommit` must pass — fix any linter issues (funlen, nestif, dupl) before declaring done
</constraints>

<verification>
Run `make precommit` — must pass.

Additional acceptance verification (requires a vault with test data):
```bash
# Task add/remove
vault-cli task add "My Task" goals "My Goal"     # appends
vault-cli task add "My Task" goals "My Goal"     # must fail: already exists
vault-cli task remove "My Task" goals "My Goal"  # removes
vault-cli task remove "My Task" goals "My Goal"  # must fail: not found
vault-cli task add "My Task" status "todo"       # must fail: not a list field

# Goal add/remove
vault-cli goal add "My Goal" tags "tag1"
vault-cli goal remove "My Goal" tags "tag1"

# Objective add/remove
vault-cli objective add "My Objective" tags "tag1"

# Theme remove
vault-cli theme remove "My Theme" tags "tag1"

# Vision add
vault-cli vision add "My Vision" tags "tag1"

# JSON output
vault-cli task add "My Task" goals "My Goal" --output json
vault-cli task add "My Task" status "todo" --output json  # must return {"success":false,...}
```
</verification>
