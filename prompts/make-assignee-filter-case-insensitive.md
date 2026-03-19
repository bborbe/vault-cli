---
status: inbox
---

<summary>
- Assignee filter in task/entity list commands becomes case-insensitive
- "localclaw", "LocalClaw", "LOCALCLAW" all match the same assignee
- Uses strings.EqualFold for comparison
- Single code change in shouldIncludeTask function
</summary>

<objective>
Make the --assignee filter case-insensitive so agents and users don't need to know the exact casing of assignee names.
</objective>

<context>
The `shouldIncludeTask` function in `pkg/ops/list.go` (line 170) compares assignee with strict equality:

```go
if assigneeFilter != "" && task.Assignee != assigneeFilter {
```

This requires exact casing (e.g., `--assignee LocalClaw`). If an agent uses `--assignee localclaw`, the filter silently returns no results.

Both CLI list commands (task list at cli.go:343 and generic entity list at cli.go:505) flow through the same `shouldIncludeTask` function.
</context>

<requirements>
1. In `pkg/ops/list.go`, change `shouldIncludeTask` (line 170) from:
   ```go
   if assigneeFilter != "" && task.Assignee != assigneeFilter {
   ```
   to:
   ```go
   if assigneeFilter != "" && !strings.EqualFold(task.Assignee, assigneeFilter) {
   ```
   Add `"strings"` to imports if not already present.

2. Add test case in `pkg/ops/list_test.go` verifying case-insensitive assignee matching:
   - Task with `Assignee: "LocalClaw"` should match filter `"localclaw"`
   - Task with `Assignee: "localclaw"` should match filter `"LocalClaw"`
   - Task with `Assignee: "alice"` should NOT match filter `"bob"`
</requirements>

<constraints>
- Only change the comparison in `shouldIncludeTask`, do not normalize assignee values on read/write
- Stored assignee values keep their original casing
- Do NOT commit
</constraints>

<verification>
make precommit
</verification>
