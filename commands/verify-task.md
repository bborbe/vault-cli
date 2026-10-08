---
description: Quick validation of task status, goal linkage, DoD existence, and checkbox tracking
argument-hint: <task-file-path>
---

<objective>
Invoke task-manager-agent for fast sanity checks: status valid, parent goal exists, DoD present (optional for recurring), checkboxes tracked.
</objective>

<process>
1. Parse task path from $ARGUMENTS
   - If no path prefix, prepend `<tasks_dir>` (resolved from vault-cli config — never a hardcoded folder name)
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
   - Goal necessity (for each linked goal, the task names a specific success criterion or a specific Definition of Done item it serves — or none — goal-necessity check). ⚠️ The goal's one-line summary sentence does NOT count: completion is the closure contract (SC + DoD), and nothing closes against a sentence.
4. Return pass/fail report with specific issues
</process>

<success_criteria>
- Agent invoked with correct action
- Quick validation checks performed
- Pass/fail output with specific issues listed
- Goal-necessity check reported (each link's serving item named as `SC<n>` or `DoD<n>`; correlation-only links flagged — a goal-sentence-only link is correlation-only)
- No detailed quality analysis (use /audit-task for that)
</success_criteria>
