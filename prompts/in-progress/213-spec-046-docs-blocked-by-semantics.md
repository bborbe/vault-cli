---
status: approved
spec: [046-blocked-by-frontmatter-model]
created: "2026-09-11T07:53:52Z"
queued: "2026-09-11T09:20:10Z"
---

# Document blocked_by as a dependency list orthogonal to status

<summary>
- The task-writing guide explains that a task can list what it depends on, and what "blocked" then means.
- The goal-writing guide gets the matching explanation for goals, where dependencies resolve among goals.
- Both guides state that a dependency counts as met only when the named item is completed, and that an unverifiable dependency counts as unmet.
- Both guides state that blocked state is derived and independent of `status` — nothing sets `hold` automatically.
- The claim that a populated dependency list is what puts an item into `hold` is removed from both lifecycle tables.
- Both guides show the field in their frontmatter example and name the JSON fields consumers read.
- Both guides say the next-task command does not recommend blocked work.
- Both guides record how to clear a dependency list.
- Documentation only — no code, no tests, no behavior change.
- The changelog records the documentation change.

</summary>

<objective>
Rewrite the `blocked_by` guidance in `docs/task-writing.md` and `docs/goal-writing.md` so it describes the shipped dependency model — a list whose derived block state is orthogonal to `status` — instead of the current "a populated `blocked_by` is what triggers `hold`" phrasing that conflates scheduling state with dependency state (spec 046 Desired Behavior 7).
</objective>

<context>
This prompt depends on prompts 1 and 2 of spec 046 being already applied — the guides must describe what actually shipped on this branch: the `blocked_by` accessor and blocked-state resolver in prompt 1, and the next-task filter in prompt 2.

Read `CLAUDE.md` first, then read in full:

- `docs/task-writing.md` — the whole file. Three places matter:
  - `### Frontmatter` (~line 79): the YAML example block ends with `recurring: weekly                                # optional, for routine tasks`.
  - The `assignee` semantics prose and table (~lines 96-110) ends with the sentence beginning `To deliberately take over a task owned by someone else, use `vault-cli task set "<name>" assignee "<current_user>"`` — that sentence is immediately followed by `### Required sections`.
  - `## Lifecycle` (~line 340): the table row for `hold` reads, verbatim:
    `| `hold` | Blocked long-term (weeks+, external dependency, unresolved upstream) | `blocked_by:` field populated, or operator sets manually |`
- `docs/goal-writing.md` — the whole file. Three places matter:
  - `### Frontmatter` (~line 105): the YAML example block ends with the `themes:` list.
  - The line after it reads `status` valid values: `in_progress`, `todo`, `backlog`, `hold`, `completed`, `aborted`. — immediately followed by `### Required sections`.
  - `## Lifecycle` (~line 386): the `hold` row reads, verbatim:
    `| `hold` | Blocked or paused | `blocked_by:` field populated, or operator sets manually |`
- `pkg/ops/blocked_by.go` and `pkg/ops/list.go` (as prompt 1 left them) — the guides must describe the real resolution rules and the real JSON field names, not an intention.
- `commands/next-task.md` (as prompt 2 left it) — the guides must describe the real next-task behaviour.
- `docs/goal-writing.md`'s `## Lifecycle` section also states that completed goals are immutable; leave that and every other unrelated statement alone.

Read this coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md` — documentation conventions.

<!-- OPEN QUESTIONS (resolved by the prompt writer; flagged for the human reviewer):

1. Both lifecycle tables currently name `blocked_by:` as a `hold` trigger. Spec Desired Behavior 7 requires that phrasing to be replaced, and spec 046 Non-goals forbid auto-setting `hold` from `blocked_by`. The `hold` row itself is NOT deleted — `hold` remains a real, operator-set status for waits that outlive the current week; only the trigger cell changes. The surrounding `**Hold vs in_progress + `[/]` subtask:**` paragraph in `docs/task-writing.md` already describes `hold` correctly and is left untouched.

2. The clearing guidance names two commands. `vault-cli task clear "<name>" blocked_by` (and `goal clear`) deletes the key entirely via `ClearField`. `vault-cli task set "<name>" blocked_by ""` empties the effective list because `blocked_by` has no `SetField` case and a scalar value reads as an empty list — it is the recovery path named in spec 046's Failure Modes table, so it is documented even though it leaves the key behind with an empty string value.

3. The guides deliberately do not document `|alias` or `#section` wikilink suffixes for `blocked_by`: the resolver strips brackets only, matching the existing entity-name resolution in `pkg/storage/base.go`. Documenting more than the code does would create drift.
-->

</context>

<requirements>

## 1. `docs/task-writing.md` — show the field in the frontmatter example

Inside the `### Frontmatter` YAML block, add this line after `recurring:` and before the closing `---`:

```yaml
blocked_by:                                      # optional — dependencies; unblocks when all complete
  - "[[Blocker Task]]"
```

## 2. `docs/task-writing.md` — add the `### Dependencies (`blocked_by`)` subsection

Insert a new `### Dependencies (`blocked_by`)` subsection between the `assignee` prose/table and `### Required sections`. Content to cover, written as flowing prose plus a short list — not a wall of bullets:

1. **What the field is.** `blocked_by` declares the tasks this task cannot start before. It is a YAML list of task names; each entry may be a plain name (`Blocker Task`) or a wikilink (`[[Blocker Task]]`) — both forms resolve the same way. A scalar value (`blocked_by: Blocker Task`) is malformed and reads as an empty list, so it never blocks.
2. **How a name resolves.** Names match task files in the tasks directory, ignoring case and stripping the `[[` `]]` brackets. Same-kind only: a task's blockers are tasks, never goals, and the lookup is exact-name — a substring is not a match.
3. **What "blocked" means.** The task is blocked while at least one named blocker is not `status: completed`. A blocker whose file is missing, unreadable, or carries no parseable status counts as **not** completed: the safe default is "cannot verify it is done, so do not start". Only a single status read is performed per blocker — a blocker's own `blocked_by` is never followed, so a dependency cycle leaves both tasks blocked and terminates immediately rather than hanging.
4. **Orthogonal to `status`.** Blocked state is derived and is never written. Nothing sets `status: hold` from `blocked_by`, and no command flips a status when a blocker completes. `hold` remains an operator decision for a wait measured in weeks (see `Hold vs in_progress` under `## Lifecycle`); a blocked task usually stays `next` or `in_progress`.
5. **Where it surfaces.** `vault-cli task list --output json` emits `blocked_by` (the raw list) and a computed `blocked` boolean for any task that declares a dependency list; a task with no `blocked_by` emits neither key. `/vault-cli:next-task` does not recommend a task whose `blocked` flag is true.
6. **How to clear it.** `vault-cli task clear "<name>" blocked_by` removes the key; `vault-cli task set "<name>" blocked_by ""` empties the effective list. Either makes the task read as unblocked again.

State the rules as facts about the current tooling. Do not add a "planned", "future" or "not yet implemented" hedge anywhere.

## 3. `docs/task-writing.md` — fix the `hold` lifecycle row

In the `## Lifecycle` table, replace the trigger cell of the `hold` row. The row must read exactly:

```
| `hold` | Blocked long-term (weeks+, external dependency, unresolved upstream) | Operator sets manually when the block outlives the current week |
```

The phrase `blocked_by:` field populated must not survive anywhere in the file. Leave the `## Lifecycle` table's other rows, the close-out-fields paragraph and the `Hold vs in_progress` paragraph unchanged.

## 4. `docs/goal-writing.md` — show the field in the frontmatter example

Inside the `### Frontmatter` YAML block, add after the `themes:` list and before the closing `---`:

```yaml
blocked_by:                                      # optional — dependencies; unblocks when all complete
  - "[[Blocker Goal]]"
```

## 5. `docs/goal-writing.md` — add the `### Dependencies (`blocked_by`)` subsection

Insert a new `### Dependencies (`blocked_by`)` subsection between the `status` valid-values line and `### Required sections`. It covers the same six points as requirement 2 with the goal-specific differences made explicit:

- Goal blockers resolve in the goals directory — a goal's blockers are goals, never tasks, and never another kind.
- The derived field surfaces on `vault-cli goal list --output json` as `blocked_by` plus the computed `blocked` boolean.
- `/vault-cli:next-task` does not recommend a goal or task whose `blocked` flag is true.
- Clearing uses `vault-cli goal clear "<name>" blocked_by` or `vault-cli goal set "<name>" blocked_by ""`.
- Blocked state is orthogonal to `status` — a blocked goal is not moved to `hold`, and `hold` stays an operator decision.

Keep it shorter than the task-writing version (goals declare dependencies less often) but do not drop any of the six rules.

## 6. `docs/goal-writing.md` — fix the `hold` lifecycle row

In the `## Lifecycle` table, replace the trigger cell of the `hold` row. The row must read exactly:

```
| `hold` | Blocked or paused | Operator sets manually when the block outlives the current week |
```

The phrase `blocked_by:` field populated must not survive anywhere in the file. Leave the other rows, including the `completed` row and the "Completed goals are immutable" sentence, unchanged.

## 7. Changelog

Read the top of `CHANGELOG.md` first. Prompt 1 of this spec creates the `## Unreleased` section — verify with `grep -n '^## ' CHANGELOG.md | head -3`; if it is somehow still absent, create it directly **below** the preamble block and **above** the newest `## vX.Y.Z` section. Append:

```
- docs: `task-writing.md` and `goal-writing.md` now describe `blocked_by` as a dependency list whose derived blocked state is orthogonal to `status` — the "`blocked_by:` field populated triggers `hold`" phrasing is removed, the resolution rules (case-insensitive, wikilink-aware, same-kind, unverifiable-blocker-counts-as-blocked, no transitive walk) are documented, and the JSON surface (`blocked_by` / `blocked`) plus the next-task behaviour are named.
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — the release agent owns those. Do NOT create a git tag.

</requirements>

<constraints>
- Documentation and changelog only: no Go code, no tests, no scenarios, no command templates, no version bumps.
- The two guide files are the only content files edited besides `CHANGELOG.md`.
- `blocked_by` is documented as a YAML list of strings (plain names or `[[wikilinks]]`); a scalar value is documented as malformed and inert.
- Both guides state that block state is derived and orthogonal to `status`, and that no automatic `hold` is applied — spec 046 Non-goals.
- Both guides state that a blocker whose file is missing, unreadable, or has no parseable status counts as not completed, and that resolution is same-kind only and never transitive.
- Neither guide may claim a behavior the code does not have: the JSON field names (`blocked_by`, `blocked`), the resolution rules and the next-task behaviour must match `pkg/ops/list.go`, `pkg/ops/blocked_by.go` and `commands/next-task.md` as they stand on this branch.
- Do not document `|alias` or `#section` wikilink forms — the resolver strips brackets only.
- Existing content that is still correct stays: the `assignee` semantics table, the `Hold vs in_progress` paragraph, the close-out fields paragraph, the required-sections lists, and every unrelated lifecycle row.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
</constraints>

<verification>
Run everything from the repository root.

**0. The sibling prompts landed** — this prompt documents code, it does not create it. Each command must print output; the second must print exactly `2`:

```
grep -n 'func IsBlocked' pkg/ops/blocked_by.go
grep -c 'json:"blocked' pkg/ops/list.go
grep -n 'unmet blocked_by' commands/next-task.md
```

If any of these fails, stop: prompt 1 or 2 of spec 046 has not landed on this branch, and writing the guides now would document behavior the code does not have.

**1. Nothing else broke:**

```
make precommit
```

Must exit 0. This prompt changes no code, so a failure here means something else in the tree broke — fix it and re-run.

**2. The old hold trigger is gone** — each command must print nothing and the shell must report success:

```
! grep -q 'blocked_by:` field populated' docs/task-writing.md
! grep -q 'blocked_by:` field populated' docs/goal-writing.md
```

**3. The new guidance landed** — each command must print at least one line:

```
grep -n '### Dependencies' docs/task-writing.md
grep -n '### Dependencies' docs/goal-writing.md
grep -n 'Operator sets manually when the block outlives the current week' docs/task-writing.md
grep -n 'Operator sets manually when the block outlives the current week' docs/goal-writing.md
grep -n 'blocked_by' docs/task-writing.md
grep -n 'blocked_by' docs/goal-writing.md
```

**4. The field appears in both frontmatter examples** — each command must print at least `3` (the example line, the subsection heading, and at least one prose mention):

```
grep -c 'blocked_by' docs/task-writing.md
grep -c 'blocked_by' docs/goal-writing.md
```

**5. The JSON surface and the next-task behaviour are documented** — each command must print a number `>= 1`. All four strings are absent from both guides today, so a non-zero count can only come from this change:

```
grep -c 'task list --output json' docs/task-writing.md
grep -c 'goal list --output json' docs/goal-writing.md
grep -c 'next-task' docs/task-writing.md
grep -c 'next-task' docs/goal-writing.md
```

**6. No hedging language was introduced** — each command must print nothing and the shell must report success:

```
! grep -qi 'not yet implemented' docs/task-writing.md
! grep -qi 'not yet implemented' docs/goal-writing.md
! grep -qi 'will be implemented' docs/task-writing.md
! grep -qi 'will be implemented' docs/goal-writing.md
```

**7. Changelog:**

```
grep -n '^## ' CHANGELOG.md | head -1
grep -c 'no transitive walk' CHANGELOG.md
```

The first must print `## Unreleased`; if it prints a version heading instead, the bullet was placed between released sections — move it into the existing `## Unreleased` section. The second must print a number `>= 1`.

**8. Self-check.** Before you finish, re-read both guide files end to end and confirm by inspection that no sentence anywhere still claims `blocked_by` triggers `hold`, that every documented resolution rule matches the code in `pkg/ops/blocked_by.go`, and that the documented JSON field names match `pkg/ops/list.go`. Then state in your final message which spec 046 Desired Behavior each requirement satisfies.
</verification>
