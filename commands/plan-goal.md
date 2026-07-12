---
description: Validate that a goal is well-formed — required sections present and every # Tasks wikilink resolves to an existing task file; conversationally fill gaps; leaves the goal at phase=planning and hands off to execute-goal (never flips phase itself).
argument-hint: "<goal-file-path-or-name> (or detects from conversation)"
allowed-tools: [Task, Read, Edit, Glob, Bash, AskUserQuestion]
---

Drive a goal to *execution-ready* through conversation. Checks that the goal has the required decision sections (`# Success Criteria`, `# Definition of Done`, `# Non-goals`) and that every task named in `# Tasks` actually exists as a file. Runs `goal-auditor` for findings, asks targeted questions, applies answers, loops until ready. Leaves the goal at `phase: planning` and points to `/vault-cli:execute-goal` to begin draining tasks — **plan-goal never flips the phase itself**; `execute-goal` owns the `planning → execution` transition.

This command **must stay inline** — it analyzes the parent conversation when no argument is given; a sub-agent cannot see the conversation.

## When to use

Right after `/vault-cli:create-goal` or `/vault-cli:launch-goal` (capture/framing → plan strict), or any time a goal feels incomplete before you start working its tasks. The goal-side mirror of `/vault-cli:plan-task`.

```bash
/vault-cli:plan-goal                              # detects from conversation (e.g. just after /launch-goal)
/vault-cli:plan-goal "Some Goal Name"
/vault-cli:plan-goal 23\ Goals/Some\ Goal.md
```

## Process

### 1. Resolve goal path

**With argument:** exact path if path-like, else `Glob` `<goals_dir>/*<arg>*.md` (vault-cli config respected). Multiple matches → list and STOP. Zero → STOP.

**Without argument — detect from conversation** (in priority order):

1. **Most recent `/create-goal` or `/launch-goal` output** — scan the parent conversation for `/vault-cli:create-goal "<name>"` / `/vault-cli:launch-goal "<name>"` (with or without slash) or its result line (`✅ Goal: <name>` / file-path output). If found and unambiguous, use that name.
2. **Most recent `[[Goal Name]]` wikilink** referenced in the conversation as a goal subject (not a generic mention in prose). Match against `<goals_dir>/`.
3. **Most recently modified file in `<goals_dir>/`** — final fallback.

Resolve the detected name via `Glob` same as the with-argument path. Multiple matches → list candidates and ask owner via `AskUserQuestion` (single-question, short options). Zero → `❌ No goal detected. Pass a goal identifier or name.` STOP.

When detection succeeds without explicit argument, print the resolved goal name on first line of output (`Detected goal: <name>`) so the owner can interrupt if wrong before any state mutation.

### 2. Read status + phase

```bash
vault-cli goal get "<name>" status --output json
vault-cli goal get "<name>" phase --output json
```

### 3. Entry contract — flip if needed

The goal is to land at `status: in_progress, phase: planning` for fresh goals; respect a deliberate post-planning phase setting.

- `status` in `next`/`todo`/`backlog` → flip status AND phase together: `vault-cli goal set "<name>" status in_progress` + `vault-cli goal set "<name>" phase planning` (if phase is empty/`todo`/`planning`). Skip the phase flip if phase is `execution` / `done` (treat as deliberate — sharpen but don't move phase backward).
- `status` already `in_progress` and `phase` is `todo`/empty → `vault-cli goal set "<name>" phase planning`
- `status` already `in_progress` and `phase` is past planning → continue without flip; step 7 will skip the phase transition.

### 4. Run goal-auditor

```
Task tool with:
  subagent_type: 'vault-cli:goal-auditor'
  prompt: 'Audit <resolved-path>. Return: score (1-10), Critical Issues, Goal Scope Fit findings, Task-Goal Alignment, top 5 Recommendations.'
```

### 5. Check the non-negotiables

Three checks beyond the auditor's general scoring — all hard (any failure → mandatory question in step 6, can't exit on auditor score alone).

**Hard:**

- **Required sections present** — each of `# Success Criteria`, `# Definition of Done`, `# Non-goals` exists as a heading. Report each missing section **by name**.
- **Success Criteria binary** — `# Success Criteria` section has ≥ 2 binary checkboxes (`- [ ]` / `- [x]`).
- **Every `# Tasks` wikilink resolves** — extract each `[[Task Title]]` from the `# Tasks` section and confirm a file `<tasks_dir>/<Task Title>.md` exists (vault-cli config `tasks_dir`; use `Glob`). Report each **unresolved task by name**. Two rules on extraction:
    1. **Skip inline-code spans** — a `[[wikilink]]` inside backticks (e.g. a literal example in prose) is NOT a task reference; exclude it. Only bare `[[...]]` in the `# Tasks` list counts.
    2. **Strip display aliases** — `[[Real Title|shown]]` resolves against `Real Title`.

    An unresolved wikilink is the goal-side equivalent of a missing subtask: the goal names work that has no home yet. The owner creates it (`/vault-cli:create-task "<title>"` or clicking the wikilink in Obsidian) before the goal is execution-ready.

Any hard check failing → mandatory question in step 6; can't exit on auditor score alone.

### 6. Surface gaps + fix loop

Translate findings (auditor + non-negotiable checks) into questions. Rules:

- Max 3 questions per turn
- Each question is short (one sentence) + tight options (single yes/no OR 2-4 numbered options)
- Lead with `(Recommended)` per global UX
- Quote the offending line/section so owner sees what triggered the question
- Use `AskUserQuestion` for the actual ask

For a **missing section**, offer to scaffold it (from the goal template / `goal-writing.md` shape) or point the owner to fill it. For an **unresolved task wikilink**, offer: create the task now (`/vault-cli:create-task "<title>"`), remove the wikilink, or leave it (blocks execution-ready). Apply each answer via `Edit`. Re-run the auditor after each batch. Print delta `Score: X → Y`. Loop until score ≥ 8 AND all three hard non-negotiables pass OR owner says "good enough."

### 7. Exit — hand off to execute-goal (no phase flip)

**plan-goal never flips the phase.** It validates and reports; `/vault-cli:execute-goal` owns the `planning → execution` transition. This keeps each lifecycle command to one job and makes "start draining the tasks" a deliberate operator action.

**Phase is `planning` AND score ≥ 8 AND hard non-negotiables pass:**

Print: `✅ Plan ready. Score: X/10. Phase stays: planning. → Run /vault-cli:execute-goal to begin working the tasks.`

**Phase is already past planning (execution / done):**

Print: `✅ Goal sharpened. Score: X/10. Phase unchanged (was <phase>).`

**Owner abort OR score < 8 after loop OR a hard check still failing:**

Print: `⚠ Goal improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run /vault-cli:plan-goal when ready.`

## Notes

- **Scope is focused on what blocks safe execution.** Plan-goal enforces three planning-gate checks (required sections present, SC binary, every task wikilink resolves) because each prevents a specific failure mode: an under-specified goal, unmeasurable success, and phantom tasks the goal names but never created. Other heuristics (SMART criteria, evidence shape, theme linkage, outcome-shaped title) stay in `goal-auditor` and `goal-writing.md` as canonical rules — surfaced via the auditor in step 4, not promoted to dedicated gates. Letting the auditor enforce general structure while plan-goal enforces the three named gates keeps the command short and the gates legible.
- **The task-wikilink gate is the signature goal check.** A goal is "planned" when the path from now to its Success Criteria is laid out as real, existing tasks. A `# Tasks` list pointing at files that don't exist is the goal-level equivalent of a task with no subtasks — the plan looks complete but there's nothing to execute.
- **No phase flip.** plan-goal never transitions phase; it validates and hands off to `/vault-cli:execute-goal`, which owns the `planning → execution` flip. Entry-contract flips (`next` → `in_progress` + `planning`) still happen in step 3.
- **Conversational on purpose.** Owner is the judge of substance. Plan-goal never silently rewrites; every change comes from an explicit answer.
- **Mechanical fixes stay in `/audit-goal`.** This command is for substance (sections, measurable SCs, real tasks), not formatting.

## Integration

Goal lifecycle:

1. `/vault-cli:create-goal` / `/vault-cli:launch-goal` — capture / frame
2. `/vault-cli:work-on-goal` — orient (pick a task, get guides)
3. **`/vault-cli:plan-goal`** — sharpen (3 hard gates); never flips `planning → execution` — this command
4. `/vault-cli:execute-goal` — gate planning → execution; flips phase + recommends the next task
5. Work the tasks via the task lifecycle (`/vault-cli:work-on-task` → `plan-task` → `execute-task` → `complete-task`)
6. `/vault-cli:complete-goal` — close the goal when all tasks + success criteria are done

Output ends with one of:
- `✅ Plan ready. Score: X/10. Phase stays: planning. → Run /vault-cli:execute-goal.` (planning success)
- `✅ Goal sharpened. Score: X/10. Phase unchanged (was <phase>).` (non-planning success)
- `⚠ Goal improved to X/10. Phase unchanged. Remaining: <bullets>. Re-run when ready.` (partial)
- `❌ No goal detected. Pass a goal identifier or name.` (input error)
