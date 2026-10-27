---
status: approved
spec: [064-bug-completed-goal-keeps-non-terminal-phase]
created: "2026-10-28T00:12:00Z"
queued: "2026-10-27T23:19:44Z"
branch: dark-factory/bug-completed-goal-keeps-non-terminal-phase
---

# Document the goal terminal phase and record the fix

<summary>
- `docs/goal-writing.md` § Phase and approval states that a terminal goal status carries `phase: done`, and that the CLI writes it in the same mutation.
- The same section names `aborted` alongside `completed`, so a reader does not infer that only completion sets the phase.
- The same section states that a non-terminal status change leaves the phase untouched, so a reader does not expect a reopen to move it.
- `CHANGELOG.md` carries a `fix:` bullet describing the write-path repair.
- The change is documentation only: no Go file, test, lint rule, or version string moves.
</summary>

<objective>
After this prompt, a reader of `docs/goal-writing.md` § Phase and approval knows that a terminal goal status carries `phase: done`, that the CLI writes it in the same mutation, and that a non-terminal status change does not touch it; `CHANGELOG.md` records the repair. This is spec 064 prompt 3 of 3: it covers Desired Behavior 4 and Acceptance Criterion 11.
</objective>

<context>
Read `CLAUDE.md` first, then `docs/dod.md`.

Read these before changing anything:

- `docs/goal-writing.md` **`### Phase and approval`** (heading at line 153, body from line 155). It currently states `phase` walks `todo → planning → execution → done` and describes `goal approve` / the `todo` approval inbox. It does **not** state that a terminal status carries `phase: done`, so the delta is real. ⚠️ The section is named **Phase and approval**, not "Phase transitions" — `### Phase transitions` lives in `docs/task-writing.md`, and this page cross-references it on line 155. Do not add a new heading.
- `CHANGELOG.md` — `## Unreleased` exists at HEAD (line 11). Add the bullet there. If a future branch has no `## Unreleased`, create it directly above the newest `## vX.X.X` heading; never add a bullet to a released `## vX.X.X` section. Structure rules: `# Changelog` → preamble → `## Unreleased` → `## vX.X.X` (newest first). Bullets use a conventional prefix (`feat:`, `fix:`, `docs:`, `chore:`).
- `specs/in-progress/064-bug-completed-goal-keeps-non-terminal-phase.md` — Desired Behavior 4 and Acceptance Criterion 11.
</context>

<requirements>
1. **`docs/goal-writing.md` § Phase and approval.** Add prose stating all three of these, in the same voice as the surrounding text:

   - a **terminal** status — `completed` **or** `aborted` — carries `phase: done`;
   - `vault-cli goal complete` and `vault-cli goal set <name> status completed|aborted` write the phase in the same mutation as the status;
   - a **non-terminal** status change leaves the phase untouched.

   Use these two phrases verbatim so the verification below is deterministic: **`a terminal status carries phase: done`** and **`a non-terminal status change leaves the phase untouched`**. Do not add a new heading; do not restructure or reorder the section.

2. **`CHANGELOG.md`.** Add exactly this bullet under `## Unreleased`:

   `- fix: a terminal goal status now lands phase: done in the same write as the status, and goal lint accepts phase: done on an aborted goal.`

3. **Self-check.** Re-run `<verification>`; walk Acceptance Criterion 11 against the change.
</requirements>

<constraints>
- Documentation only. Do not change any Go file, any test, or the lint rule.
- Do not restructure `docs/goal-writing.md` or reorder its sections — add to `### Phase and approval` only, under its existing heading.
- Do not bump a version, do not create a tag, do not rename `## Unreleased` — the releaser owns those.
- If prompts 1 and 2 have not merged when this lands, the documented behaviour is the intended contract — verify their files are in `prompts/completed/` before finishing, and if they are not, say so in the report rather than weakening the wording to match current behaviour.
- Do NOT commit — dark-factory handles git.
</constraints>

<verification>
make precommit
make check-changelog

# both pinned phrases landed in the goal doc (neither exists before the change)
grep -qF 'a terminal status carries phase: done' docs/goal-writing.md
grep -qF 'a non-terminal status change leaves the phase untouched' docs/goal-writing.md

# the exact bullet landed under ## Unreleased
grep -qF -- '- fix: a terminal goal status now lands phase: done in the same write as the status' CHANGELOG.md
</verification>

<!-- DARK-FACTORY-REPORT -->
