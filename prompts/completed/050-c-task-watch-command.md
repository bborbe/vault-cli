---
status: completed
summary: Added vault-cli task watch streaming command with fsnotify-based file watching, debouncing, and newline-delimited JSON output
container: vault-cli-050-c-task-watch-command
dark-factory-version: v0.54.0
created: "2026-03-12T22:15:00Z"
queued: "2026-03-12T21:27:58Z"
started: "2026-03-12T21:35:59Z"
completed: "2026-03-12T21:43:56Z"
---

<summary>
- A new command watches vault task folders for file changes in real time
- Each change emits a JSON line to stdout with event type, task name, and vault
- The command runs until interrupted or stdin is closed
- Multiple vault folders are watched simultaneously when no vault flag is set
- Uses fsnotify for cross-platform filesystem notifications
- Also watches goals, themes, and objectives directories alongside tasks
</summary>

<objective>
Add `vault-cli task watch` streaming command that emits newline-delimited JSON events on stdout when task files change, enabling external tools to react to vault changes without their own file watching.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/cli/cli.go` — find `createTaskListCommand` as a pattern for task subcommands that iterate over vaults.
Read `pkg/config/config.go` — find `Vault` struct with directory accessors: `GetTasksDir()`, `GetGoalsDir()`, `GetThemesDir()`, `GetObjectivesDir()`.
The `github.com/fsnotify/fsnotify` package (import: `"github.com/fsnotify/fsnotify"`) is already in go.mod as indirect — it becomes direct after import.
</context>

<requirements>
1. Create `pkg/ops/watch.go` with a `WatchOperation` interface and implementation:

```go
type WatchOperation interface {
    Execute(ctx context.Context, vaults []WatchTarget) error
}

type WatchTarget struct {
    VaultPath      string
    VaultName      string
    WatchDirs      []string // built from GetTasksDir(), GetGoalsDir(), GetThemesDir(), GetObjectivesDir()
}
```

2. The `Execute` method:
   - Creates an `fsnotify.Watcher`
   - For each vault, adds all directories from `WatchTarget.WatchDirs` (resolved as `filepath.Join(vaultPath, dir)`)
   - Skip directories that don't exist (log warning to stderr, don't error)
   - Loops on watcher events, for each event emits a JSON line to stdout
   - Blocks until context is cancelled or an error occurs

3. Event JSON format (one line per event, newline-delimited):

```json
{"event":"modified","name":"IBKR Swing Trading","vault":"personal","path":"24 Tasks/IBKR Swing Trading.md"}
```

Fields:
   - `event`: one of `modified`, `created`, `deleted`, `renamed`
   - `name`: filename without `.md` extension
   - `vault`: vault name
   - `path`: relative path from vault root

4. Map fsnotify operations to event types:
   - `fsnotify.Write` → `modified`
   - `fsnotify.Create` → `created`
   - `fsnotify.Remove` → `deleted`
   - `fsnotify.Rename` → `renamed`
   - `fsnotify.Chmod` → ignore

5. Only emit events for `.md` files — ignore other file types.

6. Debounce rapid events for the same file — if multiple events fire for the same path within 100ms, emit only one event.

7. In `pkg/cli/cli.go`, create `createTaskWatchCommand` (no `outputFormat` param — this command always outputs streaming JSON):

```go
func createTaskWatchCommand(configLoader config.Loader, watchOp ops.WatchOperation) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "watch",
        Short: "Watch task folders for changes (streaming JSON output)",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load config, build WatchTarget list from vaults:
            // for each vault, WatchDirs = []string{
            //     vault.GetTasksDir(), vault.GetGoalsDir(),
            //     vault.GetThemesDir(), vault.GetObjectivesDir(),
            // }
            // Call watchOp.Execute(ctx, targets)
        },
    }
    return cmd
}
```

8. Register: `taskCmd.AddCommand(createTaskWatchCommand(...))`

9. Add tests in `pkg/ops/watch_test.go`:
   - Test event JSON format
   - Test that non-.md files are ignored
   - Test debouncing
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- All file paths are repo-relative
- Import `"github.com/fsnotify/fsnotify"` — already in go.mod as indirect, becomes direct after import
- The command must handle missing directories gracefully (skip with warning, don't error)
- Use `json.NewEncoder(os.Stdout)` for output — flush after each event
- The command runs indefinitely — it is the caller's responsibility to kill it
- Context cancellation (SIGINT/SIGTERM) must cleanly close the watcher
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
