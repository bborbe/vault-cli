---
tags:
  - guide
  - theme
---

A theme is an ongoing strategic direction — never truly "complete." Multiple related goals cluster under one theme over months and years. Themes are *process-oriented*; goals are outcome-oriented.

## TL;DR

- **Use for**: perpetual strategic direction, organizes related goals
- **Create**: `/vault-cli:create-theme "<title>"`
- **Audit**: `/vault-cli:audit-theme "<title>"`
- **Sections**: Summary → Impact → Sub-Goals → Related
- **Forcing functions**: present-tense title (no time-bound suffix); links to a parent vision; 5+ goals cluster naturally over time

## Goal

Produce a theme page that organizes long-running goals under one strategic banner, auditable by `theme-auditor` without ambiguity.

## When to Write a Theme

| Situation | Theme? |
|-----------|------|
| Ongoing direction (never "complete") | Yes |
| Multiple related goals cluster around same focus | Yes |
| Supports a parent vision | Yes |
| Process-oriented, not outcome-oriented | Yes |
| Specific completable deliverable | No — write a goal |
| Operational task | No — write a task |
| Lifetime aspiration | No — write a vision |
| Single isolated goal | No — just write the goal |

## Title & Filename

**Title = the strategic direction itself.** Filename = title (Obsidian renders the filename as the page title — no separate H1 needed).

**Rules:**

- Name the **direction**, not a project or a milestone — themes are perpetual; a project ends, a direction continues
- Pick wording that reads as a present-tense aspiration ("Increase Income", "Health", "Be Present Father")
- Avoid status / size / phase suffixes (*v2*, *extended*, *Phase 1*) — themes don't have phases
- 2–4 words is the natural length; longer titles usually mean a goal disguised as a theme

**Good vs bad:**

| ❌ Title | Why bad | ✅ Better |
|---|---|---|
| "Scale Account to $320k" | Time-bound, measurable — it's a goal, not a theme | "Increase Income" |
| "Launch Trading Bot" | Has a clear endpoint — it's a goal | "Build Automated Trading Systems" |
| "Q3 Focus: Family" | Time-bound + jargon | "Be Present Father" |
| "Health v2" | Versioning implies a milestone | "Health" |

**Sniff test:** read the title aloud. Can you imagine the theme existing in five years, unchanged? If yes → good. If "you'd have completed it by then" → it's a goal.

## Theme Structure

### Frontmatter

```yaml
---
status: in_progress
page_type: theme
visions:                                         # optional
  - "[[Parent Vision]]"
tags:
  - category
---
```

`status` valid values: `in_progress`, `next`, `backlog`, `hold`, `completed`, `aborted`. Themes rarely transition to `completed` — they're perpetual by design.

### Required sections

In order:

1. `Tags: [[Theme]]` (after frontmatter, before content separator)
2. **Summary** — 1–2 sentences describing the strategic direction and why it matters. Present tense.
3. `# Impact` — strategic value + vision connection; long-term significance
4. `# Sub-Goals` — list of related goal pages that advance this theme
5. `# Related` — sister themes, parent vision, optional

### Sub-Goals — the execution surface

A theme without goals is unanchored. Link 3+ goals (5–15 typical over the theme's lifetime).

**Good sub-goals:**

```markdown
# Sub-Goals
- [ ] [[Goal A]] — how it advances the theme
- [ ] [[Goal B]] — how it advances the theme
```

Each entry: wikilink to a real goal page + one-line rationale. Update as goals complete and new ones emerge.

## Scope Check

Before approving a theme:

- **Title is present-tense, perpetual** — passes the five-year sniff test
- **At least one parent vision linked** (or vision link in Impact)
- **2+ goals identifiable** under this theme right now, even if not all written yet
- **Process-oriented**, not a milestone disguised as a theme

If a theme reads as a completable outcome → demote to a goal and link it under whichever parent theme it advances.

## Preflight Checklist

Before approving:

- [ ] Is this an ongoing direction, not a completable outcome?
- [ ] Does the title read as a present-tense aspiration?
- [ ] Is there a parent vision (or clear vision linkage in Impact)?
- [ ] Are 2+ goals identifiable that will live under this theme?

## Audit

```
/vault-cli:audit-theme "<theme title or path>"
```

The auditor (`theme-auditor` agent) checks structure, title-as-direction (not milestone), vision linkage, and presence of goal cluster.

## Common Anti-Patterns

| Anti-pattern | Why it fails | Fix |
|---|---|---|
| Title with time horizon ("Q3 Trading Focus") | Themes are perpetual; time horizon = goal | Drop the time horizon; create a goal for the period instead |
| Single-goal theme | Theme without a cluster is just a relabeled goal | Either find sister goals or demote to a goal |
| Title describes a milestone ("Build the platform") | Milestone has an endpoint; theme has none | Rephrase as direction ("Build Automated Trading Systems") |
| Theme with no parent vision | Themes ladder under visions; orphan theme has no strategic anchor | Link a vision in frontmatter or Impact |

## Vault-Specific Examples

This doc covers structure and conventions. For concrete examples drawn from a real vault, see the per-vault writing guide:

- Personal vault: `~/Documents/Obsidian/Personal/50 Knowledge Base/Theme Writing Guide.md`

That guide is example-rich and references real themes; this doc is the generic contract.

## References

- `goal-writing.md` — goals ladder up to themes
- `task-writing.md` — tasks ladder up to goals
- `/vault-cli:theme list` — current themes
