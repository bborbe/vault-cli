---
description: Mark goal as complete (checks subtasks and success criteria)
argument-hint: "<goal-name-or-path> [--tool] [--force]"
---

<objective>
Mark goal as complete using vault-cli. Verifies success criteria and linked subtasks before completion.
</objective>

<process>
1. Parse arguments:
   - If contains `--tool` → MODE=tool, remove flag from args
   - If contains `--force` → FORCE=true, remove flag from args
   - Otherwise → MODE=interactive
   - Extract goal name from remaining args

2. **MODE=interactive (default):**

   a. Read goal file to check completion state:
      - Find goal: `vault-cli goal show "{goal_name}" --output json`
      - Parse Success Criteria checkboxes (count `[x]`, `[/]`, `[ ]`)
      - Parse linked subtasks from `# Tasks` section
      - For each subtask: read its status

   b. If incomplete success criteria OR open subtasks (status != completed):
      - Show summary:
        - Success Criteria: X/Y complete (N%)
        - Subtasks: X/Y completed
      - List specific incomplete items (success criteria + open task names)
      - Use AskUserQuestion: 1. Complete anyway (--force) 2. Finish first 3. Show details
      - If "Finish first" → abort

   c. Run vault-cli:
      ```bash
      vault-cli goal complete "{goal_name}"{--force if FORCE or user picked option 1}
      ```

   d. Show report:
      ```
      ✅ Goal completed: [[{goal_name}]]
      ```
      - If warnings in output, show them

3. **MODE=tool (--tool flag):**

   a. Read goal file to check completion state
   b. If incomplete success criteria or open subtasks (and FORCE not set):
      Return: `{"success": false, "reason": "incomplete items"}`
      STOP.

   c. If complete (or FORCE):
      ```bash
      vault-cli goal complete "{goal_name}" --output json{--force if FORCE}
      ```
      Return: `{"success": true, "path": "..."}`
      STOP.

   d. Never ask questions, never use AskUserQuestion
</process>

<success_criteria>
- vault-cli goal complete invoked (NOT Edit tool for frontmatter)
- **MODE=tool**: Returns JSON only, never forces unless `--force` explicitly passed
- **MODE=interactive**: Shows progress, asks if incomplete, reports result
- Parent objective updated (by vault-cli)
- `--force` only used when user explicitly approves or passes the flag
</success_criteria>
