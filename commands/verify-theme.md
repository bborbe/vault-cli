---
description: Quick validation of theme structure and required sections
argument-hint: <theme-file-path>
---

<objective>
Fast sanity checks: status valid, required sections present, goals linked.
</objective>

<process>
1. Parse theme path from $ARGUMENTS
   - If no path prefix, prepend `21 Themes/`
   - If no `.md` extension, append it
2. Read the theme file
3. Check:
   - Status valid (in_progress|todo|backlog|hold|completed)
   - [[Theme]] tag present (for backlinking)
   - Summary paragraph exists (first paragraph after tags separator)
   - # Impact section exists
   - Optional: # Sub-Goals or task sections (themes can have recurring tasks instead of goals)
4. Return pass/fail report with specific issues
</process>

<success_criteria>
- Quick validation checks performed
- Pass/fail output with specific issues listed
- No detailed quality analysis (use /audit-theme for that)
</success_criteria>
