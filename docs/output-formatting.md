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

## Anchor pair (Async State Closer)

`task-status` and `goal-status` ALWAYS lead their output with the two-line anchor pair — clickable `obsidian://` links to the goal and the task, so the operator can open either from any status run:

```
🎯 Goal: [<goal name>](obsidian://open?vault=<V>&file=<enc relpath>) — <n>/<m> SC · <n>/<m> subtasks · binding: <value>
📌 Task: [<task name>](obsidian://open?vault=<V>&file=<enc relpath>) — <phase>, session <id8> (this one) · ⚠️ also claimed by peer <id8>
```

**Emitted on every run** — `Next:` / complete / error branches alike. Only the Phase-2 zero-match `❌` branch omits it (nothing to link). The pair sits ABOVE the assessment block (`Phase:`/`Plan:`/`Recommend:`), one blank line after.

### Link rule

- `vault` = basename of the matching vault's `path` from `vault-cli config list --output json` (NOT the lowercase config `name`).
- `relpath` = file path minus `<vault.path>`, no leading slash, no `.md` suffix.
- Percent-encode every char outside the unreserved set `[A-Za-z0-9-_.~]` (space `%20`, `/` `%2F`, `—` `%E2%80%94`, `+` `%2B`). Never encode the literal `?`/`=` separators between query keys.
- One-liner: `printf '%s' "${FILE#$VAULT_PATH/}" | sed 's/\.md$//' | python3 -c "import sys,urllib.parse; print(urllib.parse.quote(sys.stdin.read(), safe=''))"`

### Session suffix + peer claim

- `MINE` = `$CLAUDE_CODE_SESSION_ID` (harness-set), kept only when the transcript `~/.claude/projects/<enc-session-project-dir>/<MINE>.jsonl` exists (transcript dir name is canonical, per `session-close.md`).
- `IDS` = the linked file's `claude_session_id` ∪ `metrics_sessions[].session_id` frontmatter values.
- Always render `, session <MINE8> (this one)` when `MINE` is set; when unset, render `, session <first id8>` (no "(this one)" — can't identify self).
- **Peer clause** — append ` · ⚠️ also claimed by peer <id8>` for each `IDS` id ≠ `MINE` whose session state is `live`. Liveness per [`session-liveness.md`](session-liveness.md) — the single definition; do not restate it inline. Comma-join multiple peers.

### Counts

- `SC <n>/<m>` = goal `# Success Criteria` checkbox lines; done = verbatim `[x]` only, total = `[x]`+`[/]`+`[ ]`.
- `subtasks <n>/<m>` = goal `# Tasks` section. Checkbox items (`- [x] [[Task]] …`): same token rule. Fallback when the section has no checkboxes (plain `- [[Task]] ✅ completed` lists): count leading-`[[...]]` items, done = task file `status: completed` via `vault-cli task get "<title>" status --output json`.
- Omit a count when its total is 0 and no fallback applies.

### Conditional segments

- ` · binding: <value>` — only when goal frontmatter has a `binding:` field (see `docs/goal-writing.md` § Frontmatter).
- `📌 Task:` for `goal-status` names the goal's next open task (leading-`[[...]]` walk per `execute-goal.md` step 7, first status ∉ {completed, aborted}); none open → `📌 Task: none — all tasks complete`.
- `task-status` with no `goals:` frontmatter → `🎯 Goal: (no goal linked)`; goal file missing → `🎯 Goal: <title> — (goal file missing)`. The pair is still emitted.

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
