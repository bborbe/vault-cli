---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-04-10T19:36:13Z"
generating: "2026-04-10T19:37:01Z"
prompted: "2026-04-10T19:47:26Z"
verifying: "2026-04-10T22:53:50Z"
branch: dark-factory/flexible-frontmatter-refactor
---

## Summary

- Replace rigid Go struct-based frontmatter types with typed map wrappers so unknown YAML fields survive round-trip without code changes
- Split each entity (Task, Goal, Theme, Objective, Vision) into three concerns: frontmatter (parsed YAML map), content (markdown body), and metadata (filesystem info)
- Provide typed getter methods for known fields with validation, while allowing arbitrary key-value access for unknown fields
- Eliminate the reflect-based get/set/clear helpers by making map access the native operation
- Enable `vault-cli set` to accept arbitrary frontmatter keys without a hardcoded switch statement

## Problem

Every entity type is a Go struct with explicit YAML tags for each frontmatter field. This causes two problems. First, adding a new frontmatter field to any Obsidian note requires a code change, recompile, and release of vault-cli before it can be read or written. Second, the `set` and `get` commands use either hardcoded switch statements (tasks) or reflect-based field lookups (goals, themes, etc.) to map keys to struct fields. Both approaches reject unknown fields, meaning vault-cli silently drops frontmatter it does not recognize on write, and refuses to get/set fields it does not know about. This is fragile for a tool whose primary job is managing user-authored YAML frontmatter.

## Goal

After this work, vault-cli preserves all frontmatter fields through read-write cycles, including fields it has never seen before. Known fields have typed accessors with validation. Unknown fields are stored and retrievable as raw values. The `set` command accepts any key-value pair. Each entity's domain representation cleanly separates frontmatter, markdown content, and filesystem metadata.

## Non-goals

- Changing the CLI command surface (no new commands, no flag changes)
- Refactoring the storage interface signatures (Read/Write/Find/List stay the same from the caller's perspective)
- Migrating the Decision type (it follows a different pattern and can be done separately)
- Adding schema validation for unknown fields (they pass through unvalidated)
- Changing how content is parsed (frontmatter regex, checkbox parsing stay the same)

## Assumptions

- YAML field ordering is cosmetic — reordered fields on write are acceptable (produces git diffs but no semantic change)
- Unknown fields set via CLI are stored as strings; no automatic type coercion for unknown keys
- `docs/development-patterns.md` line 27 ("struct with YAML tags for frontmatter fields") must be updated to reflect the new map-based pattern
- Decision type is excluded from this refactor and can follow later
- Existing YAML frontmatter in all vaults is well-formed (no duplicate keys, no bare `---` inside content)

## Desired Behavior

1. **Unknown fields survive read-write cycles.** Reading a file with frontmatter fields that vault-cli does not know about, then writing it back, preserves those fields without data loss.

2. **Known fields return correctly typed values.** Accessing a known field (e.g., status, priority, goals, defer_date) returns the expected Go type. Missing or incompatible values return zero values without panicking.

3. **Known fields validate on write.** Setting a known field to an invalid value (e.g., status to "banana", priority to -1) returns a validation error and does not modify the file.

4. **Unknown fields pass through on write.** Setting an unknown field stores the value and writes it to the file. No validation is applied to unknown fields.

5. **Any frontmatter key is gettable.** The `get` command returns the value for any key present in the file's frontmatter, whether known or unknown.

6. **Any frontmatter key is settable.** The `set` command accepts any key-value pair. Known fields apply type coercion and validation. Unknown fields store the string value directly.

7. **Entity representation separates concerns.** Each entity (Task, Goal, etc.) cleanly separates its YAML frontmatter, markdown content, and filesystem metadata into distinct parts. Frontmatter is per-type; content and metadata are shared types used by all entities.

8. **Round-trip fidelity.** Reading a file and writing it back without changes produces identical frontmatter content (no fields dropped or added; field order may differ).

## Constraints

- Existing CLI commands, flags, and output formats must not change
- All existing tests must continue to pass (or be updated to use the new accessor pattern with equivalent assertions)
- Status normalization for tasks (legacy aliases like "next" -> "todo") must still work
- TaskIdentifier UUID auto-generation on write must still work
- Goal checkbox parsing from content must still work
- The storage interface method signatures should remain stable; internal implementations change but callers should not need to change their call patterns
- One type per file convention in the domain package
- `docs/development-patterns.md` must be updated to reflect the new pattern

## Security / Input Validation

| Input | Risk | Mitigation |
|-------|------|------------|
| Key names with YAML special characters (`---`, `...`, `:`, `#`) | Could break YAML serialization | YAML library handles escaping; test with edge cases |
| Key names with Obsidian-breaking characters (`[[`, `]]`, `\|`) | Could corrupt note linking | No mitigation needed — frontmatter is not rendered as wiki links |
| Very long key names or values | Potential file bloat | No mitigation — user responsibility, same as current behavior |
| Key names colliding with known fields but wrong case (`Status` vs `status`) | Could bypass validation | Keys are case-sensitive per YAML spec; document this |

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| YAML frontmatter contains a complex nested value (list of maps) | Stored in the map, round-trips correctly | None needed |
| `set` called with a known field name but invalid value (e.g., status=banana) | Validation error returned, file not modified | User corrects value |
| `set` called with an unknown field name | Value stored in map, file written | None needed |
| `get` called with a field not present in the file | Returns empty string, no error | None needed |
| Existing file has YAML field ordering that differs from original | Accepted; field order may change on write | Cosmetic only |
| Type assertion fails in getter (e.g., status stored as int) | Getter returns zero value, no panic | User fixes frontmatter manually |
| `set` writes a known field with wrong YAML type (e.g., user manually edits `priority: "high"`) | Getter returns zero value; next `set` with valid value fixes it | User corrects value |

## Acceptance Criteria

- [ ] Reading a file with unknown frontmatter fields and writing it back preserves those fields
- [ ] `vault-cli task set <name> custom_field value` succeeds and persists the field
- [ ] `vault-cli task get <name> custom_field` returns the stored value
- [ ] `vault-cli goal set <name> timeline "2026-03-30 to 2026-04-27"` succeeds (previously failed)
- [ ] Known field getters return correctly typed values (status, priority, goals, phase, dates)
- [ ] `Validate()` rejects invalid known field values (bad status, negative priority)
- [ ] `Validate()` ignores unknown fields
- [ ] Task status normalization (legacy aliases) still works on read
- [ ] TaskIdentifier UUID auto-generation still works on write
- [ ] Goal checkbox parsing still works
- [ ] `make test` passes
- [ ] No hardcoded key-to-field mapping remains in get/set/clear/show operations
- [ ] `docs/development-patterns.md` updated to reflect map-based frontmatter pattern
- [ ] Frontmatter, content, and metadata are separate types in the domain package

## Verification

```
make test
```

## Do-Nothing Option

The current struct-based approach works but is increasingly painful. Every new frontmatter field requires a code change across domain, storage, and ops layers. More critically, vault-cli silently drops unknown frontmatter fields on write, which can cause data loss when Obsidian plugins or manual edits add fields that vault-cli does not know about. The reflect-based get/set helpers are complex and fragile. This refactor eliminates an entire class of bugs and makes the tool resilient to frontmatter schema evolution.
