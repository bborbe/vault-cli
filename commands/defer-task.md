---
description: Defer task to specific date
argument-hint: "<task-name> <YYYY-MM-DD|+Nd|weekday> [--tool]"
---

<objective>
Defer task to specific date using vault-cli. Supports absolute (YYYY-MM-DD), relative (+Nd), and weekday names.
</objective>

<process>
1. Parse arguments:
   - If contains `--tool` → MODE=tool, remove flag from args
   - Otherwise → MODE=interactive
   - Split remaining args: task name = all but last, date = last word

2. **MODE=interactive (default):**

   a. Run vault-cli:
      ```bash
      vault-cli task defer "{task_name}" "{date}"
      ```

   b. Show report:
      ```
      ✅ Task deferred: [[{task_name}]]
      - Defer date: {date}
      - Removed from today's plan
      - Added to {date} daily note
      ```
      - If warnings in output, show them

3. **MODE=tool (--tool flag):**

   a. Run vault-cli:
      ```bash
      vault-cli task defer "{task_name}" "{date}" --output json
      ```

   b. Return JSON result from vault-cli
      STOP. Never ask questions.

4. vault-cli handles internally:
   - Date parsing and validation (absolute, relative, weekday)
   - Update defer_date in frontmatter
   - Clear planned_date if before defer_date
   - Remove from today's daily note
   - Add to target daily note
</process>

<success_criteria>
- vault-cli task defer invoked (NOT Edit tool for frontmatter)
- Date parsed and validated (by vault-cli)
- Task defer_date updated (by vault-cli)
- Daily notes updated (by vault-cli)
- MODE=tool: Returns JSON only
- MODE=interactive: Full report
</success_criteria>
