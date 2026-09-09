# Output Formatting

How vault-cli agents and slash commands render terminal-facing reports (status, progress, grouped checkboxes) so they read well at a glance.

## Glyph mapping (the one rule that matters)

Disk is the source of truth: vault markdown keeps literal `[x]` / `[/]` / `[ ]` checkboxes — never rewrite them. Rendering maps the token to a display glyph per line:

| Disk token | Meaning | Display glyph |
|---|---|---|
| `[x]` | completed | `✅` |
| `[/]` | in-progress | `⏳` |
| `[ ]` | pending | `❌` |

Rules:

- **Parse the disk token, render the glyph.** The agent reads `[x]`/`[/]`/`[ ]` verbatim for counting and classification; only the *output* maps them to glyphs. The vault file is never mutated by rendering.
- **Semantic note vs the vault's `Icons.md`:** the personal vault's [[Icons.md]] page uses `🔄` for "In Progress" and `❌` for "Failed/Cancelled". The status-report mapping above intentionally uses `⏳` for in-progress (an hourglass reads as "working, not waiting") and `❌` for plain pending (the not-yet-done state) — `❌` on a pending line means "not done yet", not "this failed". Where the two conflict, follow the report convention; document the divergence in the report if a reader could misread it.
- **Never echo a raw markdown token** (`[x]`, `[ ]`, `[/]`) in a terminal report — it reads like a source dump.

## Legend

When the report uses glyphs whose meaning isn't self-evident from context, emit a one-line legend with the `ℹ️` info marker, placed directly under the status header:

```
ℹ️ legend: ✅ done · ⏳ in-progress · ❌ pending
```

Include it when the report has at least one non-`[x]` item; a fully-complete report needs no legend.

## Report shape

- **Section header carries the count** — `## Success Criteria — 5/5` (or `4/6`). The header gives the ratio at a glance; the per-line glyphs carry the detail.
- **Assessment block first** — `Phase:` / `Plan:` / `Recommend:` at the very top, blank line after, before any glyph section. (task-status contract.)
- **`▶ Next:` (or `🔜 Next:`)** as the single actionable line, always last — one concrete action, never a list.
- **Omit empty sections** — a section with zero checkboxes prints no header and no body.
- **One blank line between sections.**

## Header block

Two or three lines, not a paragraph:

```
📋 <Task or Goal name>
   status: <status> · phase: <phase> · <completed>/<total> (<pct>%)
   plan: <validated · N/M subtasks | not started (missing SC/Tasks)>
```

## In-progress (`[/]`) handling

`[/]` counts as **not done** for all progress math (only verbatim `[x]` counts). It renders as `⏳` so a stuck-in-progress item is visible without reading the raw file.

## Status emoji (report-level, distinct from line glyphs)

| State | Emoji |
|---|---|
| Complete / all done | `✅` |
| Valid (structure passes) | `✅` |
| Issues / invalid | `❌` |
| Warning (minor) | `⚠️` |
| Waiting on something | `⌛` or `⏳` |
| Next action | `▶` or `🔜` |

## Anti-patterns

- Echoing raw `[x]`/`[ ]`/`[/]` tokens into a terminal report.
- Using `✅`/`❌` for *line* state and *report verdict* inconsistently (e.g. `✅` line glyph but text "not met").
- No legend when glyphs are non-obvious.
- Section header without a count.
- `Next:` naming two or more actions.

## Source of truth

- Vault icon vocabulary: [[Icons.md]] (general-purpose `✅`/`❌`/`🔄`/`⚠️`/`🎯` etc.).
- Task report contract: `agents/task-manager-agent.md` (task-status grouped-checkbox output).
- Goal report shape: `agents/goal-manager-agent.md` (aggregate-only — Criteria/Subtasks counts + `🔜 Next`).
