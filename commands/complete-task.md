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
