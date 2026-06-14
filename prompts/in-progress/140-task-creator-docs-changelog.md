---
status: approved
spec: [017-enforce-status-in-progress-on-calendar-date]
created: "2026-06-14T14:30:00Z"
queued: "2026-06-14T15:39:32Z"
branch: dark-factory/enforce-status-in-progress-on-calendar-date
---

<summary>
- `task-creator` agent flips its `status:` default: `status: in_progress` when any date field (`planned_date`, `defer_date`, `due_date`) is being written, `status: next` otherwise
- No new frontmatter fields are introduced — the existing `status:` line is the only thing that changes based on date-field presence
- `docs/task-writing.md` Lifecycle section is amended with a "Calendar-as-commitment rule" paragraph naming the invariant and the auto-fix direction (promote status, never strip date)
- `CHANGELOG.md` `## Unreleased` section gets a `feat:` bullet naming the new rule and the three enforcement points (agent emission, defer auto-promote, lint detect+fix)
- No Go code changes in this prompt — three markdown edits plus a changelog line
- Prompts 1 and 2 must be completed first; the rule is live before the docs claim it

</summary>

<objective>
Update the `task-creator` agent to emit `status: in_progress` (not `status: todo`) when any of `planned_date` / `defer_date` / `due_date` is being written to the new task. Update `docs/task-writing.md` to document the rule. Add a `CHANGELOG.md` `## Unreleased` entry.
</objective>

<context>
Read CLAUDE.md for project conventions. Per `docs/releasing-vault-cli.md`, the version-alignment rule is NOT in scope here — no `.claude-plugin/*.json` files change. Only `agents/`, `docs/`, and `CHANGELOG.md` change. Plugin JSON bumps happen at release time, not on this branch.

Read these files in full before making changes:

- `/workspace/agents/task-creator.md` — the agent being modified. Specifically:
  - **Lines 110-122**: `## 8. Compose frontmatter` section. The current line 114 is:
    ```
    - `status: todo` (interactive default; tool mode may override via flag)
    ```
    This is the rule to flip. The new rule:
    - `status: in_progress` IF any of `planned_date`, `defer_date`, or `due_date` is being written (i.e. present in the `## 8. Compose frontmatter` output)
    - `status: next` OTHERWISE (canonical replacement for the legacy `todo` alias; matches the existing `task-creator.md` "todo (alias for next)" note in the spec's NormalizeTaskStatus surface)
  - **Line 119**: `- `planned_date: <today>` — only in interactive mode if the user asked to start now` — this is the trigger for `in_progress` in interactive mode when the user says "start now"
  - **Lines 63-80**: Step 4 (incident-shaped task) — interactive only, sets a `SEVERITY` frontmatter. OUT OF SCOPE for this prompt — do NOT change Step 4. The status rule fires only on date-field presence per spec; severity is unrelated.
  - **Lines 112-122**: Step 8 has a "Do NOT set `assignee`" rule — preserve it. The new status rule is the only addition in this step.

- `/workspace/docs/task-writing.md` — the canonical task-structure doc. Specifically:
  - **Lines 172-183**: `## Lifecycle` table. The current `todo` row says "Task file created with required sections filled" — amend this to note the new rule. Or — preferred — add a new subsection after the table (between the table and the "Recurring tasks reset" paragraph at line 182) that names the calendar-as-commitment rule and references the auto-fix direction (promote status, never strip date).

- `/workspace/CHANGELOG.md` — the existing top entry is `## v0.77.0` (line 11). The `## Unreleased` section is missing (no `## Unreleased` line in the file — verify by `grep -nE '^## Unreleased' CHANGELOG.md` and confirm 0 matches before editing). The new `## Unreleased` section must be inserted IMMEDIATELY BEFORE the `## v0.77.0` heading.

- CHANGELOG bullet format follows the convention used in existing entries: `- feat: <one-liner>` for new user-visible capability (minor bump). The rule is user-visible (the agent emits the new status, the lint surfaces violations), so `feat:` is the correct prefix.

**Prompt ordering**: Prompts 1 and 2 must be completed first. The rule is live in the runtime (lint detects+fixes, defer auto-promotes) before the docs claim it. If either prompt is incomplete, STOP and run it first.
</context>

<requirements>

### File 1: `/workspace/agents/task-creator.md`

1. **Replace the `status: todo` line in Step 8 (line 114) with the new conditional rule.** The current line is:
   ```
   - `status: todo` (interactive default; tool mode may override via flag)
   ```
   Replace with:
   ```
   - `status: in_progress` IF any of `planned_date`, `defer_date`, or `due_date` is being written to this task in step 8 (per spec 017: a calendar date is a commitment, so the task must be visible to the Kanban board); `status: next` OTHERWISE (canonical replacement for the legacy `todo` alias)
   ```
   - Verify with: `grep -n 'status: in_progress.*IF.*planned_date' agents/task-creator.md` — must return 1 line.
   - Verify with: `grep -nE '^- ``status: todo``' agents/task-creator.md` — must return 0 lines (the Step 8 bullet `- ``status: todo`` ...` is gone). The Step 13 success-message echo (`Status: todo  Priority: ...` at line 177) is intentionally OUT OF SCOPE for this prompt — that's a separate follow-up task.
   - The new rule must NOT introduce new frontmatter fields — only flip the `status:` value based on date-field presence in the same step.
   - The new rule must NOT remove or change other bullets in step 8 (`priority`, `themes`/`goals`, `category`, `severity`, `planned_date`, `task_identifier`, `assignee`).

### File 2: `/workspace/docs/task-writing.md`

3. **Add a new subsection to the `## Lifecycle` section (after the table at line 180, before the "Recurring tasks reset" paragraph at line 182).** Use a level-3 heading `### Calendar-as-commitment rule`. The subsection must contain:

   - A one-sentence statement of the invariant: any task with a calendar date (`planned_date`, `defer_date`, or `due_date`) is a commitment, and its status must be `in_progress` (or terminal — `completed` / `aborted`).
   - The auto-fix direction: `vault-cli task lint --fix` promotes `next` / `backlog` to `in_progress` and leaves the date field byte-identical. The date is never stripped.
   - The runtime enforcement points: file creation (the `task-creator` agent), date assignment (`task defer` command), and audit (`task lint` / `task validate`).
   - Reference the spec for canonical authority: `(spec 017)`.

   The subsection body (paste verbatim, adjust the prose to match the surrounding doc's voice if needed but keep the four points above):

   ```markdown
   ### Calendar-as-commitment rule

   Any task with a calendar date (`planned_date`, `defer_date`, or `due_date`) is a commitment, so its status must be `in_progress` (or terminal — `completed` / `aborted`). The rule is enforced at three points: file creation (`task-creator` agent emits `in_progress` when any date field is set), date assignment (`task defer` auto-promotes `next` / `backlog` to `in_progress` in the same write), and audit (`task lint` and `task validate` both surface `STATUS_DATE_MISMATCH`). `task lint --fix` promotes the status; the date is never stripped. Terminal status takes precedence — a `completed` task with a stale `defer_date` is out of scope. See spec 017.
   ```

   - Verify with: `grep -n 'Calendar-as-commitment rule' docs/task-writing.md` — must return 1 line (the new heading).
   - Verify with: `grep -i 'calendar' docs/task-writing.md` — must return ≥1 line.
   - Verify with: `grep -i 'in_progress' docs/task-writing.md` — must return ≥1 line in the new subsection context (existing references elsewhere in the doc are fine).

4. **Verify the existing `todo` row in the Lifecycle table (line 175) still describes the historical alias correctly.** No change required — the row reads `| `todo` | Defined, not started | Task file created with required sections filled |`. The new rule is documented in the new subsection; the table row's meaning ("not started") still applies to the `todo` alias, which is now the legacy name for `next`. If desired (NOT required), add a parenthetical `(alias for next)` after `todo` in the row, but the canonical replacement is already documented in step 1 of the agent and in the new subsection.

### File 3: `/workspace/CHANGELOG.md`

5. **Insert a new `## Unreleased` section immediately before the existing `## v0.77.0` heading (line 11).** The new section must contain a single `feat:` bullet (per `changelog-guide.md`, the `feat:` prefix is for a new user-visible capability, which this rule is — minor bump). The bullet format is:

   ```
   - feat: Enforce calendar-as-commitment rule on task status — tasks with any of `planned_date`, `defer_date`, or `due_date` must have `status: in_progress` (or terminal). Enforced at file creation (`task-creator` agent emits `in_progress` when a date field is set), at date assignment (`task defer` auto-promotes `next`/`backlog` to `in_progress` in the same write), and at audit (`task lint` reports `STATUS_DATE_MISMATCH`; `task lint --fix` promotes status, never strips the date). Lint and validate share a single detector.
   ```

   - Verify with: `grep -nE '^## Unreleased' CHANGELOG.md` — must return 1 line, and that line must be ABOVE the `## v0.77.0` heading.
   - Verify with: `grep -nA 5 '^## Unreleased' CHANGELOG.md` — the 5 lines after the heading must include the `feat:` bullet naming "calendar" and "in_progress" and "STATUS_DATE_MISMATCH".
   - The frozen preamble (lines 1-9: `# Changelog` → ... → MAJOR/MINOR/PATCH bullets) must NOT be modified, moved, or have anything inserted above or inside it. Insert `## Unreleased` immediately AFTER the last preamble line and BEFORE the `## v0.77.0` heading.
   - The bullet must use the `feat:` prefix (per `changelog-guide.md`).

### Verification gates

6. **Run the static checks below** — each must pass:

   ```bash
   # 1. Step 8 in task-creator.md has the new conditional rule
   grep -n 'status: in_progress.*IF.*planned_date' agents/task-creator.md
   # Expected: 1 line

   # 2. The literal "status: todo" no longer appears in step 8
   #    (Step 13 success-message echo "Status: todo" is intentionally out of scope — separate follow-up task)
   grep -nE '^- `status: todo`' agents/task-creator.md
   # Expected: 0 lines (Step 8 bullet flipped; the Step 13 prose example "Status: todo  Priority: {N}" is NOT this grep's target)

   # 3. The new Lifecycle subsection heading exists
   grep -n 'Calendar-as-commitment rule' docs/task-writing.md
   # Expected: 1 line

   # 4. CHANGELOG.md has a new ## Unreleased section above ## v0.77.0
   grep -nE '^## Unreleased|^## v0.77.0' CHANGELOG.md
   # Expected: ## Unreleased appears BEFORE ## v0.77.0

   # 5. The new bullet names the rule
   grep -nA 3 '^## Unreleased' CHANGELOG.md
   # Expected: at least one line containing "calendar", "in_progress", AND "STATUS_DATE_MISMATCH"
   ```

7. **No Go files were modified.** Verify with `git diff --name-only` (against the prior commit on this branch) — only `agents/task-creator.md`, `docs/task-writing.md`, and `CHANGELOG.md` are in the diff.

8. **Do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`.** Per `docs/releasing-vault-cli.md`, the version-alignment rule is release-time only, not precommit. The plugin JSONs may lag the binary CHANGELOG entry on the feature branch; the operator bumps them at release time. Do not introduce a plugin version bump from a markdown-only change.

9. **`make precommit` must exit 0.** The change touches only markdown, but `make precommit` runs the full pipeline (format, generate, test, check, addlicense). The `addlicense` target may flag the new `## Unreleased` content or the markdown edits — if it does, address per the existing addlicense pattern. A non-zero exit is a failure — do not label it "preexisting" or "noise"; fix the failure or surface it.

</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — version-alignment is release-time only.
- Existing tests must continue to pass (no Go code changed in this prompt).
- No new MCP, no new tool, no new dark-factory primitive — the change is three `.md` edits plus a CHANGELOG line.
- The agent's Phases 1-7, 9-13 (parse arguments, resolve vault config, detect Jira key, detect incident shape, compose title, determine category/priority, resolve template body, compose body, check collision, write file, audit, return) MUST NOT have their text changed. Step 8 (compose frontmatter) is the only step with a rule change.
- The new status rule in step 8 is conditional on date-field presence in the SAME step 8 output — i.e. the agent evaluates at emit time, not from a user-stated intent. If the user said "no date", the agent emits `next`; if the user said "start now" (which triggers `planned_date: <today>` in step 8 line 119), the agent emits `in_progress`.
- The new Lifecycle subsection in `docs/task-writing.md` is purely documentation — it does NOT add new audit rules, new lint rules, or new validation logic. The rule it describes is the same rule implemented in Prompts 1 and 2.
- The CHANGELOG `## Unreleased` section uses the `feat:` prefix per `changelog-guide.md` — this is a new user-visible capability (the rule is now enforced at three points), so the version bump is minor.
- The frozen CHANGELOG preamble (lines 1-9) MUST NOT be modified, moved, or have anything inserted above or inside it. Insert `## Unreleased` immediately AFTER the last preamble line and BEFORE the `## v0.77.0` heading.
- The change is fully reversible — restoring the old step 8 line and removing the new subsection / CHANGELOG entry is a no-op rollback.

</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted greps and checks (each MUST hold after edits):

```bash
# 1. task-creator.md Step 8 has the new conditional rule
grep -n 'status: in_progress.*IF.*planned_date' agents/task-creator.md
# Expected: 1 line

# 2. The Step 8 bullet "status: todo" no longer appears (Step 13 echo is out of scope this prompt)
grep -nE '^- `status: todo`' agents/task-creator.md
# Expected: 0 lines

# 3. New Lifecycle subsection heading
grep -n 'Calendar-as-commitment rule' docs/task-writing.md
# Expected: 1 line

# 4. CHANGELOG.md Unreleased section above v0.77.0
grep -nE '^## Unreleased|^## v0.77.0' CHANGELOG.md
# Expected: ## Unreleased line number < ## v0.77.0 line number

# 5. New bullet names the rule
grep -nA 3 '^## Unreleased' CHANGELOG.md | grep -E 'calendar|in_progress|STATUS_DATE_MISMATCH'
# Expected: matches (the rule's name + the wire-format string + the canonical status)

# 6. Only the three markdown files changed
git diff --name-only
# Expected: agents/task-creator.md, docs/task-writing.md, CHANGELOG.md (in any order)

# 7. No Go files modified
git diff --name-only -- '*.go'
# Expected: 0 lines

# 8. make precommit
make precommit
# Expected: "ready to commit" (exit 0)
```

Runtime repro (manual, at PR verify time by the human reviewer — NOT in the daemon container):

**New-task path with date:**

```
/vault-cli:create-task "Review trading week 24" --defer 2026-06-20
```

Expect: the new task file has `status: in_progress` (because `defer_date: 2026-06-20` is in the frontmatter), per the new step 8 rule. The file does NOT have `status: next` or `status: todo`.

**New-task path without date:**

```
/vault-cli:create-task "Tidy task-writing.md" 
```

Expect: the new task file has `status: next` (no date field is being written; per the new step 8 rule's OTHERWISE branch), per the new step 8 rule.

**Lint smoke (verifies Prompt 1 is live):**

```bash
echo "---\nstatus: next\ndefer_date: 2026-12-01\n---" > /tmp/old-task.md
mkdir -p /tmp/lint-smoke/Tasks
cp /tmp/old-task.md /tmp/lint-smoke/Tasks/
vault-cli task lint /tmp/lint-smoke
# Expect: ≥1 line containing "STATUS_DATE_MISMATCH"
vault-cli task lint /tmp/lint-smoke --fix
# Expect: file now has "status: in_progress", "defer_date: 2026-12-01" (byte-identical)
```

</verification>
