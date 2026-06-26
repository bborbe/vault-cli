---
status: prompted
approved: "2026-06-26T10:44:50Z"
generating: "2026-06-26T10:45:28Z"
prompted: "2026-06-26T10:53:39Z"
branch: dark-factory/set-claude-session-name-at-headless-create
---

## Summary

- Pass `-n <task-name>` to `claude --print -p …` when `vault-cli task work-on --mode headless` creates the headless session, so the session's `custom-title` + `agent-name` are baked in at creation time.
- All downstream resumes — backend builds, frontend builds, `/resume` picker, terminal title — inherit the name without needing `-n` on every invocation.
- Empirically verified: `claude --resume <id>` (no `-n`) preserves the prior `custom-title` and `agent-name`. Once set at creation, the name survives every resume.
- Pure backend change in `pkg/ops/claude_session.go` (args extension) + `pkg/ops/workon.go` (plumb `task.Name`). No CLI flag change, no public command surface change.

## Problem

vault-cli's headless-create path (`task work-on --mode headless`) spawns `claude --print -p /vault-cli:work-on-task <task.md> --output-format json` to mint a session_id. The claude process today receives no `-n`, so the session's name is auto-generated from the first user prompt (`ai-title`) — typically a snippet of the work-on-task skill output, not the task title.

Downstream consumers all suffer the same gap:
1. **task-orchestrator backend** (`POST /api/tasks/{id}/run`) → now appends `-n` on every resume command (shipped in task-orchestrator PR #12) — redundant work per Start click.
2. **task-orchestrator frontend Resume path** (`app.js:1002`) → builds the resume command client-side as `<script> --resume <id>` with no `-n`. Resume clicks lose the title from the picker / prompt box / terminal title — the bug surfaced live 2026-06-26.
3. **`claude /resume` picker** — same: shows `ai-title`, not the task.

Fixing it at the source (the headless create) closes all three gaps in one place. Empirical probe shows `claude --resume <id>` without `-n` preserves the prior `custom-title` — so the headless `-n` is sticky, no per-resume re-supply needed.

## Goal

After this work, `vault-cli task work-on --mode headless` spawns claude with `-n "<task-name>"` baked into the invocation. Every session minted by vault-cli carries the task title in its `custom-title` and `agent-name` records from turn 1. Any downstream consumer that runs `claude --resume <session_id>` — task-orchestrator backend, task-orchestrator frontend, the user typing `claude --resume <id>` by hand, the `/resume` picker — sees the task title without supplying `-n`.

## Non-goals

- No change to the public `vault-cli` command surface (no new flag, no renamed arg, no JSON output schema change). The fix is internal plumbing.
- No revert of task-orchestrator's PR #12 `-n` on the resume command. It becomes redundant-but-harmless belt-and-suspenders; a cleanup PR can drop it later if desired. Out of scope here.
- No change to `claude_session.go`'s timeout, output parsing, error wrapping, or counterfeiter contract semantics beyond the new parameter.
- No truncation / slugification / normalisation of `task.Name`. Pass through verbatim — claude's picker handles display truncation.
- No change to the warm-resume path (`handleClaudeSession`'s `existing := task.ClaudeSessionID()` early return). Already-created sessions keep whatever name they were minted with.
- No change to the non-headless modes (interactive `task work-on` without `--mode headless`).

## Acceptance Criteria

- [ ] `ClaudeSessionStarter.StartSession` accepts a `name string` parameter (kept positional for the small surface — the interface has only one caller in production code and one counterfeiter fake). When `name` is non-empty, the spawned `claude` args include `-n <name>` immediately after `--print`. Evidence: `cd ~/Documents/workspaces/vault-cli && go test ./pkg/ops/ -run TestStartSessionPassesName -v` reports `PASS`.
- [ ] `ClaudeSessionStarter.StartSession` with `name == ""` produces args byte-identical to the pre-spec output (no `-n` token). Evidence: `cd ~/Documents/workspaces/vault-cli && go test ./pkg/ops/ -run TestStartSessionEmptyNameOmitsFlag -v` reports `PASS`.
- [ ] `workon.go`'s `handleClaudeSession` passes `task.Name` (the filename without `.md`) to `StartSession`. Evidence: `cd ~/Documents/workspaces/vault-cli && go test ./pkg/ops/ -run TestWorkOnPassesTaskNameToStarter -v` reports `PASS`.
- [ ] The `mocks/claude-session-starter.go` counterfeiter fake is regenerated to match the new interface. Evidence: `cd ~/Documents/workspaces/vault-cli && go generate ./pkg/ops/... && git diff --stat mocks/claude-session-starter.go` shows the file is updated and `make precommit` passes — proving the regenerated fake matches the new signature.
- [ ] Every existing call site to `StartSession` is updated; no compilation errors. Evidence: `go build ./...` exits 0.
- [ ] `make precommit` exits 0 (vet + lint + full test suite + race detector).
- [ ] `CHANGELOG.md` has a new bullet under `## Unreleased` describing the fix. Evidence: `awk '/^## Unreleased/,/^## v/' CHANGELOG.md | grep -niE 'session.*name|claude.*-n' | head -1` returns ≥1 line.

## Verification

```
make precommit
```

## Desired Behavior

1. `ClaudeSessionStarter.StartSession` signature becomes `StartSession(ctx context.Context, prompt string, cwd string, name string) (string, error)` — `name` is the new positional parameter.
2. Inside `StartSession`, when `name != ""`, the `args` slice gains exactly two new elements — `-n` followed by `<name>` — immediately after `--print` (before `-p <prompt>`). When `name == ""`, no `-n` token and no empty placeholder is added; the args slice length and content are byte-identical to today's output.
3. `claude_session.go`'s constructors (`NewClaudeSessionStarter`, `NewClaudeSessionStarterWithRunner`) are unchanged — the name is per-call, not per-starter.
4. `workon.go`'s `handleClaudeSession` passes `task.Name` (the filename-derived `FileMetadata.Name`) as the `name` argument when calling `StartSession`.
5. The counterfeiter fake at `mocks/claude-session-starter.go` is regenerated via `go generate ./pkg/ops/...` so the signature compiles and existing test code that asserts `StartSessionCallCount`, `StartSessionArgsForCall(i)`, etc. continues to work (with the new 4-arg shape).
6. Existing test fixtures that call `StartSession` directly (mocks, table-driven harnesses) pass `""` for `name` to assert the empty-name fallback OR the relevant task name to assert the populated path.

## Constraints

- Must not change `ClaudeSessionStarter` constructor signatures (only the method signature).
- Must not modify the `claude --output-format json` parsing — output shape from claude is unchanged.
- Must not change the 5-minute timeout, error wrapping, or `errors.Wrap` paths.
- Must not modify the prompt string (`/vault-cli:work-on-task "/path/to/task.md"`) — that's the work-on-task skill's contract.
- Must not introduce shell escaping (claude is invoked via `exec.Command` argv array, not via a shell — the `name` string is passed as a discrete argv element, so no quoting needed).
- Must not touch the warm-resume short-circuit at `handleClaudeSession:186` (`existing := task.ClaudeSessionID()`).
- Must not regress any existing test (vault-cli has a substantial Go test suite + integration tests).
- `task.Name` is passed verbatim — no truncation, no slugification.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| `task.Name` is empty (filesystem race or upstream bug) | `StartSession` omits `-n`; claude defaults to `ai-title`. Behaviour byte-identical to pre-spec. | None — graceful degradation |
| `task.Name` contains a single quote / shell-meta / newline | Passed as a discrete argv element via `exec.Command`; no shell involved → no escaping issue. claude's `-n` parser handles the literal string. | None — argv-level safety |
| User's `claude` binary doesn't recognise `-n` (older than v2.1.187) | claude exits non-zero with unknown-flag error containing `unknown flag` or `unknown option` referencing `-n`; `StartSession` returns the wrapped error via `errors.Wrap(ctx, err, "run claude")`. Operator can self-diagnose via `claude --version` (must be ≥ v2.1.187). | Operator upgrades `claude` |
| Counterfeiter fake out of date after the signature change | `go build ./...` fails at compile time; CI catches it before merge. | `go generate ./pkg/ops/...` |
| External caller of `StartSession` (none today, but defensive) | Compile-time signature break — caller updates to the 4-arg form. | None — caught by `go build` |

## Do-Nothing Option

Without this work, the gap stays: every Resume click in task-orchestrator's frontend loses the title, every `claude /resume` picker entry shows `ai-title` instead of the task, and task-orchestrator's PR #12 carries a per-resume workaround in two languages (Python + JS) that has to be maintained in two places. Fixing at the source (vault-cli, one place) eliminates all three downstream gaps and makes the PR #12 `-n` truly belt-and-suspenders rather than load-bearing.

Cost of deferring: every Resume click in the UI presents an unnamed session in the picker; the user reverts to manual `/rename` exactly as before PR #12.

Cost of shipping: ~30 lines (StartSession signature + workon.go plumbing + 3 tests + regenerated fake + CHANGELOG bullet). Single-prompt, single-package scope.
