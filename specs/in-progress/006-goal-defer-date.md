---
status: prompted
approved: "2026-03-25T09:09:56Z"
prompted: "2026-03-25T09:13:12Z"
branch: dark-factory/goal-defer-date
---

## Summary

- Add `defer_date` field to goals so `vault-cli goal set "G" defer_date 2026-04-01` works
- Add `vault-cli goal defer` subcommand with same date parsing as `task defer`
- Goals dashboard already queries `defer_date` -- this makes vault-cli match what Obsidian expects

## Problem

Goals in the Obsidian vault use `defer_date` in dataview dashboard queries to hide goals until a future date. The Goals dashboard filters with `AND (!defer_date OR defer_date <= date(today))` and shows deferred goals with `WHERE defer_date AND defer_date > date(today)`. However, vault-cli rejects `defer_date` on goals because the Goal domain struct lacks the field. Users must manually edit frontmatter, bypassing validation.

## Goal

After this work, goals support deferral identically to tasks. `vault-cli goal set "G" defer_date 2026-04-01` and `vault-cli goal defer "G" +7d` both work. The existing Goals dashboard renders correctly without changes.

## Non-goals

- No changes to task defer behavior
- No daily-note integration for goal defer (goals are not tracked in daily notes like tasks)
- No `planned_date` clearing logic for goals (goals don't have `planned_date`)
- No new dashboard queries or views

## Desired Behavior

1. `vault-cli goal set "My Goal" defer_date 2026-04-01` sets `defer_date` in the goal's frontmatter. Existing `set` infrastructure handles this automatically once the field exists on the domain struct.

2. `vault-cli goal get "My Goal" defer_date` returns the current defer date value. Empty output if unset.

3. `vault-cli goal clear "My Goal" defer_date` removes `defer_date` from frontmatter.

4. `vault-cli goal defer "My Goal" +7d` sets `defer_date` to 7 days from now. Accepts the same date formats as `task defer`: `+Nd`, weekday names (monday, tuesday...), ISO dates (YYYY-MM-DD), and RFC3339 datetimes.

5. `vault-cli goal defer "My Goal" 2026-01-01` rejects past dates with an error message, same as task defer.

6. Goal defer does NOT update daily notes (unlike task defer). Goals are tracked at a higher level and don't appear in daily note checklists.

## Constraints

- Existing task defer behavior must not change
- All existing tests must pass
- Date parsing logic should be reused, not duplicated
- The `defer_date` field uses the same `DateOrDateTime` type as tasks
- `set`/`get`/`clear` for `defer_date` on goals works automatically via the generic frontmatter infrastructure (spec 002)
- JSON output supported for `goal defer`

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `goal defer` with invalid date format | Error: "invalid date format: X (use +Nd, weekday, YYYY-MM-DD, or RFC3339)" | User corrects format |
| `goal defer` with past date | Error: "cannot defer to past date: YYYY-MM-DD" | User picks future date |
| Goal not found | Error: "goal 'X' not found", exit 1 | User checks goal name |
| `goal defer` on completed goal | Allowed -- user's responsibility | Lint can flag if needed |

## Acceptance Criteria

- [ ] `vault-cli goal set "G" defer_date 2026-04-01` succeeds and writes frontmatter
- [ ] `vault-cli goal get "G" defer_date` returns the stored date
- [ ] `vault-cli goal clear "G" defer_date` removes the field
- [ ] `vault-cli goal defer "G" +7d` sets defer_date 7 days out
- [ ] `vault-cli goal defer "G" monday` sets defer_date to next Monday
- [ ] `vault-cli goal defer "G" 2026-12-31` sets defer_date to specified ISO date
- [ ] `vault-cli goal defer "G" 2025-01-01` fails with past-date error
- [ ] `vault-cli goal defer "G" invalid` fails with format error
- [ ] JSON output works: `vault-cli goal defer "G" +7d --output json`
- [ ] All existing task defer tests pass unchanged
- [ ] `make precommit` passes

## Verification

```
make precommit
```

## Security / Abuse

Not applicable — CLI-only tool operating on local files, no network input.

## Do-Nothing Option

Users must manually edit goal frontmatter to add/change `defer_date`. This bypasses validation (no past-date check, no format validation) and requires agents to use raw file editing instead of vault-cli. The dashboard already expects the field, so the data model mismatch is a usability gap, not a blocker.
