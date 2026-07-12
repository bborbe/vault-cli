---
description: Gate planning → execution for a goal. Re-runs plan-goal's hard checks; on pass flips phase and recommends the next open task (one at a time) until the goal drains. Recommends only — never runs the task.
argument-hint: "<goal-file-path-or-name> (or detects from conversation)"
allowed-tools: [Read, Edit, Glob, Bash, AskUserQuestion, Task]
---

The **hard gate** between a goal's planning and execution, and the driver that walks its tasks one at a time. Refuses to flip `phase: planning → execution` unless plan-goal's 3 hard checks pass. Idempotent on `phase: execution` — re-run it to surface the next open task. It **recommends** the next task; the normal task lifecycle (`/vault-cli:work-on-task` → `plan-task` → `execute-task` → `complete-task`) does the work in between.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## When to use

After `/vault-cli:plan-goal` (or any time the goal plan is genuinely complete) to formally enter execution and get pointed at the first task. Re-run after finishing a task to get the next one, until the goal drains.

```bash
/vault-cli:execute-goal                              # detects from conversation
/vault-cli:execute-goal "Some Goal Name"
/vault-cli:execute-goal 23\ Goals/Some\ Goal.md
```

## Process

### 1. Resolve goal path

**With argument:** exact path if path-like, else `Glob` `<goals_dir>/*<arg>*.md` (vault-cli config respected). Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (same priority order as `/plan-goal`):

1. Most recent `/create-goal` / `/launch-goal` / `/plan-goal` / `/work-on-goal` output — scan the parent conversation for the resolved goal name.
2. Most recent `[[Goal Name]]` wikilink referenced as a goal subject.
3. Most recently modified file in `<goals_dir>/`.

Multiple matches → ask via `AskUserQuestion`. Zero → `❌ No goal detected. Pass a goal identifier or name.` STOP.

Print `Detected goal: <name>` on first line so owner can interrupt before any state mutation.

### 2. Read status + phase

```bash
vault-cli goal get "<name>" status --output json
vault-cli goal get "<name>" phase --output json
```

### 3. Refusal cases (no mutation, exit non-zero)

Refuse and STOP if any apply:

- `status: completed` OR `status: aborted` → `❌ Goal closed (status: <value>). Run reopen if you need to continue work.`
- `phase: done` → `❌ Goal phase is done. Run reopen if work needs to resume.`
- `phase: todo` OR `phase` empty AND `status: in_progress` → `❌ Planning gate not run. Run /vault-cli:plan-goal first.` (planning is non-skippable)

### 4. Status entry contract (mutate, then continue)

If `status` is in `next` / `backlog` / `hold` → flip to `in_progress`:
```bash
vault-cli goal set "<name>" status in_progress
```
Print: `ℹ️ Status: <old> → in_progress (resume from <old>)`

If `status` is already `in_progress` → continue, no mutation.

### 5. Run plan-goal's 3 hard checks (re-check planning)

Copy the *Hard* checks from `/vault-cli:plan-goal` § 5 — DO NOT factor into a shared helper yet (keeps both commands self-contained; lift to a `vault-cli goal verify-plan` CLI verb if a third caller appears). Read `~/.claude/plugins/marketplaces/vault-cli/docs/goal-writing.md` as the canonical rule source.

Check (in order, collect ALL failures — don't short-circuit):

1. **Required sections present** — each of `# Success Criteria`, `# Definition of Done`, `# Non-goals` exists as a heading. Report each missing section by name.
2. **Success Criteria binary** — `# Success Criteria` has ≥ 2 binary checkboxes (`- [ ]` / `- [x]`).
3. **Every `# Tasks` wikilink resolves** — each `[[Task Title]]` in the `# Tasks` section resolves to an existing `<tasks_dir>/<Task Title>.md` (skip `[[...]]` inside inline-code spans; strip `|alias` display text). Report each unresolved task by name.

### 6. Phase transition or refusal

**If ANY hard check failed AND `phase: planning`:**

Print:
```
❌ Plan not ready. Run /vault-cli:plan-goal first.

Failed checks:
- <check name>: <one-line reason>
...
```
STOP. Do NOT flip phase.

**If all hard checks pass AND `phase: planning`:**

```bash
vault-cli goal set "<name>" phase execution
```
Print: `✅ Phase: planning → execution`

Continue to step 7.

**If `phase: execution`:** no flip, no check (already past the gate). Continue to step 7 idempotently. Print: `ℹ️ Already in execution — surfacing the next task.`

### 7. Recommend the next open task (or signal drain-complete) — final output

Walk the `# Tasks` section's wikilinks **in listed order**. For each `[[Task Title]]` (inline-code spans skipped, `|alias` stripped), resolve to `<tasks_dir>/<Task Title>.md` and read its status:

```bash
vault-cli task get "<Task Title>" status --output json
```

- **No task wikilinks at all** under `# Tasks` → `⚠ No tasks listed under # Tasks — nothing to execute. Run /vault-cli:plan-goal to add at least one.` (do NOT report a false "drained"). STOP.
- **A wikilink that no longer resolves** to an existing task file (e.g. the task was renamed/deleted after the goal entered execution — the resolution check in step 5 only runs on the `planning` path) → `⚠ Task file missing: [[<Title>]] — re-run /vault-cli:plan-goal to repair the # Tasks list.` Report it and skip that entry rather than erroring mid-walk; continue evaluating the rest.
- **Next open task** = the first *resolving* task whose status is NOT `completed` and NOT `aborted`. Recommend exactly that one:

  ```
  🎯 Next task: [[<Task Title>]]  (status: <status>)

  → Run /vault-cli:work-on-task "<Task Title>" to start it.
  ```

  Recommend **one** task only — not the whole list. `execute-goal` never runs the task; the operator drives it through the task lifecycle, then re-runs `/vault-cli:execute-goal` for the next one.

- **All tasks complete** (every `# Tasks` wikilink resolves to a task with status `completed`, none `aborted` pending) → the goal has drained:

  ```
  ✅ All tasks complete — the goal has drained.

  → Run /vault-cli:verify-goal to confirm the Success Criteria, then /vault-cli:complete-goal to close it.
  ```

- **Only aborted tasks remain** (no open task, but ≥1 `aborted`) → print `⚠ Remaining tasks are aborted: <names>. Re-plan (/vault-cli:plan-goal) or complete the goal if the aborted work is intentionally dropped.`

## Notes

- **Recommends, never runs.** `execute-goal` surfaces the next task and stops. The task lifecycle (`work-on-task` → `plan-task` → `execute-task` → `complete-task`) does the actual work — keeping the operator in the loop between tasks (per the goal's design; no autonomous drain).
- **Idempotent re-entry.** Safe to re-run on `phase: execution` — no mutation, just re-computes the next open task from live task statuses. This is how you walk the goal: finish a task, re-run `execute-goal`, get the next.
- **Task completion is derived, not stored on the goal.** The goal's `# Tasks` list is wikilinks, not checkboxes; "done" is read from each linked task file's `status: completed`. This keeps a single source of truth (the task file) and means the goal auto-reflects task progress with no manual ticking.
- **Hard checks duplicated, not shared.** The 3 plan-goal checks are re-implemented inline rather than factored into a shared verb. Keeps both commands self-contained; revisit if a third caller needs the same logic.
- **Planning is non-skippable.** A goal in `status: in_progress, phase: todo` (or empty phase) is refused with a pointer to `/plan-goal`. Blocks, unlike `/work-on-goal`'s informational nudge.
- **Reads `~/.claude/plugins/marketplaces/vault-cli/docs/goal-writing.md`** as the canonical rule source for the 3 hard checks — same source `/plan-goal` and `goal-auditor` use.

## Integration

Goal lifecycle:

1. `/vault-cli:create-goal` / `/vault-cli:launch-goal` — capture / frame
2. `/vault-cli:work-on-goal` — orient (pick a task, get guides)
3. `/vault-cli:plan-goal` — sharpen (3 hard gates); never flips phase
4. **`/vault-cli:execute-goal`** — the gate; flips planning → execution + recommends the next open task, one at a time — this command
5. Work each task via the task lifecycle (`/vault-cli:work-on-task` → `plan-task` → `execute-task` → `complete-task`), then re-run `/vault-cli:execute-goal` for the next
6. `/vault-cli:verify-goal` → `/vault-cli:complete-goal` — confirm Success Criteria + close the goal once all tasks drain

Output ends with one of:
- `🎯 Next task: [[<Title>]] → Run /vault-cli:work-on-task "<Title>".` (gate passed, task remaining)
- `✅ All tasks complete — the goal has drained. → /vault-cli:verify-goal → /vault-cli:complete-goal.` (drained)
- `❌ Plan not ready. Run /vault-cli:plan-goal first.` (hard checks failed)
- `❌ Planning gate not run. Run /vault-cli:plan-goal first.` (phase: todo)
- `❌ Goal closed (...).` (status/phase terminal)
- `❌ No goal detected. Pass a goal identifier or name.` (input error)
