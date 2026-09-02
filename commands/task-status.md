---
description: Sync conversation progress to disk, then show grouped-checkbox task status (Success Criteria / Tasks / Definition of Done) with a phase/plan assessment, verbatim state, and next step.
argument-hint: (detects from conversation)
allowed-tools:
  - Read
  - Edit
  - Grep
  - Glob
  - Bash(vault-cli:*)
  - Bash(grep:*)
  - Bash(command -v:*)
  - Task
---

Quick "where was I?" recovery tool. Detects active task from the parent conversation, syncs any in-flight progress to disk first (via `/vault-cli:sync-progress` logic, inline), re-evaluates the task's phase and plan state against its `# Success Criteria` / `# Tasks` sections, then emits a grouped-checkbox status report.

**Important side-effect:** this command mutates the vault (daily note + task page) before reporting. The mutation reflects work the conversation has already done — not new content. If you want a pure read without disk writes, use `/vault-cli:verify-task` instead.

**This command must stay inline** — Phase 1 (sync), Phase 2 (task detection), and Phase 2.5 (phase/plan re-evaluation) all analyze the parent conversation; a sub-agent cannot see the conversation. Only the final output formatting (Phase 3) delegates to `task-manager-agent`.

## Phase 1: Sync progress from conversation

Invoke the skill. Literally this call, not an inlined equivalent:

```
Skill: vault-cli:sync-progress
```

**"Run the sync-progress logic" is not a licence to reimplement it here.** Observed 2026-08-10: the agent hand-rolled the steps, checked the daily note directly, and skipped writing the session entry — the same misread that hit `/vault-cli:session-close` Phase 2 minutes earlier.

**`commands/sync-progress.md` is the single source of truth for what this does.** Deliberately not restated here: a copy of its phase list would drift the first time that command changes, and a stale copy is exactly what invites hand-rolling it again. Read the command if you need the detail.

Log its outcome in one line before continuing — `(sync: no-op — disk already fresh)` or `(sync: wrote {N} sections; {task} ticked {M} checkboxes)`.

This phase MUST run before Phase 3 — the status report reads disk, so disk must be fresh.

## Phase 2: Detect active task

Inline. Scan the parent conversation in priority order:

1. Most recent `/vault-cli:create-task` output → use that name.
2. Most recent `[[Task Name]]` wikilink referenced as a task subject (not generic prose mention).
3. Daily note's first `[/]` checkbox.
4. Most recently modified file in `<tasks_dir>/`.

Resolve the detected name via `Glob` `<tasks_dir>/*<arg>*.md`. Multiple matches → list candidates, ask via `AskUserQuestion`. Zero → `❌ No active task detected. Pass a task identifier or name.` STOP.

Print `Detected task: <name>` on first line so the owner can interrupt if wrong before Phase 3 runs.

## Phase 2.5: Re-evaluate phase & plan state

Inline. Read the resolved task file (from Phase 2) and classify the task's phase against its plan. Do NOT delegate — the classification is a diagnostic that sits in the conversation context.

**Recommend-only, never mutating.** Per [[Task Lifecycle Guide]], manual phase-setting via `vault-cli task set` is a documented anti-pattern; `/vault-cli:execute-task` (planning → execution) and `/vault-cli:complete-task` (→ done) are the sole phase flippers, and `/plan-task` never flips itself. This step reads and classifies only — zero frontmatter writes.

1. Read frontmatter `status` + `phase`, then the `# Success Criteria` and `# Tasks` sections.
2. Compute:
   - `validated` = SC section exists with ≥ 2 binary checkboxes AND Tasks section exists with ≥ 1 checkbox
   - `all_sc_ticked` = SC exists AND every SC checkbox is `[x]`
   - `tasks_done` / `tasks_total` = checked / total checkboxes in `# Tasks`
3. Classify — first match wins:
   - status `completed` / `aborted` OR phase `done` → branch `closed` · recommend none
   - phase `ai_review` → branch `ai_review` · recommend none ("agent review in progress")
   - phase `human_review` → branch `human_review` · recommend `/vault-cli:complete-task`
   - status `next` / `backlog` (phase `todo`/empty) → branch `not-started` · recommend `/vault-cli:work-on-task` ("not started — run /work-on-task")
   - phase `planning` → branch `plan-ready` · recommend `/vault-cli:execute-task` ("plan ready — run /execute-task")
   - status `in_progress` + phase `todo` → branch `gate-not-run` · recommend `/vault-cli:plan-task` ("planning gate not run — run /plan-task")
   - phase `execution` + not `validated` → branch `plan-unvalidated` · recommend `/vault-cli:plan-task` ("plan not validated — run /plan-task")
   - phase `execution` + `all_sc_ticked` → branch `plan-complete` · recommend `/vault-cli:complete-task` ("all Success Criteria ticked — run /complete-task")
   - phase `execution` → branch `in-progress` · recommend none ("continue")
   - anything else → branch = phase verbatim · recommend none
4. Build the `Plan:` line:
   - `validated` → `Plan: validated · {tasks_done}/{tasks_total} subtasks · {all_sc_ticked ? complete : not complete}`
   - not `validated` → `Plan: not started (missing SC/Tasks)`
5. Print the one-line preview so the owner can interrupt before Phase 3:
   `Phase assessment: <branch> — <recommend | continue>`
6. Hand the three-line assessment to Phase 3 as `ASSESSMENT` (the agent renders it verbatim, does NOT recompute).

## Phase 3: Generate grouped-checkbox status report

Delegate to `task-manager-agent`:

```
Task tool with:
  subagent_type: 'vault-cli:task-manager-agent'
  prompt: 'ACTION: status
           TASK_PATH: <resolved-path-from-phase-2>
           ASSESSMENT: <the three-line block from Phase 2.5 — Phase / Plan / Recommend>
           MODE: interactive
           OUTPUT: grouped-checkbox

           Read the task file (already disk-fresh after sync). Parse # Success Criteria,
           # Tasks, # Definition of Done sections. Emit grouped-checkbox output per the
           agent contract. Render the ASSESSMENT block verbatim at the top.'
```

The agent does NOT detect from conversation in this phase — Phase 2 already resolved the path. The agent only reads, parses, formats; it renders the Phase 2.5 assessment without recomputing it.

## Output shape (from task-manager-agent)

```
{ASSESSMENT block from Phase 2.5 — Phase / Plan / Recommend, rendered verbatim}

Task: <name>
Status: <status> · phase: <phase> · <completed>/<total> (<pct>%)

## Success Criteria
[x] <SC item text, truncated to ~80 chars>
[ ] <SC item text>
[/] <SC item text>

## Tasks
[x] <subtask>
[ ] <subtask>

## Definition of Done
[x] <DoD item>
[ ] <DoD item>

Next: <first unchecked item from SC, then Tasks, then DoD — one action>
```

If a section is absent in the task file, the agent omits the header (does NOT print an empty heading).

## Output ends with one of

- `Next: <first unchecked item>` (work remaining)
- `✅ Task complete. Run /complete-task to close.` (everything ticked)
- `❌ No active task detected. Pass a task identifier or name.` (Phase 2 zero-match)
