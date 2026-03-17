---
description: Quick validation of task status, goal linkage, DoD existence, and checkbox tracking
argument-hint: <task-file-path>
---

<objective>
Invoke task-manager-agent for fast sanity checks: status valid, parent goal exists, DoD present (optional for recurring), checkboxes tracked.
</objective>

<process>
1. Parse task path from $ARGUMENTS
   - If no path prefix, prepend `24 Tasks/`
   - If no `.md` extension, append it
2. Invoke task-manager-agent with:
   - ACTION: "verify"
   - ARGS: task path
3. Agent checks:
   - Status valid (in_progress|todo|backlog|completed|hold|aborted)
   - Parent goal exists (goals field, links resolve)
   - DoD section exists (optional for recurring tasks)
   - Checkboxes present and tracked
   - Status consistency (completed → 100% checkboxes)
4. Return pass/fail report with specific issues
</process>

<success_criteria>
- Agent invoked with correct action
- Quick validation checks performed
- Pass/fail output with specific issues listed
- No detailed quality analysis (use /audit-task for that)
</success_criteria>
