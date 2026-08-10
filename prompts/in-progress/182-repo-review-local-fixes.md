---
status: approved
created: "2026-08-10T12:36:04Z"
queued: "2026-08-10T12:36:04Z"
---

<summary>
- A security suppression that currently says nothing now records why it is safe.
- Two loops that touch one file per iteration across a whole vault can be interrupted, instead of running to the end.
- The external search subprocess leaves a log line, so a failed search is diagnosable.
- "Not found in any vault" now says which vaults were searched.
- No behaviour changes beyond cancellation; no signatures change.
</summary>

<objective>
Apply four independent, self-contained code-review fixes in `/workspace`: findings M1 (unjustified `#nosec`), M2 (missing cancellation guards), S1 (unlogged external call) and S2 (error wrap missing vault names).
</objective>

<context>
Read `/workspace/CLAUDE.md` if present, otherwise `/workspace/docs/` for project conventions — this repo uses `github.com/bborbe/errors` for wrapping and `log/slog` for logging.

Read these files IN FULL before editing:
- `/workspace/pkg/ops/search.go` — `#nosec G204` at the `exec.CommandContext` line, and the whole `Execute` method.
- `/workspace/pkg/ops/claude_session.go` — **reference for the `#nosec` fix only.** Its suppression at ~line 58 reads `//#nosec G204 -- args[0] is the claude binary path from LookPath`. Match that comment style. NOTE: this file contains no logging at all — do not look for a logging pattern in it.
- `/workspace/pkg/ops/goal_workon.go` (~line 180) and `/workspace/pkg/ops/watch.go` (~lines 86, 103) — **reference for the logging fix.** These show the project's `slog` call style.
- `/workspace/pkg/ops/ensure_task_identifiers.go` — the `for _, task := range tasks` loop calling `WriteTask` per task.
- `/workspace/pkg/storage/page.go` — the `for _, entry := range entries` loop calling `readPageFromPath` (`os.ReadFile` + `os.Stat`) per entry.
- `/workspace/pkg/ops/vault_dispatcher.go` — the loop over `vaults` and the final `errors.Wrap(ctx, lastErr, "not found in any vault")`.
</context>

<requirements>
1. **M1** — in `/workspace/pkg/ops/search.go`, extend the `// #nosec G204` comment on the `exec.CommandContext(ctx, "semantic-search-mcp", "search", query)` line with a concrete justification. State that the arguments are passed via `exec.Cmd.Args` and never through a shell, so shell metacharacters in `query` are not interpreted. Match the comment style used by the `#nosec` in `claude_session.go`.
2. **M2a** — in `/workspace/pkg/ops/ensure_task_identifiers.go`, add a non-blocking cancellation check at the TOP of the `for _, task := range tasks` loop body:
   ```go
   select {
   case <-ctx.Done():
       return errors.Wrap(ctx, ctx.Err(), "context cancelled")
   default:
   }
   ```
   `Execute` returns a `BackfillResult` accumulated across the loop. On cancellation return the **zero-value** `BackfillResult{}` alongside the error, discarding partial progress — matching the precedent in `pkg/storage/decision.go`, where the guarded helper returns `nil` outright. Do not return the partially-accumulated result.
3. **M2b** — same guard at the TOP of the `for _, entry := range entries` loop body in `/workspace/pkg/storage/page.go`, again matching that function's return signature.
4. **S1** — in `/workspace/pkg/ops/search.go`, add `log/slog` logging around the `semantic-search-mcp` subprocess call: one line on failure including the error, and one on success including the result count. Do not log the full query text — the vault path and result count are the useful fields. Match the `slog` call style in `goal_workon.go` / `watch.go`; add the `log/slog` import.
5. **S2** — in `/workspace/pkg/ops/vault_dispatcher.go`, collect each attempted `vault.Name` into a slice inside the loop and change the final wrap to `errors.Wrapf(ctx, lastErr, "not found in any vault (tried: %s)", strings.Join(names, ", "))`. Add the `strings` import if absent.
6. Do NOT change any function signature or interface. Findings M4 (threading ctx into domain setters) and S4 (splitting storage interfaces) are deliberately OUT OF SCOPE for this prompt.
7. Do NOT touch any file marked `// Code generated ... DO NOT EDIT.`
8. Add a `## Unreleased` section at the top of `/workspace/CHANGELOG.md` if one does not already exist, with one bullet per fix, matching the style of the entries below it.
</requirements>

<verification>
- `cd /workspace && make precommit` exits 0.
- `grep -n 'nosec G204' /workspace/pkg/ops/search.go` shows a comment with explanatory text beyond the bare directive.
- `grep -c 'ctx.Done()' /workspace/pkg/ops/ensure_task_identifiers.go` is at least 1.
- `grep -c 'ctx.Done()' /workspace/pkg/storage/page.go` is at least 1.
- `grep -cE 'slog\.(Debug|Info|Warn|Error)' /workspace/pkg/ops/search.go` is at least 2.
- `grep -n 'tried:' /workspace/pkg/ops/vault_dispatcher.go` returns a match.
- `grep -n '## Unreleased' /workspace/CHANGELOG.md` returns exactly one match.
- `cd /workspace && git diff --name-only` lists ONLY: `CHANGELOG.md`, `pkg/ops/ensure_task_identifiers.go`, `pkg/ops/search.go`, `pkg/ops/vault_dispatcher.go`, `pkg/storage/page.go`.
</verification>

<allowed_files>
- /workspace/pkg/ops/search.go
- /workspace/pkg/ops/ensure_task_identifiers.go
- /workspace/pkg/storage/page.go
- /workspace/pkg/ops/vault_dispatcher.go
- /workspace/CHANGELOG.md
</allowed_files>
