---
status: completed
---

<objective>
Add `goal`, `theme`, `objective`, and `vision` subcommand groups to vault-cli, each with a `list` subcommand.

Reuse the existing ListOperation — make it generic by passing the directory path, so all types share the same code.
</objective>

<context>
Go CLI project using cobra.
Read CLAUDE.md for project conventions.

After prompt 011, the CLI structure is:
```
rootCmd
  └── task
        ├── list
        ├── lint
        ├── complete
        ├── defer
        └── update
```

Target structure:
```
rootCmd
  ├── task
  │     ├── list
  │     ├── lint
  │     ├── complete
  │     ├── defer
  │     └── update
  ├── goal
  │     └── list
  ├── theme
  │     └── list
  ├── objective
  │     └── list
  └── vision
        └── list
```

Config already has:
- `Vault.TasksDir` (e.g. "24 Tasks")
- `Vault.GoalsDir` (e.g. "23 Goals")

Need to add to `./pkg/config/config.go`:
- `ThemesDir` with default "21 Themes"
- `ObjectivesDir` with default "22 Objectives"
- `VisionDir` with default "20 Vision"

ListOperation in `./pkg/ops/list.go` currently takes `vaultPath string` and uses `storage.ListTasks()`.
Make it accept a `pagesDir string` parameter so it can list any directory.
</context>

<requirements>
1. Add `ThemesDir`, `ObjectivesDir`, `VisionDir` to `Vault` struct in `./pkg/config/config.go` with defaults
2. Add `GetThemesDir()`, `GetObjectivesDir()`, `GetVisionDir()` methods following existing pattern
3. Update `ListOperation.Execute()` to accept `pagesDir string` parameter
4. Update storage to have a generic `ListPages(ctx, vaultPath, pagesDir)` method (or reuse ListTasks with dir param)
5. Add `createGoalCommands`, `createThemeCommands`, `createObjectiveCommands`, `createVisionCommands` in `./pkg/cli/cli.go`
6. Register each group on rootCmd
7. Update existing `task list` call to pass `storageConfig.TasksDir`
8. Update tests for the new ListOperation signature
</requirements>

<implementation>
Storage interface addition:
```go
ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Task, error)
```

CLI pattern for each group:
```go
func createGoalCommands(ctx context.Context, configLoader *config.Loader, vaultName *string) *cobra.Command {
    cmd := &cobra.Command{Use: "goal", Short: "Manage goals in the vault"}
    cmd.AddCommand(createGoalListCommand(ctx, configLoader, vaultName))
    return cmd
}
```
</implementation>

<output>
Modify in place:
- `./pkg/config/config.go`
- `./pkg/ops/list.go`
- `./pkg/ops/list_test.go`
- `./pkg/storage/markdown.go`
- `./pkg/cli/cli.go`
- `./integration/cli_test.go`
</output>

<verification>
```
make test
go run main.go goal list
go run main.go theme list
go run main.go objective list
go run main.go vision list
go run main.go task list
```
</verification>

<success_criteria>
- `make test` passes
- All 5 list commands work
- `task list` unchanged
- `goal list`, `theme list`, `objective list`, `vision list` all return pages from correct dirs
</success_criteria>
