---
spec: ["061-goal-topic-approve"]
status: draft
created: "2026-10-20T10:03:18Z"
---

# Teach the goal and topic commands the approve verb (spec 061, prompt 5 of 5)

<summary>
- The goal planning command now approves a goal in the inbox instead of setting its phase directly.
- The topic execution command now approves a topic in the inbox instead of setting its phase directly.
- The reopen paths for closed or done rows keep their existing phase-set command, which is still legal outside the inbox.
- The goal writing guide documents the approval transition, the recorded approver and instant, and why the generic set is refused.
- The changelog carries one unreleased entry describing the new verbs and the closed bypass.
- No plugin version is hand-bumped; the release tail owns that.
</summary>

<objective>
Update the two command docs that currently teach the generic-set bypass, and document the approval transition where goal authors read it, so an agent following the docs reaches for `goal approve` / `topic approve` instead of a command that is now refused. Closes acceptance criterion 5 of spec 061.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully, in this order:
- `commands/plan-goal.md` — step 3 ("Entry contract — flip if needed"), lines 44-50. The two `todo` arms currently instruct `vault-cli goal set "<name>" phase planning`, which prompt 1 now refuses.
- `commands/execute-topic.md` — step 3 ("Refusal cases") and step 6. The `phase: todo` arm at line 56 currently instructs `vault-cli topic set "<name>" phase planning`, which is now refused. The two `done`-reopen arms (lines 54-55) keep their `set … phase planning` command: those rows are not in the inbox, so the guard does not fire there.
- `docs/goal-writing.md` — § Frontmatter (the `status` valid-values paragraph) and § Lifecycle. There is no `phase` section yet; add one.
- `docs/task-writing.md` § Phase transitions — the contract this documentation mirrors. Reference it rather than restating the whole rule.
- `CHANGELOG.md` — the preamble and the newest section (`## v0.174.1`). There is currently no `## Unreleased` section.

Coding-plugin docs (in-container path):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` placement and the `feat:` prefix.
</context>

<requirements>
1. **`commands/plan-goal.md` step 3.** Replace the two `todo` arms so the phase flip is the approval verb, and leave the deliberate-skip arms alone. Target wording:

   ```
   - `status` in `next`/`todo`/`backlog` → `vault-cli goal set "<name>" status in_progress`, then the phase entry:
     - `phase: todo` → `vault-cli goal approve "<name>"` — the approval records `approved_by` and `approved_at` and flips to `planning` in one write; `vault-cli goal set "<name>" phase planning` is refused on a `todo` row.
     - `phase` empty → `vault-cli goal set "<name>" phase todo`, then `vault-cli goal approve "<name>"` — a goal with no phase is exempt from the gate and cannot be approved until it is in the inbox.
     - `phase: planning` → no flip.
     - `phase` is `execution` / `done` → skip the flip (treat as deliberate — sharpen but don't move phase backward).
   - `status` already `in_progress` and `phase` is `todo` → `vault-cli goal approve "<name>"`; `phase` empty → `vault-cli goal set "<name>" phase todo`, then `vault-cli goal approve "<name>"`.
   - `status` already `in_progress` and `phase` is past planning → continue without flip; step 7 will skip the phase transition.
   ```

   Keep the rest of the command unchanged, including the "plan-goal never flips the phase" rule for `planning → execution`.

2. **`commands/execute-topic.md` step 3.** Change the `phase: todo` / empty-phase arm to point at the approve verb:

   ```
   - `phase: todo` OR `phase` empty AND `status: in_progress` → `❌ Topic has no plan yet (phase: todo). Approve it with vault-cli topic approve "<name>" once the topic passes /verify-topic.` (planning is non-skippable; a topic with no `phase` key is exempt from the gate, so set `phase: todo` first with `vault-cli topic set "<name>" phase todo`)
   ```

   Leave the two `done`-reopen arms (lines 54-55) naming `vault-cli topic set "<name>" phase planning` — those rows are past the inbox and the set remains legal. Add a short parenthetical to one of them: `(the phase set is legal here because the topic is not in the inbox)`. Leave the step 6 output list and the `planning → execution` gate unchanged.

3. **`docs/goal-writing.md`.** Add a `### Phase and approval` subsection under § Frontmatter, immediately after the paragraph that lists the valid `status` values. It must state:
   - `phase` walks `todo → planning → execution → done`, and `todo` is the operator's approval inbox.
   - `vault-cli goal approve "<name>"` performs `todo → planning` and records `approved_by` (default `operator`; `--by <name>` names another) and `approved_at` in the same write.
   - `vault-cli goal set "<name>" phase planning` is refused on a `todo` goal; the refusal points at `goal approve`. This is the same contract as `task approve` in `task-writing.md` § Phase transitions.
   - A goal with no `phase` key is exempt from the gate and cannot be approved; set `phase: todo` first.
   - A goal that already carries `approved_by` or `approved_at` is refused rather than re-approved.

   Do not add a new document — topics have no writing guide (spec Non-goal).

4. **`CHANGELOG.md`.** Insert one `## Unreleased` section below the preamble and above `## v0.174.1`, with one `feat:` bullet naming both verbs, the recorded fields, and the closed bypass:

   ```
   ## Unreleased

   - feat: Add `vault-cli goal approve` and `vault-cli topic approve`, which move a row from the approval inbox (`phase: todo`) to `phase: planning` and record `approved_by` / `approved_at` in the same write; `goal set … phase planning` and `topic set … phase planning` on a `todo` row are now refused and point at the approve verb.
   ```

   Do not touch the preamble or any released section.

5. **Self-check before finishing.** Re-run the `<verification>` block and confirm every row; walk acceptance criterion 5 of spec 061 against the change.
</requirements>

<constraints>
- Only `commands/plan-goal.md`, `commands/execute-topic.md`, `docs/goal-writing.md`, and `CHANGELOG.md` change. Do not touch `pkg/`, `integration/`, or any other command doc.
- The `done`-reopen arms must keep naming `vault-cli topic set "<name>" phase planning` — the guard fires only on `todo → planning`, so those arms are correct as written and must not be "fixed" into `topic approve` (which would refuse them).
- A `commands/` change requires a plugin version bump — do not hand-bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`; the release tail owns that bump.
- The approval contract being mirrored is `docs/task-writing.md` § Phase transitions; reference it instead of restating it.
- No backfilling of `phase` on existing goals or topics; no owner assignment at approval time; no `--force`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
EVIDENCE GREPS — run each and confirm the count. Absence assertions use `! grep -q` because `grep -c` exits 1 when the count is 0.

```
grep -c 'goal approve' commands/plan-goal.md                       # >= 1 (0 at HEAD)
grep -c 'topic approve' commands/execute-topic.md                  # >= 1 (0 at HEAD)
grep -c 'goal approve' docs/goal-writing.md                        # >= 1 (0 at HEAD) — acceptance criterion 5
grep -c '^## Unreleased' CHANGELOG.md                              # 1 (0 at HEAD)
awk '/^## /{sec=$0} /goal approve/{print sec}' CHANGELOG.md        # must print exactly: ## Unreleased
grep -c 'Reopen: vault-cli topic set' commands/execute-topic.md   # >= 1 — the done-reopen arms survive the edit
```

⚠️ The `awk` row is the authoritative placement check: `grep -c 'goal approve' CHANGELOG.md` would pass with the bullet under a released section, which is the fold this repo's changelog rules guard against.

TESTS:

```
make check-changelog
make precommit
```

`make check-changelog` must print `CHANGELOG structure OK`. `make precommit` must exit 0 apart from the named pre-existing `topic defer` integration failure; this prompt changes no Go code, so any other failure is yours to fix.

SYNTAX:

```
grep -n '^## ' CHANGELOG.md | head -5     # first section is ## Unreleased, above ## v0.174.1
```
</verification>
