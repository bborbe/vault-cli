---
status: completed
---

<objective>
Refactor vault-cli commands to group task operations under a `task` subcommand.

Instead of `vault-cli list`, `vault-cli lint`, `vault-cli complete`, `vault-cli defer`, `vault-cli update` at the root level, group them as:
- `vault-cli task list`
- `vault-cli task lint`
- `vault-cli task complete`
- `vault-cli task defer`
- `vault-cli task update`

This leaves room for future groups like `vault-cli goal list`, `vault-cli note list`.
</objective>

<context>
Go CLI project using cobra.
Read CLAUDE.md for project conventions.

Key file: `./pkg/cli/cli.go`

Current structure:
```
rootCmd
  ├── complete
  ├── defer
  ├── update
  ├── list
  └── lint
```

Target structure:
```
rootCmd
  └── task
        ├── list
        ├── lint
        ├── complete
        ├── defer
        └── update
```

All command implementations (createListCommand, createLintCommand, etc.) stay the same — only the registration changes.
</context>

<requirements>
1. Add a `task` parent command to rootCmd (no RunE — just a group)
2. Register all existing subcommands under `task` instead of root
3. Update integration tests in `./integration/cli_test.go` to use new command paths (e.g. `"task", "list"` instead of `"list"`)
4. No changes to command implementations — only wiring changes
</requirements>

<implementation>
```go
taskCmd := &cobra.Command{
    Use:   "task",
    Short: "Manage tasks in the vault",
}

taskCmd.AddCommand(createListCommand(ctx, &configLoader, &vaultName))
taskCmd.AddCommand(createLintCommand(ctx, &configLoader, &vaultName))
taskCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName))
taskCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName))
taskCmd.AddCommand(createUpdateCommand(ctx, &configLoader, &vaultName))

rootCmd.AddCommand(taskCmd)
```
</implementation>

<output>
Modify in place:
- `./pkg/cli/cli.go`
- `./integration/cli_test.go`
</output>

<verification>
```
make test
go run main.go task list
go run main.go task lint
go run main.go task complete "test"
```
</verification>

<success_criteria>
- `make test` passes including integration tests
- `vault-cli task list` works
- `vault-cli task lint` works
- Old root-level commands (`vault-cli list`) no longer exist
- `vault-cli --help` shows `task` as the only subcommand group
</success_criteria>
