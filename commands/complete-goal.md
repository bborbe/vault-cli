---
description: Mark goal as complete (checks subtasks and success criteria)
argument-hint: "<goal-name-or-path> [--non-interactive] [--force]"
---

<objective>
Mark goal as complete using vault-cli. Verifies success criteria and linked subtasks before completion.
</objective>

<process>
1. Parse arguments:
   - If contains `--non-interactive` (or deprecated `--tool`) → MODE=non_interactive, remove flag from args
   - If contains `--force` → FORCE=true, remove flag from args
   - Otherwise → MODE=interactive
   - Extract goal name from remaining args

2. **MODE=interactive (default):**

   a. Read goal file to check completion state:
      - Find goal: `vault-cli goal show "{goal_name}" --output json`
      - Parse Success Criteria checkboxes (count `[x]`, `[/]`, `[ ]`)
      - Enumerate linked subtasks from **frontmatter**, per b.i — the page's `# Tasks` list is a derived view with no writer
      - For each subtask: read its status

   b. **Check the recorded closure route** — see [[Goal Closure Checklist]] § Closing with evidenced-but-unticked criteria. Run this BEFORE offering `--force`: it is the legitimate second door, and `--force` records nothing.

      i. **Read tasks from frontmatter — the goal page's `# Tasks` list is a derived view.** Since v0.139.0 nothing writes it, so it may be stale or absent. Enumerate with `vault-cli task list --goal "[[{goal_name}]]" --all`, which filters each task's own `goals:` frontmatter. Exact-match caveat: without the `[[ ]]` the filter returns JSON `null` with exit 0 — treat `null` as an error, not as "no tasks".

      ii. **Compare rows, never totals.** Pair each task's `status:` against *that task's own* entry on the goal page. A count difference is NEVER drift — sibling sessions completing tasks move the totals while a specific row's pair stays fixed. A per-row mismatch (task reads `status: completed`, its entry reads `[ ]`) IS drift: report the task name with both readings and repair the page before closing. Frontmatter is authoritative; the page follows it.

      iii. Collect `unticked_criteria:` entries from the goal's frontmatter and their `# Results` counterparts.
          - Any entry with `state: evidenced-and-tickable` → **REFUSE**. Print the criterion and the tick it needs. Leaving it unticked is a defect, not a choice.
          - Every incomplete criterion must be covered by an `evidenced-not-ticked` entry whose `# Results` quote can be found at its `source`. A criterion with no entry, or an entry whose quote is absent from its source, is NOT closable this way.

      iv. If every incomplete criterion is covered → skip c's prompt, run d's command, and report the entries in e. Otherwise fall through to c.

   c. If incomplete success criteria OR open subtasks (status != completed), and b did not cover them:
      - Show summary:
        - Success Criteria: X/Y complete (N%)
        - Subtasks: X/Y completed
      - List specific incomplete items (success criteria + open task names)
      - Use AskUserQuestion: 1. Complete anyway (--force) 2. Finish first 3. Show details
      - If "Finish first" → abort

   d. Run vault-cli:
      ```bash
      vault-cli goal complete "{goal_name}"{--force if FORCE or user picked option 1}
      ```

   e. Show report:
      ```
      ✅ Goal completed: [[{goal_name}]]
      ```
      - If closed via the recorded route (b), list each `evidenced-not-ticked` criterion and its cited source
      - If warnings in output, show them

3. **MODE=non_interactive (--non-interactive flag):**

   a. Read goal file to check completion state
   b. Run step 2b's recorded-route check. A refusal there — an `evidenced-and-tickable` entry, or a per-row goal/task mismatch — returns `{"success": false, "reason": "..."}` and STOPs. Never fall through to `--force`.
   c. If incomplete success criteria or open subtasks, not covered by the recorded route, and FORCE not set:
      Return: `{"success": false, "reason": "incomplete items"}`
      STOP.

   d. If complete (or FORCE):
      ```bash
      vault-cli goal complete "{goal_name}" --output json{--force if FORCE}
      ```
      Return: `{"success": true, "path": "..."}`
      STOP.

   e. Never ask questions, never use AskUserQuestion
</process>

<success_criteria>
- vault-cli goal complete invoked (NOT Edit tool for frontmatter)
- **MODE=non_interactive**: Returns JSON only, never forces unless `--force` explicitly passed
- **MODE=interactive**: Shows progress, asks if incomplete, reports result
- Parent objective updated (by vault-cli)
- `--force` only used when user explicitly approves or passes the flag
- The recorded route (`unticked_criteria:` + `# Results`) is checked BEFORE `--force` is ever offered
- An `evidenced-and-tickable` entry, or a per-row goal-page/task-frontmatter mismatch, REFUSES closure in both modes
</success_criteria>
