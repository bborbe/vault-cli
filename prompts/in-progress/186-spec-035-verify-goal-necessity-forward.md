---
status: failed
spec: [035-verify-goal-necessity-checks]
execution_id: vault-cli-verify-necessity-exec-186-spec-035-verify-goal-necessity-forward
dark-factory-version: dev
created: "2026-08-21T09:48:00Z"
queued: "2026-08-21T10:07:08Z"
started: "2026-08-21T10:07:10Z"
completed: "2026-08-21T10:15:02Z"
branch: dark-factory/verify-goal-necessity-checks
lastFailReason: 'validate completion report: completion report status: failed'
---

# Add a goal-necessity forward check to verify-task

<summary>
- `verify-task` gains a goal-necessity forward check: when a task links a goal, the verify report now flags the link if none of that goal's success criteria needs the task's outcome (a correlation-only link).
- The check runs for every goal linked in the task's `goals` field; the report names the specific offending goal link and the reason, so a false positive is visible to the operator for a deliberate fix.
- A goal whose `# Non-goals` section explicitly excludes the task's domain is also treated as "not needed", and work-breakdown slices / scope-creep items / padding are flagged.
- Explicitly framed foundation tasks (per the goal-writing guide) are treated as needed — the check does not flag them.
- A linked goal with a missing or unparseable `# Success Criteria` section produces an informational line instead of a verdict, and the check stays advisory: nothing is modified, re-linked, or auto-fixed.
- Clean links (the task advances at least one success criterion) produce no necessity row; the existing pass/fail report shape, status checks, and DoD checks are untouched.
- The thin `verify-task` command doc gains one bullet naming the check in its process list and one line in its success criteria — no detection logic or judgment heuristics live there.
- The changelog records the new check under `## Unreleased`; no version strings move (the github-releaser owns bumps).
- Reviewer note: the spec cites `{{task_name}}` / `{{goal_name}}` double-brace placeholders, but the agent files actually use single-brace `{task_name}` / `{goal_name}`. This prompt preserves the files' existing forms and does not touch the existing report placeholders; the new report strings use the spec's angle-bracket tokens (`<goal>`), so no acceptance criterion depends on the brace style.
</summary>

<objective>
Add the goal-necessity forward check to the `task-manager-agent` `verify` action (a new step right after the parent-linkage step) and document it in `commands/verify-task.md`, so a task whose linked goal's success criteria need none of its outcome is flagged in the verify report instead of passing silently. Markdown-only — no Go code, no CLI surface change.
</objective>

<context>
Read `CLAUDE.md` for project conventions (changelog, version alignment, plugin release rules).

**Files to edit — read fully before changing:**
- `agents/task-manager-agent.md` — the `### verify` action (steps 1–9 today): step 4 is `4. **Check parent linkage (goal OR theme):**` (extracts the `goals` and `themes` frontmatter fields and verifies linked files exist), step 9 is `9. **Report:**` (the `✅ Task Valid` / `❌ Task Issues` shapes with `✗ {specific issues}`). The new check inserts as a new step 5, and old steps 5–9 renumber to 6–10.
- `commands/verify-task.md` — the thin command doc. Process step 3 lists the `Agent checks:` bullets; the `success_criteria` section lists the pass criteria. Frontmatter is `description` + `argument-hint` (it currently has NO `allowed-tools` — do not add one).

**Semantic anchor (the fixed judge rule the new step must quote):**
- `docs/goal-writing.md` § Non-goals — the scope-creep guard: a goal's `# Non-goals` lists concrete deferrals (what's out of scope); a task whose domain a goal's Non-goals explicitly exclude is by definition not needed by that goal.
- `docs/goal-writing.md` § Tasks as Business-Value Milestones → Foundation/skeleton work: tasks that enable but don't directly advance a success criterion are allowed when explicitly framed as foundation (e.g. "foundation; enables iteration").

**Coding plugin (in-container path — the YOLO container mounts the coding plugin at `/home/node/.claude/plugins/marketplaces/coding/docs/`):**
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `feat:` prefix (minor bump), `## Unreleased` placement rules, one bullet per logical change.

The change is additive: no existing structural check (status validity, link resolution, DoD presence, checkbox tracking, status consistency) is altered. The `### status` action of `task-manager-agent.md` is NOT touched.
</context>

<requirements>

### File 1: `agents/task-manager-agent.md`

1. **Insert a new step into the `### verify` action**, immediately after the existing step `4. **Check parent linkage (goal OR theme):**` (the step that extracts the `goals` and `themes` fields and verifies linked files exist). The new step becomes step 5. Renumber the remaining verify-action steps by +1: old step 5 (`Check Success Criteria section`) → 6, old 6 (`Check DoD section`) → 7, old 7 (`Check checkboxes`) → 8, old 8 (`Check status consistency`) → 9, old 9 (`Report`) → 10. Do NOT change the content of any existing step — only their numbers. The `### status` action and its step numbers are untouched.

2. **Write the new step 5 with this exact heading and body.** The heading must be verbatim `5. **Check goal-necessity (forward):**` — note the lowercase `goal-necessity` token (the acceptance criteria run case-sensitive `grep -n 'goal-necessity'` on this file, so the lowercase form is load-bearing), and the heading follows the existing step-naming convention (`Check parent linkage (goal OR theme):`, `Check Success Criteria section:`, …). The body:

   ```
   - For each goal linked in the `goals` field (resolved in step 4), locate the goal file under `23 Goals/` (fall back to `22 Goals/` for compatibility) and read its `# Success Criteria` section. Skip any linked goal whose file was already flagged unresolvable in step 4.
   - For each readable linked goal, evaluate whether ANY success criterion needs this task's outcome — does completing this task advance, produce evidence for, or unblock at least one success criterion of that goal?
   - Judge with this fixed semantic anchor (cite it when reasoning; see `docs/goal-writing.md` § Non-goals — the scope-creep guard and § Tasks as Business-Value Milestones → Foundation/skeleton work): a task is *needed* iff it advances ≥ 1 success criterion of the linked goal OR is explicitly framed as a needed foundation task (e.g. "foundation; enables iteration"). Work-breakdown slices, scope-creep items, and padding are NOT needed. A task whose domain the goal's `# Non-goals` section explicitly excludes is also NOT needed.
   - If NO success criterion needs the task's outcome → report issue: `✗ task not needed by linked goal <goal> — correlation-only (advances no success criterion)`. If the goal's `# Non-goals` explicitly exclude the task's domain → report instead: `✗ task not needed by linked goal <goal> — goal Non-goals exclude this task's domain`. In both cases the issue names the specific linked goal (`<goal>`) and the reason, so a misread is visible to the operator.
   - If a linked goal has no parseable `# Success Criteria` (section missing or no checkbox lines) → emit info line `cannot evaluate necessity — goal <goal> has no parseable Success Criteria` and skip the necessity verdict for that goal (the structural checks already flag the missing section).
   - Clean links (the task advances ≥ 1 SC of the linked goal) produce NO necessity issue and NO necessity row in the report.
   - Necessity issues join the report step's `✗ {specific issues}` list (flipping the report to the `❌ Task Issues` shape); info lines are appended to the report body as plain lines — not `✗` issues, not pass/fail.
   - Advisory only: report only — never modify goal or task files, never auto-remove or re-link.
   ```

   These strings are load-bearing and MUST appear verbatim in the step: the heading token `goal-necessity`, the pinned issue lines containing `not needed by linked goal`, and the info line containing `cannot evaluate necessity`. Use the spec's `<goal>` token as written (single angle brackets) — do NOT substitute double-brace or single-brace placeholders in the new messages.

3. **Leave the `9.`/`10.` Report step's content unchanged** (the `✅ Task Valid: [[{task_name}]]` / `❌ Task Issues: [[{task_name}]]` shapes with `✗ {specific issues}`). The new step's issues flow into that generic `✗ {specific issues}` list; no report-shape rewrite is needed.

### File 2: `commands/verify-task.md`

4. **Add one bullet to the process "Agent checks:" list** (the bulleted list under `3. Agent checks:`), after the existing `- Status consistency (completed → 100% checkboxes)` bullet, exactly:
   ```
   - Goal necessity (for each linked goal, its success criteria need the task's outcome — goal-necessity check)
   ```

5. **Add one line to the `<success_criteria>` list**, after the existing `- Pass/fail output with specific issues listed` line, exactly:
   ```
   - Goal-necessity check reported (correlation-only goal links flagged)
   ```

6. **Do NOT modify the frontmatter** of `commands/verify-task.md` (currently `description` + `argument-hint`, no `allowed-tools`). The command file stays thin: it names the check but contains no detection logic and no judgment heuristics (no prose about how to judge necessity).

### File 3: `CHANGELOG.md`

7. **Add the `## Unreleased` section.** The file currently has NO `## Unreleased` section — its newest versioned section is `## v0.114.1`. Insert a new section directly between the preamble block (the line `* PATCH version when you make backwards-compatible bug fixes.`) and `## v0.114.1`, exactly:
   ```
   ## Unreleased

   - feat: verify-task now runs a goal-necessity forward check — for each linked goal, a task whose outcome advances none of the goal's success criteria (or whose domain the goal's `# Non-goals` explicitly exclude) is flagged in the verify report as not needed by the linked goal; goals with missing or unparseable `# Success Criteria` yield an informational line instead. Advisory only — nothing is auto-fixed.
   ```
   Do NOT create a `## vX.Y.Z` section, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT tag — the github-releaser owns version bumps for this repo (`.maintainer.yaml` sets `autoRelease: true`) and converts this `## Unreleased` bullet post-merge.

### Boundaries

8. **Only these three files may change**: `agents/task-manager-agent.md`, `commands/verify-task.md`, `CHANGELOG.md`. Do NOT touch `agents/goal-manager-agent.md` or `commands/verify-goal.md` (the inverse check is a separate prompt) and no `.go` files, no `.claude-plugin/*.json`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Additive: the existing structural checks (status validity, link resolution, DoD presence, checkbox tracking, status consistency) remain unchanged; their verify-action step numbers are preserved except for the +1 renumbering caused by the single inserted step (no step or file references those numbers).
- The checks live in the agent `### verify` action and the thin command doc only — no Go code, no `pkg/ops/` changes, no CLI flag changes.
- The necessity judgment is LLM-based (verify commands are agent-dispatched). The fixed semantic anchor — a task is "needed" iff it advances ≥ 1 success criterion of the linked goal, or is explicitly framed as a needed foundation task; tasks that are work-breakdown slices, scope-creep items, or padding are flagged — is quoted in the new step so the judge has a fixed rule.
- `commands/verify-task.md` keeps its current frontmatter (`description` + `argument-hint`; it has no `allowed-tools` — do not add one).
- Preserve the existing report vocabulary (`✅` / `❌` / `✗`) and the file's existing single-brace placeholders (`{task_name}`); the new report messages use the spec's angle-bracket `<goal>` token.
- No scenarios are required for this change: it is markdown-only (no `.go`, `.mod`, `.sum`, or `Makefile` diffs), so a scenario walk would add no signal; the spec's fixture-vault smoke test is an operator-side step after merge, not this prompt.
- Existing tests must still pass; `make precommit` must exit 0.
</constraints>

<verification>
Run all of the following from the repo root. Every `grep` uses the exact command form given; a count that differs from the stated expectation means the requirement is not met — fix the file, then re-run.

The forward check is a documented step in the verify action, with the pinned strings present:

```
grep -n 'goal-necessity' agents/task-manager-agent.md
grep -n 'not needed by linked goal' agents/task-manager-agent.md
grep -n 'cannot evaluate necessity' agents/task-manager-agent.md
```

Each must print `>= 1` line. The step was inserted in the right place and the verify action was renumbered:

```
grep -c '^5\. \*\*Check goal-necessity (forward):\*\*' agents/task-manager-agent.md
grep -c '^10\. \*\*Report:\*\*' agents/task-manager-agent.md
grep -c 'Check Success Criteria section' agents/task-manager-agent.md
```

Expect `1`, `1`, `1` respectively. The third proves the old step 5 content survived the renumber (it is now step 6).

The thin command doc names the check without adding logic:

```
grep -n 'goal-necessity' commands/verify-task.md
```

Must print `>= 1` line.

The changelog records the change inside a single `## Unreleased` section and no version string moved:

```
grep -c '^## Unreleased$' CHANGELOG.md
sed -n '/^## Unreleased$/,/^## v/p' CHANGELOG.md | grep -c 'goal-necessity forward check'
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

Expect `1`, `>= 1`, `## v0.114.1`, and the plugin version unchanged from its current value (still `0.114.0` — do not bump; the repo's version strings already drift: changelog at v0.114.1 vs plugin.json at 0.114.0, and `make precommit` does not run `check-versions`) respectively.

The inverse check is NOT this prompt's scope — the goal-manager side must be untouched (guarded by the inverse-specific pinned message, not the shared `goal-necessity` token, so the check stays valid regardless of queue order):

```
grep -c 'not needed to complete goal' agents/goal-manager-agent.md
```

Must print exactly `0`. (The command doc `commands/verify-goal.md` will legitimately contain `goal-necessity` once the inverse prompt lands; its non-leak is enforced by that prompt's own boundary, not here.)

Finally, run the suite and the full gate once:

```
make test
make precommit
```

Both must exit 0. `make precommit` runs `ensure format generate test check addlicense`; this change touches no `.go` files, so `format` / `generate` / `addlicense` produce no churn. If `make precommit` fails, fix the issue and re-run only the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. A non-zero exit code from `make precommit` means `"status":"failed"` in the completion report — no exceptions.
</verification>
