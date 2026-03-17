---
description: Show status of current task with progress and next step
argument-hint: (detects from conversation)
---

<objective>
Show task status using task-manager-agent. Quick "where was I?" recovery tool that detects task from conversation.
</objective>

<process>
1. Invoke task-manager-agent with:
   - ACTION: "status"
   - ARGS: (none - agent detects from conversation)
2. Agent handles:
   - Detect task from conversation context
   - Read task file
   - Parse checkboxes
   - Calculate progress
   - Extract next step
   - Generate status report
</process>

<success_criteria>
- Agent invoked with correct action
- Task detected from conversation
- Progress calculated correctly
- Next step identified
- Clear status report generated
</success_criteria>
