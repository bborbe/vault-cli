---
tags:
  - guide
  - daily-note
---

# Daily Notes

A daily note is a markdown file named `YYYY-MM-DD.md` in the vault's `daily_dir`. It tracks tasks worked on that day using checkbox syntax.

## Task Entry Contract

A task's **own entry** on a daily note is a checkbox line whose text begins with `[[<taskName>]]`, optionally preceded by decoration — a category emoji, a symbol, or markdown emphasis such as `**`. The following lines are all own entries:

```markdown
- [/] [[Feed Worms]]
- [/] 🐟 [[Feed Worms]]
- [/] **[[Feed Worms]]**
- [/] 🚨 [[Feed Worms]] — analysis done
```

**Decoration rule:** everything before the wikilink is skipped as decoration as long as it contains no letter and no digit. The first letter or digit ends the decoration run and makes the rest prose. Because the test is Unicode-aware, letters in **any script** end the run — `作業 [[Feed Worms]]` (Japanese) and `Задача по [[Feed Worms]]` (Russian) are mentions, not own entries. A digit also ends the run, so a keycap prefix such as `1️⃣ [[Feed Worms]]` is not recognised.

The character-class test means decoration is matched by rule, not by an enumerated list. Category emoji, symbols such as section-sign (`§`) or double-dagger (`‡`), and markdown emphasis markers (`**`, `__`) are all decoration because none are letters or digits.

Decoation is **preserved** when a command rewrites the line: `task complete` on `- [/] 🐟 [[Feed Worms]]` produces `- [x] 🐟 [[Feed Worms]]`.

A wikilink to the task appearing **anywhere else** on the line — after prose — is a **mention**, not an own entry. A mention is never rewritten, never deleted, and is not counted as "the task is already tracked" by `task complete`, `task defer`, or `task work-on`.

### Own Entry vs Mention

Given these two lines in a daily note:

```markdown
- [x] 🔧 Nuke-reboot chain — [[Shutdown K3s - 2026W32-sat]] → [[Feed Worms]].
- [/] 🐟 [[Feed Worms]]
```

The first line is a **mention** of `Feed Worms` (the wikilink appears after prose, not at the start — the word `Nuke-reboot` is prose). The second line is the **own entry** (the wikilink is at the start after decoration).

### Wikilink Forms

Alias and heading link forms resolve to the same task:

- `[[Task Name|label]]` — alias form, the label is ignored when matching
- `[[Task Name#Section]]` — heading form, the heading is ignored when matching
- `[[Task Name#Section|label]]` — canonical Obsidian form, both are ignored

Matching is **case-insensitive**: `[[Turn on hell]]` matches task name `turn on hell`.

### Literal Comparison

Task names are compared as **literal strings**, not as regular expression patterns. A task named `Fix (urgent) c++ build` matches only the exact wikilink `[[Fix (urgent) c++ build]]`, not `[[Fix XurgentX cXX build]]`.

## Commands That Read the Contract

The following commands read the daily-note entry contract to determine which checkbox line belongs to a given task:

- **`task complete`** — flips a task's own entry from `[ ]` or `[/]` to `[x]`; mentions are left untouched
- **`task defer`** — removes a task's own entry from today's note and adds a new entry to the target day's note; mentions are left untouched
- **`task work-on`** — promotes a `[ ]` entry to `[/]`, or adds a new `[/]` entry when the task has no own entry yet; mentions are left untouched
