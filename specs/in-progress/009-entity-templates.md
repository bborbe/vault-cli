---
status: generating
approved: "2026-04-27T09:44:27Z"
generating: "2026-04-27T09:45:02Z"
branch: dark-factory/entity-templates
---

## Summary

- Vault config gains five optional template path fields, one per entity type (task, goal, theme, objective, vision)
- Accessor methods return the resolved absolute path for each, or empty string if unset
- Path resolution mirrors existing `*_dir` fields: relative paths resolve against the vault root; `~` expands; absolute paths pass through
- Consumers (e.g. Claude commands / agents that create entities) read the configured path and decide what to do with the file
- Existing configs without these fields continue to parse and behave identically

## Problem

Tooling that creates entities (slash commands, agents, scripts) currently has no portable way to find a vault's template files. Each vault stores its templates in its own location (`90 Templates/Task Template.md`, `Templates/task.md`, custom paths). Tools either inline a hardcoded path or duplicate the path across every consumer.

This blocks generic, vault-agnostic creation tooling. A generic `/create-task` slash command in a plugin marketplace cannot know whether a vault has a template, where it lives, or what the conventions are.

Storing the template path in vault config — alongside the existing `tasks_dir`, `goals_dir`, etc. — gives consumers a single source of truth and lets the plugin remain template-agnostic.

## Goal

`vault-cli`'s `Vault` config exposes an optional template path per entity type. Consumers can call a typed accessor and receive a resolved path string (or empty string if unset). The plugin holds no template content, performs no template parsing, and does no entity creation. It only stores and resolves the paths.

## Non-goals

- Reading, parsing, or producing template content from `vault-cli`
- Creating entities, merging frontmatter, or writing entity files
- Embedded default templates inside the plugin
- Convention-based template discovery (no implicit search of `90 Templates/`, etc.)
- Template variable substitution
- Validating that the configured template file exists (out of scope; handled by consumers)
- Changing entity creation flow (no entity creation exists in `vault-cli` today)

## Assumptions

- Existing `*_dir` config pattern (`tasks_dir`, `goals_dir`, etc.) is the right model to mirror
- Five entity types in scope: task, goal, theme, objective, vision (matches existing `*_dir` fields)
- Template files are markdown; `vault-cli` does not need to know their format
- Consumers (slash commands, agents) handle file existence checks, content reading, and entity body emission

## Desired Behavior

1. The `Vault` config supports an optional template path field for each of the five entity types (task, goal, theme, objective, vision), with snake_case YAML field names and `omitempty` serialization
2. A typed accessor per entity type returns the resolved absolute path or empty string when the field is unset
3. Path resolution reuses the existing vault path handling: relative paths resolve against the vault root, `~` expands to the user home, absolute paths pass through unchanged
4. Existing vault configs without any `*_template` fields parse, serialize, and behave identically to today

## Constraints

- New fields and accessors must mirror the existing per-entity-type directory config pattern: same struct, same `omitempty` YAML tags, same accessor naming convention
- Field naming must follow the YAML snake_case `{entity_type}_template` convention to match the existing `{entity_type}_dir` pattern
- Path resolution must reuse the existing path handling logic — no parallel implementation
- All existing `vault-cli` operations and tests must pass without modification
- No new dependencies introduced
- No CLI commands added or changed by this spec
- No changes to entity domain types, storage, or operations

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Template field missing or empty in config | Accessor returns empty string | Consumer treats as "no template" |
| Template field set to a relative path | Accessor returns path resolved against vault root | None needed |
| Template field set to an absolute path | Accessor returns path unchanged | None needed |
| Template field set to a `~`-prefixed path | Accessor returns path with `~` expanded | None needed |
| Existing config without any `*_template` fields | Parses and round-trips unchanged | None needed |

## Do-Nothing Option

If we don't ship this:

- Tooling that creates entities cannot be made generic or moved into the plugin marketplace
- Per-vault Claude command copies (`40 Tasks/`, `24 Tasks/`, paths hardcoded in two `create-task.md` files) continue to drift
- Each new vault that wants templated creation must duplicate command + agent files
- Each new entity creation feature multiplies that duplication across all vaults

Cost grows with each new vault and each new entity creation command.

## Acceptance Criteria

- [ ] `Vault` config supports a template path field for each of the five entity types (task, goal, theme, objective, vision), with `omitempty` YAML serialization
- [ ] A typed accessor returns the resolved absolute path per field, or empty string if unset
- [ ] Path resolution behaves identically to the existing `*_dir` resolution (vault-relative default, `~` expansion, absolute pass-through)
- [ ] Existing vault configs without any `*_template` fields parse and round-trip unchanged
- [ ] Path resolution is verified for relative, absolute, `~`-prefixed, empty, and unset cases; serialization round-trip is verified with and without `*_template` fields
- [ ] CHANGELOG.md has an entry under `## Unreleased`

## Verification

```bash
make precommit
```

Unit-level checks must demonstrate:

- A YAML config containing all five `*_template` fields parses into the `Vault` struct
- Each typed accessor returns the resolved absolute path for its field, applying vault-relative resolution and `~` expansion
- Empty / missing template fields return empty string from accessors (no error)
- An existing vault config without any `*_template` fields parses identically and round-trips through serialize unchanged
- Adding a new `*_template` field does not break any existing test fixture
