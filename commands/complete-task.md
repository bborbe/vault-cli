---
description: Mark task as complete (normal or recurring)
argument-hint: "<task-name-or-path> [--tool]"
---

<objective>
Mark task as complete using vault-cli. Handles normal and recurring tasks appropriately.
</objective>

<process>
1. Parse arguments:
   - If contains `--tool` → MODE=tool, remove flag from args
   - Otherwise → MODE=interactive
   - Extract task name from remaining args

2. **MODE=interactive (default):**

   a. Read task file to check completion state:
      - Find task: `vault-cli task show "{task_name}" --output json`
      - Parse checkboxes (count `[x]`, `[/]`, `[ ]`)

   b. If incomplete items (pending > 0 or in-progress > 0):
      - Show completion status (X/Y checkboxes, N%)
      - List specific incomplete items
      - Use AskUserQuestion: "Complete anyway? / Finish first? / Show details?"
      - If "Finish first" → abort

   c. Run vault-cli:
      ```bash
      vault-cli task complete "{task_name}"
      ```

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

      Why this closer is the only correct one here:

      - **One task per session.** Completing a task = THIS session is done. Queued items on today's daily note are NOT "queued in this session" — they are picked up by the orchestrator in fresh Claude sessions, never by appending more tasks to the current one.
      - **Never recommend `/vault-cli:next-task` here.** That command exists for the orchestrator (or the user opening a new session); it is not a follow-up to `/vault-cli:complete-task`.
      - **Never recommend a specific next task by name.** Same reason — the next session's anchor selection belongs to the orchestrator, not to this command.
      - **The "no end-of-day suggestions" global rule does NOT override this.** Session-close ≠ day-close. The rule forbids unsolicited *stop for the day* nudges; closing one task's session is the routine step between two task sessions, not a wind-down.

      MODE=tool MUST NOT emit this panel (see step 3 — JSON only).

3. **MODE=tool (--tool flag):**

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

   d. Never ask questions, never use AskUserQuestion

   e. **Never emit the `⚪ DONE` closer panel** — MODE=tool output is JSON only. The closer panel from step 2e is interactive-mode only.

4. Task types (handled by vault-cli internally):
   - Normal tasks: status→completed, goals updated, daily note checked
   - Recurring: Reset checkboxes, update defer_date, keep status in_progress
</process>

<success_criteria>
- vault-cli task complete invoked (NOT Edit tool for frontmatter)
- **MODE=tool**: Returns JSON only, sets phase=human_review if incomplete
- **MODE=interactive**: Shows completion %, asks if incomplete, reports result
- Goal files updated (by vault-cli)
- Daily note updated (by vault-cli)
</success_criteria>
