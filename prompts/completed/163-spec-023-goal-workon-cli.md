---
status: completed
spec: [023-goal-work-on]
summary: Added vault-cli goal work-on command wiring GoalWorkOnOperation into the goal command group, with integration test entry and CHANGELOG
execution_id: vault-cli-exec-163-spec-023-goal-workon-cli
dark-factory-version: v0.191.0
created: "2026-07-11T08:42:00Z"
queued: "2026-07-11T08:29:33Z"
started: "2026-07-11T08:39:47Z"
completed: "2026-07-11T08:42:03Z"
---

<summary>
- Wires up the `vault-cli goal work-on <goal-name>` command under the existing `goal` command group.
- Running it marks the goal `in_progress`, assigns it to the current user (subject to the ownership rule), and starts or resumes a Claude session, printing the goal name, any warnings, and the session id.
- `--output json` returns the structured result including the session id; `--mode auto|interactive|headless` picks the session mode exactly like `task work-on`.
- A missing `claude` binary prints a warning and exits 0; a rejected / zero-turn Claude run exits non-zero.
- `vault-cli goal work-on --help` lists the command under `goal`.
- `make precommit` passes.
</summary>

<objective>
Expose the `GoalWorkOnOperation` as `vault-cli goal work-on <goal-name>`, registered in the `goal` command group, reusing the same mode-resolution and multi-vault dispatch as `task work-on`, with plain and `--output json` formatting. Thin adapter only — all behavior lives in the operation from prompt 2.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-cli-guide.md` — cobra command wiring conventions.

Read these files before implementing:
- `pkg/cli/cli.go`:
  - `createWorkOnCommand` (lines 308-375) — the reference command. Copy its structure: resolve `--mode` via `resolveSessionMode`, get current user via `(*configLoader).GetCurrentUser(ctx)`, get vaults via `getVaults`, dispatch via `ops.NewVaultDispatcher().FirstSuccess`, build the op inside the closure, compute `sessionDir` from `vault.GetSessionProjectDir()` falling back to `vault.Path`, then `formatWorkOnResult`.
  - `formatWorkOnResult` (lines 377-401) — REUSE as-is; it already prints name + assignee, warnings, and the session id, and handles JSON. Do not fork it; the goal command should call it too. (It prints "Now working on: <name>" — acceptable for goals; do NOT add a goal-specific formatter.)
  - `resolveSessionMode` (lines 403-419) — reuse.
  - `createGoalCommands` (lines 1215-1289) — the `goal` command group; add the new subcommand here alongside `createGoalCompleteCommand` / `createGoalDeferCommand` (around line 1287).
  - `createGoalCompleteCommand` (line 1291) and `createGoalDeferCommand` (line 1349) — reference for goal-command signature `(ctx, configLoader, vaultName, outputFormat)` and the `storage.NewConfigFromVault(vault)` / `storage.NewGoalStorage(storageConfig)` idiom.
- `pkg/ops/goal_workon.go` (from prompt 2) — `ops.NewGoalWorkOnOperation(goalStorage, starter, resumer)` and `Execute(ctx, vaultPath, goalName, assignee, vaultName, isInteractive, sessionDir, vault)`.
- `pkg/ops/claude_session.go` / `pkg/ops/claude_resume.go` — `ops.NewClaudeSessionStarter(vault.GetClaudeScript())` and `ops.NewClaudeResumer(vault.GetClaudeScript())` (both may return nil → soft failure, handled in the op).

Depends on prompt 2 (`ops.NewGoalWorkOnOperation`) — that prompt lands first.
</context>

<requirements>
1. Add `createWorkOnGoalCommand(ctx, configLoader, vaultName, outputFormat)` to `pkg/cli/cli.go`, structured like `createWorkOnCommand` but for goals:
   ```go
   func createWorkOnGoalCommand(
       ctx context.Context,
       configLoader *config.Loader,
       vaultName *string,
       outputFormat *string,
   ) *cobra.Command {
       var mode string
       cmd := &cobra.Command{
           Use:   "work-on <goal-name>",
           Short: "Mark a goal as in_progress and start a Claude session",
           Args:  cobra.ExactArgs(1),
           RunE: func(cmd *cobra.Command, args []string) error {
               goalName := args[0]
               isInteractive, err := resolveSessionMode(mode)
               if err != nil {
                   return err
               }
               currentUser, err := (*configLoader).GetCurrentUser(ctx)
               if err != nil {
                   return errors.Wrap(ctx, err, "get current user")
               }
               vaults, err := getVaults(ctx, configLoader, vaultName)
               if err != nil {
                   return errors.Wrap(ctx, err, "get vaults")
               }
               dispatcher := ops.NewVaultDispatcher()
               return dispatcher.FirstSuccess(ctx, vaults, func(vault *config.Vault) error {
                   starter := ops.NewClaudeSessionStarter(vault.GetClaudeScript())
                   resumer := ops.NewClaudeResumer(vault.GetClaudeScript())
                   storageConfig := storage.NewConfigFromVault(vault)
                   goalStore := storage.NewGoalStorage(storageConfig)
                   workOnOp := ops.NewGoalWorkOnOperation(goalStore, starter, resumer)
                   sessionDir := vault.Path
                   if dir := vault.GetSessionProjectDir(); dir != "" {
                       sessionDir = dir
                   }
                   result, err := workOnOp.Execute(
                       ctx,
                       vault.Path,
                       goalName,
                       currentUser,
                       vault.Name,
                       isInteractive,
                       sessionDir,
                       vault,
                   )
                   return formatWorkOnResult(result, err, currentUser, *outputFormat)
               })
           },
       }
       cmd.Flags().StringVar(&mode, "mode", "auto", "Session mode: auto, interactive, or headless")
       return cmd
   }
   ```
   Note: the goal operation takes NO `dailyNoteStorage` and NO `currentDateTime` — do not construct or pass them.

2. Register the command in `createGoalCommands`, alongside the other goal subcommands (near line 1287):
   ```go
   cmd.AddCommand(createWorkOnGoalCommand(ctx, configLoader, vaultName, outputFormat))
   ```

3. Add a cobra-registration test (so `make precommit` proves the command is wired, not just the manual `--help` smoke) in the appropriate `pkg/cli` test file, mirroring how other goal subcommands are covered if such a test exists: build the goal command group and assert it has a `work-on` subcommand (e.g. find the `goal` command via the root/`createGoalCommands`, then assert one of its `.Commands()` has `Use` starting with `work-on`). Keep it in the external `_test` package.

4. Add a CHANGELOG entry under `## Unreleased` in `CHANGELOG.md` (e.g. `- add(goal): \`vault-cli goal work-on <goal-name>\` marks a goal in_progress, applies the assignee-ownership rule, and starts/resumes a Claude session recorded in \`claude_session_id\` (mirrors \`task work-on\`, minus daily-note and phase steps)`). Do NOT bump any version strings — the release bot versions `## Unreleased` on merge.
</requirements>

<constraints>
- Reuse `formatWorkOnResult`, `resolveSessionMode`, `getVaults`, and `ops.NewVaultDispatcher` — do NOT duplicate or fork them.
- Do NOT change `createWorkOnCommand` (task) or any task behavior.
- Do NOT update daily notes or add phase handling — the goal operation has neither (spec Non-goals).
- Error handling: `github.com/bborbe/errors` wrapping with `ctx`; never `fmt.Errorf` in new code; never `context.Background()`.
- Soft failure (missing `claude`) must exit 0 with a warning; hard failure (zero-turn/rejected run) must exit non-zero — both come from the operation and `formatWorkOnResult`; do NOT re-interpret the error in the CLI.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `make precommit` — must pass (lint + format + generate + test + version checks).
Run `go build ./...` then `./vault-cli goal work-on --help` (or `go run . goal work-on --help`) — exit 0, stdout contains `work-on`.
Run `./vault-cli goal --help` — lists `work-on` under the goal group.
Run `grep -n "createWorkOnGoalCommand" pkg/cli/cli.go` — ≥2 lines (definition + registration).
</verification>
