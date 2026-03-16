---
status: completed
summary: 'Migrated all fmt.Fprintf(os.Stderr) calls to log/slog structured logging and added --verbose flag to control log level (default: warn, verbose: debug)'
container: vault-cli-063-migrate-to-slog
dark-factory-version: v0.57.3
created: "2026-03-16T15:19:37Z"
queued: "2026-03-16T15:19:37Z"
started: "2026-03-16T15:19:44Z"
completed: "2026-03-16T15:27:09Z"
---

<summary>
- Replace all raw stderr warning prints with structured logging using stdlib slog
- Add --verbose global flag (default: warnings only, verbose: debug messages visible)
- Storage walk noise (parse failures, symlinks) hidden by default, visible with --verbose
- Ops warnings (incomplete subtasks, unknown intervals) always visible
- Fatal error exit print in Execute() unchanged
</summary>

<objective>
Migrate vault-cli from raw fmt.Fprintf(os.Stderr) to log/slog structured logging. Add a --verbose flag so users control verbosity. Default level is Warn — in normal execution, only warnings and errors show. With --verbose, debug messages appear too. This eliminates noisy output when walking vaults with non-matching files.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Files to read before making changes (read ALL of these first):

- `pkg/cli/cli.go` — root command setup, `Run()`, `Execute()`, all `create*` functions. IMPORTANT: `PersistentPreRunE` already exists at ~line 54 for config loading — slog init must be merged into it, not overwrite it.
- `pkg/ops/watch.go` — 3 stderr prints for watch errors and missing dirs
- `pkg/ops/workon.go` — 2 stderr warning prints + 1 info print (Claude session)
- `pkg/ops/update.go` — 1 stderr warning print
- `pkg/ops/defer.go` — 2 stderr warning prints
- `pkg/ops/complete.go` — 5 stderr prints: 3 in Execute (~lines 134, 145, 268), 1 in checkSubtaskCompletion (~line 196), 1 in calculateNextDeferDate (~line 310)
- `pkg/storage/task.go` — 1 stderr print for failed task reads in ListTasks
- `pkg/storage/decision.go` — 3 stderr prints: symlink skip (~line 63), relative path failure (~line 69), frontmatter parse failure (~line 85)
- `pkg/storage/page.go` — 1 stderr print for failed page reads

Key logging rules (from go-logging-guide):
- Lowercase messages: `"processing prompt"` not `"Processing prompt"`
- Don't log + return error — do one or the other
- Key-value pairs: never `slog.Info(fmt.Sprintf(...))`
- Log at boundaries, not deep internals
</context>

<constraints>
- Use `log/slog` from stdlib — no external logging library
- Default log level: `slog.LevelWarn`
- `--verbose` flag: sets level to `slog.LevelDebug`
- Use structured key-value pairs: `slog.Debug("msg", "key", value)` not string formatting
- Keep `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` — that is the final error exit, not logging
- Do NOT add slog to test files
- Do NOT change any functional behavior — only replace print statements with slog calls
- Do NOT overwrite existing `PersistentPreRunE` — merge slog init into the existing hook
</constraints>

<requirements>

## 1. `pkg/cli/cli.go` — Add --verbose flag and slog init

Add `--verbose` persistent flag to root command. Add slog initialization into the EXISTING `PersistentPreRunE` (do not overwrite it).

```go
import "log/slog"

// Add flag:
var verbose bool
rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")

// Merge into existing PersistentPreRunE (which already initializes configLoader):
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    level := slog.LevelWarn
    if verbose {
        level = slog.LevelDebug
    }
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
    configLoader = config.NewLoader(configPath)
    return nil
},
```

Replace in `createDecisionListCommand`:
- `fmt.Fprintf(os.Stderr, "Warning: vault %s: %v\n", ...)` → `slog.Warn("vault error", "vault", vault.Name, "error", err)`

Keep unchanged:
- `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` in `Execute()` — stays as-is

## 2. `pkg/ops/watch.go` — 3 replacements

- `fmt.Fprintf(os.Stderr, "watch error: %v\n", watchErr)` → `slog.Warn("watch error", "error", watchErr)`
- `fmt.Fprintf(os.Stderr, "watch: skipping missing directory: %s\n", absDir)` → `slog.Debug("watch skipping missing directory", "dir", absDir)`
- `fmt.Fprintf(os.Stderr, "watch: failed to watch %s: %v\n", absDir, err)` → `slog.Warn("watch failed", "dir", absDir, "error", err)`

## 3. `pkg/ops/workon.go` — 3 replacements

- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` (both occurrences) → `slog.Warn("workon warning", "warning", warning)`
- `fmt.Fprintf(os.Stderr, "Starting Claude session for %s...\n", task.Name)` → `slog.Info("starting claude session", "task", task.Name)`

## 4. `pkg/ops/update.go` — 1 replacement

- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` → `slog.Warn("update warning", "warning", warning)`

## 5. `pkg/ops/defer.go` — 2 replacements

- `fmt.Fprintf(os.Stderr, "Warning: %s\n", w)` (both occurrences) → `slog.Warn("defer warning", "warning", w)`

## 6. `pkg/ops/complete.go` — 5 replacements

In `Execute` method:
- `fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)` (3 occurrences at ~lines 134, 145, 268) → `slog.Warn("complete warning", "warning", warning)`

In `checkSubtaskCompletion` (~line 196):
- `fmt.Fprintf(os.Stderr, "⚠️  Warning: %d/%d subtasks incomplete (%d pending, %d in-progress). Completing anyway.\n", ...)` → `slog.Warn("subtasks incomplete, completing anyway", "incomplete", pending+inProgress, "total", total, "pending", pending, "in_progress", inProgress)`

In `calculateNextDeferDate` (~line 310):
- `fmt.Fprintf(os.Stderr, "Warning: unknown recurring interval %q, treating as daily\n", recurring)` → `slog.Warn("unknown recurring interval, treating as daily", "interval", recurring)`

## 7. `pkg/storage/task.go` — 1 replacement

In `ListTasks`:
- `fmt.Fprintf(os.Stderr, "Warning: failed to read task %s: %v\n", fileName, err)` → `slog.Debug("skipping unreadable task", "file", fileName, "error", err)`

## 8. `pkg/storage/decision.go` — 3 replacements

In `ListDecisions` WalkDir callback:
- `fmt.Fprintf(os.Stderr, "Warning: skipping symlink outside vault %s\n", path)` → `slog.Debug("skipping symlink outside vault", "path", path)`
- `fmt.Fprintf(os.Stderr, "Warning: failed to get relative path for %s: %v\n", path, relErr)` → `slog.Debug("skipping file, failed to get relative path", "path", path, "error", relErr)`
- `fmt.Fprintf(os.Stderr, "Warning: failed to parse decision frontmatter %s: %v\n", path, decErr)` → `slog.Debug("skipping non-decision file", "path", path, "error", decErr)`

## 9. `pkg/storage/page.go` — 1 replacement

In `ListPages`:
- `fmt.Fprintf(os.Stderr, "Warning: failed to read page %s: %v\n", fileName, err)` → `slog.Debug("skipping unreadable page", "file", fileName, "error", err)`

</requirements>

<level-mapping>
| Old pattern | New slog level | Reason |
|---|---|---|
| "Warning: failed to parse/read ..." (storage) | slog.Debug | Expected for non-matching files during vault walk |
| "Warning: skipping symlink ..." | slog.Debug | Expected edge case, not actionable |
| "Warning: failed to get relative path ..." | slog.Debug | Expected edge case in WalkDir |
| "Warning: ..." (ops warnings) | slog.Warn | Actionable user warnings (lint issues, missing checkboxes) |
| "subtasks incomplete" | slog.Warn | User should know subtasks were skipped |
| "unknown recurring interval" | slog.Warn | User should know fallback occurred |
| "watch error: ..." | slog.Warn | Unexpected runtime error |
| "watch: skipping missing dir" | slog.Debug | Expected when vault dirs don't exist |
| "Starting Claude session" | slog.Info | User-facing progress |
| "Error: ..." in Execute() | Keep fmt.Fprintf | Fatal exit, not logging |
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
