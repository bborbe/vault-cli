---
status: created
spec: ["002"]
created: "2026-03-17T10:00:00Z"
branch: dark-factory/generic-frontmatter-ops
---

<summary>
- Goal, theme, objective, and vision entities gain get, set, clear, and show subcommands
- All four entity types share the same command structure as existing task commands
- Commands automatically search across all configured vaults
- JSON output mode supported for all new commands
- Invalid field names produce a clear error with a non-zero exit code
</summary>

<objective>
Wire the generic entity frontmatter operations (created in prompt 2) into the CLI by adding get/set/clear/show subcommands to the goal, theme, objective, and vision command groups. Follow the exact same VaultDispatcher pattern used by `createTaskGetCommand` and friends.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read these files before making changes (read ALL of them first):

- `pkg/cli/cli.go` — full file; focus on:
  - `createTaskGetCommand` (~line 904) — exact pattern to replicate for each entity
  - `createTaskSetCommand` (~line 959) — same pattern
  - `createTaskClearCommand` (~line 1015) — same pattern
  - `createTaskShowCommand` (~line 1068) — same pattern
  - `createGoalCommands` (~line 555) — where to add goal subcommands
  - `createThemeCommands` (~line 598) — where to add theme subcommands
  - `createObjectiveCommands` (~line 641) — where to add objective subcommands
  - `createVisionCommands` (~line 684) — where to add vision subcommands
- `pkg/ops/frontmatter_entity.go` — EntityGetOperation, EntitySetOperation, EntityClearOperation, EntityShowOperation and their constructors
- `pkg/storage/storage.go` — GoalStorage, ThemeStorage, ObjectiveStorage, VisionStorage interfaces
- `pkg/cli/output.go` — OutputFormatJSON constant and PrintJSON helper
</context>

<requirements>

## 1. Add helper factory functions in `pkg/cli/cli.go`

Create four private helper functions that create the four command types for a generic entity. Each helper accepts:
- Storage constructors/factories (as closures that accept `*storage.Config` and return the narrow storage interface)
- Entity type name string (for output messages: "goal", "theme", "objective", "vision")

### `createEntityGetCommand`

```go
func createEntityGetCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
    entityType string,
    newGetOp func(cfg *storage.Config) ops.EntityGetOperation,
) *cobra.Command {
    return &cobra.Command{
        Use:   "get <name> <key>",
        Short: fmt.Sprintf("Get a frontmatter field value from a %s", entityType),
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            entityName := args[0]
            key := args[1]

            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            dispatcher := ops.NewVaultDispatcher()
            err = dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                getOp := newGetOp(storageConfig)
                value, err := getOp.Execute(ctx, vault.Path, entityName, key)
                if err != nil {
                    return err
                }
                if *outputFormat == OutputFormatJSON {
                    return PrintJSON(map[string]any{
                        "key":   key,
                        "value": value,
                        "name":  entityName,
                    })
                }
                fmt.Println(value)
                return nil
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
            return nil
        },
    }
}
```

### `createEntitySetCommand`

Same structure, `Args: cobra.ExactArgs(3)`, calls `setOp.Execute(ctx, vault.Path, entityName, key, value)`.

Success plain output: `fmt.Printf("✅ Set %s=%s on: %s\n", key, value, entityName)`
Success JSON: `{"success": true, "key": key, "value": value, "name": entityName}`

### `createEntityClearCommand`

Same structure, `Args: cobra.ExactArgs(2)`, calls `clearOp.Execute(ctx, vault.Path, entityName, key)`.

Success plain output: `fmt.Printf("✅ Cleared %s on: %s\n", key, entityName)`
Success JSON: `{"success": true, "key": key, "name": entityName}`

### `createEntityShowCommand`

Same structure, `Args: cobra.ExactArgs(1)`, calls `showOp.Execute(ctx, vault.Path, vault.Name, entityName, *outputFormat)`.

No explicit success output (handled inside the operation).

## 2. Update `createGoalCommands` — add get/set/clear/show

Inside `createGoalCommands`, after the existing list/lint/search commands, add:

```go
cmd.AddCommand(createEntityGetCommand(
    ctx, configLoader, vaultName, outputFormat,
    "goal",
    func(cfg *storage.Config) ops.EntityGetOperation {
        return ops.NewGoalGetOperation(storage.NewGoalStorage(cfg))
    },
))
cmd.AddCommand(createEntitySetCommand(
    ctx, configLoader, vaultName, outputFormat,
    "goal",
    func(cfg *storage.Config) ops.EntitySetOperation {
        return ops.NewGoalSetOperation(storage.NewGoalStorage(cfg))
    },
))
cmd.AddCommand(createEntityClearCommand(
    ctx, configLoader, vaultName, outputFormat,
    "goal",
    func(cfg *storage.Config) ops.EntityClearOperation {
        return ops.NewGoalClearOperation(storage.NewGoalStorage(cfg))
    },
))
cmd.AddCommand(createEntityShowCommand(
    ctx, configLoader, vaultName, outputFormat,
    "goal",
    func(cfg *storage.Config) ops.EntityShowOperation {
        return ops.NewGoalShowOperation(storage.NewGoalStorage(cfg))
    },
))
```

## 3. Update `createThemeCommands` — add get/set/clear/show

Same pattern as goal, using `storage.NewThemeStorage(cfg)` and `ops.NewThemeGetOperation`, `ops.NewThemeSetOperation`, `ops.NewThemeClearOperation`, `ops.NewThemeShowOperation`.

## 4. Update `createObjectiveCommands` — add get/set/clear/show

Same pattern, using `storage.NewObjectiveStorage(cfg)` and `ops.NewObjectiveGetOperation`, `ops.NewObjectiveSetOperation`, `ops.NewObjectiveClearOperation`, `ops.NewObjectiveShowOperation`.

## 5. Update `createVisionCommands` — add get/set/clear/show

Same pattern, using `storage.NewVisionStorage(cfg)` and `ops.NewVisionGetOperation`, `ops.NewVisionSetOperation`, `ops.NewVisionClearOperation`, `ops.NewVisionShowOperation`.

## 6. Add `//nolint` directives if needed

The four `createEntityXxxCommand` helper functions may trigger `dupl` or `gocognit` linter warnings since they are structurally similar. Add `//nolint:dupl` to each if the linter complains. Check `make lint` output.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- All existing task get/set/clear/show commands must remain unchanged — only add new commands for goal/theme/objective/vision
- All existing goal/theme/objective/vision list/lint/search commands must remain unchanged — only ADD the new subcommands
- Multi-vault dispatch: use `ops.NewVaultDispatcher().FirstSuccess(...)` — same pattern as createTaskGetCommand
- JSON output format: use `OutputFormatJSON` constant and `PrintJSON` helper from `pkg/cli/output.go`
- Error output in JSON mode: `{"success": false, "error": "..."}` — same as task commands
- Plain success output: use ✅ prefix for set/clear confirmation (matching existing task commands)
- Unknown field errors propagate as non-zero exit codes automatically (returned from RunE)
- Do NOT add `ReadGoalByID` or similar — use the narrow storage constructors (`storage.NewGoalStorage`, `storage.NewThemeStorage`, etc.) only
- Factory closures in helper functions receive `*storage.Config` (not `*config.Vault`) — construct narrow storage from config inside the closure
- Use `github.com/bborbe/errors` for error wrapping — never `fmt.Errorf` for wrapping
- `make precommit` must pass — fix any linter issues (funlen, nestif, dupl) before declaring done
</constraints>

<verification>
Run `make precommit` — must pass.

Manual smoke-test verification commands (require a vault with test entities):
```bash
# Goal
vault-cli goal get "My Goal" status
vault-cli goal set "My Goal" status active
vault-cli goal clear "My Goal" assignee
vault-cli goal show "My Goal"
vault-cli goal set "My Goal" xyz "val"      # must exit 1, unknown field error

# Theme
vault-cli theme get "My Theme" status
vault-cli theme set "My Theme" status active
vault-cli theme show "My Theme"

# Objective
vault-cli objective get "My Objective" status
vault-cli objective set "My Objective" status active
vault-cli objective show "My Objective"

# Vision
vault-cli vision get "My Vision" status
vault-cli vision set "My Vision" status active
vault-cli vision show "My Vision"

# JSON output
vault-cli goal get "My Goal" status --output json
vault-cli goal set "My Goal" status completed --output json
```
</verification>
