---
status: completed
completed: "2026-08-19T08:16:51Z"
branch: feat/work-on-goal-find-or-create
retroactive: true
---

## Summary

- `/vault-cli:work-on-goal` erred and stopped when the requested goal did not exist, while its sibling `/vault-cli:work-on-task` had auto-created the missing task since spec 016. The asymmetry forced a manual `create-goal` detour before a greenfield goal session could start.
- Fix mirrors spec 016's Single Responsibility split, applied to the goal route: `work-on-goal-assistant` emits a structured `not_found:` verdict and STOPS; `commands/work-on-goal.md` owns creation via `Skill: vault-cli:create-goal`, then re-invokes the assistant against the newly created goal.
- The agent keeps no creation capability — it has no `Skill` tool, so the split is enforced by capability, not prose (same architectural property spec 016 established).
- The command's `allowed-tools` was `Task` only, which is why creation could not live there before; it now carries `[Task, AskUserQuestion, Skill, Bash(vault-cli *)]` plus `--non-interactive` MODE parsing.
- `/vault-cli:work-on` needed no behavioral change — it already created task-or-goal on free-text `not_found`. Only the dedicated goal route had the gap.

**Retroactive spec.** This documents a change that shipped as v0.112.0 before the spec was written. See *Process deviation* below — it is recorded deliberately rather than omitted, because the omission is the lesson.

## Problem

`/vault-cli:work-on-goal BRO-20702` (and any goal name with no vault page) reported not-found and stopped. The user's stated requirement: *"WorkonGoal should behave the same as WorkonTask. If the goal doesn't exist, then it should create one."*

The three `work-on-*` commands were inconsistent:

| Command | On `not_found` (before) |
|---|---|
| `/vault-cli:work-on-task` | always creates the task (spec 016 Phase 4) |
| `/vault-cli:work-on` | always creates task-or-goal (free text asks which type; Jira ID → task) |
| `/vault-cli:work-on-goal` | **errors out, suggests `/vault-cli:create-goal`** |

Two blockers made the goal route unable to follow spec 016's pattern:

1. `agents/work-on-goal-assistant.md` handled not-found itself as a prose error (`If not found: error with searched paths + suggest /vault-cli:create-goal`) — no structured verdict for a caller to parse.
2. `commands/work-on-goal.md` had `allowed-tools: Task` — it could not invoke `Skill: vault-cli:create-goal` even if it wanted to.

## Goal

`/vault-cli:work-on-goal "X"` with no existing goal X creates the goal and proceeds to work on it, with no consent prompt (the invocation *is* the intent; the create-goal skill's own interactive flow remains the operator's back-out).

`work-on-goal-assistant` emits a `not_found:` verdict block — `Suggested goal name:` (the Jira summary when the input is a Jira key, else the input verbatim) plus a `Searched:` evidence block (Jira hit/miss/skipped, Glob paths tried, semantic-search misses) — then STOPS. It does not create, does not ask.

`commands/work-on-goal.md` Phase 4 parses that verdict, invokes `Skill: vault-cli:create-goal "<SUGGESTED_NAME>"`, and on success re-invokes the assistant so the standard prep (status promotion, guide search, task analysis) runs against the new goal. On create failure or cancel it stops without re-invoking.

Jira-key input additionally resolves an *existing* goal by `jira:` frontmatter before declaring not-found, so a Jira key never creates a duplicate of a goal that is already tracked.

## Non-goals

- Changing the `create-goal` / `create-task` skills themselves — only command wiring.
- Reworking `/vault-cli:work-on-task`'s existing create-on-`not_found` (spec 016) — untouched.
- Changing `/vault-cli:work-on`'s classification or its free-text task-vs-goal prompt — it already covered both types.
- Any error path other than `not_found` (terminal-state goals, unreadable files, CLI failures keep their current handling).
- Routing a Jira key that already has a *task* page to that task — a deliberate product decision (2026-08-19): `work-on-goal` creates a goal even when a task exists for the same key.

## Acceptance criteria

- `/vault-cli:work-on-goal "<nonexistent>"` creates the goal via the create-goal skill, then runs work preparation against it.
- `work-on-goal-assistant` emits the `not_found:` block with `Suggested goal name:` and never calls `Skill` / `AskUserQuestion` on the miss path.
- A Jira-key input whose goal already exists (by `jira:` frontmatter) is treated as found — no duplicate created.
- Found cases are unchanged: existing goal → located and worked, no creation.
- `MODE=non_interactive` creates nothing and prints the `not_found:` report plus the non-interactive notice.
- `/vault-cli:work-on-task` behavior unchanged.

## Process deviation (recorded deliberately)

This change was hand-authored directly instead of routed through the spec → prompt → dark-factory flow that `.dark-factory.yaml` (`autoGeneratePrompts: true`) mandates for this repo, and the first edits were made on the `master` working tree rather than in a feature worktree. Both were caught by the operator mid-flight; the worktree and the local `/coding:pr-review` loop were retrofitted before merge, but the spec was written only afterward — hence this file.

Notably, the sibling change this one mirrors (spec 016) *did* get a spec. The guide's rule is explicit: default to the spec flow whenever `.dark-factory.yaml` exists, even for small changes; a change touching `commands/` and `agents/` is not a carve-out.

## Shipped

- PR: bborbe/vault-cli#93, merged 2026-08-19T08:16:51Z as merge commit `2ec365c` (feature commit `b15938c`).
- Release: v0.112.0 (`9d14ddf`), minor bump from the `feat:` bullet.
- Files: `agents/work-on-goal-assistant.md`, `commands/work-on-goal.md`, `commands/work-on.md`, `CHANGELOG.md`.
- Verification: `make test` + `make precommit` green; local `/coding:pr-review` clean (no Must-Fix / Should-Fix); pr-reviewer bot APPROVED on head SHA `b15938c`; CI green; changelog fold guard clean; plugin deployed and verified in the load path `~/.claude/plugins/cache/vault-cli/vault-cli/0.112.0/`.
- Outstanding: live end-to-end exercise of the create path after a Claude Code restart.
