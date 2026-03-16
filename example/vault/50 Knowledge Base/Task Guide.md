---
page_type: guide
---

---

## Task Frontmatter

| Field | Type | Description |
|-------|------|-------------|
| status | string | todo, in_progress, backlog, completed, hold, aborted |
| priority | int | 1 (highest) to 5 (lowest) |
| assignee | string | Current owner |
| defer_date | date | Earliest date to act on this task |
| planned_date | date | Target completion date |
| recurring | string | Recurrence interval (daily, weekly, monthly) |
| goals | list | Linked goal names |
| tags | list | Classification tags |
| phase | string | Current work phase |

## Lifecycle

1. Create task file in `24 Tasks/`
2. Set `status: todo`
3. `vault-cli task work-on` to start (sets in_progress + assignee)
4. `vault-cli task defer +Nd` to postpone
5. `vault-cli task complete` to finish
   - Recurring tasks: checkboxes reset, defer_date bumped, stays in_progress
   - Non-recurring tasks: status set to completed
