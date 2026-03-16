---
status: completed
spec: ["001"]
summary: Wired vault-cli decision list and decision ack commands into pkg/cli/cli.go using the multi-vault pattern
container: vault-cli-055-spec-001-decision-cli
dark-factory-version: v0.54.0
created: "2026-03-16T00:00:00Z"
queued: "2026-03-16T10:36:41Z"
started: "2026-03-16T10:53:19Z"
completed: "2026-03-16T10:57:17Z"
branch: dark-factory/decision-list-ack
---

<summary>
- Two new CLI commands are wired: vault-cli decision list and vault-cli decision ack
- decision list supports --reviewed and --all filter flags alongside the shared --vault and --output flags
- decision ack accepts a positional decision name and an optional --status flag
- Both commands use the multi-vault pattern: try each configured vault, stop on first success (for ack) or aggregate (for list)
- A createDecisionCommands() factory function returns the parent "decision" cobra command with both subcommands
- The decision parent command is registered on rootCmd alongside task, goal, and theme
</summary>

<objective>
Wire `vault-cli decision list` and `vault-cli decision ack` into `pkg/cli/cli.go` using the established multi-vault pattern, completing the end-to-end feature so users can list and acknowledge decisions from the command line.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/cli/cli.go` — study `createGoalCommands` (parent + subcommands pattern), `createTaskListCommand` (multi-vault list aggregation), and `createCompleteCommand` (multi-vault mutation: try-each-until-success).
Read `pkg/ops/decision_list.go` — `DecisionListOperation.Execute` signature: `(ctx, vaultPath, vaultName, showReviewed bool, showAll bool, outputFormat string) error`.
Read `pkg/ops/decision_ack.go` — `DecisionAckOperation.Execute` signature: `(ctx, vaultPath, vaultName, decisionName, statusOverride, outputFormat string) error`.
Read `pkg/storage/markdown.go` — `NewStorage`, `NewConfigFromVault`.
Read `docs/development-patterns.md` — "Multi-Vault Pattern" and "Naming" sections.
</context>

<requirements>
1. Add `createDecisionCommands` to `pkg/cli/cli.go`:

```go
func createDecisionCommands(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    decisionCmd := &cobra.Command{
        Use:   "decision",
        Short: "Manage decisions in the vault",
    }
    decisionCmd.AddCommand(createDecisionListCommand(ctx, configLoader, vaultName, outputFormat))
    decisionCmd.AddCommand(createDecisionAckCommand(ctx, configLoader, vaultName, outputFormat))
    return decisionCmd
}
```

2. Implement `createDecisionListCommand`:

```go
func createDecisionListCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    var showReviewed bool
    var showAll bool

    cmd := &cobra.Command{
        Use:   "list",
        Short: "List decisions pending review",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return fmt.Errorf("get vaults: %w", err)
            }
            for _, vault := range vaults {
                storageConfig := storage.NewConfigFromVault(vault)
                store := storage.NewStorage(storageConfig)
                listOp := ops.NewDecisionListOperation(store)
                if err := listOp.Execute(ctx, vault.Path, vault.Name, showReviewed, showAll, *outputFormat); err != nil {
                    fmt.Fprintf(os.Stderr, "Warning: vault %s: %v\n", vault.Name, err)
                }
            }
            return nil
        },
    }

    cmd.Flags().BoolVar(&showReviewed, "reviewed", false, "Show only reviewed decisions")
    cmd.Flags().BoolVar(&showAll, "all", false, "Show all decisions (reviewed and unreviewed)")
    return cmd
}
```

3. Implement `createDecisionAckCommand`:

```go
func createDecisionAckCommand(
    ctx context.Context,
    configLoader *config.Loader,
    vaultName *string,
    outputFormat *string,
) *cobra.Command {
    var statusOverride string

    cmd := &cobra.Command{
        Use:   "ack <decision-name>",
        Short: "Acknowledge a decision (mark as reviewed)",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            decisionName := args[0]
            vaults, err := getVaults(ctx, configLoader, vaultName)
            if err != nil {
                return fmt.Errorf("get vaults: %w", err)
            }

            currentDateTime := libtime.NewCurrentDateTime()

            // Multiple vaults: try each until successful
            var lastErr error
            for _, vault := range vaults {
                storageConfig := storage.NewConfigFromVault(vault)
                store := storage.NewStorage(storageConfig)
                ackOp := ops.NewDecisionAckOperation(store, currentDateTime)
                if err := ackOp.Execute(ctx, vault.Path, vault.Name, decisionName, statusOverride, *outputFormat); err == nil {
                    return nil
                } else {
                    lastErr = err
                }
            }
            return fmt.Errorf("decision not found in any vault: %w", lastErr)
        },
    }

    cmd.Flags().StringVar(&statusOverride, "status", "", "Override the decision's status field")
    return cmd
}
```

4. Register the decision command in `Run`:

```go
rootCmd.AddCommand(createDecisionCommands(ctx, &configLoader, &vaultName, &outputFormat))
```

Add this line after the existing `rootCmd.AddCommand(createVisionCommands(...))` line (before the `configCmd` block).

5. Ensure all required imports are present in `pkg/cli/cli.go`:
   - `"os"` (already imported)
   - `libtime "github.com/bborbe/time"` (already imported)
   - No new imports should be needed if the file already imports `ops` and `storage`
</requirements>

<constraints>
- `decision list` aggregates across all vaults: vault errors are logged to stderr as warnings, not returned — scanning continues (NOTE: this differs from `task list` which returns errors immediately; decisions intentionally use a lenient approach since they span the whole vault and individual vault failures should not block others)
- `decision ack` uses try-each-until-success (like `task complete`): stop at first vault where the decision is found
- Both commands rely on the `--vault` and `--output` flags inherited from rootCmd's `PersistentFlags` — do NOT re-declare them
- `--reviewed` and `--all` are local flags on `decision list` only
- `--status` is a local flag on `decision ack` only
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
</constraints>

<verification>
Run `make precommit` — must pass.

Manual smoke test (after binary rebuild):
```
vault-cli decision list
vault-cli decision list --output json
vault-cli decision list --reviewed
vault-cli decision list --all
vault-cli decision ack "Some Decision Name"
vault-cli decision ack "Some Decision Name" --status accepted
```
</verification>
