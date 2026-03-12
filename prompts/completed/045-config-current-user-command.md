---
status: completed
summary: Added `vault-cli config current-user` subcommand that prints the current user from the config file using plain text output.
container: vault-cli-045-config-current-user-command
dark-factory-version: v0.54.0
created: "2026-03-12T18:53:10Z"
queued: "2026-03-12T18:53:10Z"
started: "2026-03-12T18:53:20Z"
completed: "2026-03-12T18:56:41Z"
---

<summary>
- A new subcommand `config current-user` prints the current user from the config file
- Plain text output by default, no JSON needed for a single string value
- The command reuses the existing `GetCurrentUser` method — no new config logic required
- Missing or unreadable config produces a clear error message
- All existing commands and tests continue to work unchanged
</summary>

<objective>
Add a `vault-cli config current-user` subcommand that prints the `current_user` value from the config file. Task-orchestrator needs this value to determine which tasks are assigned to the local user for session cleanup.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/cli/cli.go` for the CLI structure and command registration pattern.
Read `pkg/config/config.go` for the `Loader` interface — `GetCurrentUser(ctx)` already exists at ~line 89.
Read `pkg/cli/output.go` for `PrintJSON` helper and output format constants.
Follow the pattern of `createConfigListCommand` in `pkg/cli/cli.go` (~line 1071) for structure and registration.
</context>

<requirements>
1. Add `createConfigCurrentUserCommand` function in `pkg/cli/cli.go`. Signature: `func createConfigCurrentUserCommand(ctx context.Context, configLoader *config.Loader) *cobra.Command`. Use `cobra.NoArgs`. Call `(*configLoader).GetCurrentUser(ctx)` and print the result with `fmt.Println(user)`.

2. Register the command in the `Run` function: after `configCmd.AddCommand(createConfigListCommand(...))` (~line 105), add `configCmd.AddCommand(createConfigCurrentUserCommand(ctx, &configLoader))`.

3. This command always outputs plain text (a single username string). Do NOT add `--output` format support or use `PrintJSON` — it's unnecessary for a single value.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Do NOT modify `pkg/config/config.go` — `GetCurrentUser` already exists
- Do NOT add `encoding/json` imports — not needed for this command
- Use `github.com/spf13/cobra` for the command (already imported)
</constraints>

<verification>
Run `make precommit` — must pass.
Run `go run main.go config current-user` — should print the current username (e.g. `bborbe`).
Run `go run main.go config current-user --help` — should show "Print the current user".
</verification>
