---
status: completed
summary: Removed root-level command aliases (complete, defer, lint, list) from vault-cli
container: vault-cli-027-remove-root-level-aliases
dark-factory-version: v0.13.2
created: "2026-03-03T16:19:46Z"
queued: "2026-03-03T16:19:46Z"
started: "2026-03-03T16:25:06Z"
completed: "2026-03-03T16:29:05Z"
---
<objective>
Remove root-level command aliases from vault-cli. The commands `complete`, `defer`, `list`, and `lint` exist both under `task` subcommand AND at the root level. The root-level duplicates were kept for backwards compatibility but should now be removed.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read pkg/cli/cli.go — all changes are in the `Run` function in this file.
</context>

<requirements>
In `pkg/cli/cli.go`, remove the four root-level alias registrations (lines ~92-96):

```go
// Root-level aliases for common task commands (backwards compatibility)
rootCmd.AddCommand(createTaskListCommand(ctx, &configLoader, &vaultName, &outputFormat))
rootCmd.AddCommand(createLintCommand(ctx, &configLoader, &vaultName, &outputFormat))
rootCmd.AddCommand(createCompleteCommand(ctx, &configLoader, &vaultName, &outputFormat))
rootCmd.AddCommand(createDeferCommand(ctx, &configLoader, &vaultName, &outputFormat))
```

Remove these 5 lines (including the comment). Do not remove the identical calls inside `taskCmd.AddCommand(...)` — those stay.
</requirements>

<constraints>
- Do NOT remove any commands from the `taskCmd` subcommand group
- Do NOT modify any other files
- Do NOT change function signatures or implementations
</constraints>

<verification>
Run: `make test`
Confirm:
- `vault-cli --help` no longer shows `complete`, `defer`, `list`, or `lint` at the top level
- `vault-cli task --help` still shows all four commands
- All tests pass
</verification>

<success_criteria>
`vault-cli --help` output contains only: `completion`, `goal`, `help`, `objective`, `search`, `task`, `theme`, `vision`
`vault-cli task --help` contains: `clear`, `complete`, `defer`, `get`, `lint`, `list`, `search`, `set`, `update`, `validate`, `work-on`
</success_criteria>
