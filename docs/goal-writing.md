---
tags:
  - guide
  - goal
---

A goal is a personal-ambition deliverable that takes weeks-to-months to achieve and breaks down into ≤ 8 tasks. It states *what* will be true when done, not *how* to get there.

## TL;DR

- **Use for**: personal ambitions, weeks-to-months, ≤ 8 tasks
- **Create**: `/vault-cli:create-goal "<title>"`
- **Audit**: `/vault-cli:audit-goal "<title>"`
- **Sections**: Summary → Impact → Status Summary → Success Criteria → **Non-goals** → Tasks → Related
- **Forcing functions**: Non-goals enumerated; every task contributes to ≥ 1 success criterion

## Goal

Produce a goal page that an agent can scaffold, audit, and complete without ambiguity. Every section is binary-checkable; the contract holds whether the goal is operational, learning, or project-shaped.

## When to Write a Goal

| Situation | Goal? |
|-----------|------|
| Personal ambition you WANT to achieve, weeks-to-months effort | Yes |
| Specific measurable deliverable | Yes |
| Supports a theme or objective | Yes |
| Operational task taking days | No — write a task |
| Ongoing strategic direction (years, never-ending) | No — write a theme |
| Quarterly outcome (3-12 months, multi-goal umbrella) | No — write an objective |
| Lifetime aspiration | No — write a vision |

## Creating a Goal

Use the slash command:

```
/vault-cli:create-goal "<title>"
```

The command invokes `goal-creator`, which scaffolds a file in the configured vault's `goals_dir` (typically `23 Goals/`). The agent reads vault config via `vault-cli config list --output json` — never hardcode paths.

## Goal Structure

### Frontmatter

```yaml
---
status: todo
page_type: goal
priority: 3                                      # optional, 1-3
category: <domain>                               # optional
timeline: 2026-MM-DD to 2026-MM-DD               # optional, ≤ 4 weeks for tactical goals
objective: "[[Parent Objective]]"                # optional
themes:                                          # optional
  - "[[Parent Theme]]"
---
```

`status` valid values: `in_progress`, `todo`, `backlog`, `hold`, `completed`, `aborted`.

### Required sections

In order:

1. `Tags: [[Goal]]` (after frontmatter, before content separator)
2. **Summary** — first paragraph after the `---` separator. 1-2 sentences stating concrete outcome + benefit.
3. `# Impact` — strategic value, theme connection, quantified where possible
4. `# Status Summary` — Progress / Current / Next / Blockers (one line each)
5. `# Success Criteria` — 3-5 binary, measurable checkbox outcomes
6. `# Non-goals` — 3-7 concrete deferrals (what's out of scope; link follow-up tasks/goals)
7. `# Tasks` — 4-8 linked task pages, logical order
8. `# Related` — themes / sister goals / docs

Optional: `# Risk Management` (appendix for high-stakes goals).

### Non-goals — the scope-creep guard

Adopted from dark-factory spec convention. Forces explicit articulation of what's out of scope BEFORE the task list bloats.

**Good non-goals:**

```markdown
# Non-goals

- Auto-retry of transient agent failures — separate concern, [[Auto-Retry Transient Agent Failures Before Human Review]]
- Refactoring existing CRDs (assignee names stay as they are)
- Backfilling historical tasks — only new emissions get the new shape
```

**Bad non-goals:**

```markdown
# Non-goals

- We won't be perfect
- Anything not in tasks
```

**Rules:**
- 3-7 items typical. Fewer = scope wasn't really challenged. More = goal itself is too broad.
- Each item is a *concrete* deferral, not a vague disclaimer.
- Link follow-up goals/tasks where the deferred work lives.
- After drafting tasks: re-read Non-goals. If any task is also listed under Non-goals, fix one or the other.

When in doubt: if a reader might ask "does this goal include X?" and the answer is no, X belongs in Non-goals.

## Scope Check

Before approving a goal, verify these signals:

- **Task count ≤ 8**, all tasks contribute to ≥ 1 success criterion
- **Tasks-to-criteria ratio ≤ 2.5×** (e.g. ≤ 8 tasks for 3 criteria)
- **Non-goals enumerates 3-7 deferrals** — not vague disclaimers
- **All tasks share one mental model** (one operator outcome, one domain)
- **Goal title states an outcome**, not an activity or capability

If 3+ smells fail → goal is over-scoped. Split into multiple goals or move items to Non-goals.

## Preflight Checklist

Before approving:

- [ ] What strategic outcome are we achieving?
- [ ] What does "done" look like (binary, measurable)?
- [ ] What's NOT in scope (Non-goals enumerated)?
- [ ] Does every task contribute to ≥ 1 success criterion?
- [ ] Is the goal title outcome-shaped (not activity-shaped)?
- [ ] Are tasks weeks-to-months in aggregate (not days, not years)?

## Audit

Always audit before committing publicly:

```
/vault-cli:audit-goal "<goal title or path>"
```

The auditor (`goal-auditor` agent) checks structure, SMART criteria, Non-goals presence, Goal Scope Fit smells (8 indicators of over-scoping; 3+ → flag), and per-task alignment to success criteria.

## Lifecycle

| Status | Meaning | Trigger to enter |
|--------|---------|------------------|
| `todo` | Defined, not started | Goal file created with required sections filled |
| `in_progress` | Actively working (limit to 3-5 in flight) | First linked task transitions to `in_progress` |
| `hold` | Blocked or paused | `blocked_by:` field populated, or operator sets manually |
| `completed` | All success criteria met | `/vault-cli:complete-goal` — checks every `# Success Criteria` checkbox is `[x]` |
| `aborted` | Abandoned without completion | Operator sets manually with reason in body |
| `backlog` | Potential future, not committed | Initial state before commitment |

Completed goals are immutable — for new outcomes, create a new goal with `parent_goal: [[Previous Goal]]` if a continuation.

## Vault-Specific Examples

This doc covers structure and conventions. For concrete examples drawn from a real vault (good vs weak goals, vault-quality assessment, common mistakes), see the per-vault writing guide:

- Personal vault: `~/Documents/Obsidian/Personal/50 Knowledge Base/Goal Writing Guide.md`

That guide is example-rich and references real goals; this doc is the generic contract.

## References

- `task-writing.md` — tasks roll up to goals
- `/vault-cli:goal-status` — progress on an active goal
- `/vault-cli:verify-goal` — mechanical structure check
