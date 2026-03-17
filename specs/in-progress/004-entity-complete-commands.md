---
status: approved
approved: "2026-03-17T10:33:48Z"
branch: dark-factory/entity-complete-commands
---

## Summary

- Add `complete` subcommand for goal and objective entities
- Sets status to completed and records the completion date
- Goal complete checks whether all linked tasks are resolved (completed or aborted) before allowing completion
- Follows the same CLI pattern as `task complete`

## Problem

Completing a goal requires manually editing frontmatter to set status and date. The `/complete-goal` slash command must use raw file editing instead of vault-cli. There is no validation that linked tasks are actually done before marking a goal complete, leading to premature closures.

## Goal

After this work:
- `vault-cli goal complete "My Goal"` marks the goal as completed with today's date
- `vault-cli objective complete "My Objective"` marks the objective as completed with today's date
- Goal completion validates that all linked tasks are resolved
- Agents and slash commands use vault-cli instead of raw file editing for completion workflows

## Non-goals

- No `complete` for theme or vision — these are ongoing directions, not completable items
- No cascade (completing a goal does not auto-complete its tasks)
- No undo/reopen command
- No body content changes (verdict/summary writing remains manual via Edit tool)

## Desired Behavior

1. `vault-cli goal complete "My Goal"` sets `status: completed` and `completed: 2026-03-17` (today's date) in the goal's frontmatter.

2. Before completing, the command checks all tasks linked to this goal. If any task has status `todo` or `in_progress`, it returns an error listing the unresolved tasks: `"cannot complete goal: 2 tasks still open: Task A (in_progress), Task B (todo)"`.

3. Tasks with status `completed`, `aborted`, or `hold` do not block goal completion.

4. `--force` flag bypasses the task check, allowing completion even with open tasks.

5. `vault-cli objective complete "My Objective"` sets `status: completed` and `completed: <today>`. No linked-entity validation (objectives link to goals, but that check is not needed).

6. Completing an already-completed entity returns an error: `"goal 'X' is already completed"`.

## Assumptions

- Spec "Generic Frontmatter Operations" is completed first — goal and objective need `set` infrastructure
- Tasks link to goals via the `goals` frontmatter field — this is the source of truth for the linkage check
- Goal and objective domain structs include a `completed` date field

## Constraints

- Existing `task complete` behavior must not change — all current tests must pass
- Task linkage check reads task frontmatter to find tasks with the goal in their `goals` list
- A goal with zero linked tasks is completable (no tasks = nothing to block)
- Completion date is always today's date (injected, not hardcoded)
- JSON output includes entity name, new status, and completion date
- Multi-vault dispatch works the same as other mutation commands

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Open tasks linked to goal | Error listing unresolved tasks, exit 1 | User completes/aborts tasks first, or uses --force |
| Goal already completed | Error: "already completed", exit 1 | No action needed |
| Goal not found | Error: "not found", exit 1 | User checks name |
| `--force` on non-existent goal | Error: "not found", exit 1 | --force does not bypass entity lookup |

## Security / Abuse

- Entity names are resolved only within configured entity directories, preventing path traversal
- The `--force` flag only bypasses task validation, not entity existence checks

## Acceptance Criteria

- [ ] `vault-cli goal complete "G"` sets status=completed and completed=today
- [ ] Goal with open tasks fails with descriptive error listing task names
- [ ] `--force` bypasses task check
- [ ] Tasks with status completed/aborted/hold do not block completion
- [ ] `vault-cli objective complete "O"` sets status=completed and completed=today
- [ ] Goal with zero linked tasks completes successfully
- [ ] Already-completed entity returns error
- [ ] JSON output includes entity name, new status, and completion date
- [ ] All existing tests pass unchanged
- [ ] `make precommit` passes

## Verification

```
make precommit
```

## Do-Nothing Option

Users manually edit frontmatter to set status and date. No validation of linked task status — goals get marked complete while tasks are still open, causing stale state. The `/complete-goal` slash command implements its own validation logic instead of delegating to vault-cli.
