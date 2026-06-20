---
tags:
  - guide
  - vision
---

A vision is a lifetime aspirational identity — who you want to become, not what you want to do. Visions are timeless and directional; they sit at the top of the hierarchy and everything else (themes, objectives, goals, tasks) ladders up to them.

## TL;DR

- **Use for**: lifetime aspirations, identity-based "be …" statements, north star for all downstream work
- **Create**: edit by hand (vision creation is a reflective act, not a CLI form)
- **Audit**: `/vault-cli:audit-vision "<title>"` (optional — visions evolve rarely)
- **Sections**: Vision Statement → Identity Declaration → Connected Themes
- **Forcing functions**: title is identity-shaped (`Be …` / `Help …`); 2–6 visions total per vault — more than 6 dilutes the north-star quality

## Goal

Produce a vision page that names an aspirational identity, anchors downstream themes, and survives the years without rewrite.

## When to Write a Vision

| Situation | Vision? |
|-----------|------|
| Lifetime aspiration ("who I want to become") | Yes |
| Identity-based, not achievement-based | Yes |
| North star for many themes / goals | Yes |
| Specific completable outcome | No — write a goal |
| Time-bound strategic outcome | No — write an objective |
| Ongoing direction with goals clustering under it | No — write a theme |

## Title & Filename

**Title = aspirational identity statement.** Filename = title (Obsidian renders the filename as the page title — no separate H1 needed).

**Format:** `Be [Identity / State]` or `Help [Others Achieve …]`. Two to four words.

**Examples:**

- ✅ Be Financial Free (identity aspiration)
- ✅ Be Healthy (ongoing pursuit)
- ✅ Be Present Father (relational identity)
- ✅ Help Others Achieve Financial Freedom (purpose)
- ❌ Earn $1M (specific goal — has an endpoint)
- ❌ Complete Marathon (specific goal — has an endpoint)

**Sniff test:** read the title aloud after each life chapter (career change, family change, major life event). Does it still describe who you want to become? If it'd need rewriting, it's a goal, not a vision.

## Vision Structure

Visions are intentionally freeform — the structure exists to support reflection, not to gate it. Minimal page layout:

### Frontmatter (optional)

```yaml
---
page_type: vision
---
```

Visions rarely need status or other frontmatter — they're not tracked through a lifecycle.

### Required content

1. `Tags: [[Vision]]` (after frontmatter, before content separator)
2. **Vision Statement** — 1–3 paragraphs describing the aspirational identity. Present tense. Include what this identity means in practice, how it shapes choices, what's true when you're living it.
3. **Identity Declaration** — one-line crystallization (the elevator-pitch form of the vision). Often used as a header or a closing line.
4. **Connected Themes** — wikilinks to themes that ladder up to this vision

Optional: motivational quote, life chapter context, anti-vision ("what I am NOT").

## Scope Check

- **2–6 visions total** in a vault — fewer than 2 and the north-star quality is diluted across overly broad aspirations; more than 6 and the vault loses focus
- **Each vision is identity-shaped** — `Be …` / `Help …`, not `Achieve …` / `Build …`
- **No completion date** — visions are ongoing
- **2+ themes ladder under it** (over the vision's lifetime)

## Preflight Checklist

Before approving:

- [ ] Is this an identity, not a deliverable?
- [ ] Does the title take the `Be …` / `Help …` form?
- [ ] Is it timeless (no specific completion date)?
- [ ] Will themes naturally cluster under it?

## Audit

```
/vault-cli:audit-vision "<vision title or path>"
```

The auditor (`vision-auditor` agent if available, otherwise hand-audit) checks identity-shape title, timelessness, and theme cluster.

## Common Anti-Patterns

| Anti-pattern | Why it fails | Fix |
|---|---|---|
| Achievement title ("Earn $1M") | Achievement is a goal; vision is identity | Rephrase as identity ("Be Financial Free") |
| Time-bound aspiration ("Be Successful by 2030") | Visions are timeless | Drop the horizon; create an objective if a date matters |
| Too many visions (>6) | North-star dilution | Cluster related visions, demote some to themes |
| Vision with no themes ladder | Orphan — nothing flows from it | Connect 2+ themes |

## Vault-Specific Examples

This doc covers structure and conventions. For concrete examples drawn from a real vault, see the per-vault writing guide:

- Personal vault: `~/Documents/Obsidian/Personal/50 Knowledge Base/Vision Writing Guide.md`

That guide is example-rich and references real visions; this doc is the generic contract.

## References

- `theme-writing.md` — themes ladder up to visions
- `objective-writing.md` — objectives belong to exactly one vision
- `/vault-cli:vision list` — current visions
