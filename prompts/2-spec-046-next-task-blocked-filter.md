---
spec: ["046-blocked-by-frontmatter-model"]
status: draft
created: "2026-09-11T07:53:52Z"
---

# next-task reads the typed blocked flag instead of content-scanning

<summary>
- The next-task command stops recommending work whose declared dependencies are unmet.
- It learns that from the typed `blocked` flag in `vault-cli task list --output json`, not from guessing at file contents.
- The old content-scanning heuristic — looking for `**Blocker:**`, `Blocked by:` and `Prerequisites` sections in task bodies — is deleted, so there is exactly one source of truth for blocked state.
- A blocked task is never named in the output and never recommended; the output reports how many were filtered instead.
- Tasks that are merely parked (`status: hold`) keep their current treatment and are still listed.
- Unblocked tasks are neither reordered nor hidden — the priority cascade behaves as before.
- Boss mode applies the same rule to the children it lists, for both task children and goal children.
- When every candidate is blocked, the command recommends nothing and names the dependency to resolve instead.
- No Go code changes — the command template is the only file touched.
- The changelog records the change.

</summary>

<objective>
Make `/vault-cli:next-task` refuse to recommend a task whose dependencies are unmet, by consuming the typed `blocked` flag that `vault-cli task list --output json` now emits, and by deleting the template's content-scanning blocker heuristic so the typed field is the single source of truth (spec 046 Desired Behavior 6, Acceptance Criterion 4). This is a plugin command-template change only — no Go code.
</objective>

<context>
This prompt depends on prompt 1 of spec 046 being already applied: `vault-cli task list --output json` and `vault-cli goal list --output json` now emit `blocked_by` (the raw dependency list) and a computed `blocked` boolean. `blocked` is present only when the entity has a `blocked_by` list; its absence means "no declared dependency". The flag is already true iff any named blocker is not `completed`.

Read `CLAUDE.md` first.

Then read `commands/next-task.md` in full — it is the only file you change. Verified current shape of the parts this prompt rewrites:

- Frontmatter lists `allowed-tools: Read, Bash, Edit, Grep, Glob, AskUserQuestion`. `Bash` is already there; do not add or remove entries.
- `### Step 4: Filter and group` ends with:
  ```
  Group:
  - **In Progress**: `status == in_progress` OR daily-note `[/]`
  - **Blocked**: `status == hold` OR file content contains blocker refs (see Step 5)
  - **Pending**: `status in (next, todo)` and not blocked
  ```
- `### Step 5: Detect blockers` is the heuristic being deleted. It reads:
  ```
  Scan each task's content for:
  - `**Blocker:** [[Task]]`
  - `Blocked by: [[Task]]`
  - `⚠️ Blocked by: [[Task]]`
  - `- [ ] [[Task]]` lines under a `Prerequisites` section

  For each blocker, resolve to a task file. If found and `status != completed`, it's an active blocker.
  ```
- `### Step 6: Pick recommended task` is a five-step priority cascade whose steps 4 and 5 read `Any unblocked pending → first by daily-note order` and `Only blocked tasks → first blocker to resolve`.
- `### Step 7: Present worker-mode output` renders an `In Progress (n)` / `Pending (n)` / `Blocked (n)` block, where the Blocked line is `⚠️ [[Task]] — blocked by [[Blocker]] (<status>)`.
- `### Step 12: Filter children` ends with `Group as in Step 4-5 (in-progress / blocked / pending).`
- `### Step 13: Present boss-mode output` repeats the same three groups, with `⚠️ <child> — blocked by ...` under Blocked.
- `## Output rules` (bottom of the file) already contains the convention `Hide deferred tasks but show count`.

Read these files as well:

- `pkg/ops/list.go` — confirm the emitted JSON field names (`blocked_by`, `blocked`) and that `blocked` is omitted when the entity has no `blocked_by`.
- `pkg/cli/cli.go` — the `createTaskListCommand` and `createGenericListCommand` flag sets, so the exact CLI invocations you write are real: `task list` accepts `--status`, `--all`, `--assignee`, `--goal`; `goal list` accepts `--status`, `--all`, `--assignee`; `--vault` and `--output` are persistent root flags.
- `specs/in-progress/046-blocked-by-frontmatter-model.md` — Desired Behavior 6 and Acceptance Criterion 4.

Read this coding-plugin doc (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/agent-command-development-guide.md` — slash-command template conventions.

<!-- OPEN QUESTIONS (resolved by the prompt writer; flagged for the human reviewer):

1. Desired Behavior 6 says the command "filters out tasks whose JSON `blocked` flag is true from its recommendations", while Acceptance Criterion 4's negative evidence is "`/vault-cli:next-task` output greps for the blocked task's name return 0 matches". Those two differ in scope: filtering only the *recommendation* would still let the task's name appear in a `Blocked (n)` listing, and the AC's grep would then return a match. The AC is the binary closure check, so the stronger reading is implemented: a task whose JSON `blocked` is true is not named anywhere in the output — the output reports a count instead (requirement 4).

2. A task with `status: hold` and NO `blocked_by` list is unaffected: Desired Behavior 6 also says the command "does not reorder or hide unblocked tasks", and a parked task with no declared dependency is unblocked by the spec's own definition. Such tasks keep their current named listing in the `Blocked (n)` group. Only `blocked: true` removes a task from the named output. A task that is both `hold` and `blocked: true` is removed (requirement 4).

3. Subtask checkbox lines parsed from a task's body (`Step 11` for a TASK argument) have no entity file and therefore no JSON `blocked` flag. Their handling is deliberately unchanged by this prompt.
-->

</context>

<requirements>

## 1. Replace `### Step 5: Detect blockers` with a JSON lookup

Delete the four content-scan patterns and the "resolve each blocker to a task file" paragraph. Replace the whole section with:

````markdown
### Step 5: Detect blocked tasks (JSON)

```bash
vault-cli --vault <name> task list --all --output json
```

Run once per vault in scope (the active vault first, then sibling vaults). Build a lookup from each item's `name` to its `blocked` and `blocked_by` fields.

- `"blocked": true` → the task is blocked; its `blocked_by` array names the unmet dependencies.
- `"blocked": false`, or the `blocked` key absent → the task is not blocked. The key is absent when the task declares no `blocked_by` list.
- A task that appears in no vault's output → treat it as not blocked (no declared dependency was found).

The JSON field is the single source of truth for blocked state. Never derive blocked state from task content — the body-marker heuristic this section replaced is gone and must not be reintroduced. Do not restate the removed patterns here: naming them invites a future reader to re-implement them.
````

Add one line to `## Runtime context` near the top of the file recording that blocked state comes from the JSON listing, so a future reader does not go looking for a content heuristic:

```
Blocked state comes from `vault-cli task list --all --output json` (`blocked` / `blocked_by`), never from task content.
```

## 2. Rewrite the grouping in `### Step 4: Filter and group`

Replace the three bullet lines quoted in `<context>` with:

```
- **In Progress**: `status == in_progress` OR daily-note `[/]`, and not filtered by Step 5
- **Blocked**: `status == hold` (parked — still listed by name)
- **Pending**: `status in (next, todo)` and not filtered by Step 5
```

Immediately after the bullets, add:

```
A task whose Step 5 `blocked` flag is true is removed from the candidate set entirely: it is not recommended, not listed by name, and not counted in any of the three groups above. Report it only as a count (Step 7).
```

Do not change the "Skip if `defer_date > today`", "Skip if `status == completed` or `status == aborted`" or "Accept any of" lines.

## 3. Update the priority cascade in `### Step 6: Pick recommended task`

- Leave steps 1-3 as they are; they operate on the candidate set left by Step 4, so blocked tasks are already gone.
- Rewrite step 4 to `Any pending → first by daily-note order`.
- Rewrite step 5 to `Every candidate was filtered as blocked (candidate set empty) → recommend nothing: state that every candidate is blocked and name the unmet dependency (from `blocked_by`) to resolve. Do not name the blocked task itself.`

## 4. Update the worker-mode output in `### Step 7: Present worker-mode output`

Replace the `Blocked (n):` block in the template with:

```markdown
Blocked (n):
⚠️ [[Task]] — status: hold, waiting
⛔ <N> hidden — unmet blocked_by (see `vault-cli task list --all --output json`)
```

Rules for the two lines:

- The `⚠️` line lists only tasks in the `Blocked` group of Step 4 — that is, `status: hold` tasks with no `blocked` flag. Omit the whole line when there are none.
- The `⛔` line carries the count of tasks filtered by Step 5 and no task names at all. Omit the line when the count is zero.
- No task filtered by Step 5 may appear by name anywhere in the output — not in a group, not in the rationale, not in a `Why:` line.

Keep the rest of the template (header, `In Progress`, `Pending`, `🎯 Recommended`, `Why:`) as it is.

## 5. Update boss mode (`### Step 12` and `### Step 13`)

In `### Step 12: Filter children`:

- Replace `Group as in Step 4-5 (in-progress / blocked / pending).` with `Group as in Step 4-5 (in-progress / blocked / pending); the Step 5 filter applies to task children and to goal children alike.`
- Add: `For goal children (a THEME argument), read blocked state from `vault-cli --vault <name> goal list --all --output json` using the same `blocked` / `blocked_by` fields. For task children, use the Step 5 task listing. Subtask checkbox lines parsed from a TASK's content have no JSON entity behind them and are listed unchanged.`

In `### Step 13: Present boss-mode output`, apply the same treatment as Step 7: the `Blocked (n):` block lists only `status: hold` children by name, plus a name-free `⛔ <N> hidden — unmet blocked_by` line for children filtered by Step 5. Never name a filtered child.

## 6. Update `## Output rules`

Add one bullet to the existing list:

```
- Never name a task or child whose `blocked` flag is true — report the count only. `status: hold` items without a `blocked_by` list keep their named listing.
```

Leave the other output rules untouched, including `Hide deferred tasks but show count` and `Max 5 items per group`.

## 7. Changelog

Read the top of `CHANGELOG.md` first. Prompt 1 of this spec creates the `## Unreleased` section — verify with `grep -n '^## ' CHANGELOG.md | head -3`; if it is somehow still absent, create it directly **below** the preamble block and **above** the newest `## vX.Y.Z` section, never between the `# Changelog` title and the preamble. Append:

```
- fix: `/vault-cli:next-task` no longer recommends tasks whose dependencies are unmet — it reads the typed `blocked` flag from `vault-cli task list --output json` instead of content-scanning for `**Blocker:**` / `Blocked by:` / `Prerequisites` patterns, and reports filtered tasks as a count rather than by name.
```

Do NOT bump or hand-edit any version string in `CHANGELOG.md`, `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — the release agent owns those. Do NOT create a git tag.

</requirements>

<constraints>
- The command template is the only production file changed. No Go code, no tests, no scenarios, no new vault-cli subcommand or flag.
- `commands/next-task.md` frontmatter (`description`, `allowed-tools`, `argument-hint`) stays as it is — `Bash` is already in `allowed-tools`.
- The typed JSON field is the single source of truth for blocked state: no content-scanning for `**Blocker:**`, `Blocked by:`, `⚠️ Blocked by:`, or `Prerequisites` remains anywhere in the file.
- Block state does not reorder or hide unblocked tasks: the priority cascade, the defer-date skip and the completed/aborted skip are unchanged.
- `status: hold` is not derived from `blocked_by` and no command may set `status` from a dependency — block state stays derived and orthogonal to status (spec 046 Non-goals).
- Same-kind resolution only: a task's blockers resolve in the tasks directory, a goal's in the goals directory.
- Worker mode and boss mode both honor the filter; neither gains a new mode, flag, or argument.
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
</constraints>

<verification>
Run everything from the repository root.

**1. No regression elsewhere:**

```
make precommit
```

Must exit 0. This prompt changes no Go code, so a failure here means something else in the tree broke — fix it and re-run.

**2. The content heuristic is gone** — each command must print nothing and the shell must report success:

```
! grep -qi 'prerequisites' commands/next-task.md
! grep -qi 'blocked by:' commands/next-task.md
! grep -q '\*\*Blocker:\*\*' commands/next-task.md
! grep -qi 'file content contains blocker refs' commands/next-task.md
! grep -qi 'active blocker' commands/next-task.md
! grep -q 'Scan each task' commands/next-task.md
```

**3. The JSON lookup landed** — each command must print at least one line:

```
grep -n 'task list --all --output json' commands/next-task.md
grep -n 'blocked_by' commands/next-task.md
grep -n 'goal list --all --output json' commands/next-task.md
grep -n 'unmet blocked_by' commands/next-task.md
grep -n 'single source of truth' commands/next-task.md
```

**4. The output never names a filtered task** — this must print `>= 1` (it matches the rule sentence) and the file must contain no per-task blocked line of the old shape:

```
grep -c 'filtered by Step 5' commands/next-task.md
! grep -q 'blocked by \[\[Blocker\]\]' commands/next-task.md
```

**5. The file is still a valid command template** — the frontmatter must be intact:

```
head -12 commands/next-task.md
grep -c 'allowed-tools' commands/next-task.md
```

The first must show the `description`, `allowed-tools` and `argument-hint` keys unchanged; the second must print exactly `1`.

**6. Changelog:**

```
grep -n '^## ' CHANGELOG.md | head -1
grep -c 'dependencies are unmet' CHANGELOG.md
```

The first must print `## Unreleased`; if it prints a version heading instead, the bullet was placed between released sections — move it into the existing `## Unreleased` section. The second must print a number `>= 1`.

**7. Self-check.** Before you finish, re-read `commands/next-task.md` end to end and confirm by inspection that (a) no sentence anywhere still describes content-scanning for blockers, (b) every blocked-state statement points at the JSON `blocked` flag, and (c) no output template names a task whose `blocked` flag is true. Then state in your final message which spec 046 Desired Behavior and Acceptance Criterion each requirement satisfies. Note that Acceptance Criterion 4's runtime evidence (`/vault-cli:next-task` invoked in a live session) is operator-side and is not reproducible inside this container — the greps above are the container-executable proof that the template no longer contains a path that would name a blocked task.
</verification>
