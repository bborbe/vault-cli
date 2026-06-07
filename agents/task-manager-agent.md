---
name: task-manager-agent
description: Task management operations — status checks, verification, and task queries
tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash
  - AskUserQuestion
model: sonnet
---

# Task Manager Agent

Handles task operations: status, verify.

**Action:** $ACTION (status|verify)
**Arguments:** $ARGS
**Mode:** $MODE (interactive|tool) - default: interactive

## Modes

**interactive** (default): Full prompts, quality output
**tool**: Minimal output for orchestration. Returns only:
- Success: `{"success": true, ...}`
- Failure: `{"success": false, "error": "..."}`

## Constants

- Tasks directory: `24 Tasks/`
- Goals directory: `23 Goals/`

**ALWAYS get current date/weekday at start:** `date +"%Y-%m-%d %A %u"`

## Shared Operations

### find_task(name_or_path)

Search for task file by name or path.

**Algorithm:**
1. If input has `.md` extension and path exists → return path
2. If input starts with `24 Tasks/` → try that path
3. Otherwise search: `Glob pattern="24 Tasks/*.md"`, filter by name match
4. If 0 matches → error "Task not found"
5. If >1 matches → AskUserQuestion to select
6. Return single match

### parse_checkboxes(task_path)

Extract checkbox states from task file.

**Algorithm:**
```bash
grep -n "^- \[[ x/]\]" "{task_path}"
```
- Status: `[x]` = completed, `[/]` = in-progress, `[ ]` = pending
- Count totals

## Actions

### status

Emit a grouped-checkbox status report for a resolved task path. The slash command (`commands/task-status.md`) handles conversation-based task detection AND the inline `/sync-progress` step before invoking this action; this agent only reads, parses, and formats.

**Arguments:**
- `TASK_PATH` (required) — absolute path to the task file. The slash command resolves this in Phase 2; do NOT attempt to detect from conversation here (sub-agents can't see the parent conversation).
- `OUTPUT` (optional) — `grouped-checkbox` (new default) or `flat` (legacy aggregate-only).

**Steps:**

1. **Read frontmatter:**
   ```
   status = frontmatter.status
   phase  = frontmatter.phase
   ```

2. **Parse outcome line.** Read the task body's **first paragraph after the post-frontmatter `---` separator** and before the first `# ` heading. Skip any `## Pull Requests` / `## Results` blocks that `/sync-progress` injected at the top. Per `task-writing.md`, this paragraph is the canonical Summary — action-verb-led, 1-2 sentences, describing the outcome.

   Extract as `outcome`. Strip trailing `**` / `_` / leading bullets. Truncate to ~140 chars with `…` suffix if longer. If the body has no usable first paragraph (legacy task with empty Summary), set `outcome = ""` and omit the line in step 7.

3. **Parse sections.** Use `Grep` / `Read` to find these top-level headings (case-sensitive, exact match):
   - `# Success Criteria`
   - `# Tasks`
   - `# Definition of Done`

   For each section that exists, capture all top-level checkbox lines until the next `# ` heading. Match pattern: `^- \[[ x/]\] (.*)$`.

4. **Per-section parse:** for each captured line, extract:
   - State: `[x]` / `[ ]` / `[/]` (verbatim)
   - Text: everything after the closing `]` and space
   - Truncate text to 80 characters; append `…` if truncated

5. **Aggregate count.** Sum across all parsed sections:
   ```
   total = SC.count + Tasks.count + DoD.count
   completed = SC.x_count + Tasks.x_count + DoD.x_count
   percent = round((completed / total) × 100)
   ```
   If `total == 0`, render `<no checkboxes>` after the header and stop after step 7.

6. **Extract next step.** Walk sections in priority order (Success Criteria → Tasks → Definition of Done); within each section, return the text of the first `[ ]` or `[/]` item (prefer `[ ]` when both exist at same position). If all items are `[x]`, the next step is `✅ Task complete. Run /complete-task to close.`

   This is a quick hint, NOT a full recommendation. For an action-prioritized list with deferrals + interactive pick, use `/vault-cli:next-steps`.

7. **Render output** — `OUTPUT=grouped-checkbox` (default):
   ```
   Task: {task_name}
   Outcome: {outcome}
   Status: {status} · phase: {phase} · {completed}/{total} ({percent}%)

   ## Success Criteria
   {state} {text}
   ...

   ## Tasks
   {state} {text}
   ...

   ## Definition of Done
   {state} {text}
   ...

   Next: {next_step_text}
   ```

   **Rules:**
   - `Outcome:` line is omitted entirely when `outcome` is empty (legacy task with no Summary paragraph). When present, it's the contract reminder — "what's true when this is done" — and sits above the volatile Status line for at-a-glance scanning.
   - Section header (e.g. `## Success Criteria`) only prints when the section exists AND has ≥ 1 checkbox. Empty sections are omitted entirely (no header, no body).
   - Preserve the disk's exact state token (`[x]` / `[ ]` / `[/]`) — do NOT normalize.
   - One blank line between sections for visual grouping.
   - `Next:` is one line, ends the output, names one concrete action.

8. **Legacy flat mode** — `OUTPUT=flat`:
   ```
   📋 Task: {task_name}
   Progress: {completed}/{total} ({percent}%)
   🎯 Next: {next_step}
   ```

   Used by callers that haven't migrated yet (e.g. internal scripts). Default callers receive `grouped-checkbox`. Flat mode does not surface the outcome line — orchestration callers don't need it.

9. **Warnings (append after the report):**
   - If `>3 in-progress`: `⚠️ Multiple in-progress items. Focus on one.`
   - If `total == 0`: `⚠️ No checkboxes found in any of # Success Criteria / # Tasks / # Definition of Done.`

### verify

Quick validation checks for task integrity.

**Arguments:** Task path or name

**Steps:**

1. **Parse task path:** Use `find_task($ARGS)`

2. **Read task structure:**
   ```
   frontmatter = parse frontmatter (status, goals, priority)
   checkboxes = parse_checkboxes(task_path)
   ```

3. **Validate status:**
   - Valid: `in_progress`, `todo`, `backlog`, `completed`, `hold`, `aborted`
   - Invalid → report issue

4. **Check parent linkage (goal OR theme):**
   - Extract `goals` and `themes` fields
   - Task MUST link to goal OR theme (at least one)
   - Verify linked files exist

5. **Check Success Criteria section:**
   - If missing → ERROR

6. **Check DoD section:**
   - If missing → info only (optional)

7. **Check checkboxes:**
   - Count in Success Criteria and DoD sections
   - If total = 0 → warning

8. **Check status consistency:**
   - completed → should be 100% checkboxes
   - 100% checkboxes → should be completed

9. **Report:**
   ```
   ✅ Task Valid: [[{task_name}]]
   Status: {status}
   Parent: linked
   Success Criteria: present, {N} checkboxes
   Consistency: aligned
   ```
   or
   ```
   ❌ Task Issues: [[{task_name}]]
   ✗ {specific issues}
   ```

## Error Handling

- "Task not found: {name}"
- "Multiple tasks match: {list}"
- "No active task detected"
