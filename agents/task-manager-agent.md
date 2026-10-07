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

- Tasks directory: `<tasks_dir>` (resolved from vault-cli config — never a hardcoded folder name)
- Goals directory: `<goals_dir>` (resolved from vault-cli config — never a hardcoded folder name)

**ALWAYS get current date/weekday at start:** `date +"%Y-%m-%d %A %u"`

## Shared Operations

### find_task(name_or_path)

Search for task file by name or path.

**Algorithm:**
1. If input has `.md` extension and path exists → return path
2. If input starts with `<tasks_dir>/` → try that path
3. Otherwise search: `Glob pattern="<tasks_dir>/*.md"`, filter by name match
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

### parse_goal_sentence(goal_path)

Extract the goal sentence — the goal's own one-line statement of intent (the writing guide names this paragraph the **Summary**), and the first source a linked task may serve. ⚠️ **Mirrors `goal-manager-agent.md` § parse_goal_sentence, and the two must stay identical:** both agents run the same three-source check against the same goal file, so a divergence here is two different answers to one question.

**Algorithm:**
1. Skip the frontmatter block (the first two `---` lines), the `Tags:` line, and the `---` content separator that follows it — **three `---` lines in all.** ⚠️ **The vault page template carries three** (`example/vault/23 Goals/Example Goal.md` lines 1, 4, 7). Skipping too few misfires two ways — stop after the first and the frontmatter keys are read as the paragraph; stop after the second and the separator itself is. ⚠️ A page carrying only **two** (authored without a `Tags:` line) is **not** this case: return empty and let step 5's absent-source rule handle it, rather than returning a truncated sentence.
2. Take the prose paragraph that follows, up to the first `# ` heading (normally `# Impact`)
3. Return it verbatim

### parse_definition_of_done(goal_path)

Extract the goal's Definition of Done items — the third source a linked task may serve. ⚠️ **Mirrors `goal-manager-agent.md` § parse_definition_of_done, for the same reason:** the `DoD<n>` index this file cites must be the one the goal side cites.

**Algorithm:**
1. Find `# Definition of Done` section
2. Walk the section as a sequence of blocks and extract each block **once**: first the lines between the `# Definition of Done` heading and the next `## ` heading, then the body of each `## <subsection>` in turn. ⚠️ **Block-by-block is the point** — reading "the section body" as everything up to the next `# ` heading also swallows the subsections, and every nested line is then counted twice, shifting the `DoD<n>` indices a verdict cites
3. Return the item lines verbatim, with their 1-based index
4. ⚠️ **Absent section → return an empty list, not an error** — a goal without one has two serving sources, not three, and the absence is never itself reported as a necessity defect
- Count totals

## Actions

### status

Emit a grouped-checkbox status report for a resolved task path, including a read-only phase/plan assessment. The slash command (`commands/task-status.md`) handles conversation-based task detection, the inline `/sync-progress` step, AND the Async State Closer anchor pair (`🎯 Goal:` / `📌 Task:` clickable lines) before invoking this action; this agent only reads, parses, classifies, and formats. Do NOT emit links — the anchor pair is the command's output, not the agent's.

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

   Extract as `outcome`. Strip trailing `**` / `_` / leading bullets. Truncate to ~140 chars **at the nearest preceding word boundary** (split on whitespace, not on mid-word characters); append `…` suffix if truncated. The boundary rule keeps multi-byte / grapheme-cluster characters intact and avoids visual artifacts on emoji / CJK content.

   Edge cases that all resolve to `outcome = ""` (and the line is omitted in step 7):
   - Task file has no frontmatter / no post-frontmatter `---` separator (legacy or hand-edited files)
   - First non-heading block is empty or whitespace-only
   - First non-heading block is itself an injected `## Pull Requests` / `## Results` section with no Summary paragraph above it

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
   If `total == 0`, render `<no checkboxes>` after the header and stop after step 8.

6. **Compute phase/plan assessment.** Classify the task's phase against its plan. **Recommend-only, never mutating:** do NOT write status, phase, or any checkbox — this step reads and classifies only. Per the Task Lifecycle Guide, manual phase-setting is an anti-pattern; `/vault-cli:execute-task` and `/vault-cli:complete-task` are the sole phase flippers, and `/plan-task` never flips itself.

   Compute from step 1 (`status`, `phase`) and step 3's parsed sections:
   - `validated` = SC section exists with ≥ 2 binary checkboxes AND Tasks section exists with ≥ 1 checkbox
   - `all_sc_ticked` = SC exists AND every SC checkbox is `[x]`
   - `tasks_done` / `tasks_total` = `[x]`-count / total count in the `# Tasks` section (only verbatim `[x]` counts as done; `[/]` is not done)

   Classify — first match wins:
   - status `completed` / `aborted` OR phase `done` → branch `closed` · recommend none
   - phase `ai_review` → branch `ai_review` · recommend none ("agent review in progress")
   - phase `human_review` → branch `human_review` · recommend `/vault-cli:complete-task`
   - status `next` / `backlog` (phase `todo`/empty) → branch `not-started` · recommend `/vault-cli:work-on-task`
   - phase `planning` → branch `plan-ready` · recommend `/vault-cli:execute-task`
   - status `in_progress` + phase `todo` → branch `gate-not-run` · recommend `/vault-cli:plan-task`
   - phase `execution` + not `validated` → branch `plan-unvalidated` · recommend `/vault-cli:plan-task`
   - phase `execution` + `all_sc_ticked` → branch `plan-complete` · recommend `/vault-cli:complete-task`
   - phase `execution` → branch `in-progress` · recommend none ("continue")
   - anything else → branch = phase verbatim · recommend none

   Build the `Plan:` line:
   - `validated` → `Plan: validated · {tasks_done}/{tasks_total} subtasks · {all_sc_ticked ? complete : not complete}`
   - not `validated` → `Plan: not started (missing SC/Tasks)`

7. **Extract next step.** Walk sections in priority order (Success Criteria → Tasks → Definition of Done); within each section, return the text of the first `[ ]` or `[/]` item (prefer `[ ]` when both exist at same position). If all items are `[x]`, the next step is `✅ Task complete. Run /complete-task to close.`

   This is a quick hint, NOT a full recommendation. For an action-prioritized list with deferrals + interactive pick, use `/vault-cli:next-steps`.

8. **Render output** — `OUTPUT=grouped-checkbox` (default):
   ```
   Phase: {branch}
   Plan: {plan line}
   Recommend: {command | none — reason}

   Task: {task_name}
   Outcome: {outcome}
   Status: {status} · phase: {phase} · {completed}/{total} ({percent}%)

   ## Success Criteria
   {glyph} {text}
   ...

   ## Tasks
   {glyph} {text}
   ...

   ## Definition of Done
   {glyph} {text}
   ...

   Next: {next_step_text}
   ```

   **Rules:**
   - The assessment block (`Phase:` / `Plan:` / `Recommend:`) always renders at the very top for `grouped-checkbox` output, blank line after. It is computed in step 6 — never omit it, never mutate state to produce it.
   - `Outcome:` line is omitted entirely when `outcome` is empty (legacy task with no Summary paragraph). When present, it's the contract reminder — "what's true when this is done" — and sits above the volatile Status line for at-a-glance scanning.
   - Section header (e.g. `## Success Criteria`) only prints when the section exists AND has ≥ 1 checkbox. Empty sections are omitted entirely (no header, no body).
   - Map the disk's state token to a display glyph per line — never echo the raw token: `[x]` → `✅`, `[/]` → `⏳`, `[ ]` → `❌`. (Disk is untouched; the markdown checkbox is the source of truth and stays `[x]`/`[/]`/`[ ]` in the vault file.)
   - Emit a one-line legend with the `ℹ️` info marker directly under the Status line when the report has ≥ 1 non-`[x]` item: `ℹ️ legend: ✅ done · ⏳ in-progress · ❌ pending`. Omit it when everything is `[x]` (the glyphs are self-evident from a fully-complete report).
   - One blank line between sections for visual grouping.
   - `Next:` is one line, ends the output, names one concrete action.

9. **Legacy flat mode** — `OUTPUT=flat`:
   ```
   📋 Task: {task_name}
   Progress: {completed}/{total} ({percent}%)
   🎯 Next: {next_step}
   ```

   Used by callers that haven't migrated yet (e.g. internal scripts). Default callers receive `grouped-checkbox`. Flat mode does not surface the outcome line — orchestration callers don't need it.

10. **Warnings (append after the report):**
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

5. **Check goal-necessity (forward) — the serving item is named:**
   - For each goal linked in the `goals` field (resolved in step 4), locate the goal file under `<goals_dir>` (resolved from vault-cli config — never a hardcoded folder name) and read its three sources **through the shared operations, never by an ad-hoc read**: `# Success Criteria` via `parse_success_criteria`, `# Definition of Done` via `parse_definition_of_done`, and the goal sentence (the writing guide names this paragraph the **Summary**) via `parse_goal_sentence`. ⚠️ **The parsers are what make the reads correct** — the DoD walk de-duplicates nested `## <subsection>` bodies and the sentence parser skips exactly three `---` lines; an inline re-read loses both and shifts the `DoD<n>` index this file cites away from the one `goal-manager-agent` cites for the same goal. Skip any linked goal whose file was already flagged unresolvable in step 4.
   - For each readable linked goal, determine **which of three sources this task's outcome serves, or that it serves none**:
     - **(a) the goal sentence**
     - **(b) a specific Success Criterion**, cited as `SC<n>`
     - **(c) a specific Definition of Done item**, cited as `DoD<n>`
   - ⚠️ **A passing verdict quotes three things**: the source kind (`goal sentence` / `SC<n>` / `DoD<n>`), the **exact line served**, and **the task line that advances it**. Judge with this fixed semantic anchor (cite it when reasoning; see `docs/goal-writing.md` § Non-goals — the scope-creep guard and § Tasks as Business-Value Milestones → Foundation/skeleton work): a task is *needed* iff it serves one of the three sources above OR is explicitly framed as a needed foundation task (e.g. "foundation; enables iteration"). Work-breakdown slices, scope-creep items, and padding are NOT needed. A task whose domain the goal's `# Non-goals` section explicitly excludes is also NOT needed.
   - ⚠️ **STRICTER THAN A BARE "advances ≥ 1 SC" TEST.** A match inferred from a task title, from a shared theme, or from a criterion that is only loosely related is reported `unproven` and counts as a **fail**, never a pass. If you cannot quote the served line **and** the task line, the verdict is not a pass — never round an unquotable match up to `needed`. ⚠️ **The foundation row below is the one route to a pass that is not one of the three sources, and it is not an escape hatch:** the semantic anchor's *"explicitly framed as a needed foundation task"* is satisfied only when **a quotable line names the criterion it is a foundation for** — either this task's own file, **or the linked goal's `# Tasks` entry for it**, which is the form `docs/goal-writing.md` § Foundation/skeleton work itself exemplifies (*`1. [[Set Up Multi-Provider Proxy Project Skeleton]] — … (foundation; enables iteration)`*). ⚠️ **Prefer the task's own file when both exist:** the goal-side list is a derived copy that can be stale. Either way the line is quoted. A foundation claim with nothing to cite is `unproven`, exactly like any other unquotable match.
   - Report **one row per linked goal** — the row is the deliverable, a count is not:
     - `✓ goal <goal> — serves <goal sentence|SC<n>|DoD<n>>: "<served line>" ← "<task line>"`
     - `✓ goal <goal> — foundation for <goal sentence|SC<n>|DoD<n>>: "<the foundation line>" ← "<task line>"`
     - `✗ task not needed by linked goal <goal> — serves none of {goal sentence, Success Criteria, Definition of Done}`
     - `? goal <goal> — unproven: <why neither line could be quoted>`
   - ⚠️ **The `✗` row names the three sources it tested against**, so a `none` verdict is distinguishable from a run that tested SCs only. ⚠️ **The `✓` row is mandatory for every clean link** — a run emitting rows only for failures is indistinguishable from one that judged nothing.
   - If the goal's `# Non-goals` explicitly exclude the task's domain → report `✗ task not needed by linked goal <goal> — goal Non-goals exclude this task's domain` instead. ⚠️ **That row counts in `{none}` like the other `✗`** — the scope exclusion is carried by the row's own text, not by a separate counter, so the reconciliation stays four-term.
   - If a linked goal has **no parseable `# Success Criteria` AND no `# Definition of Done` AND no goal sentence** → emit info line `cannot evaluate necessity — goal <goal> has no parseable Success Criteria, Definition of Done or goal sentence` and take no verdict for that goal (the structural checks already flag the missing sections). ⚠️ **That link still counts in `{skipped}`**, so the count line reconciles with the link present and no verdict issued for it. ⚠️ **One missing source is not this case** — a goal with an SC section but no DoD is evaluated over the two sources it has, and the absence of a DoD is never itself reported as a necessity defect.
   - Necessity rows are appended to the report body; the `✗` and `?` rows also join its `✗ {specific issues}` list (flipping the report to the `❌ Task Issues` shape). Info lines are plain lines — not `✗` issues, not pass/fail.
   - Advisory only: report only — never modify goal or task files, never auto-remove or re-link.

6. **Check Success Criteria section:**
   - If missing → ERROR

7. **Check DoD section:**
   - If missing → info only (optional)

8. **Check checkboxes:**
   - Count in Success Criteria and DoD sections
   - If total = 0 → warning

9. **Check status consistency:**
   - completed → should be 100% checkboxes
   - 100% checkboxes → should be completed

10. **Report:**
   ```
   ✅ Task Valid: [[{task_name}]]
   Status: {status}
   Parent: linked
   Success Criteria: present, {N} checkboxes
   Consistency: aligned
   Necessity: {linked} links · {serving} serving · {none} none · {unproven} unproven · {skipped} skipped
     ✓ goal <goal> — serves <goal sentence|SC<n>|DoD<n>>: "<served line>" ← "<task line>"
   ```
   or
   ```
   ❌ Task Issues: [[{task_name}]]
   ✗ {specific issues}
   Necessity: {linked} links · {serving} serving · {none} none · {unproven} unproven · {skipped} skipped
     ✓ … / ? … / ✗ …
   ```
   ⚠️ **The `Necessity:` block renders in BOTH shapes, and is not optional.** It is a reading, not an issue list — a task whose links are all clean still carries its `✓` rows, because without them a run that judged every link and found it serving is indistinguishable from a run that judged nothing. That is the exact failure the mandatory-`✓` rule exists to prevent. In the `❌` shape the `✗` and `?` rows appear **twice on purpose**: once in the `Necessity:` block under their own prefixes, and once in `✗ {specific issues}` — an unproven link is an issue, and a reader scanning the issue list must not have to find the necessity block to see it. ⚠️ **`{serving}` counts both `✓` row kinds** — the `serves` row and the `foundation` row are both passes, and `{none}` likewise covers both `✗` kinds (serves-none and Non-goals-excluded). ⚠️ **`{skipped}` covers links the check could not judge** — a linked goal whose file was unresolvable (step 4) or that carries none of the three sources (the info-line path). It is listed separately so the line **reconciles**: `serving + none + unproven + skipped = linked`. Without it those links count in `{linked}` and in no verdict, and a count line whose parts do not sum reads as a measurement when it is an omission.

## Error Handling

- "Task not found: {name}"
- "Multiple tasks match: {list}"
- "No active task detected"
