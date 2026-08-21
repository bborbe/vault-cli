---
status: approved
spec: [035-verify-goal-necessity-checks]
created: "2026-08-21T09:48:00Z"
queued: "2026-08-21T10:07:08Z"
branch: dark-factory/verify-goal-necessity-checks
---

# Add a goal-necessity inverse check to verify-goal

<summary>
- `verify-goal` gains a goal-necessity inverse check: every task linked in a goal's `# Tasks` section is now evaluated against the goal's own success criteria, and the verify report flags any task that advances none of them.
- The report names the specific offending task link and the reason, so a false positive is visible to the operator for a deliberate fix.
- Tasks the goal explicitly frames as needed foundation work are treated as needed and not flagged; work-breakdown slices, scope-creep items, and padding are flagged.
- A task whose domain the goal's `# Non-goals` section explicitly excludes is also treated as "not needed".
- A goal with a missing or unparseable `# Success Criteria` section produces an informational line instead of a verdict, and the check stays advisory: nothing is modified, re-linked, or auto-fixed.
- Clean links (the task advances at least one success criterion) produce no necessity row; the existing pass/fail report shape, status checks, and task-existence checks are untouched.
- The thin `verify-goal` command doc gains one bullet naming the check in its process list and one line in its success criteria — no detection logic or judgment heuristics live there.
- The changelog records the new check as a second bullet under the existing `## Unreleased` section (created by the companion forward-check prompt); no version strings move.
- Reviewer note: this prompt depends on the forward-check prompt (task-manager side) having run first — it mirrors its phrasing and expects its strings already present. The spec cites `{{task_name}}` / `{{goal_name}}` double-brace placeholders, but the agent files actually use single-brace `{goal_name}` / `{task_name}`; this prompt preserves the existing forms and uses the spec's angle-bracket tokens (`<task>`, `<goal>`) in the new report strings, so no acceptance criterion depends on the brace style.
</summary>

<objective>
Add the goal-necessity inverse check to the `goal-manager-agent` `verify` action (a new step right after the task/PRD-linkage step) and document it in `commands/verify-goal.md`, so a task linked in a goal's `# Tasks` section that advances none of that goal's success criteria is flagged in the verify report instead of passing silently. Markdown-only — no Go code, no CLI surface change.
</objective>

<context>
Read `CLAUDE.md` for project conventions (changelog, version alignment, plugin release rules).

**Files to edit — read fully before changing:**
- `agents/goal-manager-agent.md` — the `### verify` action (steps 1–8 today): step 7 is `7. **Check task/PRD linkage:**`, step 8 is `8. **Report:**` (the `✅ Goal Valid` / `❌ Goal Issues` shapes with `✗ {specific issues}`). The shared operation `parse_success_criteria(goal_path)` (in the Shared Operations section) extracts the `# Success Criteria` checkboxes. The new check inserts as a new step 8, and old step 8 (`Report`) renumbers to 9.
- `commands/verify-goal.md` — the thin command doc. Process step 3 lists the `Agent checks:` bullets; the `success_criteria` section lists the pass criteria. Frontmatter is `description` + `argument-hint` + `allowed-tools: [Task]` — keep it exactly as-is.

**Companion forward check (already shipped by the prior prompt — verify presence, do NOT recreate):**
- `agents/task-manager-agent.md` `### verify` step `5. **Check goal-necessity (forward):**` and `commands/verify-task.md` — both already contain the string `goal-necessity` and the pinned forward messages. This prompt does not touch either file.

**Semantic anchor (the fixed judge rule the new step must quote):**
- `docs/goal-writing.md` § Tasks as Business-Value Milestones → Foundation/skeleton work: tasks that enable but don't directly advance a success criterion are allowed when explicitly framed as foundation (e.g. "foundation; enables iteration").
- `docs/goal-writing.md` § Non-goals — the scope-creep guard: a goal's `# Non-goals` lists concrete deferrals (what's out of scope); a task whose domain a goal's Non-goals explicitly exclude is by definition not needed by that goal.

**Coding plugin (in-container path — the YOLO container mounts the coding plugin at `/home/node/.claude/plugins/marketplaces/coding/docs/`):**
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `feat:` prefix (minor bump), `## Unreleased` append rules (one bullet per logical change; if `## Unreleased` already exists, append to it, don't replace).

The change is additive: no existing structural check (status validity, Status Summary, subtask existence, status consistency, task/PRD linkage) is altered. The `### status` action of `goal-manager-agent.md` is NOT touched.
</context>

<requirements>

### File 1: `agents/goal-manager-agent.md`

1. **Insert a new step into the `### verify` action**, immediately after the existing step `7. **Check task/PRD linkage:**`. The new step becomes step 8. Renumber the remaining verify-action steps by +1: old step 8 (`Report`) → 9. Do NOT change the content of any existing step — only its number. The `### status` action and its step numbers are untouched.

2. **Write the new step 8 with this exact heading and body.** The heading must be verbatim `8. **Check goal-necessity (inverse):**` — note the lowercase `goal-necessity` token (the acceptance criteria run case-sensitive `grep -n 'goal-necessity'` on this file, so the lowercase form is load-bearing), and the heading follows the existing step-naming convention (`Check Status Summary section:`, `Check subtask existence:`, `Check task/PRD linkage:`, …). The body:

   ```
   - For each task linked in the goal's `# Tasks` section (resolved in step 5), evaluate whether it advances ≥ 1 success criterion of THIS goal (use `parse_success_criteria(goal_path)`), or is explicitly framed by the goal as a needed foundation task.
   - Judge with this fixed semantic anchor (cite it when reasoning; see `docs/goal-writing.md` § Tasks as Business-Value Milestones → Foundation/skeleton work and § Non-goals — the scope-creep guard): a linked task is *needed* iff it advances ≥ 1 success criterion of the goal OR is explicitly framed as a needed foundation task (e.g. "foundation; enables iteration"). Work-breakdown slices, scope-creep items, and padding are NOT needed. A task whose domain the goal's `# Non-goals` section explicitly excludes is also NOT needed.
   - If a linked task advances no success criterion → report issue: `✗ task <task> not needed to complete goal — advances no success criterion`. If the goal's `# Non-goals` explicitly exclude the task's domain → report instead: `✗ task <task> not needed to complete goal — goal Non-goals exclude this task's domain`. In both cases the issue names the specific linked task (`<task>`) and the reason.
   - If the goal has no parseable `# Success Criteria` (section missing or no checkbox lines) → emit info line `cannot evaluate necessity — goal <goal> has no parseable Success Criteria` and skip the necessity verdict (the structural checks already flag the missing section).
   - Clean links (the task advances ≥ 1 SC of the goal) produce NO necessity issue and NO necessity row in the report.
   - Necessity issues join the report step's `✗ {specific issues}` list (flipping the report to the `❌ Goal Issues` shape); info lines are appended to the report body as plain lines — not `✗` issues, not pass/fail.
   - Advisory only: report only — never modify goal or task files, never auto-remove or re-link.
   ```

   These strings are load-bearing and MUST appear verbatim in the step: the heading token `goal-necessity`, the pinned issue lines containing `not needed to complete goal`, and the info line containing `cannot evaluate necessity`. Use the spec's `<task>` / `<goal>` tokens as written (single angle brackets) — do NOT substitute double-brace or single-brace placeholders in the new messages.

3. **Leave the `9.` Report step's content unchanged** (the `✅ Goal Valid: [[{goal_name}]]` / `❌ Goal Issues: [[{goal_name}]]` shapes with `✗ {specific issues}`). The new step's issues flow into that generic `✗ {specific issues}` list; no report-shape rewrite is needed.

### File 2: `commands/verify-goal.md`

4. **Add one bullet to the process "Agent checks:" list** (the bulleted list under `3. Agent checks:`), after the existing `- Tasks/PRDs linked` bullet, exactly:
   ```
   - Goal necessity (each linked task advances ≥ 1 of the goal's success criteria — goal-necessity check)
   ```

5. **Add one line to the `<success_criteria>` list**, after the existing `- Pass/fail output with specific issues listed` line, exactly:
   ```
   - Goal-necessity check reported (tasks advancing no success criterion flagged)
   ```

6. **Do NOT modify the frontmatter** of `commands/verify-goal.md` (keep `description`, `argument-hint`, and `allowed-tools: [Task]` exactly as-is). The command file stays thin: it names the check but contains no detection logic and no judgment heuristics (no prose about how to judge necessity).

### File 3: `CHANGELOG.md`

7. **Append one bullet to the `## Unreleased` section.** The prior forward-check prompt created `## Unreleased` (immediately above `## v0.114.1`); append the inverse-check bullet after the existing `feat:` bullet, exactly:
   ```
   - feat: verify-goal now runs a goal-necessity inverse check — a task linked in a goal's `# Tasks` section that advances no success criterion of the goal (and is not explicitly framed as a needed foundation task, or whose domain the goal's `# Non-goals` explicitly exclude) is flagged in the verify report as not needed to complete the goal; goals with missing or unparseable `# Success Criteria` yield an informational line instead. Advisory only — nothing is auto-fixed.
   ```
   If `## Unreleased` does not exist (defensive — the prior prompt may not have executed yet), create it in the same position and shape as the forward prompt specified (between the preamble block and `## v0.114.1`) and place this bullet in it. Do NOT create a `## vX.Y.Z` section, do NOT bump `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT tag — the github-releaser owns version bumps for this repo (`.maintainer.yaml` sets `autoRelease: true`).

### Boundaries

8. **Only these three files may change**: `agents/goal-manager-agent.md`, `commands/verify-goal.md`, `CHANGELOG.md`. Do NOT touch `agents/task-manager-agent.md` or `commands/verify-task.md` (the forward check from the prior prompt), and no `.go` files, no `.claude-plugin/*.json`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Additive: the existing structural checks (status validity, Status Summary, subtask existence, status consistency, task/PRD linkage) remain unchanged; the only renumbering is old verify step 8 (`Report`) → 9 caused by the single inserted step (no step or file references that number).
- The checks live in the agent `### verify` action and the thin command doc only — no Go code, no `pkg/ops/` changes, no CLI flag changes.
- The necessity judgment is LLM-based (verify commands are agent-dispatched). The fixed semantic anchor — a linked task is "needed" iff it advances ≥ 1 success criterion of the goal, or is explicitly framed as a needed foundation task; tasks that are work-breakdown slices, scope-creep items, or padding are flagged — is quoted in the new step so the judge has a fixed rule.
- `commands/verify-goal.md` keeps its current frontmatter, including `allowed-tools: [Task]` — do not add or remove tools.
- Preserve the existing report vocabulary (`✅` / `❌` / `✗`) and the file's existing single-brace placeholders (`{goal_name}`); the new report messages use the spec's angle-bracket `<task>` / `<goal>` tokens.
- This prompt mirrors the forward-check prompt's phrasing and expects its strings (`goal-necessity` in `agents/task-manager-agent.md` and `commands/verify-task.md`) already present — do not recreate or alter them.
- No scenarios are required for this change: it is markdown-only (no `.go`, `.mod`, `.sum`, or `Makefile` diffs), so a scenario walk would add no signal; the spec's fixture-vault smoke test is an operator-side step after merge, not this prompt.
- Existing tests must still pass; `make precommit` must exit 0.
</constraints>

<verification>
Run all of the following from the repo root. Every `grep` uses the exact command form given; a count that differs from the stated expectation means the requirement is not met — fix the file, then re-run.

The inverse check is a documented step in the verify action, with the pinned strings present:

```
grep -n 'goal-necessity' agents/goal-manager-agent.md
grep -n 'not needed to complete goal' agents/goal-manager-agent.md
grep -n 'cannot evaluate necessity' agents/goal-manager-agent.md
```

Each must print `>= 1` line. The step was inserted in the right place and the verify action was renumbered:

```
grep -c '^8\. \*\*Check goal-necessity (inverse):\*\*' agents/goal-manager-agent.md
grep -c '^9\. \*\*Report:\*\*' agents/goal-manager-agent.md
grep -c '^7\. \*\*Check task/PRD linkage:\*\*' agents/goal-manager-agent.md
```

Expect `1`, `1`, `1` respectively. The third proves the old step 7 content is intact immediately before the new step.

The thin command doc names the check without adding logic:

```
grep -n 'goal-necessity' commands/verify-goal.md
```

Must print `>= 1` line.

The changelog records the change inside the `## Unreleased` section and no version string moved:

```
grep -c '^## Unreleased$' CHANGELOG.md
sed -n '/^## Unreleased$/,/^## v/p' CHANGELOG.md | grep -c 'goal-necessity inverse check'
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```

Expect `1`, `>= 1`, `## v0.114.1`, and the plugin version unchanged from its current value (still `0.114.0` — do not bump; the repo's version strings already drift: changelog at v0.114.1 vs plugin.json at 0.114.0, and `make precommit` does not run `check-versions`) respectively.

The forward check from the prior prompt is untouched, and neither check leaks into the other agent:

```
grep -c 'goal-necessity' agents/task-manager-agent.md
grep -c 'goal-necessity' commands/verify-task.md
grep -c 'not needed to complete goal' agents/task-manager-agent.md
grep -c 'not needed by linked goal' agents/goal-manager-agent.md
```

Expect `>= 1`, `>= 1`, `0`, `0` respectively.

Finally, run the suite and the full gate once:

```
make test
make precommit
```

Both must exit 0. `make precommit` runs `ensure format generate test check addlicense`; this change touches no `.go` files, so `format` / `generate` / `addlicense` produce no churn. If `make precommit` fails, fix the issue and re-run only the failing target (`make lint`, `make vet`, `make test`, `make check-changelog`, …) until it passes, then run `make precommit` once more. A non-zero exit code from `make precommit` means `"status":"failed"` in the completion report — no exceptions.
</verification>
