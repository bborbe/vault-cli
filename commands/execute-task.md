---
description: Gate planning → execution. Re-runs plan-task's hard non-negotiables; on pass, flips phase + prints first subtask + DoD reminder.
argument-hint: "<task-file-path-or-name> (or detects from conversation)"
allowed-tools: [Read, Edit, Glob, Bash, AskUserQuestion, Task]
---

The **hard gate** between planning and execution. Refuses to flip `phase: planning → execution` unless plan-task's 4 hard non-negotiables pass. Idempotent on `phase: execution` — re-prints first subtask + DoD as a session-start reminder. Closes the lifecycle's final operational gap: every phase transition now has an enforced command.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## When to use

After `/vault-cli:plan-task` (or any time the plan is genuinely complete) to formally enter execution. Can also be re-run mid-session as "where was I?" — it re-surfaces the first unchecked subtask + DoD without side effects.

```bash
/vault-cli:execute-task                              # detects from conversation
/vault-cli:execute-task "Some Task Name"
/vault-cli:execute-task 24\ Tasks/Some\ Task.md
```

## Process

### 1. Resolve task path

**With argument:** exact path if path-like, else `Glob` `<tasks_dir>/*<arg>*.md` (vault-cli config respected). Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (same priority order as `/plan-task`):

1. Most recent `/create-task` / `/plan-task` / `/work-on-task` output — scan the parent conversation for resolved task name.
2. Most recent `[[Task Name]]` wikilink referenced as a task subject.
3. Daily note's first `[/]` checkbox.
4. Most recently modified file in `<tasks_dir>/`.

Multiple matches → ask via `AskUserQuestion`. Zero → `❌ No task detected. Pass a task identifier or name.` STOP.

Print `Detected task: <name>` on first line so owner can interrupt before any state mutation.

### 2. Read status + phase

```bash
vault-cli task get "<name>" status --output json
vault-cli task get "<name>" phase --output json
```

### 3. Refusal cases (no mutation, exit non-zero)

Refuse and STOP if any apply:

- `status: completed` OR `status: aborted` → `❌ Task closed (status: <value>). Run reopen if you need to continue work.`
- `phase: done` → `❌ Task phase is done. Run reopen if work needs to resume.`
- `phase: todo` OR `phase` empty AND `status: in_progress` → `❌ Planning gate not run. Run /vault-cli:plan-task first.` (planning is non-skippable per [[Phase-Gated Task Flow]])

### 4. Status entry contract (mutate, then continue)

If `status` is in `next` / `backlog` / `hold` → flip to `in_progress`:
```bash
vault-cli task set "<name>" status in_progress
```
Print: `ℹ️ Status: <old> → in_progress (resume from <old>)`

If `status` is already `in_progress` → continue, no mutation.

### 5. Run the 4 hard non-negotiables (re-check planning)

Copy-paste from `/vault-cli:plan-task` § 5 *Hard* checks — DO NOT factor into a shared helper yet (keeps both commands self-contained; lift to a `vault-cli task verify-plan` CLI verb if a third caller appears). Read `~/.claude/plugins/marketplaces/vault-cli/docs/task-writing.md` as the canonical rule source.

Check (in order, collect ALL failures — don't short-circuit):

1. **Success Criteria defined** — `# Success Criteria` section exists with ≥ 2 binary checkboxes.
2. **Subtasks reach the goal** — `# Tasks` section lists concrete steps that, if completed, produce the SC outcomes.
3. **E2E verify subtask present** — for shipping-class tasks (PR / release / plugin update / agent / deploy / library publish; subtasks reference git repo / marketplace / registry), `# Tasks` must include a subtask with concrete procedure + observable outcome (no dishonest-tick phrases — see `plan-task.md:71-80` for the rejection list).
4. **Subtask-goal alignment** — every `# Tasks` checkbox maps by topic to ≥ 1 `# Success Criteria` outcome, OR is the e2e verify subtask.

Skip check #3 entirely for non-shipping-class tasks (pure research, decision, doc-only with no published artifact).

### 6. Phase transition or refusal

**If ANY hard check failed AND `phase: planning`:**

Print:
```
❌ Plan not ready. Run /vault-cli:plan-task first.

Failed checks:
- <check name>: <one-line reason>
...
```
STOP. Do NOT flip phase.

**If all hard checks pass AND `phase: planning`:**

```bash
vault-cli task set "<name>" phase execution
```
Print: `✅ Phase: planning → execution`

Continue to step 7.

**If `phase: execution` / `ai_review` / `human_review`:** no flip, no check (already past the gate). Continue to step 7 idempotently. Print: `ℹ️ Already in execution (phase: <value>) — re-surfacing context.`

### 7. Surface first subtask + DoD (always — final output)

Parse the task file:
- First unchecked `- [ ]` checkbox under `# Tasks` (or equivalent section) → "Start with"
- All `# Definition of Done` items → "When done, verify"

Print:
```
🎯 Start with: <first unchecked subtask text, truncated to ~120 chars>

📋 When done, verify:
- <DoD bullet 1>
- <DoD bullet 2>
...
```

If `# Tasks` has zero unchecked items: print `✅ All subtasks complete — run /vault-cli:complete-task` instead of the "Start with" line.

If `# Definition of Done` is absent or empty: omit the "When done, verify" block (no warning — some non-shipping-class tasks legitimately have no DoD).

## Notes

- **Idempotent re-entry.** Safe to re-run on `phase: execution` — no mutation, just re-prints the work block + destination. Useful as a session-start "where was I?" command.
- **Hard checks duplicated, not shared.** The 4 plan-task checks are re-implemented inline rather than factored into a sub-agent or shared CLI verb. Keeps both commands self-contained and fast; revisit if a third caller (e.g. `/vault-cli:complete-task` pre-check) needs the same logic.
- **Planning is non-skippable.** A task in `status: in_progress, phase: todo` (or empty phase) is refused with a pointer to `/plan-task`. This is the stricter sibling of `/work-on-task`'s informational nudge: nudge informs, execute-task blocks.
- **Status flips happen, phase flips don't (when planning gates fail).** Resume-from-paused is a separate concern from "is planning complete" — flipping `hold → in_progress` is always safe; flipping `planning → execution` requires the gates.
- **No daily-note tracking, no guide search.** Those belong to `/vault-cli:work-on-task`. This command is purely the gate + work-block kickoff.
- **Reads `~/.claude/plugins/marketplaces/vault-cli/docs/task-writing.md`** as the canonical rule source for the 4 hard checks — same source `/plan-task` and `task-auditor` use.

## Integration

Task lifecycle:

1. `/vault-cli:create-task` — capture (lenient)
2. `/vault-cli:work-on-task` — orient (status + guides + daily note), then auto-chain plan → execute (interactive)
3. `/vault-cli:plan-task` — sharpen (5 hard gates); never flips phase
4. **`/vault-cli:execute-task`** — the sole gate; flips planning → execution + prints first subtask + DoD reminder — this command
5. Start work — while working, use any of:
   - `/vault-cli:update-task` — log completed work, sync to daily note / parent goal
   - `/vault-cli:task-status` — grouped-checkbox status (Success Criteria / Tasks / DoD) + next step
   - `/vault-cli:next-steps` — next actionable steps; offer defer if nothing left today
6. `/vault-cli:sync-progress` — flush conversation to daily note + task pages
7. `/vault-cli:complete-task` — close task
8. `/vault-cli:session-close` — verify session is safe to end (synced, committed, no orphaned state)

Output ends with one of:
- `🎯 Start with: <subtask>` + `📋 When done, verify: <DoD>` (gate passed or idempotent re-entry)
- `❌ Plan not ready. Run /vault-cli:plan-task first.` (hard checks failed)
- `❌ Task closed (...). Run reopen if you need to continue work.` (status/phase terminal)
- `❌ Planning gate not run. Run /vault-cli:plan-task first.` (phase: todo)
- `❌ No task detected. Pass a task identifier or name.` (input error)
