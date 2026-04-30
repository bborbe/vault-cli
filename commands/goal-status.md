---
description: Show status of current goal with progress and next task
argument-hint: (detects from conversation)
---

<objective>
Show goal status using goal-manager-agent. Quick "where am I?" recovery tool that detects goal from conversation.
</objective>

<process>
1. Invoke goal-manager-agent with:
   - ACTION: "status"
   - ARGS: (none - agent detects from conversation)
2. Agent handles:
   - Detect goal from conversation context
   - Read goal file
   - Parse Success Criteria checkboxes
   - Parse linked subtasks and their statuses
   - Calculate progress
   - Extract next step (next pending task or pending success criterion)
   - Generate status report
</process>

<success_criteria>
- Agent invoked with correct action
- Goal detected from conversation
- Progress calculated correctly (criteria + subtasks)
- Next step identified
- Clear status report generated
</success_criteria>
