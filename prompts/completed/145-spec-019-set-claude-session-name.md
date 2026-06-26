---
status: completed
spec: [019-set-claude-session-name-at-headless-create]
summary: Added `name string` parameter to `ClaudeSessionStarter.StartSession`, updated the concrete implementation to insert `-n <name>` args when non-empty, plumbed `task.Name` through `handleClaudeSession`, regenerated the counterfeiter mock, updated all tests including new naming boundary tests, and added a CHANGELOG Unreleased entry.
container: vault-cli-session-name-exec-145-spec-019-set-claude-session-name
dark-factory-version: v0.183.0
created: "2026-06-26T00:00:00Z"
queued: "2026-06-26T11:03:10Z"
started: "2026-06-26T11:03:12Z"
completed: "2026-06-26T11:05:37Z"
---
<summary>
- Headless work-on sessions created by `vault-cli task work-on --mode headless` are now named with the task title from turn 1, instead of an auto-generated snippet of the skill output.
- The task title is baked into the `claude --print` invocation via `-n "<task-name>"`, so the session's window title and agent name show the task immediately.
- Every later resume (backend builds, frontend builds, `/resume` picker, terminal title) inherits the name automatically — no per-resume flag needed.
- Purely an internal backend change: no new CLI flags, no change to any user-facing command surface.
- When a task has no name, behavior is byte-identical to before (no `-n` token is added).
- The session-starter mock is regenerated and existing tests are updated to match the new signature.
</summary>

<objective>
Make `vault-cli task work-on --mode headless` spawn `claude` with `-n "<task-name>"` so every headless session minted by vault-cli carries the task title in its `custom-title`/`agent-name` from the first turn. Pure backend change in `pkg/ops` — no CLI flag or command-surface change.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read /home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md for Ginkgo v2/Gomega test patterns.
Read /home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md for counterfeiter regeneration.
Read /home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md for changelog entry format.

Read these source files fully before editing:
- pkg/ops/claude_session.go — `ClaudeSessionStarter` interface + `claudeSessionStarter.StartSession`; the `//counterfeiter:generate` directive is on line 17.
- pkg/ops/workon.go — `handleClaudeSession` (calls `w.starter.StartSession(ctx, prompt, vaultPath)` at line 194); `task.Name` is available via the embedded `domain.FileMetadata.Name` (already used at lines 108/124/132/141/193).
- pkg/ops/claude_session_test.go — Ginkgo tests for `StartSession`; the "args passed to command runner" context (around line 145) asserts the exact arg slice.
- pkg/ops/workon_test.go — the `StartSessionArgsForCall(0)` destructure at line 188 currently unpacks 3 values.
- mocks/claude-session-starter.go — generated counterfeiter fake (DO NOT hand-edit; it will be regenerated).

`domain.FileMetadata.Name` is defined at pkg/domain/file_metadata.go:14 as `Name string` ("filename without the .md extension"). `domain.Task` embeds `FileMetadata` (pkg/domain/task.go:12), so `task.Name` resolves to that field.
</context>

<requirements>
1. In `pkg/ops/claude_session.go`, change the `ClaudeSessionStarter` interface method `StartSession` to take a `name string` parameter as the last argument. New signature:
   ```go
   // StartSession runs claude in headless mode to create a session, returns session_id.
   // When name is non-empty, the session is created with -n <name> so its
   // custom-title and agent-name are set from turn 1.
   StartSession(ctx context.Context, prompt string, cwd string, name string) (string, error)
   ```

2. In `pkg/ops/claude_session.go`, update the concrete `(*claudeSessionStarter).StartSession` method to match the new signature (add the `name string` parameter after `cwd string`).

3. In the same method, when `name != ""`, insert `-n` and `name` into `args` immediately after `--print` and BEFORE `-p`. When `name == ""`, the args slice must be byte-identical to the current output. Build the args like this (replacing the current literal `args := []string{...}`):
   ```go
   args := []string{
       c.claudePath,
       "--print",
   }
   if name != "" {
       args = append(args, "-n", name)
   }
   args = append(args, "-p", prompt, "--output-format", "json")
   ```
   Do NOT change the existing `if c.maxTurns > 0 { ... }` block, the timeout, the runCmd call, the JSON parsing, or any error wrapping.

4. In `pkg/ops/workon.go`, update `handleClaudeSession` to pass `task.Name` as the new `name` argument:
   - Change `sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath)` to
     `sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath, task.Name)`.
   - Pass `task.Name` verbatim — no trimming, escaping, or transformation.

5. Regenerate the counterfeiter fake. From the repo root run:
   ```
   go generate ./pkg/ops/...
   ```
   This rewrites `mocks/claude-session-starter.go` to a 4-argument `StartSession`. Do NOT hand-edit the generated file. If `go generate` is unavailable, the project's standard regen path is `make generate` (confirm via the Makefile) — use that instead. Verify the regenerated `StartSessionStub` is `func(context.Context, string, string, string) (string, error)` and `StartSessionArgsForCall` returns four values.

6. Update `pkg/ops/claude_session_test.go`:
   - Every existing `starter.StartSession(ctx, <prompt>, <cwd>)` call must gain a fourth argument. For all pre-existing contexts that do not test naming, pass `""` as the name so behavior stays identical (e.g. `starter.StartSession(ctx, "prompt", "/vault", "")`).
   - In the "args passed to command runner" context (around line 161), keep the existing assertion but call with `""` name, so the expected slice remains exactly:
     ```go
     []string{"/bin/claude", "--print", "-p", "my prompt", "--output-format", "json"}
     ```
   - ADD a new `Context` that exercises the boundary: call `StartSession(ctx, "my prompt", "/my/vault", "My Task Title")` and assert `capturedArgs` equals:
     ```go
     []string{"/bin/claude", "--print", "-n", "My Task Title", "-p", "my prompt", "--output-format", "json"}
     ```
     Use the same `NewClaudeSessionStarterWithRunner` + captured-args pattern already present in the "args passed to command runner" context.

7. Update `pkg/ops/workon_test.go`:
   - At line 188, the `StartSessionArgsForCall(0)` destructure currently unpacks 3 values (`_, prompt, _`). After regeneration it returns 4 values; change it to unpack 4 (e.g. `_, prompt, _, _ := mockStarter.StartSessionArgsForCall(0)`).
   - ADD an assertion (in the same or a new `It`) that the fourth captured argument equals the task's name, proving `task.Name` is plumbed through. Use the existing test task's name; capture it via `_, _, _, name := mockStarter.StartSessionArgsForCall(0)` and `Expect(name).To(Equal(<expected task name>))`. Determine the expected name by reading how the test task is constructed earlier in `workon_test.go` — use whatever `FileMetadata.Name` that task fixture sets.

8. Add a CHANGELOG entry. `CHANGELOG.md` currently has NO `## Unreleased` section (top versioned section is `## v0.86.0` at line 11). Insert a new `## Unreleased` section immediately after the intro block (after line 9, before `## v0.86.0`) with a single bullet:
   ```
   ## Unreleased

   - feat: Pass `-n "<task-name>"` to `claude` when `task work-on --mode headless` mints a session, so the session's custom-title and agent-name carry the task title from turn 1 (inherited by all later resumes)
   ```
   Do NOT bump any version number or touch the four version strings — this is a feature bullet under Unreleased; dark-factory handles release versioning.
</requirements>

<constraints>
- Copied from spec 019:
  - Must not change constructor signatures (`NewClaudeSessionStarter`, `NewClaudeSessionStarterWithRunner`).
  - Must not modify output parsing (the JSON `result` struct + unmarshal).
  - Must not change the 5-minute timeout or any error wrapping.
  - Must not introduce shell escaping — args go through the `exec.Command` argv array; `name` is a discrete slice element, never interpolated into a string.
  - Must not touch the warm-resume short-circuit in `handleClaudeSession` (`if existing := task.ClaudeSessionID(); existing != "" { return existing, nil }`).
  - Must not regress any existing test.
  - `task.Name` must be passed verbatim.
- When `name == ""`, the spawned args must be byte-identical to pre-spec output (the new test in requirement 6 second bullet guards this).
- Do NOT change any CLI flag, command definition, or public command surface — this is a backend-only change in `pkg/ops`.
- Do NOT hand-edit `mocks/claude-session-starter.go` — regenerate it.
- Do NOT commit — dark-factory handles git.
- Use `make test` iteratively for fast feedback; run `make precommit` once at the very end.
</constraints>

<verification>
Run `make test` after edits to confirm `pkg/ops` passes.
Confirm the regenerated `mocks/claude-session-starter.go` declares a 4-argument `StartSession` (grep: `grep -n "func (fake \*ClaudeSessionStarter) StartSession(" mocks/claude-session-starter.go` should show four `string`/`context.Context` params).
Confirm the new naming test asserts the `-n "My Task Title"` arg appears immediately after `--print` and before `-p`.
Confirm the empty-name test still asserts the original 6-element arg slice with no `-n` token.
Run `make precommit` — must exit 0. If it fails, fix the failing target and re-run only that target until green, then run `make precommit` once more.
</verification>
