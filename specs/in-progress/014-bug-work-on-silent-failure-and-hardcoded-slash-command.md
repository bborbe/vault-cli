---
status: verifying
approved: "2026-05-24T13:56:58Z"
generating: "2026-05-24T13:56:59Z"
prompted: "2026-05-24T14:05:29Z"
verifying: "2026-05-24T14:35:01Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

## Summary

- `vault-cli task work-on --mode headless` returns a session_id even when the underlying `claude --print` call fails to execute the slash command
- The slash command sent to claude is hardcoded to `/work-on-task`, but the actual command in this user's setup is `/vault-cli:work-on-task`
- Callers (task-orchestrator UI) see a successful response, store the dead session_id, show "Session Ready" to the user
- When the user runs the printed `claude --resume <id>` command, claude reports `No conversation found with session ID: …` because the headless run never produced a real conversation
- Fix: surface claude headless failures as errors, AND make the slash command configurable per vault

## Problem

A user pressed "Start" on a task in task-orchestrator. The UI showed "Session Ready" with a copy-paste command. Running the command in a terminal printed `No conversation found with session ID: 2c6fe05a-3b19-4c32-8c2d-2e052b85b74e`. The work-on flow failed end-to-end without any error surfacing to the user.

Root cause is two coupled defects in vault-cli:

1. `pkg/ops/workon.go:154` hardcodes the slash command as `/work-on-task`. The current Claude Code slash command for this functionality is `/vault-cli:work-on-task` (renamed under the `vault-cli:` namespace). Claude responds: `Unknown command: /work-on-task` with `num_turns: 0`.
2. `pkg/ops/claude_session.go` reads only `session_id` from claude's JSON output. It does not inspect `is_error`, `subtype`, `num_turns`, or `result`. A zero-turn run with an error message still returns a session_id, which vault-cli accepts and returns to the caller as if everything worked.

The session_id IS valid (claude wrote a `.jsonl` file with `{"type":"ai-title", …}`), but the file contains no user/assistant messages, so `claude --resume` later refuses to resume it.

## Goal

`vault-cli task work-on --mode headless` produces a resumable Claude session OR returns a non-zero exit code with an error message that identifies the cause (slash command unknown, network error, no turns executed). The slash command sent to claude is configurable per vault and defaults to `/vault-cli:work-on-task`.

## Non-goals

- Fixing `--mode interactive` behavior — interactive sessions launch a real claude REPL and were never affected by this bug.
- Validating that the returned session_id is later resumable — that's claude's job; vault-cli's contract ends at "I called claude, here's the session it reported."
- Auto-migrating existing `~/.vault-cli/config.yaml` files — unset `work_on_command` falls back to the new default.
- Updating task-orchestrator UI copy — separate downstream change (its error toast already surfaces stderr from vault-cli when the call exits non-zero).
- Renaming the `claude_script` config field or otherwise reshaping unrelated vault config.

## Do-Nothing Option

Leaving the bug in place costs ~5–15 min per occurrence: user clicks Start, copies the printed command, runs it, gets `No conversation found`, has to manually debug. The hardcoded slash command means every vault using the new `vault-cli:` namespace silently breaks until vault-cli is patched; downstream tools (task-orchestrator) cannot work around it because they have no way to know vault-cli's success was a lie. Workaround for the user is to bypass `task work-on` entirely and run claude directly with the right slash command — defeats the orchestration value. Not fixing also degrades trust in vault-cli's exit codes for any future automation that depends on them. Cost of fix is low (3 files, all in vault-cli, well-isolated). Strongly prefer fixing.

## Reproduction

dark-factory version: not relevant — bug is in vault-cli.
vault-cli HEAD: master at time of filing.

Smallest config (`~/.vault-cli/config.yaml`):

```yaml
vaults:
  - name: Personal
    path: /Users/bborbe/Documents/Obsidian/Personal
    claude_script: /Users/bborbe/Documents/workspaces/scripts/claude-obsidian-personal.sh
```

Exact command sequence:

```bash
cd ~/Documents/Obsidian/Personal
vault-cli task work-on "ORB DE40 W21 Sunday Review and Extend Closing to W22" \
  --mode headless --vault Personal --output json
```

Observed evidence (verbatim from `/tmp/task-orchestrator.log` at 14:59:17 and terminal):

```
{"session_id":"2c6fe05a-3b19-4c32-8c2d-2e052b85b74e", ...}   # vault-cli returns success
```

```text
$ /Users/bborbe/Documents/workspaces/scripts/claude-obsidian-personal.sh \
    --resume 2c6fe05a-3b19-4c32-8c2d-2e052b85b74e
No conversation found with session ID: 2c6fe05a-3b19-4c32-8c2d-2e052b85b74e
```

Direct reproduction of claude's response (bypassing vault-cli):

```bash
cd ~/Documents/Obsidian/Personal
/Users/bborbe/Documents/workspaces/scripts/claude-obsidian-personal.sh \
  --print -p '/work-on-task "24 Tasks/ORB DE40 W21 Sunday Review and Extend Closing to W22.md"' \
  --output-format json
```

Returns:

```json
{"type":"result","subtype":"success","is_error":false,"duration_ms":6,
 "num_turns":0,"result":"Unknown command: /work-on-task",
 "session_id":"1b08ddc8-bc8b-46f6-9930-225064545f8d", ...}
```

`num_turns: 0` and `result: "Unknown command: /work-on-task"` indicate the slash command was rejected, yet `is_error: false` and `subtype: "success"` mask the failure.

The resulting `.jsonl` file contains only:

```json
{"type":"ai-title","aiTitle":"...","sessionId":"1b08ddc8-..."}
```

— no user or assistant message, hence `--resume` fails.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Slash command sent | `/vault-cli:work-on-task` (configurable, this default) | `/work-on-task` (hardcoded) |
| When claude returns `num_turns: 0` | vault-cli returns non-zero exit + error referencing claude's `result` field | vault-cli returns 0, prints session_id as if success |
| When claude returns `is_error: true` | Same — surface as error | Same — silently swallowed |
| When the session file lacks user messages | `--resume` later fails (caller's burden — out of vault-cli's scope) | Same (no change needed here) |

Expected behavior is per `pkg/ops/workon.go` and `pkg/ops/claude_session.go` docstrings — `StartSession` "runs claude in headless mode to create a session." A session that cannot be resumed is not a created session.

## Why this is a bug

`StartSession`'s contract (`pkg/ops/claude_session.go:21-22`) promises a session was created. Returning a session_id for a zero-turn run that produced no resumable conversation violates that contract. The downstream caller (task-orchestrator) trusts the success and displays "Session Ready" to the user — a misleading message that costs the user time to discover the failure manually.

The hardcoded `/work-on-task` is a separate defect: the slash command namespace was reorganized (commands are now under `vault-cli:` prefix), and vault-cli still references the pre-rename name. Any vault using the new namespace silently breaks.

## Constraints

- `--mode interactive` behavior must not regress — interactive sessions launch the user into a real claude REPL and were never affected by this bug.
- `--output json` and `--output plain` formats must continue to emit the existing fields on success; only the error path changes.
- Existing `claude_script` config field semantics unchanged.
- Existing tests in `pkg/ops/workon_test.go` and `pkg/ops/claude_session_test.go` must continue to pass (update where the contract changes; do not delete).
- Vaults that do not set the new config field must default to `/vault-cli:work-on-task` — no required config migration.
- `StartSession`'s Go signature (`func StartSession(ctx context.Context, prompt string, cwd string) (string, error)`) MUST NOT change — the error return already exists. Only the error-return *conditions* tighten. Downstream callers (`task-orchestrator`) keep compiling without modification.

## Failure Modes

| Trigger | Detection | Expected behavior | Recovery |
|---|---|---|---|
| Claude returns `num_turns: 0` with non-empty `result` | Parse claude JSON output | `StartSession` returns `errors.Wrap(ctx, …, "claude returned 0 turns: %s", result)` and non-zero exit from CLI | User reads error, fixes config or slash command, re-runs |
| Claude returns `is_error: true` | Parse claude JSON output | `StartSession` returns wrapped error including `result` field | User reads error, addresses cause |
| Claude returns valid session with ≥1 turn | Parse claude JSON output | Existing happy path — return session_id | n/a |
| Vault config sets `work_on_command: /custom-command` | Config load | `workon.go` sends `/custom-command "…"` to claude | n/a — intentional |
| Vault config omits `work_on_command` | Config load | `GetWorkOnCommand()` returns `/vault-cli:work-on-task` | n/a — default |
| Claude binary missing | `exec.LookPath` in `NewClaudeSessionStarter` | Existing behavior — `starter` is nil, `handleClaudeSession` returns `claude session starter unavailable` | Unchanged |

## Acceptance Criteria

- [ ] `pkg/config/config.go` defines a `WorkOnCommand string` field on `Vault` with `yaml:"work_on_command,omitempty"` and `json:"work_on_command,omitempty"` tags — evidence: `grep -n 'WorkOnCommand' pkg/config/config.go` returns ≥1 line.
- [ ] `Vault.GetWorkOnCommand()` returns `v.WorkOnCommand` if non-empty, else `/vault-cli:work-on-task` — evidence: table test in `pkg/config/vault_test.go` asserts both branches; `go test ./pkg/config/... -run GetWorkOnCommand` exits 0.
- [ ] `pkg/ops/workon.go:154` uses `vault.GetWorkOnCommand()` instead of the literal `/work-on-task` — evidence: `grep -n '"/work-on-task"' pkg/ops/workon.go` returns 0 lines; `grep -n 'GetWorkOnCommand' pkg/ops/workon.go` returns ≥1 line.
- [ ] `pkg/ops/claude_session.go` `StartSession` parses `num_turns`, `is_error`, `subtype`, `result` from claude's JSON output — evidence: `grep -nE '"num_turns"|"is_error"|"result"' pkg/ops/claude_session.go` returns ≥3 lines.
- [ ] `StartSession` returns a non-nil error when `num_turns == 0` OR `is_error == true`. The error message includes the `result` field text — evidence: unit test in `pkg/ops/claude_session_test.go` mocks claude returning `{"num_turns":0,"result":"Unknown command: /x","session_id":"abc",...}` and asserts the returned error's `Error()` contains `Unknown command: /x`.
- [ ] `StartSession` does NOT error on the happy path: a mocked claude response with `num_turns: 3`, `is_error: false`, `result: "done"`, and a non-empty `session_id` returns that `session_id` and nil error — evidence: explicit named test case in `pkg/ops/claude_session_test.go` (`it("returns session_id when num_turns >= 1 and is_error is false")` or equivalent Ginkgo `It`) that `go test -run StartSession -v` lists as passing.
- [ ] Existing happy-path unit tests still pass — evidence: `go test ./pkg/ops/... -run StartSession -v` exits 0 and `-v` output lists ≥2 distinct passing test cases, including the named happy-path case above.
- [ ] **Runtime repro — forced unknown command** (does not depend on host Claude Code state): create a throwaway vault config with `work_on_command: /definitely-not-a-real-command-xyz` and run `vault-cli task work-on <task> --mode headless --vault <throwaway>`. Evidence: command exits non-zero AND stderr matches the regex `Unknown command|claude returned 0 turns`. Repro script lives in the prompt body, not the spec.
- [ ] **Runtime repro — happy path** (depends on the filer's Claude Code having `/vault-cli:work-on-task` registered; mark as manual verification on the filer's machine): on the user's actual machine, running `vault-cli task work-on "ORB DE40 W21 Sunday Review and Extend Closing to W22" --mode headless --vault Personal --output json` exits 0 AND running the printed `claude --resume <session_id>` enters interactive mode AND does NOT print `No conversation found`. Evidence: filer pastes the terminal session into the verify-spec output.
- [ ] `make precommit` exits 0.
- [ ] CHANGELOG.md has a new `## Unreleased` section listing both fixes — evidence: `grep -nE '^## Unreleased' CHANGELOG.md` returns ≥1 line AND `awk '/^## Unreleased/,/^## v/' CHANGELOG.md` includes the substrings `work_on_command` AND (`StartSession` OR `silent`).

## Verification

```bash
cd ~/Documents/workspaces/vault-cli
make precommit          # full check
go test ./pkg/config/... ./pkg/ops/...
```

Then the reproduction in this spec — see Acceptance Criteria for the runtime checks.

## Open Questions

None — both fixes are scoped to vault-cli and the user has confirmed both the rename (to `/vault-cli:work-on-task`) and the desired configurability default.
