---
description: Audit task file against Task Writing Guide for quality, completeness, goal dependency, and DoD validation
argument-hint: <task-file-path>
---

<objective>
Invoke the task-auditor agent to audit the task at $ARGUMENTS for compliance with Task Writing Guide best practices.
</objective>

<process>
1. Parse task path from $ARGUMENTS
   - If no path prefix, prepend `24 Tasks/`
   - If no `.md` extension, append it
2. Invoke task-auditor agent with the task path
3. Agent reads Task Writing Guide and Task Template first
4. Agent evaluates:
   - Structure and YAML frontmatter
   - Goal linkage and dependency validation
   - Impact and success criteria quality
   - Subtasks organization and scope
   - **Definition of Done presence/quality for complex tasks**
5. Review detailed findings with severity levels, scores, and recommendations
</process>

<success_criteria>
- Agent invoked successfully
- Task path passed correctly
- Audit includes all 12 evaluation areas from Task Writing Guide
- Report shows score, critical issues, recommendations, and strengths
- Complex tasks flagged if missing Definition of Done
</success_criteria>
