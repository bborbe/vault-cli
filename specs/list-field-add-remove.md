---
status: draft
---

## Summary

- Add `add` and `remove` subcommands for appending/removing items from list-type frontmatter fields (goals, tags)
- These commands work across all entity types: task, goal, theme, objective, vision
- Field type (scalar vs list) is detected automatically — no user configuration needed
- `add`/`remove` reject scalar fields with a clear error
- Duplicate adds and missing removes return descriptive errors — no silent no-ops

## Problem

vault-cli treats all frontmatter mutations as scalar replacements. Tasks and goals have list fields (`goals`, `tags`) but users must replace the entire list with `set` to modify a single item. This is error-prone (easy to accidentally drop existing values) and impractical for agents that need to programmatically manage list memberships.

## Goal

After this work:
- Users can add/remove individual items from list fields without knowing the current list contents
- The system distinguishes scalar vs list fields automatically based on the domain structs
- All five entity types (task, goal, theme, objective, vision) support add/remove
- Agents can safely modify list fields without read-modify-write races

## Non-goals

- No new field types beyond what already exists in domain structs
- No changes to `set` behavior — it still replaces the whole value
- No bulk operations (add/remove multiple values in one call)
- No interactive/prompt-based UX

## Desired Behavior

1. `vault-cli task add "My Task" goals "My Goal"` appends "My Goal" to the task's goals list. Same pattern for all entity types and all list fields (tags, etc.).

2. `vault-cli task remove "My Task" goals "My Goal"` removes "My Goal" from the task's goals list. Same pattern for all entity types.

3. Running `add` on a scalar field (e.g., `status`, `assignee`) returns an error: `"field 'status' is not a list field"`. No file is modified.

4. Running `remove` on a scalar field returns the same category of error. No file is modified.

5. Running `add` with a value that already exists in the list returns an error: `"value 'X' already exists in field 'goals'"`. No file is modified.

6. Running `remove` with a value not present in the list returns an error: `"value 'X' not found in field 'goals'"`. No file is modified.

7. Field type detection is automatic — the system knows which fields are lists and which are scalars without user configuration.

## Assumptions

- Spec "Generic Frontmatter Operations" is completed first — provides field access infrastructure for all entity types
- All list fields are string lists — no nested or complex list types
- Entity names are unique within their type

## Constraints

- Existing `get`, `set`, `clear` commands must not change behavior — all current tests must pass
- `set` on a list field continues to replace the whole list, preserving backward compatibility
- JSON output follows the same format as existing mutation commands
- Multi-vault dispatch works the same as other mutation commands

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `add` on scalar field | Error: "not a list field", exit 1, no file write | User uses `set` instead |
| `remove` on scalar field | Error: "not a list field", exit 1, no file write | User uses `set` or `clear` |
| `add` duplicate value | Error: "already exists", exit 1, no file write | User checks current value with `get` |
| `remove` missing value | Error: "not found", exit 1, no file write | User checks current value with `get` |
| `add`/`remove` on unknown field | Error: "unknown field", exit 1 | User checks valid fields with `show` |
| Entity not found | Existing "not found" error propagates | Same as current get/set/clear |

## Security / Abuse

- Field values come from CLI arguments — YAML injection via crafted values (e.g., values containing newlines or YAML special characters) must be handled by the YAML serializer, not raw string concatenation
- Entity names are resolved only within configured entity directories, preventing path traversal

## Acceptance Criteria

- [ ] `vault-cli task add "T" goals "G"` appends G to goals list
- [ ] `vault-cli task remove "T" goals "G"` removes G from goals list
- [ ] `vault-cli task add "T" status "todo"` fails with "not a list field" error
- [ ] `vault-cli task add "T" goals "G"` when G already present fails with duplicate error
- [ ] `vault-cli task remove "T" goals "G"` when G absent fails with not-found error
- [ ] `vault-cli goal add "G" tags "tag1"` works (non-task entity)
- [ ] `vault-cli objective add "O" tags "tag1"` works (non-task entity)
- [ ] `vault-cli theme remove "T" tags "tag1"` works (remove on non-task entity)
- [ ] `vault-cli vision add "V" tags "tag1"` works (vision entity)
- [ ] All existing tests pass unchanged
- [ ] JSON output works for add and remove
- [ ] `make precommit` passes

## Verification

```
make precommit
```

## Do-Nothing Option

Users continue replacing entire lists via `set`. They must first `get` the current list, manually construct the new list, then `set` the whole thing. Error-prone, especially for agents — a read-modify-write race can silently drop concurrent changes.
