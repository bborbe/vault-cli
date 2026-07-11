---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-07-11T08:19:17Z"
generating: "2026-07-11T08:27:38Z"
prompted: "2026-07-11T08:27:38Z"
verifying: "2026-07-11T08:42:03Z"
completed: "2026-07-11T08:52:35Z"
branch: dark-factory/goal-work-on
---

## Summary

- Add a `vault-cli goal work-on <goal-name>` command that mints (or resumes) a headless Claude session for a goal, mirroring the existing `task work-on`.
- Goals gain a `claude_session_id` frontmatter field with typed accessors — today only tasks carry one.
- The command sets the goal to `in_progress`, applies the same assignee-ownership rule as tasks, writes the goal, then starts a Claude session and records the returned session id on the goal.
- A rejected / zero-turn Claude run is a hard failure (non-zero exit); a missing `claude` binary is a soft failure (warning, exit 0) — the exact invariants the task command already guarantees.
- This is the missing Go primitive that unblocks the vault-ui goal Start/Resume button. It deliberately omits the daily-note step (goals aren't tracked on daily notes) and phase advancement (goals have no phase).

## Problem

The vault UI wants goal cards to have the same ▶ Start/Resume session button that task cards have, so a person can launch a Claude working session on a goal directly from the board. `task work-on` already mints a headless Claude session and records the session id back onto the task, but goals have no equivalent primitive — there is no command that starts a session for a goal or persists its `claude_session_id`. Without this primitive the UI work cannot proceed, and goal/task session handling stays asymmetric.

## Goal

After this work, a person or autonomous agent can run `vault-cli goal work-on <goal-name>` and have the named goal marked `in_progress`, assigned to the current user (subject to the ownership rule), and backed by a Claude session whose id is persisted in the goal's frontmatter. Re-running the command on a goal that already has a session id resumes/returns that session instead of minting a new one. The behavior matches `task work-on` in every respect except the three explicitly-excluded steps below.

## Non-goals

- Do NOT update daily notes — goals are not tracked on daily notes; that entire step from `task work-on` is omitted for goals.
- Do NOT add phase advancement — goals have no `phase` field; there is no planning-phase transition.
- Do NOT build the vault-ui Start/Resume button or any HTTP run/session endpoints — that is a separate task ([[Add Goal Start-Resume Session Button to Vault UI]]).
- Do NOT add a per-goal opt-out flag for session minting — invariant; if a future consumer needs a no-session variant, that's a separate spec.
- Do NOT change `task work-on`, its config field, or the shared `ClaudeSessionStarter` / `ClaudeResumer` / goal storage — reuse them as-is.

## Desired Behavior

1. `vault-cli goal work-on <goal-name>` finds the goal by name in the resolved vault(s) and sets its status to `in_progress`.
2. The command applies the assignee-ownership rule identically to `task work-on`: blank assignee → set to current user; already current user → unchanged; owned by a different non-blank user → left unchanged plus a warning naming both users. Status is set to `in_progress` regardless of the assignee outcome.
3. The (possibly-modified) goal is written back before any session work, preserving all unknown frontmatter fields and the markdown body.
4. If the goal already has a non-empty `claude_session_id`, the command short-circuits: no new session is started and the existing id is returned.
5. Otherwise the command starts a headless Claude session using the prompt built from the configured work-on-goal command, the goal's file path, and a trailing `--non-interactive`; the returned session id is written back onto the goal's `claude_session_id`.
6. A Claude run that is rejected or produces zero turns is a hard failure: the command reports failure and exits non-zero, while the goal remains `in_progress` (the status write already happened). A missing `claude` binary is a soft failure: a warning is emitted and the command exits 0.
7. `--output json` returns the structured result including the session id; plain output prints the goal name, any warnings, and the session id when present.
8. Goals expose `claude_session_id` through the same generic frontmatter get/set surface as every other known goal field.

## Constraints

- Reuse `FindGoalByName` / `WriteGoal` (`pkg/storage/goal.go`) and the `ClaudeSessionStarter` / `ClaudeResumer` (`pkg/ops/claude_session.go`) unchanged.
- `pkg/ops/` stays a library layer — the new operation returns a structured result and must not write to stdout; the CLI layer owns all output formatting.
- The new config field defaults to `/vault-cli:work-on-goal`; the existing `work_on_command` field and its `/vault-cli:work-on-task` default are untouched.
- Follow repo conventions: `github.com/bborbe/errors` only (no `fmt.Errorf`, no bare `return err`, no `context.Background()` in ops), `libtime` types for time, factory functions are pure composition (no conditionals/I-O), Ginkgo v2 + Gomega tests, Counterfeiter mocks in `mocks/`.
- Unknown frontmatter fields and the markdown body of a goal survive the read-write cycle.
- Existing tests must continue to pass; `task work-on` behavior is unchanged.
- The hard-fail-on-rejected-run invariant must match the task command (the guarantee added in the v0.66.13 task fix): a zero-turn/rejected Claude run must not be silently swallowed.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---------|-------------------|----------|-----------|---------------|
| Goal name not found | Command reports not-found and exits non-zero; no goal is written, no session started | Re-run with a correct name | Non-zero exit + error message | N/A (no write) |
| Goal already has non-empty `claude_session_id` | No new session started; existing id returned; exit 0 | None needed | Plain/JSON output shows the existing session id; starter call count is 0 | N/A |
| `claude` binary missing (starter unavailable) | Soft failure: goal is still set `in_progress` and written; warning emitted; exit 0; empty session id | Install `claude`, re-run to mint the session | Warning line "claude session: ... unavailable"; exit 0 | Reversible (re-run mints session) |
| Claude returns zero turns / rejected run | Hard failure: command exits non-zero; goal remains `in_progress` (status write already committed); no session id persisted | Fix the underlying command/prompt, re-run | Non-zero exit + error containing the zero-turns message | Partial — status already flipped; session not created |
| Goal write fails (disk/permission) | Command exits non-zero; no session is started | Fix filesystem, re-run | Non-zero exit + wrapped write error | N/A (write did not land) |
| Session start times out (5m headless cap) | Hard failure surfaced by the reused starter; exits non-zero; goal remains `in_progress` | Re-run | Non-zero exit + timeout error | Partial — status flipped, no session |

## Security / Abuse Cases

- The goal name is user-controlled input that resolves to a file path via `FindGoalByName`; path traversal / symlink-escape protection is already enforced by the reused goal storage (symlink-outside-vault refusal) and must not be weakened.
- The Claude prompt embeds the goal's file path; no new shell interpolation is introduced beyond what `task work-on` already does via the reused starter.
- No new network or HTTP surface is added.

## Acceptance Criteria

- [ ] `vault-cli goal work-on --help` lists the command under the `goal` group — evidence: command exit code 0 and stdout contains `work-on`
- [ ] Running `goal work-on` on a goal with blank assignee sets status `in_progress` and assignee to the current user — evidence: unit test asserts the written goal's `Status()` == `in_progress` and `Assignee()` == current user (mock `WriteGoal` args)
- [ ] Running `goal work-on` on a goal owned by a different user leaves the assignee unchanged and returns a warning naming both users, while still setting status `in_progress` — evidence: unit test asserts warning substring + unchanged assignee + `in_progress` status
- [ ] When the goal already has a non-empty `claude_session_id`, no session is started and that id is returned — evidence: unit test asserts `StartSession` call count == 0 and result session id == the cached id (session-id short-circuit path)
- [ ] When the goal has no session id, the built prompt starts with the configured work-on-goal command and ends with `--non-interactive` and contains the goal file path — evidence: unit test matches the `StartSession` prompt arg against `^<command> "` and ` --non-interactive$` and the file path
- [ ] A zero-turn / rejected Claude run causes a hard failure — evidence: unit test asserts `Execute` returns a non-nil error wrapped with the work-on-session context and `Success` == false, while the written goal is still `in_progress` (hard-fail path)
- [ ] A missing starter (nil) with no cached session id is a soft failure — evidence: unit test asserts no error, empty session id, and a warning containing "unavailable"
- [ ] `GetWorkOnGoalCommand()` returns `/vault-cli:work-on-goal` when `work_on_goal_command` is unset and the configured value otherwise — evidence: unit test on the config accessor
- [ ] The goal domain exposes `ClaudeSessionID()` / `SetClaudeSessionID()` and round-trips `claude_session_id` through generic get/set — evidence: unit test sets the field via the generic setter and reads it back; unknown-field round-trip preserved
- [ ] A Counterfeiter mock exists for the new operation interface — evidence: `go generate ./...` produces a mock file under `mocks/` and `make precommit` exits 0
- [ ] `make precommit` exits 0 — evidence: exit code 0

Scenario coverage: NO new scenario. The behavior is fully reachable by unit tests over the operation (mock storage + mock starter/resumer), exactly as `task work-on` is covered today; no real Claude binary or cluster is required to exercise the load-bearing paths.

## Verification

```
go generate ./...
make precommit
vault-cli goal work-on --help
```

Expected: `go generate` regenerates the operation mock with no diff surprises; `make precommit` exits 0 (lint + format + generate + test + version checks); `--help` prints the new command under `goal`.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Goal domain `claude_session_id` accessors + generic get/set wiring; `WorkOnGoalCommand` config field + `GetWorkOnGoalCommand()` default | 8 | goal-domain AC, config AC | — |
| 2 | New `GoalWorkOnOperation` in `pkg/ops` (find → status → assignee matrix → write → session handling with short-circuit + hard/soft fail) + Counterfeiter mock + unit tests | 1-6 | operation ACs, mock AC | prompt 1 |
| 3 | CLI `createWorkOnGoalCommand` wired under the `goal` group; plain + `--output json` formatting | 7 | help AC, precommit AC | prompt 2 |

Rationale: domain + config are leaf dependencies the operation needs, so they land first; the operation is the behavioral core and depends only on those accessors; the CLI wiring is a thin adapter over the operation and lands last. No cycles.

## Do-Nothing Option

If we skip this, goals keep having no way to mint a Claude session, and the vault-ui goal Start/Resume button cannot be built — the goal/task session asymmetry persists. The current approach (only `task work-on`) is not acceptable for the parent goal of unifying the task and goal views, since a kind-parameterized view needs a session primitive for both kinds.
