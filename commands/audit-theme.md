---
description: Audit theme file against Theme Writing Guide for quality and completeness
argument-hint: <theme-file-path>
---

<objective>
Invoke the theme-auditor agent to audit the theme at $ARGUMENTS for compliance with Theme Writing Guide best practices.
</objective>

<process>
1. Parse theme path from $ARGUMENTS
   - If no path prefix, prepend `21 Themes/`
   - If no `.md` extension, append it
2. Invoke theme-auditor agent with the theme path
3. Agent reads Theme Writing Guide and Theme Template first
4. Agent evaluates structure, vision linkage, impact, sub-goals, strategic direction
5. Review detailed findings with severity levels, scores, and recommendations
</process>

<success_criteria>
- Agent invoked successfully
- Theme path passed correctly
- Audit includes all evaluation areas from Theme Writing Guide
- Report shows score, critical issues, recommendations, and strengths
</success_criteria>
