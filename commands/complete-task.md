---
description: Mark task as complete (normal or recurring)
argument-hint: "<task-name-or-path> [--non-interactive] [--force]"
allowed-tools:
  - Bash(vault-cli:*)
  - Read
---

<objective>
Mark task as complete using vault-cli. Handles normal and recurring tasks appropriately.
</objective>

<process>
1. Parse arguments:
   - If contains `--non-interactive` (or deprecated `--tool`) → MODE=non_interactive, remove flag from args
   - Otherwise → MODE=interactive
   - If contains `--force` → FORCE=true, remove flag from args (interactive mode only — `--non-interactive` overrides as defined in step 3)
   - Extract task name from remaining args

2. **MODE=interactive (default):**

   a. Read task file to check completion state:
      - Find task: `vault-cli task show "{task_name}" --output json`
      - Parse checkboxes (count `[x]`, `[/]`, `[ ]`)

   b. If incomplete items (pending > 0 or in-progress > 0) AND FORCE=false:
      - Print completion status (`X/Y checkboxes, N%`)
      - List specific incomplete items
      - Print: `❌ Task has incomplete items. Finish them first, or re-run with --force to complete anyway.`
      - STOP. Do NOT call `vault-cli task complete`. No interactive prompt.

   c. Run vault-cli. Pass `--force` through whenever FORCE=true — the CLI enforces
      the same incomplete-checkbox guard, so omitting it makes the flag a no-op and
      the run fails with `incomplete subtasks: N pending`:
      ```bash
      vault-cli task complete "{task_name}"            # no incomplete items
      vault-cli task complete "{task_name}" --force    # incomplete items + FORCE=true
      ```

      **`--force` is for items that were never this task's work** — follow-ups filed
      for a separate task, or notes that are checkboxes only by formatting. It is not
      a way to close unfinished work. If the unchecked items are genuinely part of the
      task, finish them.

   d. Show report:
      ```
      ✅ Task completed: [[{task_name}]]
      ```
      - If warnings in output, show them

   e. **Emit closer panel** — append below the report, verbatim, no rewording:
      ```
      ⚪ DONE
      👤 You: approve: /vault-cli:session-close
      ⏰ Next: your reply
      ```

      Rationale for this closer (one-task-per-session contract, why never `/vault-cli:next-task` here, why the "no end-of-day suggestions" global rule does NOT override): see [`sync-progress.md` Phase 6](sync-progress.md) — same contract applies. Single source of truth lives there to prevent drift.

      MODE=non_interactive MUST NOT emit this panel (see step 3 — JSON only).

3. **MODE=non_interactive (--non-interactive flag):**

   a. Read task file to check completion state
   b. If incomplete items:
      ```bash
      vault-cli task set "{task_name}" phase human_review
      ```
      Return: `{"success": false, "reason": "incomplete items"}`
      STOP.

   c. If complete:
      ```bash
      vault-cli task complete "{task_name}" --output json
      ```
      Return: `{"success": true, "path": "..."}`
      STOP.

   d. Never ask questions, never prompt.

   e. **Never emit the `⚪ DONE` closer panel** — MODE=non_interactive output is JSON only. The closer panel from step 2e is interactive-mode only.

4. Task types (handled by vault-cli internally):
   - Normal tasks: status→completed, goals updated, daily note checked
   - Recurring: Reset checkboxes, update defer_date, keep status in_progress
</process>

<success_criteria>
- vault-cli task complete invoked (NOT Edit tool for frontmatter)
- **MODE=non_interactive**: Returns JSON only, sets phase=human_review if incomplete
- **MODE=interactive**: Shows completion %, aborts with `--force` hint if incomplete (no prompts), reports result on success
- Goal files updated (by vault-cli)
- Daily note updated (by vault-cli)
</success_criteria>

## Passive metrics

Completion writes `metrics_completed_at` and, when the task was worked on,
`metrics_interaction_count`. Recurring completion archives the finished cycle into
`metrics_cycles` and resets the accumulator. These metrics fields are written passively
by vault-cli and must not be hand-edited.
