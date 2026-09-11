---
spec: ["046-blocked-by-frontmatter-model"]
status: draft
created: "2026-09-11T08:20:00Z"
---

# work-on-goal-assistant reads the typed blocked flag instead of content-scanning

<summary>
- The work-on-goal-assistant agent stops guessing blocked state by scanning task file contents.
- It reads the typed `blocked` flag from the `vault-cli task list --goal … --all --output json` call it already makes — no extra call, no extra read.
- The Blocked group becomes `status == hold` OR the typed `blocked` flag: one source of truth, matching `next-task`.
- The recommendation cascade keeps its shape; only the source of the blocked signal changes.
- The Blocked output line names the blocker from the task's `blocked_by` list.
- No Go code changes — the agent definition is the only file touched.
- The changelog records the change.
</summary>

<objective>
Make `agents/work-on-goal-assistant.md` derive blocked state from the typed `blocked` flag that `vault-cli task list --output json` now emits, and delete the agent's content-scanning blocker heuristic, so every recommendation path in the plugin reads blocked state from the same typed source (spec 046 Desired Behavior 8, Acceptance Criterion 7). This is a plugin agent-definition change only — no Go code.
</objective>

<context>
This prompt depends on prompt 1 of spec 046 being already applied: `vault-cli task list --output json` emits `blocked_by` (the raw dependency list) and a computed `blocked` boolean. `blocked` is present only when the entity has a `blocked_by` list; its absence means "no declared dependency". The flag is already true iff any named blocker is not `completed`.

Read `CLAUDE.md` first.

Then read `agents/work-on-goal-assistant.md` in full — it is the only file you change. Verified current shape of the parts this prompt rewrites:

- `## Phase 3: Analyze task progress` lists, under `For each task ref:`, the bullet `- Scan content for blocker patterns (`Blocker:`, `Blocked by:`, `⚠️ Blocked by:`)` — the heuristic being deleted.
- The same phase fetches every child task's status in one call:
  ```bash
  vault-cli task list --goal "[[{goal_name}]]" --all --output json
  ```
  and its prose says the returned values drive "the defer filter, the grouping, the progress line, and the recommendation". The typed `blocked` field arrives on exactly this response.
- The `Group:` list defines `- **Blocked**: `status == hold` OR any active blocker`.
- `## Phase 5: Recommendation logic` is a three-step cascade; step 2 already reads `Else if any unblocked pending → …` and step 3 reads `Else if only blocked tasks remain → recommend first blocker to resolve`.
- The `<output_format>` block renders `Blocked (n):` followed by `○ <task> — blocked by [[<blocker>]] (<status>)`.
- `grep -n -i 'block' agents/work-on-goal-assistant.md` returns exactly these hits plus the terminal-status line at 27 and the `not_found` prose at 81 — neither mentions blockers and neither changes.

Read these files as well:

- `commands/next-task.md` as prompt 2 of spec 046 left it — the sibling implementation of the same single-source-of-truth rule; match its vocabulary ("the typed `blocked` flag", "unmet dependency") so the two commands read alike.
- `pkg/ops/list.go` — confirm the emitted JSON field names (`blocked_by`, `blocked`) and that `blocked` is omitted when the entity has no `blocked_by`.
</context>

<requirements>

## 1. Delete the content-scan bullet in Phase 3

Remove this line entirely from the `For each task ref:` list:

```
- Scan content for blocker patterns (`Blocker:`, `Blocked by:`, `⚠️ Blocked by:`)
```

Do not replace it with a different scan and do not restate the removed patterns anywhere in the file — a restated pattern would make the verification greps below fail on your own prose.

## 2. Group from the typed flag

Replace the Group list's Blocked entry so the grouping reads:

```
- **Blocked**: `status == hold` OR `blocked == true`
```

Leave the In Progress, Pending and Completed entries byte-identical. Add one sentence directly below the list, in the existing prose style:

```
The `blocked` flag comes from the same `task list --goal … --output json` call that supplies `status` — a task with no `blocked_by` list has no `blocked` key and is never blocked by this rule. `hold` remains an operator decision and is independent of the derived flag.
```

## 3. Recommendation cascade — reference the typed flag

Leave steps 1 and 2 unchanged. Rewrite step 3 so it names the source rather than an undefined "blocker":

```
3. Else if only blocked tasks remain → recommend nothing actionable; name the unmet dependency (from the task's `blocked_by` list) and the task that declares it, so the operator knows what to resolve. Do not recommend the blocked task itself.
```

## 4. Output format — name the blocker from `blocked_by`

In the `<output_format>` block, keep the `Blocked (n):` header and change the entry template to name the blocker from the typed list:

```
○ <task> — blocked by [[<blocker>]] (<status>)
```

`<blocker>` is the first entry of the task's `blocked_by` list (strip the `[[` `]]` brackets for display, as the rest of the file already does for task names), and `<status>` is that blocker's `status` from the same listing call. When the task is blocked by `status == hold` rather than by a dependency, render the existing `○ <task> — on hold` form instead — do not invent a blocker name.

## 5. No other behavior changes

Do not touch the frontmatter (`description`, `tools`), the `not_found` verdict block, the session-handoff prose, or any phase this prompt does not name. Do not add a new tool, a new CLI call, a new flag, or a new argument — the data is already in hand.

## 6. Changelog

Read the top of `CHANGELOG.md` first. The `## Unreleased` section already exists (created by prompt 1 of spec 046). Append one bullet describing the change:

```
- feat: `work-on-goal-assistant` groups and recommends from the typed `blocked` flag emitted by `vault-cli task list --output json` instead of scanning task file contents for blocker patterns — every plugin recommendation path now reads blocked state from one typed source.
```

Do not modify or reorder the existing bullets and do not create a second `## Unreleased` section.

## 7. Self-check

Before finishing, re-run every command in `<verification>` and confirm each passes; then walk each numbered requirement above against the change and confirm each is satisfied. State in your final message which spec 046 Desired Behavior and Acceptance Criterion each requirement satisfies.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Do NOT bump any version string and do NOT create a git tag.
- No Go code changes: `pkg/`, `commands/` and `integration/` are out of scope for this prompt.
- Do not widen scope to other agents or commands — `commands/next-task.md` is handled by prompt 2 of this spec and is read-only here.
- Do not restate the deleted content-scan patterns (`Blocker:`, `Blocked by:`) anywhere in the file, including in comments or explanatory prose.
- No new CLI invocation, flag, argument, config key, or tool entry in the agent frontmatter.
- Preserve the file's existing structure, heading names and prose register — this is a surgical edit, not a rewrite.
</constraints>

<verification>
Run everything from the repository root.

**0. The sibling prompts landed** — this prompt consumes the typed flag, it does not create it. Both commands must print output; the second must print at least `1`:

```
grep -n 'json:"blocked' pkg/ops/list.go
grep -n 'unmet blocked_by' commands/next-task.md
```

If either fails, stop: prompt 1 or 2 of spec 046 has not landed on this branch, and the typed flag this prompt reads does not exist yet.

**1. Nothing else broke:**

```
make precommit
```

Must exit 0.

**2. The content-scan is gone** — each must print nothing:

```
grep -n 'Blocker:' agents/work-on-goal-assistant.md
grep -n 'Blocked by:' agents/work-on-goal-assistant.md
grep -n 'Scan content for blocker patterns' agents/work-on-goal-assistant.md
```

**3. The typed flag is referenced:**

```
grep -n 'blocked == true' agents/work-on-goal-assistant.md
grep -c 'blocked' agents/work-on-goal-assistant.md
```

The first must print a line; the second must print a number `>= 3` (the grouping rule, the prose note and the cascade step).

**4. The rest of the file is intact:**

```
grep -n 'Phase 5: Recommendation logic' agents/work-on-goal-assistant.md
grep -n 'not_found:' agents/work-on-goal-assistant.md
```

Both must print a line — the cascade and the not-found verdict survived the edit.

**5. Changelog:**

```
grep -n '^## ' CHANGELOG.md | head -1
grep -c 'work-on-goal-assistant' CHANGELOG.md
```

The first must print `## Unreleased`; if it prints a version heading instead, the bullet was placed between released sections — move it into the existing `## Unreleased` section. The second must print a number `>= 1`.
</verification>
