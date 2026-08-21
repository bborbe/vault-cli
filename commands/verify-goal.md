---
description: Quick validation of goal status, subtask existence, and status consistency
argument-hint: <goal-file-path>
allowed-tools: [Task]
---

<objective>
Invoke goal-manager-agent for fast sanity checks: status valid, subtasks exist, status consistency.
</objective>

<process>
1. Parse goal path from $ARGUMENTS
   - If no path prefix, prepend `23 Goals/`
   - If no `.md` extension, append it
2. Invoke goal-manager-agent with:
   - ACTION: "verify"
   - ARGS: goal path
3. Agent checks:
   - Status valid (in_progress|todo|backlog|completed|hold|aborted)
   - All subtasks exist (links resolve)
   - Status consistency (goal in_progress → subtasks should be too)
   - Tasks/PRDs linked
   - Goal necessity (each linked task advances ≥ 1 of the goal's success criteria — goal-necessity check)
4. Return pass/fail report with specific issues
</process>

<success_criteria>
- Agent invoked with correct action
- Quick validation checks performed
- Pass/fail output with specific issues listed
- Goal-necessity check reported (tasks advancing no success criterion flagged)
- No detailed quality analysis (use /audit-goal for that)
</success_criteria>
