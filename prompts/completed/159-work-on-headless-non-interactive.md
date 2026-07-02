---
status: completed
summary: Appended `--non-interactive` to the headless bootstrap prompt in `handleClaudeSession` and added unit test verifying the flag is present and the task file path is preserved
execution_id: vault-cli-exec-159-work-on-headless-non-interactive
dark-factory-version: v0.191.0
created: "2026-07-02T18:15:00Z"
queued: "2026-07-02T17:51:16Z"
started: "2026-07-02T17:54:37Z"
completed: "2026-07-02T17:56:03Z"
---

<summary>
- Clicking "Start" on a task in the Vault UI can hang for five minutes and then fail with a "claude session start timed out" error whenever the task needs any input to get going.
- Root cause: the background step that opens the work session runs Claude in headless mode, where it cannot answer questions. If the work-on command stops to ask something, it waits for an answer that never comes until the timeout kills it.
- Fix: tell that headless step to run the work-on command in non-interactive mode, so it takes safe defaults instead of stopping to ask. The interactive part still happens later, when the session is resumed in a real terminal.
- Add a unit test proving the headless bootstrap now runs the command in non-interactive mode.
- After this change, starting a task from the Vault UI no longer hangs, and `make precommit` passes.
</summary>

<objective>
Stop the headless `work-on` Claude bootstrap from hanging on interactive prompts by appending `--non-interactive` to the bootstrap slash-command invocation in `pkg/ops/workon.go`, and add a unit test. No changes outside `pkg/ops/`; no interface or constructor signature changes.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, Counterfeiter mocks, external `_test` package convention.

Read these files before implementing:
- `pkg/ops/workon.go`:
  - `handleClaudeSession` (around line 179) — builds the bootstrap prompt at line 191 as `fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)` and calls `w.starter.StartSession(ctx, prompt, vaultPath, task.Name)`. This is the ONLY place the bootstrap prompt is built.
- `pkg/ops/claude_session.go`:
  - `StartSession` (line 69) — always shells out to `claude --print -p <prompt> --output-format json`, a headless turn that cannot answer `AskUserQuestion`. Read for understanding only; do NOT modify.
- `pkg/ops/workon_test.go` (around line 194) — the existing assertion on the bootstrap prompt uses `MatchRegexp(`^/custom-cmd "`)` (a prefix match), which still passes after a suffix is appended. Add a new assertion for the `--non-interactive` suffix here.
- `mocks/claude-session-starter.go` — Counterfeiter fake `ClaudeSessionStarter`; `StartSessionArgsForCall(0)` returns `(ctx, prompt, cwd, name)` — the prompt is the 2nd value.

Design note (resolved for this prompt):
- The append is UNCONDITIONAL. `StartSession` always runs headless `claude --print` to CREATE the session; that turn cannot answer `AskUserQuestion` even when vault-cli is in interactive mode (interactive sharpening happens afterward in `ResumeSession`, the sibling path in `pkg/ops/claude_resume.go`, which is correctly left alone). So the bootstrap must always be non-interactive.
- The default work-on command `/vault-cli:work-on-task` honors `--non-interactive` as of the plugin change that added non-interactive Phase 4 / Phase 5 gating (already merged to master). This Go change ONLY makes vault-cli *pass* the flag; the runtime contract — the command actually skipping its `AskUserQuestion` gates — lives in the slash-command definition and cannot be unit-tested from Go. Passing the flag to a custom `work_on_command` that ignores it is harmless (extra prompt text).
</context>

<requirements>
1. In `pkg/ops/workon.go` `handleClaudeSession`, append ` --non-interactive` to the bootstrap prompt:
   ```go
   // The bootstrap always runs headless `claude --print`, which cannot answer
   // AskUserQuestion; --non-interactive tells the work-on command to take safe
   // defaults instead of prompting (prevents the 5m headless hang).
   prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
   ```

2. In `pkg/ops/workon_test.go`, add an assertion that the prompt handed to the fake starter ends with `--non-interactive` and still contains the task file path — via `starter.StartSessionArgsForCall(0)` (the 2nd return value is the prompt). Keep the existing prefix assertion.

3. Add a CHANGELOG entry under `## Unreleased`. Do NOT bump the four version strings — the autoRelease bot versions `## Unreleased` on merge.
</requirements>

<constraints>
- Error handling: `github.com/bborbe/errors` wrapping with `ctx`; never `fmt.Errorf`; never `context.Background()` in `pkg/`.
- Do NOT change the `ClaudeSessionStarter` interface signature or any constructor.
- The append is unconditional (bootstrap is always headless) — do NOT thread `isInteractive` into `handleClaudeSession`.
- Do NOT modify `pkg/ops/claude_session.go` or the resume path (`pkg/ops/claude_resume.go`).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.

Non-goals: making the hardcoded 5-minute session-start timeout configurable is intentionally OUT OF SCOPE. Fix 1 above resolves the hang; a tunable timeout is a separate concern and, if ever needed, belongs in its own prompt with a named consumer.
</constraints>

<verification>
Run `make precommit` — must pass.
Run `make test` — unit suite passes.
Run `grep -n "non-interactive" pkg/ops/workon.go` — ≥1 line.
Run `grep -n "non-interactive" pkg/ops/workon_test.go` — ≥1 line (new assertion).

Note: `make precommit` / `make test` only prove the flag string is appended and tests compile. The end-to-end no-hang behavior depends on the deployed plugin's `/vault-cli:work-on-task` honoring `--non-interactive` (that handling is on master already). This Go-only prompt cannot exercise that boundary — confirm it at deploy time, where the slash-command runs.
</verification>
