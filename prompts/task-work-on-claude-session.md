---
status: created
created: "2026-03-12T20:30:00Z"
---

<summary>
- The work-on command now starts or resumes an AI coding session for the task
- If no session exists yet, one is created automatically in the background
- If a session already exists, it is resumed instead of starting a new one
- By default, the command detects whether the user is at a terminal and picks the right behavior
- A --mode flag lets the user override: "interactive" opens the session directly, "headless" prints the session identifier and exits
- When called from automation (no terminal), the session identifier is printed to stdout for downstream use
- If the AI tool is not installed, session management is skipped gracefully — the rest of work-on still works
</summary>

<objective>
Extend `vault-cli task work-on` so that starting work on a task also starts or resumes an AI coding session. This lets task-orchestrator track which tasks have active sessions and enables users to jump straight into a session from the CLI.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `pkg/ops/workon.go` — find `WorkOnOperation` interface and `workOnOperation.Execute` method. This is the current work-on logic (set status, assign user, update daily note).
Read `pkg/domain/task.go` — find `Task` struct with `ClaudeSessionID` field.
Read `pkg/cli/cli.go` — find `createWorkOnCommand` function (~line 273) that wires the cobra command.
</context>

<requirements>
1. Create a new `pkg/ops/claude_session.go` file with a `ClaudeSessionStarter` interface and implementation:

```go
type ClaudeSessionStarter interface {
    // StartSession runs claude in headless mode to create a session, returns session_id.
    StartSession(ctx context.Context, prompt string, cwd string) (string, error)
}
```

The implementation runs:
```
claude --print -p "<prompt>" --output-format json --max-turns 1
```
with the given `cwd` as the working directory. Parse the JSON response and extract the `session_id` field. Return error if `session_id` is empty or command fails.

2. Create a new `pkg/ops/claude_resume.go` file with a `ClaudeResumer` interface and implementation:

```go
type ClaudeResumer interface {
    // ResumeSession replaces the current process with an interactive claude --resume session.
    ResumeSession(sessionID string, cwd string) error
}
```

The implementation uses `syscall.Exec` to replace the current process with `claude --resume <sessionID>` in the given working directory. Look up the `claude` binary path via `exec.LookPath("claude")`.

3. Modify `WorkOnOperation` to accept optional `ClaudeSessionStarter` and `ClaudeResumer` dependencies. Add an `isInteractive bool` parameter to `Execute` (or pass it via a new options struct). Update the interface and `NewWorkOnOperation` constructor accordingly.

4. After the existing work-on logic (set status, assign, update daily note), add Claude session handling at the end of `Execute`:

```
if task.ClaudeSessionID == "" {
    // No existing session — start a new headless session
    prompt := fmt.Sprintf(`/work-on-task "%s"`, taskFilePath)
    sessionID, err := starter.StartSession(ctx, prompt, vaultPath)
    // Save session ID to task frontmatter
    task.ClaudeSessionID = sessionID
    w.storage.WriteTask(ctx, task)
}

if isInteractive {
    // TTY mode — exec into claude, replacing current process
    resumer.ResumeSession(task.ClaudeSessionID, vaultPath)
} else {
    // Non-interactive — print session ID to stdout
    // JSON mode: include session_id in MutationResult
    // Plain mode: print "session_id: <id>"
}
```

5. In `createWorkOnCommand` in `pkg/cli/cli.go`, add a `--mode` string flag with three valid values:

- `"auto"` (default) — detect from TTY: `term.IsTerminal(int(os.Stdin.Fd()))`
- `"interactive"` — force interactive (exec into claude)
- `"headless"` — force headless (print session ID and exit)

```go
import "golang.org/x/term"

var mode string
cmd.Flags().StringVar(&mode, "mode", "auto", "Session mode: auto, interactive, or headless")
```

Resolve the effective mode before calling Execute:

```go
isInteractive := false
switch mode {
case "interactive":
    isInteractive = true
case "headless":
    isInteractive = false
case "auto":
    isInteractive = term.IsTerminal(int(os.Stdin.Fd()))
default:
    return fmt.Errorf("invalid --mode value: %s (must be auto, interactive, or headless)", mode)
}
```

Pass `isInteractive` to the operation. Wire `ClaudeSessionStarter` and `ClaudeResumer` implementations.

6. The task file path for the prompt comes from `task.FilePath` (set by `storage.FindTaskByName` during resolution). Use it directly — do not reconstruct the path manually.

7. Update `MutationResult` in `pkg/ops/complete.go` (~line 50) to include an optional `SessionID string `json:"session_id,omitempty"`` field for JSON output.

8. Add tests in `pkg/ops/claude_session_test.go` and `pkg/ops/claude_resume_test.go`:
   - Test `StartSession` with a mock command runner (don't call real `claude`)
   - Test JSON parsing of session_id from claude output
   - Test error handling for missing session_id, non-zero exit code
   - For `ResumeSession`, test that the correct args are constructed (mock syscall.Exec)

9. Update tests in `pkg/ops/workon_test.go` for the new dependencies and session flow.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- All file paths are repo-relative
- `golang.org/x/term` is already in go.mod (indirect) — use it directly
- The `claude` binary may not be installed — if `exec.LookPath("claude")` fails, skip session handling with a warning (don't error)
- `syscall.Exec` replaces the process — any code after it never runs; handle errors and cleanup before calling it
- The headless `claude --print` call may take 30+ seconds — use a 60s timeout via `context.WithTimeout`; on timeout return a clear error ("claude session start timed out after 60s")
- Task domain model: `ClaudeSessionID string` field with YAML tag `claude_session_id,omitempty`
- Non-interactive mode must work when called from task-orchestrator (stdout is captured)
- When `ClaudeSessionStarter` or `ClaudeResumer` is nil (claude not available), skip session handling gracefully
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
