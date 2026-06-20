---
tags:
  - guide
  - objective
---

An objective is a 3-to-12-month strategic outcome that organizes multiple goals into one quarterly or annual achievement. Objectives are explicitly time-bound; the horizon is part of the identity.

## TL;DR

- **Use for**: quarterly/annual strategic outcomes, 3–12 months, 3–10 contributing goals
- **Create**: `/vault-cli:create-objective "<title>"`
- **Audit**: `/vault-cli:audit-objective "<title>"`
- **Sections**: Summary → Impact → Status Summary → Success Criteria → Non-goals → Contributing Goals → Related
- **Forcing functions**: time-horizon in the title; observable end-state; ladders under exactly one vision

## Goal

Produce an objective page that names a horizon-end outcome, surfaces the goals that ladder up to it, and is auditable without ambiguity.

## When to Write an Objective

| Situation | Objective? |
|-----------|------|
| Quarterly / annual strategic outcome (3–12 months) | Yes |
| Multiple goals (1–4 weeks each) ladder up | Yes |
| Theme needs concrete near-term direction | Yes |
| Periodic note (Quarterly / Yearly) needs trackable result | Yes |
| Single 1–4 week deliverable | No — write a goal |
| Never-ending direction | No — write a theme |
| Operational task | No — write a task |

## Title & Filename

**Title = the outcome you'll have at the end of the period.** Filename = title (Obsidian renders the filename as the page title — no separate H1 needed).

**Rules:**

- Name the **outcome at horizon-end**, not the activity that gets you there ("Restore Trading Profitability Q1 2026", not "Work on Trading Q1")
- Include the **time horizon** in the title — objectives are explicitly time-bound; the horizon is part of the identity ("Q1 2026", "H1 2026", "2026")
- Make the outcome **observable**: a future reader should be able to look at the world on the end-date and answer yes/no
- Avoid status / phase suffixes — if you need "v2", split into a new objective for the new horizon
- 5–10 words is the natural length

**Good vs bad:**

| ❌ Title | Why bad | ✅ Better |
|---|---|---|
| "Work on Trading Q1 2026" | Activity, not outcome | "Restore Trading Profitability Q1 2026" |
| "Scale Account" | No horizon — could be this quarter or this decade | "Scale Account to $320k by Q3 2026" |
| "Trading Goals Q1" | Names the container, not the outcome | "Reach Trading Breakeven Q1 2026" |
| "Improve Health" | Theme-shaped (perpetual); no horizon | "Lose 8 kg by June 2026" |

**Sniff test:** on the objective's end date, can a third party look at the world and answer "did this happen, yes or no" without asking you? If yes → good. If "depends on interpretation" → re-anchor on the observable outcome.

## Objective Structure

### Frontmatter

```yaml
---
status: in_progress
page_type: objective
priority: 1                                      # 1 (highest) – 3
duration: 2026-01-01 to 2026-03-31               # ISO format, required
vision: "[[Parent Vision]]"                      # exactly one vision
themes:
  - "[[Parent Theme]]"                           # one or more themes
---
```

`status` valid values: `in_progress`, `next`, `backlog`, `hold`, `completed`, `aborted`. The objective transitions to `completed` (or `aborted` with reason) on its end date — not before.

### Required sections

In order:

1. `Tags: [[Objective]]` (after frontmatter, before content separator)
2. **Summary** — 1–2 sentences naming the end-state outcome and benefit
3. `# Impact` — strategic value + vision/theme connection + quantified where possible
4. `# Status Summary` — Progress / Current / Next / Blockers (one line each)
5. `# Success Criteria` — 3–5 binary, observable checkbox outcomes (each verifiable on the end date)
6. `# Non-goals` — 3–7 concrete deferrals (parallels goals' Non-goals)
7. `# Contributing Goals` — wikilinks to the goals that ladder up
8. `# Related` — parent vision, sister objectives, parent theme(s)

## Scope Check

Before approving an objective:

- **Title has explicit horizon** (Q1 2026, H1 2026, 2026) AND names an observable outcome
- **3–10 contributing goals identifiable** (even if not all written yet)
- **Each success criterion is verifiable on the end date** (no "depends on interpretation")
- **Exactly one vision in `vision:` frontmatter** — objectives belong to one life area
- **Duration field present** in ISO format

If 3+ smells fail → split into multiple objectives or demote the over-scoped one to a goal.

## Preflight Checklist

Before approving:

- [ ] Does the title name an observable outcome at horizon-end?
- [ ] Is the time horizon explicit in the title?
- [ ] Can each success criterion be verified yes/no on the end date?
- [ ] Are 3–10 contributing goals identifiable?
- [ ] Is exactly one parent vision linked?

## Audit

```
/vault-cli:audit-objective "<objective title or path>"
```

The auditor (`objective-auditor` agent) checks structure, horizon-in-title, observable success criteria, vision singleness, and contributing-goal cluster.

## Common Anti-Patterns

| Anti-pattern | Why it fails | Fix |
|---|---|---|
| Title without horizon ("Restore Trading Profitability") | Can't be verified on a date that doesn't exist | Add explicit horizon |
| Title names the activity ("Work on Trading Q1") | Activity ≠ outcome; can be busy and miss the outcome | Re-anchor on the end-state |
| Aspirational success criterion ("Trading is healthier") | Not verifiable on the end date | Replace with observable: "TR profit factor > 1.5 on Q1 trades" |
| Multiple visions in frontmatter | Forces unbalanced allocation across life areas | Split into one objective per vision |
| Contributing goals not yet written | Objective is unanchored — the laddering doesn't exist | Write at least 2–3 of the contributing goals before approving |

## Vault-Specific Examples

This doc covers structure and conventions. For concrete examples drawn from a real vault, see the per-vault writing guide:

- Personal vault: `~/Documents/Obsidian/Personal/50 Knowledge Base/Objective Writing Guide.md`

That guide is example-rich and references real objectives; this doc is the generic contract.

## References

- `goal-writing.md` — goals ladder up to objectives
- `theme-writing.md` — themes are perpetual; objectives are time-bound under them
- `/vault-cli:objective list` — current objectives
