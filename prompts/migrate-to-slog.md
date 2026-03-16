---
status: queued
---

<summary>
- Replace all `fmt.Fprintf(os.Stderr, ...)` logging with `log/slog` structured logging
- Add `--verbose` global flag to root command (sets slog level to Debug, default is Warn)
- Warnings become `slog.Warn`, debug/skip messages become `slog.Debug`, info messages become `slog.Info`
- The fatal error print in `Execute()` stays as `fmt.Fprintf(os.Stderr)` — it is not logging
- Initialize slog in `Run()` before command execution
</summary>

<objective>
Migrate vault-cli from raw `fmt.Fprintf(os.Stderr)` to `log/slog` structured logging. Add a `--verbose` flag so users control verbosity. Default level is `Warn` — in normal execution, only warnings and errors show. With `--verbose`, debug messages appear too.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — root command setup, `Run()`, `Execute()`, all `create*` functions
- `pkg/ops/watch.go` — stderr prints for watch errors
- `pkg/ops/workon.go` — stderr prints for warnings and Claude session info
- `pkg/ops/update.go` — stderr prints for warnings
- `pkg/ops/defer.go` — stderr prints for warnings
- `pkg/ops/complete.go` — stderr prints for warnings
- `pkg/storage/task.go` — stderr print for failed task reads
- `pkg/storage/decision.go` — stderr print for symlink skip + silent skip comment
- `pkg/storage/page.go` — stderr print for failed page reads

Logging conventions (read this guide):
- `~/.claude-yolo/docs/go-logging-guide.md`

Key rules from guide:
- Lowercase messages: `"processing prompt"` not `"Processing prompt"`
- Don't log + return error — do one or the other
- Key-value pairs: never `slog.Info(fmt.Sprintf(...))`
- Log at boundaries, not deep internals

Reference for slog pattern (dark-factory uses same approach):
```go
// In main or Run():
level := slog.LevelWarn
if verbose {
    level = slog.LevelDebug
}
slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
```
</context>

<rules>
- Use `log/slog` from stdlib — no external logging library
- Default log level: `slog.LevelWarn`
- `--verbose` flag: sets level to `slog.LevelDebug`
- Use structured key-value pairs: `slog.Debug("msg", "key", value)` not string formatting
- Keep `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` — that is the final error exit, not logging
- Keep `fmt.Fprintf(os.Stderr, "Starting Claude session for %s...\n", task.Name)` in workon.go as `slog.Info` — user-facing progress
- Do NOT add slog to test files
- Do NOT change any functional behavior — only replace print statements with slog calls
</rules>

<changes>

## 1. `pkg/cli/cli.go` — Add `--verbose` flag and slog init

Add `--verbose` persistent flag to root command. Initialize slog at the start of `Run()`.

```go
import "log/slog"

// In Run(), add before rootCmd.Execute():
var verbose bool
rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

// After flag parsing (use PersistentPreRunE on rootCmd):
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    level := slog.LevelWarn
    if verbose {
        level = slog.LevelDebug
    }
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
    return nil
}
```

Replace in `cli.go`:
- `fmt.Fprintf(os.Stderr, "Warning: vault %s: %v\n", ...)` → `slog.Warn("vault error", "vault", vault.Name, "error", err)`

Keep unchanged:
- `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` — stays as-is

## 2. `pkg/ops/watch.go`

Replace:
- `fmt.Fprintf(os.Stderr, "watch error: %v\n", watchErr)` → `slog.Warn("watch error", "error", watchErr)`
- `fmt.Fprintf(os.Stderr, "watch: skipping missing directory: %s\n", absDir)` → `slog.Debug("watch skipping missing directory", "dir", absDir)`
- `fmt.Fprintf(os.Stderr, "watch: failed to watch %s: %v\n", absDir, err)` → `slog.Warn("watch failed", "dir", absDir, "error", err)`

## 3. `pkg/ops/workon.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` (both) → `slog.Warn("workon warning", "warning", warning)`
- `fmt.Fprintf(os.Stderr, "Starting Claude session for %s...\n", task.Name)` → `slog.Info("starting Claude session", "task", task.Name)`

## 4. `pkg/ops/update.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` → `slog.Warn("update warning", "warning", warning)`

## 5. `pkg/ops/defer.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: %s\n", w)` (both) → `slog.Warn("defer warning", "warning", w)`

## 6. `pkg/ops/complete.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` (all three) → `slog.Warn("complete warning", "warning", warning)`

## 7. `pkg/storage/task.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: failed to read task %s: %v\n", fileName, err)` → `slog.Debug("skipping unreadable task", "file", fileName, "error", err)`

## 8. `pkg/storage/decision.go`

Replace the silent skip comment with slog:
- `// Silently skip files without valid frontmatter — they are not decisions` + bare `return nil` → `slog.Debug("skipping non-decision file", "path", path, "error", decErr)` then `return nil`
- `fmt.Fprintf(os.Stderr, "Warning: skipping symlink outside vault %s\n", path)` → `slog.Debug("skipping symlink outside vault", "path", path)`

## 9. `pkg/storage/page.go`

Replace:
- `fmt.Fprintf(os.Stderr, "Warning: failed to read page %s: %v\n", fileName, err)` → `slog.Debug("skipping unreadable page", "file", fileName, "error", err)`

</changes>

<level-mapping>
| Old pattern | New slog level | Reason |
|---|---|---|
| "Warning: failed to parse/read ..." (storage) | `slog.Debug` | Expected for non-matching files during vault walk |
| "Warning: skipping symlink ..." | `slog.Debug` | Expected edge case, not actionable |
| "Warning: ..." (ops warnings) | `slog.Warn` | Actionable user warnings (lint issues, missing checkboxes) |
| "watch error: ..." | `slog.Warn` | Unexpected runtime error |
| "watch: skipping missing dir" | `slog.Debug` | Expected when vault dirs don't exist |
| "Starting Claude session" | `slog.Info` | User-facing progress |
| "Error: ..." in Execute() | Keep `fmt.Fprintf` | Fatal exit, not logging |
</level-mapping>

<verification>
make precommit

# Verify no more fmt.Fprintf(os.Stderr) in pkg/ except Execute():
grep -rn 'fmt.Fprintf(os.Stderr' pkg/ | grep -v '_test.go'
# Expected: only pkg/cli/cli.go Execute() line

# Verify slog import in changed files:
grep -rn '"log/slog"' pkg/

# Verify --verbose flag works:
go run . task list --verbose 2>&1 | head -20
go run . task list 2>&1 | head -20
</verification>
