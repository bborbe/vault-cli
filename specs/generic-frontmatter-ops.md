---
status: draft
---

## Summary

- Generalize `show`, `set`, `get`, `clear` commands to work with all entity types (task, goal, theme, objective, vision)
- Adding a new frontmatter field automatically makes it available via CLI — no code changes needed beyond the field definition
- Add missing domain structs and storage for objective and vision entities
- All five entity types gain identical frontmatter CRUD subcommands

## Problem

Only tasks support `show`, `set`, `get`, `clear`. Goals, themes, objectives, and visions lack these basic operations. The current task implementation uses hardcoded switch statements per field name, making it impractical to duplicate for each entity type. Agents and slash commands that manage goals or objectives must fall back to raw file editing instead of using vault-cli.

## Goal

After this work:
- `vault-cli <entity> show/set/get/clear` works for all five entity types
- Adding a new frontmatter field to an entity automatically makes it available via set/get/clear — no additional wiring needed
- Objective and vision entities have typed domain structs, storage interfaces, and implementations
- No entity-specific field switch statements remain in frontmatter operations

## Non-goals

- No new subcommands beyond show/set/get/clear (add/remove are a separate spec)
- No changes to task-specific commands (complete, defer, update, work-on)
- No changes to the generic list/lint/search commands (already work for all entities)
- No body content editing — frontmatter only

## Desired Behavior

1. `vault-cli goal set "My Goal" status completed` sets the status field on the goal's frontmatter. Same pattern for theme, objective, vision.

2. `vault-cli goal get "My Goal" status` returns the current value of the status field. Works for any frontmatter field defined in the domain struct.

3. `vault-cli objective show "My Objective"` displays all frontmatter fields and content, same format as `task show`.

4. `vault-cli vision clear "My Vision" priority` removes the priority field from frontmatter.

5. `set` on an unknown field (not defined in the domain struct) returns an error: `"unknown field 'xyz' for goal"`. No file is modified.

6. `get` on an unset field returns empty output (no error), same as current task behavior.

7. Field names in CLI match the YAML frontmatter key names (e.g., `defer_date`, `page_type`, `target_date`), not internal identifiers.

8. `vault-cli objective set/get/show/clear` and `vault-cli vision set/get/show/clear` work identically to task/goal/theme equivalents.

## Assumptions

- All five entity types share a compatible frontmatter structure (status, page_type, tags at minimum)
- The existing YAML serializer handles all field types correctly (strings, dates, string lists)
- No objective or vision domain structs exist yet — they must be created as part of this work

## Constraints

- Existing task `show`/`set`/`get`/`clear` behavior must not change — all current tests must pass
- Field access is automatic from domain struct definitions — no hardcoded field maps
- Domain structs for objective and vision follow the same pattern as goal and theme
- JSON output format supported for all operations
- Multi-vault dispatch works for all entity types
- Follow development-patterns.md layering: Domain → Storage → Operation → CLI
- Factory functions are pure composition — no I/O, no conditionals

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| `set` unknown field | Error: "unknown field 'xyz' for goal", exit 1 | User checks valid fields with `show` |
| `set` on metadata field (Name, FilePath) | Error: "field 'name' is read-only" | These fields are derived, not settable |
| Entity not found | Error: "goal 'X' not found", exit 1 | Same pattern as current task errors |
| `clear` on required field (status) | Allowed — user's choice, lint will flag it | User runs `lint --fix` to restore defaults |
| Malformed YAML frontmatter | Parse error propagates | User fixes file manually |

## Security / Abuse

- Field values come from CLI arguments — YAML injection via crafted values (e.g., values containing newlines or YAML special characters) must be handled by the YAML serializer, not raw string concatenation
- Entity names are resolved only within configured entity directories, preventing path traversal

## Acceptance Criteria

- [ ] `vault-cli goal set "G" status completed` updates goal frontmatter
- [ ] `vault-cli goal get "G" status` returns current status value
- [ ] `vault-cli goal show "G"` displays all fields and content
- [ ] `vault-cli goal clear "G" priority` removes priority field
- [ ] `vault-cli theme set "T" status active` works for themes
- [ ] `vault-cli objective set "O" status active` works for objectives
- [ ] `vault-cli objective show "O"` works for objectives
- [ ] `vault-cli vision set "V" status active` works for visions
- [ ] `vault-cli goal set "G" xyz "val"` fails with unknown field error
- [ ] `vault-cli goal set "G" name "X"` fails with read-only field error
- [ ] Malformed YAML frontmatter returns a parse error
- [ ] All existing task tests pass unchanged
- [ ] JSON output works for all entity show/set/get/clear operations
- [ ] `make precommit` passes

## Verification

```
make precommit
```

## Do-Nothing Option

Agents and slash commands continue using raw file editing (Read + Edit tools) to modify goal/objective/vision frontmatter. This works but bypasses validation, is error-prone (YAML formatting issues), and prevents vault-cli from being the single source of truth for entity mutations. Each new slash command must reinvent frontmatter parsing instead of calling vault-cli.
