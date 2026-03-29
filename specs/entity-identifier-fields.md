---
tags:
  - dark-factory
  - spec
status: idea
---

## Summary

- Add optional `*_identifier` frontmatter fields to goals, themes, visions, objectives, and decisions
- When present, entity lookup should match on identifier before falling back to name matching
- Follows existing `task_identifier` pattern already in use for Jira linking
- Enables external system disambiguation (e.g., linking goals to OKR tools, decisions to ADR numbers)
- Low priority — captures pattern for future use

## Problem

Tasks already support a `task_identifier` field for external disambiguation (e.g., Jira IDs like `TRADE-4304`). Other entity types lack this capability. If a user wants to reference a goal, theme, vision, objective, or decision by an external ID, there is no structured way to do so. Name-based lookup alone is fragile when entities have similar names or when external systems need stable references.

## Goal

All entity types support an optional identifier field in frontmatter. Looking up any entity by name also checks the identifier field, so users can reference entities by either their filename or their external identifier interchangeably.

## Non-goals

- Making identifiers required or auto-generated (they remain optional, user-supplied)
- Changing the existing `task_identifier` behavior or auto-UUID generation for tasks
- Adding identifier-based CLI flags or subcommands (future work)
- Cross-entity uniqueness enforcement

## Desired Behavior

1. Goals, themes, visions, objectives, and decisions each gain an optional `*_identifier` frontmatter field (e.g., `goal_identifier`, `theme_identifier`)
2. The identifier field is omitted from frontmatter when empty (consistent with `omitempty` pattern)
3. Entity lookup functions check identifier for exact match before falling back to existing name matching
4. Identifier match takes priority over partial name match but not over exact filename match
5. Existing entities without identifier fields continue to work unchanged

## Constraints

- Existing `task_identifier` behavior must not change (including UUID auto-generation)
- `findFileByName` lookup priority must remain: exact filename > identifier match > partial name match
- All existing tests must pass without modification
- Frontmatter field naming must follow `{entity_type}_identifier` convention

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Two entities share the same identifier | First match wins (directory walk order) | User corrects duplicate |
| Identifier contains special characters | Treated as opaque string, exact match only | No recovery needed |
| Entity file has identifier but lookup uses name | Both paths work, name match still functions | None needed |

## Acceptance Criteria

- [ ] Each entity domain struct has an optional `*_identifier` field with `omitempty`
- [ ] Lookup by identifier returns the correct entity
- [ ] Lookup by name still works when identifier is absent
- [ ] Exact filename match still takes priority over identifier match
- [ ] Existing task_identifier behavior is unchanged
- [ ] All existing tests pass

## Verification

```
make test
```

## Do-Nothing Option

Acceptable for now. No immediate user need. This spec captures the pattern so it is ready when external system integration requires it. Current name-based lookup works but is less stable for programmatic references.
