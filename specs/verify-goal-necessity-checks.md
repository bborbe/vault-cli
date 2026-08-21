---
status: draft
---

## Summary

- `verify-task` and `verify-goal` validate structure only (status valid, link resolves, DoD present, checkboxes tracked, status consistency) — never *intent*. A task linked to a goal that merely correlates with its topic but needs none of its output passes every existing check.
- Add a **goal-necessity** check to both agents' `### verify` actions: a goal-task link is only valid when the goal's success criteria actually need the task's outcome.
- Forward check (in `task-manager-agent.md`): when a task links a goal, flag it if no success criterion of that goal needs the task's outcome (correlation-only link).
- Inverse check (in `goal-manager-agent.md`): for each task linked from a goal's Tasks section, flag it if it advances no success criterion of the goal.
- Pure markdown change: two agent definition files + their thin command docs. No Go code, no CLI surface change. Checks are advisory (report), never auto-fixing.

## Problem

The trigger was the reconcile-task episode (2026-08-21): a task got linked to a goal that merely correlated with its topic but needed none of its output — `[[Establish a Working Exchange with the Agentic AI Platform Team]]` was linked to a two-meeting follow-up task whose output (a reconciled idea-status list) advances none of that goal's success criteria. Only a manual review caught it. The user's principle: **goals and tasks should be linked by necessity, not correlation** — the question to ask is "does this goal need this task?", not "is there anything related?"

Today neither `verify-task` (→ `task-manager-agent` verify, steps 3–8) nor `verify-goal` (→ `goal-manager-agent` verify, steps 1–8) can distinguish a necessity-based link from a correlation-only one. Correlation drift accumulates silently across the vault: tasks and goals that look connected on the board but exchange nothing, and goals whose task list omits or pads its own work under-report progress. Enforcing necessity is a judgment call today; it should be a repeatable pass/fail like the rest of the verify report.

Two refinements keep the rule honest, both anchored in the existing writing guides: a task in a domain the goal's `# Non-goals` explicitly excludes is by definition not needed, and `goal-writing.md`'s "Tasks as Business-Value Milestones" allows explicitly-framed foundation tasks (those *are* needed).

## Goal

A goal-task link is only reported valid by `verify-task` / `verify-goal` when the goal's success criteria actually need the task's outcome. Correlation-only links are flagged in the verify report with the offending link and the reason, so the fix is a deliberate decision instead of a silent drift.

## Non-goals

- Auto-fixing links — the checks report and flag only; nothing rewrites goal or task files.
- Applying the check retroactively across the whole vault — on-demand verification only, no bulk sweep or remediation.
- Re-reconciling already-mislinked tasks — a separate follow-up once the checks exist.
- Changing writing guides or templates to make necessity an explicit frontmatter contract — docs-only, separate concern.
- Altering the existing structural checks (status, DoD, link resolution) — additive only.
- Any Go / CLI code change — the verify commands remain agent-based; the checks live in the agent prompts.

## Acceptance Criteria

- [ ] `grep -n 'goal-necessity' agents/task-manager-agent.md` returns ≥ 1 line AND `grep -n 'not needed by linked goal' agents/task-manager-agent.md` returns ≥ 1 line — the forward check is a documented step in the verify action and carries the pinned report message.
- [ ] `grep -n 'goal-necessity' agents/goal-manager-agent.md` returns ≥ 1 line AND `grep -n 'not needed to complete goal' agents/goal-manager-agent.md` returns ≥ 1 line — the inverse check is a documented step in the verify action and carries the pinned report message.
- [ ] `grep -n 'goal-necessity' commands/verify-task.md` returns ≥ 1 line and `grep -n 'goal-necessity' commands/verify-goal.md` returns ≥ 1 line — the thin commands document the new checks in their process / success-criteria without adding detection logic (commands stay thin; no prose describing *how* to judge necessity in the command files).
- [ ] `make precommit` exits 0 on the feature branch.
- [ ] Operator smoke test (see Verification): a temp vault fixture containing a goal with one necessary task and one correlation-only task yields — `/vault-cli:verify-goal <goal>` output names the correlation-only task as not needed, and does not flag the necessary task; `/vault-cli:verify-task <correlation-only task>` output flags the link as not needed by the goal.
- [ ] Negative evidence: `/vault-cli:verify-task <necessary task>` output contains no goal-necessity issue; the clean link reports valid.

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — exits 0
- `make test` — suite green (no Go behavior changed; guards the tree)
- `grep -n 'goal-necessity' agents/task-manager-agent.md` — ≥ 1
- `grep -n 'goal-necessity' agents/goal-manager-agent.md` — ≥ 1
- `grep -n 'goal-necessity' commands/verify-task.md commands/verify-goal.md` — ≥ 1 line total

### Operator-executable (runs on the host after PR merge, spec verification ladder)

- Build a temp fixture vault (a throwaway `24 Tasks/` + `23 Goals/` pair registered via `vault-cli config` or `--vault`): a goal `G` with success criteria, one task `N` that advances an SC of `G`, one task `C` that shares `G`'s topic but advances no SC. Run `/vault-cli:verify-goal G` → output names `C` as not needed, does not flag `N`. Run `/vault-cli:verify-task C` → output flags the link as not needed; `/vault-cli:verify-task N` → no necessity issue.
- Install the feature branch (`make install` per `docs/releasing-vault-cli.md`) and confirm `vault-cli --version` matches HEAD tag after release.

## Desired Behavior

1. **Forward check (task-manager-agent verify):** after the parent-linkage step, for each linked goal, evaluate whether any success criterion of that goal needs the task's outcome. If none → report `❌` issue: "task not needed by linked goal <goal> — correlation-only (advances no success criterion)". If the goal's Non-goals explicitly exclude the task's domain, that also counts as not needed.
2. **Inverse check (goal-manager-agent verify):** after the task/PRD-linkage step, for each task linked in the goal's Tasks section, evaluate whether it advances ≥ 1 success criterion of the goal (or is explicitly framed by the goal as a needed foundation task). If not → report `❌` issue: "task <task> not needed to complete goal — advances no success criterion".
3. **Clean links:** a task that advances ≥ 1 SC of its linked goal produces no necessity issue; the verify report shows the existing `✅` pass shape with no necessity row.
4. **Report shape preserved:** both agents keep the existing `✅ … / ❌ … ✗ {issue}` report format; the necessity issue names the specific offending link and the reason.
5. **Commands stay thin:** `commands/verify-task.md` and `commands/verify-goal.md` gain a line describing the new check in their process/check list and success criteria, but contain no detection logic and no judgment heuristics.
6. **Advisory only:** the check reports; it never modifies files, never fails the command, and never auto-removes or re-links.

## Constraints

- Additive: the existing structural checks (status validity, link resolution, DoD presence, checkbox tracking, status consistency, task/PRD linkage) remain unchanged and their step numbering is preserved where other steps reference it.
- The checks live in the agent `### verify` actions and the thin command docs only — no Go code, no `pkg/ops/` changes, no CLI flag changes.
- The necessity judgment is LLM-based (the verify commands are agent-dispatched). The spec's semantic anchor: a task is "needed" iff it advances ≥ 1 success criterion of the linked goal, or is explicitly framed as a needed foundation task; tasks that are work-breakdown slices, scope-creep items, or padding are flagged.
- Both commands keep their current `allowed-tools` and argument-hint frontmatter.
- Preserve the `{{goal_name}}` / `{{task_name}}` report placeholders and the `✅`/`❌`/`✗` report vocabulary.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| Agent misreads a success criterion and false-flags a necessary task | The verify report shows the issue; report is advisory, no state change | Operator reviews the report and the goal's SC text; the reason quoted in the issue makes the misread visible |
| Agent misses a correlation-only link (under-flags) | No issue reported for a link that is correlation-only | Existing manual review remains; the check is an added signal, not a gate |
| LLM non-determinism produces inconsistent verdicts across runs | Different report outcomes on the same link | The spec's semantic anchor is quoted in the agent step so the judge has a fixed rule; a re-run confirms |
| Goal or task has a missing / unparseable `# Success Criteria` section | The check has nothing to evaluate | Report an info line "cannot evaluate necessity — goal <name> has no parseable Success Criteria"; the structural checks already flag the missing section; fix the goal file |

## Suggested Decomposition

The two checks are symmetric but independent; the forward check is the one with the live trigger example, so it goes first.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Forward check in task-manager-agent + commands/verify-task.md doc line | 1 | 1, 4, 5, 6 | — |
| 2 | Inverse check in goal-manager-agent + commands/verify-goal.md doc line | 2 | 2, 4, 5, 6 | prompt 1 (mirrors its phrasing) |

Rationale: prompt 1 establishes the necessity semantics and the reason-quoting report style; prompt 2 mirrors them for the goal side. Both are single-file-pair edits; no shared code.

## Do-Nothing Option

The cost of not doing this: correlation-only goal-task links keep passing every verify check, so the drift the reconcile-task episode caught by hand becomes a permanent blind spot. `verify-task` and `verify-goal` will keep reporting "parent linked, all good" for links that exchange nothing — and the next correlation link (and the manual review it requires) is not a matter of if but when.
