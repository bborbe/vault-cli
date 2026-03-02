---
status: completed
---

<objective>
Suppress cobra's usage/help output when a command returns a runtime error.

Currently `vault-cli lint` prints the full usage/flags after any error, which is noisy and confusing. Only lint issue lines should appear.
</objective>

<context>
Go CLI project using cobra.
Read CLAUDE.md for project conventions.

Key file: `./pkg/cli/cli.go`

Cobra prints usage on error by default. This can be disabled per-command or globally with:
```go
cmd.SilenceUsage = true
```

Or globally on the root command:
```go
rootCmd.SilenceUsage = true
```
</context>

<requirements>
1. Set `SilenceUsage = true` on the root command in `./pkg/cli/cli.go`
2. No other changes needed
</requirements>

<output>
Modify in place:
- `./pkg/cli/cli.go`
</output>

<verification>
```
make test
go run main.go lint 2>&1
```

Confirm: only lint issue lines printed, no usage/flags block after the error.
</verification>

<success_criteria>
- `make test` passes
- Error output contains only lint issues, no usage block
</success_criteria>
