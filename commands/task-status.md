---
description: Sync conversation progress to disk, then show grouped-checkbox task status (Success Criteria / Tasks / Definition of Done) with verbatim state and next step.
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

Quick "where was I?" recovery tool. Detects active task from the parent conversation, syncs any in-flight progress to disk first (via `/vault-cli:sync-progress` logic, inline), then emits a grouped-checkbox status report.

**Important side-effect:** this command mutates the vault (daily note + task page) before reporting. The mutation reflects work the conversation has already done — not new content. If you want a pure read without disk writes, use `/vault-cli:verify-task` instead.

**This command must stay inline** — Phase 1 (sync) and Phase 2 (task detection) both analyze the parent conversation; a sub-agent cannot see the conversation. Only the final output formatting (Phase 3) delegates to `task-manager-agent`.

## Phase 1: Sync progress from conversation

Inline. Run the `/vault-cli:sync-progress` logic against the parent conversation:

1. Detect vault context (cwd or wiki-link evidence).
2. Analyze conversation for completion signals (PRs opened, files committed, tests passing, releases shipped).
3. Update daily note + task page + task-page `## Pull Requests` / `## Results` sections.
4. Run Phase 4 auto-complete check (strict 4-criteria objective gate from `commands/sync-progress.md`).
5. If sync produced no changes, log `(sync: no-op — disk already fresh)` and continue.
6. If sync produced changes, log a one-line summary of what was written (`(sync: wrote {N} sections; {task} ticked {M} checkboxes)`).

This phase MUST run before Phase 3 — the status report reads disk, so disk must be fresh.

## Phase 2: Detect active task

Inline. Scan the parent conversation in priority order:

1. Most recent `/vault-cli:create-task` output → use that name.
2. Most recent `[[Task Name]]` wikilink referenced as a task subject (not generic prose mention).
3. Daily note's first `[/]` checkbox.
4. Most recently modified file in `<tasks_dir>/`.

Resolve the detected name via `Glob` `<tasks_dir>/*<arg>*.md`. Multiple matches → list candidates, ask via `AskUserQuestion`. Zero → `❌ No active task detected. Pass a task identifier or name.` STOP.

Print `Detected task: <name>` on first line so the owner can interrupt if wrong before Phase 3 runs.

## Phase 3: Generate grouped-checkbox status report

Delegate to `task-manager-agent`:

```
Task tool with:
  subagent_type: 'vault-cli:task-manager-agent'
  prompt: 'ACTION: status
           TASK_PATH: <resolved-path-from-phase-2>
           MODE: interactive
           OUTPUT: grouped-checkbox

           Read the task file (already disk-fresh after sync). Parse # Success Criteria,
           # Tasks, # Definition of Done sections. Emit grouped-checkbox output per the
           agent contract.'
```

The agent does NOT detect from conversation in this phase — Phase 2 already resolved the path. The agent only reads, parses, formats.

## Output shape (from task-manager-agent)

```
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
