---
status: approved
spec: ["021"]
created: "2026-07-02T10:00:00Z"
queued: "2026-07-02T09:46:53Z"
---

<summary>
- Wires the `vault-cli resolve` top-level command into the CLI root
- Creates a `createResolveCommand` that follows existing command patterns: getVaults → create storage → call operation → JSON output
- JSON-only output via `PrintJSON` — plain mode is a silent no-op (resolve is a machine contract, not human-facing)
- Not-found is NOT an error — plain mode exits 0 silently; JSON mode returns `found:false`
- Adds integration test entry in `integration/cli_test.go`
</summary>

<objective>
Add a top-level `vault-cli resolve <name> [--vault X] --output json` Cobra command that calls `ResolveOperation.Execute` and prints the result as JSON, plus its integration test registration.
</objective>

<context>
Read CLAUDE.md for project conventions.

Read these files before implementing:
- `pkg/cli/cli.go` — the full file. Focus on: `createWorkOnCommand` (lines 306-373) as the structural template for a single-arg mutation command; `NewRootCommand` (line 77) for where to wire the new command; `getVaults` (lines 31-44) for vault resolution; `PrintJSON` (from `output.go` — read it to confirm signature).
- `pkg/ops/resolve.go` — the `ResolveOperation` interface created by prompt 02. If this file does not exist yet, STOP and report `Status: failed` with message `"ResolveOperation not yet deployed (prompt 02)"` — do not create the CLI command first.
- `pkg/domain/resolve_result.go` — the `ResolveResult` struct created by prompt 01. Same guard: if missing, STOP.
- `integration/cli_test.go` — the command registration table structure. Find the pattern for adding a new top-level command (resolve is top-level, not `vault-cli task resolve`).
</context>

<requirements>
1. Add `createResolveCommand` function to `pkg/cli/cli.go`, modeled on `createWorkOnCommand` structure:

```go
func createResolveCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    return &cobra.Command{
        Use:   "resolve <name>",
        Short: "Resolve a name to its entity type (task or goal)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            name := args[0]
            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return errors.Wrap(ctx, err, "get vaults")
            }

            dispatcher := ops.NewVaultDispatcher()
            return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                storageConfig := storage.NewConfigFromVault(vault)
                taskStore := storage.NewTaskStorage(storageConfig)
                goalStore := storage.NewGoalStorage(storageConfig)
                resolveOp := ops.NewResolveOperation(taskStore, goalStore)
                result, err := resolveOp.Execute(ctx, vault.Path, name)
                // resolve always returns nil error under current contract; guard for future change
                if err != nil {
                    return err
                }
                if *outputFormat == OutputFormatJSON {
                    return PrintJSON(result)
                }
                // plain mode: silent no-op — resolve is a machine contract
                return nil
            })
        },
    }
}
```

2. Wire into `NewRootCommand` (after the search command, before task commands):
   ```go
   rootCmd.AddCommand(createResolveCommand(ctx, &configLoader, &vaultName, &outputFormat))
   ```

3. Add entry in `integration/cli_test.go` command registration table. Find the multi-vault command list array and add `"resolve"` as a top-level command entry.

4. Import `pkg/ops` is already imported in `cli.go` — no new imports needed beyond what `createWorkOnCommand` uses. Verify `ResolveResult`, `ResolveOperation`, `NewResolveOperation`, and `NewGoalStorage` are all reachable.

5. Run `go vet ./...` and `make test` — must pass.
</requirements>

<constraints>
- **JSON-only output contract**: `--output json` returns `PrintJSON(result)`. Plain mode prints nothing, exits 0 — even when not-found. This is a machine contract for slash-command consumption.
- **Not-found is not an error**: the CLI layer never wraps a not-found as an error. The operation returns `found:false` with nil error; the CLI prints it and exits 0.
- **Multi-vault via VaultDispatcher.FirstSuccess**: follows existing pattern — probes vaults in order, first success wins.
- **No new storage constructors**: use existing `NewTaskStorage`, `NewGoalStorage`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

Acceptance criteria advanced by this prompt (from spec 021):
- [ ] AC1 — Task match end-to-end: `vault-cli resolve "Existing Task Name" --output json` → `{"type":"task","name":"Existing Task Name","found":true}`
- [ ] AC2 — Goal match end-to-end
- [ ] AC3 — Not-found end-to-end: `found:false`, exit 0
- [ ] AC5 — Vault scoping via `--vault` flag
- [ ] AC6 — `make precommit` passes
- [ ] AC7 — Integration test entry: `grep -n "resolve" integration/cli_test.go` returns ≥1 line
- [ ] AC8 — No regression: existing commands unaffected
</constraints>

<verification>
Run `make test` — must pass.
Run `grep -rn "createResolveCommand" pkg/cli/cli.go` — function exists and is wired.
Run `grep -n "resolve" integration/cli_test.go` — integration test entry exists.
Build: `go build -mod=vendor -o /dev/null .` — compiles cleanly.
</verification>
