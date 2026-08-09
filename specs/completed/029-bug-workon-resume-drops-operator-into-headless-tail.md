---
status: completed
approved: "2026-08-09T09:08:40Z"
generating: "2026-08-09T09:20:36Z"
prompted: "2026-08-09T09:20:36Z"
verifying: "2026-08-09T09:49:27Z"
completed: "2026-08-09T19:18:11Z"
branch: dark-factory/bug-workon-resume-drops-operator-into-headless-tail
---

## Summary

- `vault-cli task work-on <task>` starts a headless Claude session, then immediately resumes it interactively — but the session is never told that interactivity became available.
- The operator lands on the tail of a headless turn whose instructions were "print the signal and STOP", so the task stalls behind a manual approval turn.
- A second, independent defect compounds it: even once the chain runs, `/vault-cli:execute-task` only *prints* the first subtask. For a task whose subtask is purely a slash-command call, it never invokes it.
- Net effect: a task file that says "Run `/start-day`" still requires the operator to approve running `/start-day`.
- Both defects are in the resumed-turn path only. Turn 1 (`claude --print`) is correct and must not change.

## Problem

`work-on` exists so a task can be picked up with one command. Today it hands back a session that has already been told to stop, and a chain that will not run the one command the task names. The operator pays an approval turn on every routine task — exactly the manual step `work-on` was built to remove — and, worse, learns to distrust the signal, because "stopped" here does not mean "needs a decision", it means "the flag from turn 1 is still in effect".

## Goal

Running `vault-cli task work-on <task>` on a task whose `# Tasks` list contains a bare slash-command subtask leaves the operator in an interactive session that has already invoked that slash command, with no approval turn in between. Turn-1 headless behaviour is unchanged.

## Reproduction

vault-cli `v0.102.6` (`git describe` at `db892f7`); observed 2026-08-09.

Setup — a task file whose subtask is purely a slash-command call:

```
24 Tasks/Start Day - 2026-08-09.md
  frontmatter: status: in_progress, phase: execution
  # Tasks
  - [ ] Run `/start-day`
```

Action:

```bash
vault-cli task work-on "Start Day - 2026-08-09"
```

Observed — the resumed session's first visible output is the tail of the headless bootstrap turn:

```
✅ Oriented: Start Day - 2026-08-09. Next:
→ /vault-cli:plan-task "Start Day - 2026-08-09"     — validate the plan (Success Criteria + subtasks)
→ /vault-cli:execute-task "Start Day - 2026-08-09"  — begin executing (flips planning → execution)
→ /vault-cli:complete-task "Start Day - 2026-08-09" — close when done
```

The session then idles. `/start-day` is not invoked. The operator must type an approval to proceed.

Root-cause evidence, `pkg/ops/workon.go:194`:

```go
prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
```

and `pkg/ops/workon.go:128-135`:

```go
if isInteractive && w.resumer != nil && sessionID != "" {
	return MutationResult{...}, w.resumer.ResumeSession(ctx, sessionID, sessionDir)
}
```

`ResumeSession` (`pkg/ops/claude_resume.go:58`) execs `claude --resume <id>` with no prompt argument, so nothing in turn 2 countermands the turn-1 flag.

### Defect #2 — inferred from the command doc, not yet observed running

The above reproduces defect #1. Defect #2 is asserted from reading `commands/execute-task.md:101-117`, which prints `🎯 Start with: <subtask>` and stops. Direct repro to run before the fix, to convert this from inferred to observed:

```
In an interactive session: /vault-cli:execute-task "Start Day - 2026-08-09"
Observe: "🎯 Start with: Run /start-day"  — the command is printed, never invoked.
```

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Turn 1 (`claude --print`) | oriented headlessly, no `AskUserQuestion` — per `commands/work-on-task.md` Phase 5 step 2 | correct, unchanged |
| Turn 2 (after `claude --resume`) | session knows it is interactive and continues the plan → execute chain | still bound by turn-1's "print the signal and STOP" |
| Subtask `- [ ] Run /start-day` | `execute-task` invokes `/start-day` | `execute-task` prints the subtask text and waits |

## Why this is a bug

`commands/work-on-task.md` documents `--non-interactive` as existing solely because "a headless `claude --print` caller cannot answer, so chaining would hang". That rationale expires the instant `ResumeSession` execs an interactive session against the same session id. The flag is scoped to a transport condition that is no longer true, and nothing re-scopes it. The operator sees a stop signal that encodes no decision — the failure mode the `work-on` command was built to eliminate.

## Acceptance Criteria

- [ ] **Post-Deploy (Rung-3):** the Reproduction no longer reproduces — evidence: on a task whose sole `# Tasks` entry is a bare slash-command call, `vault-cli task work-on "<task>"` yields a resumed session whose first action is the invocation of that command, with zero approval turns between resume and invocation.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q; git describe --tags --abbrev=0)`
  - `deploy_check:` `claude plugin list | grep -A1 'vault-cli@vault-cli' | awk '/Version/{print "v"$2}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0)`
  - `deploy_check:` `grep -c '^### Subtask classification' ~/.claude/plugins/cache/vault-cli/vault-cli/$(git describe --tags --abbrev=0 | tr -d v)/commands/execute-task.md`
  - `deploy_target:` `1`

  Three checks, not one: this fix ships through two independently-versioned channels. The continuation-prompt half lives in the **binary** (`pkg/ops/`), gated by check 1. The classification / auto-invoke half lives in the **plugin** (`commands/execute-task.md`), which `vault-cli --version` says nothing about — checks 2 and 3 gate that. Check 3 greps the load path directly because `claude plugin list` reports the *pinned* version while the *running* process has whatever it loaded at startup; note the path is doubly nested (`cache/vault-cli/vault-cli/<version>/`), and a singly-nested path silently returns 0 and reads as a stale-deploy FAIL. A restart is still a Setup precondition — no command observes the loaded-vs-pinned gap.
- [ ] `ResumeSession` takes a prompt and appends it to argv — evidence: unit test asserts argv is exactly `["claude","--resume",<id>,<prompt>]` for a non-empty prompt.
- [ ] An empty prompt changes nothing — evidence: unit test asserts argv is exactly `["claude","--resume",<id>]` when prompt is `""` (negative evidence: no trailing empty-string arg).
- [ ] The continuation prompt re-invokes the work-on command **interactively** — evidence: unit test asserts the string `workon.go` passes to `ResumeSession` equals `<GetWorkOnCommand()> "<task.FilePath>"`, and that `strings.Contains(prompt, "--non-interactive")` is false.
- [ ] `goal_workon.go` compiles and is behaviour-neutral — evidence: `grep -c 'ResumeSession(ctx, sessionID, sessionDir, "")' pkg/ops/goal_workon.go` returns exactly 1.
- [ ] Turn-1 behaviour is unchanged — container-side evidence (git is masked under `hideGit`, so this is content-based, not diff-based): `grep -c -- '--non-interactive' pkg/ops/workon.go` returns ≥1 for the bootstrap line, and `md5sum pkg/ops/claude_session.go` equals `6fd7090f033d6c3156d74dd2cde041f0`. Host-side diff confirmation is in the operator rung. (Do **not** assert `grep -c 'prompt string' pkg/ops/claude_session.go` is 0 — `StartSession` already takes a `prompt string` parameter, so it is 2 on unmodified code.)
- [ ] `commands/execute-task.md` carries an executable classification rule — evidence: `grep -n '^### Subtask classification' commands/execute-task.md` returns 1 line, and within that section `grep -c 'bare slash-command call → invoke'` and `grep -c 'anything else → print'` each return ≥1.
- [ ] Auto-invoke is bounded to an allowlist — evidence: the same section names the permitted command prefixes, and `grep -c 'never auto-invoke' commands/execute-task.md` returns ≥1.
- [ ] Step 7 actually consumes the classification rule rather than only defining it — evidence: `grep -c 'Subtask classification' commands/execute-task.md` returns ≥2 (the section heading plus a reference from the step that currently prints `🎯 Start with:`).
- [ ] Mixed prose-and-command subtasks are explicitly disqualified — evidence: the classification section names the disqualifiers (surrounding prose, shell metacharacters in arguments, a second command on the line), `grep -c 'disqualifies' commands/execute-task.md` returns ≥1.
- [ ] `make precommit` exits 0.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint + format + generate + test + version checks, exits 0
- `make test` — unit tests pass, including the new `ResumeSession` argv tests
- `grep -n 'prompt string' pkg/ops/claude_resume.go` — signature change landed
- `md5sum pkg/ops/claude_session.go` — equals `6fd7090f033d6c3156d74dd2cde041f0`, turn 1 untouched

**No git commands in this rung.** Worktree runs require `hideGit=true`, which masks `/workspace/.git` as a character device — every `git diff` / `git rev-list` / `git describe` fails container-side while passing on the host. See [[HideGit Containers Break Git-Dependent Tests]]. Diff-based regression locks belong in the operator rung below.

### Operator-executable (runs on the host after PR merge)

- `git diff origin/master -- pkg/ops/claude_session.go` and `git diff origin/master -- commands/work-on-task.md` — both empty (the diff-based turn-1 regression lock; host-only because git works here)
- `make update && claude plugin update vault-cli@vault-cli` then restart Claude Code — plugin at the new version
- `vault-cli task work-on "<a task whose subtask is a bare slash-command call>"` — the resumed session invokes that command with no approval turn (this is the reproduction from above, replayed)

## Desired Behavior

1. `ResumeSession` takes a continuation prompt and appends it to the `claude --resume` argv when non-empty.
2. When the prompt is empty, argv is byte-identical to today's — `pkg/ops/goal_workon.go:126` relies on this.
3. `work-on`'s `Execute` passes a continuation prompt equal to `<GetWorkOnCommand()> "<task.FilePath>"` — the same bootstrap invocation **without** `--non-interactive`.
4. `execute-task` classifies its first unchecked subtask: a bare slash-command call from the allowlist → invoke it; anything else → print it as today.
5. Turn 1 keeps `--non-interactive` and keeps its stop-at-signal behaviour.

## Constraints

- `StartSession` and the turn-1 `--non-interactive` bootstrap must not change.
- **All four affected sites** break on the signature change and MUST be updated in the same prompt that changes the interface: call sites `pkg/ops/workon.go:135` and `pkg/ops/goal_workon.go:126` (both 3-arg), and test destructures `pkg/ops/workon_test.go:328` and `pkg/ops/goal_workon_test.go:288` (both `_, sessionID, cwd := ...ArgsForCall(0)`). `workon.go` takes a temporary `""` in prompt 1 that prompt 2 replaces, so each prompt stays independently compilable.
- **Scope decision (deliberate):** `vault-cli goal work-on` carries the same defect, but this spec does NOT fix it. `goal_workon.go` passes `""` — compiles, behaviour unchanged. The goal-side fix is a separate spec so the bug's blast radius stays one command. Reviewer may overturn this at approve time.
- `ClaudeResumer` is a counterfeiter-generated interface — regenerate `mocks/claude-resumer.go` via `go generate ./...`, never hand-edit.
- `NewClaudeResumerForTesting` must keep its injectable `execFn` shape so existing tests keep compiling.
- `pkg/ops/` stays a library layer — no stdout writes (see CLAUDE.md § Key Design Decisions).
- Release: this repo is `autoRelease: true` at the maintainer level (`.maintainer.yaml`), so add one `## Unreleased` CHANGELOG bullet and let the releaser cut the version — do not hand-bump the plugin manifests and do not tag. (`make precommit` does not run `check-versions`; only `release-check` does — see `Makefile:31`.)

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection |
|---|---|---|---|
| `claude --resume` rejects a trailing prompt argument on some future CLI version | `ResumeSession` returns the exec error; `work-on` surfaces it rather than silently dropping the operator into a bare session | operator re-runs `vault-cli task work-on` after the CLI is fixed; empty-prompt path still works | non-zero exit from the exec'd process |
| Continuation prompt is empty or whitespace | argv omits it entirely — identical to current behaviour, no empty positional arg | none needed | unit test asserts exact argv |
| `execute-task` misclassifies a prose subtask as a command | subtask is printed, not invoked — classification defaults to the safe side | operator invokes manually as today | the mixed-prose disqualifier AC |
| Task file's first unchecked subtask names a command outside the allowlist | printed, not invoked — the allowlist is the boundary, not operator vigilance | operator invokes manually as today | the allowlist AC |
| Task file is authored or edited by a non-operator (agent, synced vault, recurring-task template) | only allowlisted `/vault-cli:*` and `/`-prefixed vault commands can auto-invoke; arbitrary shell never does | revoke by removing the prefix from the allowlist | — |

## Security / Abuse

DB4 converts a string parsed from a user-authored markdown file into an executed command, and removes the approval turn that previously bounded it. That is a real privilege change, so the boundary is specified rather than assumed:

- Auto-invoke applies **only** to a subtask that reduces to a single allowlisted slash-command token after a fixed normalization: trim whitespace, strip an optional leading imperative (`Run `, `Invoke `, case-insensitive), then strip surrounding backticks. Nothing else is stripped — a subtask left with trailing punctuation or any other residue falls through to print, which is the safe side. Any remaining prose, arguments containing shell metacharacters, or a second command on the line disqualifies it.
  - This normalization is load-bearing, not a convenience: the Reproduction's own subtask is ``- [ ] Run `/start-day` ``, which AC1 replays. A stricter "bare token only" rule would make AC1 unsatisfiable. (Caught at prompt-generation time — the first draft of this section did exactly that.)
- The allowlist is limited to first-party Claude Code slash commands (`/vault-cli:*`, `/dark-factory:*`, `/coding:*` and the bare-name skills shipped with them). Bash, file paths, and URLs are never auto-invoked.
- Task files are not always operator-authored — `recurring-task-creator` generates them from YAML, and vaults sync across machines. The allowlist, not the operator's attention, is what bounds blast radius.
- Out of scope: sandboxing what an allowlisted command itself does once invoked. That is the existing permission model's job and is unchanged here.

## Suggested Decomposition

Prompts should be generated in this order.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | `ResumeSession` prompt parameter — interface, impl, `NewClaudeResumerForTesting`, regenerated mock, argv unit tests (non-empty + empty). **Must also update `goal_workon.go:126` to pass `""` and fix `goal_workon_test.go:288`'s 3-arg destructure, or the package will not compile.** | 1, 2 | 2, 3, 5 | — |
| 2 | `workon.go` call site passes the continuation prompt (work-on command without `--non-interactive`); assert turn 1 untouched | 3, 5 | 4, 6 | prompt 1 (needs the new signature) |
| 3 | `commands/execute-task.md` subtask classification, allowlist-bounded auto-invoke | 4 | 7, 8 | — |

Rationale: prompt 1 establishes the signature, owns the generated mock, and must carry the `goal_workon` call-site fix because the package will not build otherwise; prompt 2 is a one-line call-site change that cannot compile until 1 lands; prompt 3 is markdown-only and independent of both.

The Rung-3 repro-replay AC (AC1) and `make precommit` (AC9) are cross-cutting — every prompt runs precommit, and AC1 is verified once by the operator after merge and deploy, not by any single prompt.

**CHANGELOG:** all three prompts contribute to a single `## Unreleased` entry. Whichever prompt lands last should ensure one bullet describes the user-visible outcome (work-on resumes into an interactive chain and auto-runs a bare slash-command subtask), rather than three implementation-shaped bullets.

## Workaround

Until this ships: after `vault-cli task work-on` drops you into the resumed session, type the slash command the task names. The task file already records it — the fix removes the retyping, not the information.

## Verification Result

**Verified:** 2026-08-09T19:17:29Z (HEAD 9be94a1)
**Binary:** installed `vault-cli` v0.105.0 (`/opt/homebrew/bin/vault-cli`) + plugin `vault-cli@vault-cli` 0.105.0; `git diff v0.105.0 HEAD -- pkg/ cmd/ commands/ main.go go.mod` is empty, and the loaded `cache/vault-cli/vault-cli/0.105.0/commands/execute-task.md` is byte-identical to HEAD's, so the deployed artifacts are code-identical to HEAD despite the v0.106.0 tag (v0.105.0..HEAD is docs-only). AC1's `deploy_target` keyed on the latest tag rather than the installed version — the same tag-vs-installed false negative HEAD's `## Unreleased` entry fixes in scenario 005.
**Scenario:** `scenarios/005-work-on-resume-auto-invokes-subtask.md` walked from a real TTY on 2026-08-09 against fixture `24 Tasks/Verify Spec 029 Auto-Invoke Fixture.md` (sole unchecked subtask ``- [ ] Run `/vault-cli:next-task` ``).
**Evidence:**
- Turn 1 prompt `/vault-cli:work-on-task "<abs path>" --non-interactive` — produced only by `workon.go:199`; turn 2 prompt `/vault-cli:work-on-task "<abs path>"` (no flag) — produced only by the continuation at `workon.go:133`. Neither string is producible by a human keystroke sequence or by an in-session chain: no command markdown re-invokes `work-on-task`, and `next-task.md:112`'s re-entry is gated behind `AskUserQuestion` and ran *after* the auto-invoke.
- Post-continuation output: `Interactive mode → chaining` → `Skill(vault-cli:plan-task)` → `Skill(vault-cli:execute-task)` → DoD block → `🚀 Running: /vault-cli:next-task` → `Skill(vault-cli:next-task)` → `📋 Today's Tasks: 2026-08-09`. The `🚀 Running:` + `Skill:` form is prescribed only by `commands/execute-task.md:159`, so the *loaded* plugin carried the classification rule. No `🎯 Start with:`, no fresh `✅ Oriented:`/`Next: →`, no approval turn (the replayed `✅ Oriented:` block is turn 1, reprinted by `claude --resume`).
- On-disk durable proof, vault git history of the fixture: no `claude_session_id` at 20:54 → `claude_session_id: f045e800-1695-4533-aefc-645c9b1c4d80` at 21:09, matching the transcript's session. Written only by `workon.go:205`, reachable only on a fresh bootstrap.
- Negative control, same fixture and binary: the 20:53 attempt through a non-TTY shell took the `workon.go:128` fall-through, persisted `84f1fe1a-…` and never resumed — no turn 2. Deploy ordering holds: binary 17:37, plugin 18:30, run 21:07.
- ACs 2-4 unit argv/continuation assertions in `pkg/ops/claude_resume_test.go` + `workon_test.go`; AC5 `grep -c` = 1; AC6 `md5 pkg/ops/claude_session.go` = `6fd7090f033d6c3156d74dd2cde041f0` and `git diff v0.102.7 v0.103.0 -- pkg/ops/claude_session.go commands/work-on-task.md` empty; ACs 7-10 greps 1/1/1/1/3/1; AC11 `make precommit` exit 0.
**Verdict:** PASS
