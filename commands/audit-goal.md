---
description: Audit goal file against Goal Writing Guide for quality and completeness
argument-hint: <goal-file-path>
---

<objective>
Invoke the goal-auditor agent to audit the goal at $ARGUMENTS for compliance with Goal Writing Guide best practices.
</objective>

<process>
1. Parse goal path from $ARGUMENTS
   - If no path prefix, prepend `23 Goals/`
   - If no `.md` extension, append it
2. Invoke goal-auditor agent with the goal path
3. Agent reads Goal Writing Guide and Goal Template first
4. Agent evaluates structure, SMART criteria, theme linkage, impact, success criteria, tasks
5. Review detailed findings with severity levels, scores, and recommendations
</process>

<success_criteria>
- Agent invoked successfully
- Goal path passed correctly
- Audit includes all evaluation areas from Goal Writing Guide
- Report shows score, critical issues, recommendations, and strengths
</success_criteria>
