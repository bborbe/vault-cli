---
tags:
  - dark-factory
  - spec
status: idea
---

## Summary

- Drop the legacy `last_completed` frontmatter key from Task writes; keep `last_completed_date` only.
- Reads continue to accept both keys for one more release after this lands (transitional).
- Closes the dual-write window opened by spec 010 (`unify-date-fields-to-dateordatetime`).

## Problem

Spec 010 introduced a one-release dual-write window: Task writes both `last_completed` (legacy) and `last_completed_date` (canonical) to ease external-consumer migration. The dual-write is intentional debt — every recurring-task save now emits two YAML keys for the same value, doubling the field on disk and adding a write path that has to be deleted later. This spec captures the deletion work.

## Goal

After one release cycle (TBD: confirm one is enough), drop the dual-write:

- Writes emit only `last_completed_date`.
- Reads accept both keys (legacy fallback) for one more release, then drop the fallback in a successor spec.

## Non-goals

- Removing the read-fallback in this spec — that's a separate later step.
- Renaming `last_completed_date` further.
- Migrating existing vault files in place.

## Desired Behavior

1. `Task.SetLastCompletedDate()` and any other write path emits only the `last_completed_date` key. The legacy `last_completed` key is no longer set on writes.
2. `Task.LastCompletedDate()` (or equivalent getter) still reads the legacy `last_completed` key as a fallback when the canonical key is absent — for one more release.
3. Existing vault files containing only the legacy key continue to read correctly.
4. Tests cover: write only emits canonical key; read accepts canonical-only; read falls back to legacy when canonical absent; round-trip preserves form.

## Constraints

- See spec 010 for the round-trip contract on `*libtime.DateOrDateTime`.
- No external producer migration is in scope — this spec only changes vault-cli's write behavior.
- All existing tests must pass.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Vault file has only `last_completed: 2025-01-15` (legacy key) | Read returns the value; on next write, only `last_completed_date` is emitted (legacy key disappears from that file) | Documented behavior |
| Vault file has only `last_completed_date: 2025-01-15` | Reads and writes use canonical key only | None needed |
| Vault file has both keys with different values | Canonical key wins on read; legacy key dropped on next write | Documented behavior |

## Acceptance Criteria

- [ ] `grep -n '"last_completed"' pkg/domain/task_frontmatter.go` finds only read-fallback paths, no `Set` calls.
- [ ] Tests assert: writing a Task with `LastCompletedDate` set produces YAML with `last_completed_date` only.
- [ ] Tests assert: reading a Task with only legacy `last_completed` returns the value (read fallback intact).
- [ ] All existing tests pass.
- [ ] CHANGELOG entry under `## Unreleased`.
- [ ] Successor follow-up captured (drop the read-fallback after one more release).

## Verification

```
make precommit
```

## Open Questions

- **Release-cycle count**: spec 010 assumed ONE release of dual-writing. Confirm one is enough by reviewing producer/consumer adoption before promoting this spec from `idea` → `draft`.
- **Trigger for promotion**: tag-driven (after release vX.Y.Z ships) or time-driven (after N days)?

## Do-Nothing Option

Tolerable but accumulates write-side noise. Every recurring-task save emits a redundant key. Cost is low per save but compounds across the vault. Not acceptable as a permanent state.

## Related

- Parent spec: `specs/completed/010-unify-date-fields-to-dateordatetime.md` (introduced the dual-write).
- Successor (planned, not yet captured): drop the read-fallback for legacy `last_completed` after this spec lands.
