---
description: Defer goal to specific date
argument-hint: "<goal-name> <YYYY-MM-DD|+Nd|weekday> [--tool]"
allowed-tools: Bash(vault-cli goal defer:*)
---

<objective>
Defer goal to specific date using vault-cli. Supports absolute (YYYY-MM-DD), relative (+Nd), and weekday names.
</objective>

<process>
1. Parse arguments:
   - If contains `--tool` → MODE=tool, remove flag from args
   - Otherwise → MODE=interactive
   - Split remaining args: goal name = all but last, date = last word

2. **MODE=interactive (default):**

   a. Run vault-cli:
      ```bash
      vault-cli goal defer "{goal_name}" "{date}"
      ```

   b. Show report:
      ```
      ✅ Goal deferred: [[{goal_name}]]
      - Defer date: {date}
      - Hidden from "In Progress" until {date}
      - Visible in "Deferred Goals" dashboard section
      ```
      - If warnings in output, show them

3. **MODE=tool (--tool flag):**

   a. Run vault-cli:
      ```bash
      vault-cli goal defer "{goal_name}" "{date}" --output json
      ```

   b. Return JSON result from vault-cli
      STOP. Never ask questions.

4. vault-cli handles internally:
   - Date parsing and validation (absolute, relative, weekday)
   - Update defer_date in frontmatter
   - Past-date rejection
</process>

<success_criteria>
- vault-cli goal defer invoked (NOT Edit tool for frontmatter)
- Date parsed and validated (by vault-cli)
- Goal defer_date updated (by vault-cli)
- MODE=tool: Returns JSON only
- MODE=interactive: Full report
</success_criteria>
