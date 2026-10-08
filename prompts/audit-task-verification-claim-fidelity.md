---
status: draft
created: "2026-09-27T16:02:46Z"
---

# Add a verification-claim fidelity check to the task auditor

<summary>
- Adds one rigor pass to the task auditor that makes it re-run a command whose result a task file *asserts*, instead of trusting the file's report of it.
- Closes a real gap: re-running the convention's reader command is what surfaced a real defect in a live task file, but the checklist has no line for it, so the re-run is not routine.
- A task file that reports "this grep matched only X" is now checked by re-running that grep, so a false report of a command's output is caught instead of propagated into the task's own record.
- This is the defect class the vault already documents in `An Asserted Match Is Not Evidence of Matching` — the knowledge exists; the auditor's checklist does not enforce it.
- Does not touch any Go source, the CLI surface, or the plugin version strings — this repository's release agent owns version bumps and tagging after merge, so the change only adds an unreleased bullet.
- Runs `make precommit` as final validation and records the gate's output.
</summary>

<objective>
The task auditor gains a rigor pass that checks any command result a task file asserts by re-running the cheapest instance of that command, so a false report of a command's output is caught during audit rather than propagated into the task's record.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully:
- `agents/task-auditor.md` — the whole file. The new check is inserted into its `## Rigor Passes (always run)` section, which today opens *"Six passes that apply to every task regardless of class."* § 13–§ 16 each carry one labelled item (three bold, one plain) — `**Flagged words:**` (§13), `**MANDATORY report output.**` (§14), `**Negative criteria need an explicit probe.**` (§15), `Signals it has:` (§16) — and none uses a `Fails: / Passes:` pair (that shape belongs to `commands/plan-task.md` sub-check 3, which asks whether a criterion *discriminates*). The new block is given verbatim below and its `Fails: / Passes:` form is intentional. ⚠️ **Re-read every anchor below against the file before editing** — this prompt was written 2026-09-27 and §17, §18 and §19 landed after it, so the pass count, the section count, the report-template tail and the `CHANGELOG.md` heading have all moved at least once already.
- `docs/task-writing.md` — the canonical rule source the auditor enforces. Confirm the new check does not restate an existing rule there; it is an *auditor* check, not a new writing rule.

Read for context only (do not modify):
- `CHANGELOG.md` — read the top ~12 lines. There is no `## Unreleased` section today (the file goes straight from the preamble to the newest versioned heading); this prompt creates it.
- `$HOME/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the unreleased-section placement rule, the frozen-preamble rule, the required conventional prefixes, and the rule that in an auto-release repository a feature branch adds bullets under the unreleased section and does NOT bump version strings. ⚠️ Resolve this via `$HOME`, not a container-absolute `/home/node/...` path — the latter does not exist outside the dark-factory image.
- `50 Knowledge Base/An Asserted Match Is Not Evidence of Matching.md` in the Personal vault — the KB page § 16.5's wording is drawn from. ⚠️ **Not reachable from this container** (the Personal vault is not mounted; do not search the filesystem for it and do not report having read it). The two clauses quoted here are the whole of what the inserted block borrows: the page's *"a claim about a comparison"* verbatim, and *"one of them being false is evidence about the disposition generator, not about that one line"* — § 16.5 retargets that second clause from the disposition generator to the task file's whole record.

Background (why this check, stated so the requirement is not applied as a blind insert). The auditor's § 13 (hedge words) and § 15 (evidence shape) already audit what a task file *says about its criteria*. Neither covers what a task file *says about a command's output* — a claim in `# Progress` or `# Results` of the form "X matched Y", "the probe returned Z", "the check passed". That is a claim about a comparison, and it can be false in the direction that matters: it suppresses the reader's own look.

Observed 2026-09-27 on a task whose `# Progress` recorded that the marker convention's own reader `grep -rnE -- '^- \[.\] (⚠️ )?production-touching'` "returned only the DoD box". The claim was true and re-running reproduced it — the file's other marker sat mid-line inside a longer bullet, where the pattern's `^- \[.\]` anchor could not reach it. The auditor ran that grep while checking the task's own `production-touching` marker — the task's `# Definition of Done` box carries one, so the marker convention was already in play on this file; the task's subject was a jump-link hang, not the convention. On a task carrying no such marker, the same class of claim would have gone unchecked. The checklist line is what makes the re-run routine rather than incidental — the incident above is § 16.5's own *Passes* case, not a *Fails* case, and it justifies the check's existence rather than illustrating a catch.

⚠️ Scope: § 16.5 sits inside `<evaluation_areas>`, which `<pipeline_artifact_preflight>` skips wholesale for agent-pipeline artifacts (*"do NOT run `<evaluation_areas>`"*). That is correct and intended — the pass applies to human-authored task files, which are where a task's own `# Progress`/`# Results` records a command's output. Do not add it to the reduced path.
</context>

<requirements>
1. **Insert the new rigor pass into `agents/task-auditor.md`.** Place it as § 16.5, immediately after the `### 16. MVP Framing` block and before the `### 17. Citation Fidelity` heading. (Those were adjacent when this prompt was written; § 17 and § 18 have since landed between § 16 and `## Quick Fixes (Minor)`, so the old "before `## Quick Fixes (Minor)`" anchor is no longer a single point and must not be used.) Insert this block verbatim (do not add frontmatter, do not renumber any existing section):

   ```
   ### 16.5 Verification-Claim Fidelity (free — re-run the cheapest instance)

   A task file may **assert what a command returned** — in `# Progress`, `# Results`, or a
   verification note: *"`grep X` matched only Y"*, *"the probe returned 400"*, *"the check
   passed"*. That sentence is a claim about a comparison, not a result.

   **Re-run the cheapest such command and compare.** One of them being false is evidence
   about the file's whole record, not about that line.

   Fails: *"the reader grep finds every `production-touching` marker in this file"* when one
   marker sits mid-line inside a longer bullet, so the pattern's `^- \[.\]` anchor cannot
   reach it — the command's output does not support the generalisation, and the pattern's
   blind spot was read as a clean result.
   Passes: a claim whose quoted output is reproduced by re-running it — e.g. the same reader
   grep returning exactly one hit on a file whose only other marker is mid-line.

   Cite the command and the actual output in your report. Do not accept the file's own
   report of it.
   ```

2. **Repair the pass count and the report template, and alter nothing else.** The section's intro line counts the passes, and the report template enumerates them — so the insert makes both false unless they are updated. Make exactly these three edits and no others — no renumbering of § 17+, no edits to § 13/§ 15:

   **(a)** In `agents/task-auditor.md`, immediately under `## Rigor Passes (always run)`:
   ```
   OLD: Six passes that apply to every task regardless of class. The first three were ported from `dark-factory`'s `spec-auditor` on 2026-09-05 and adapted spec→task (Acceptance Criteria → Success Criteria, prompts → subtasks); the fourth has no upstream; the fifth was added 2026-09-30 and opens the artifacts a task cites; the sixth was added 2026-10-05 and checks the empirical premises a task asserts about the running system.
   NEW: Seven passes that apply to every task regardless of class. The first three were ported from `dark-factory`'s `spec-auditor` on 2026-09-05 and adapted spec→task (Acceptance Criteria → Success Criteria, prompts → subtasks); the fourth has no upstream; the fifth was added 2026-09-30 and opens the artifacts a task cites; the sixth was added 2026-10-05 and checks the empirical premises a task asserts about the running system; the seventh is added by this prompt.
   ```
   ⚠️ The OLD string must match the file **as it stands when you run**. It was `Four passes …` when this prompt was written; § 17 and § 18 have since been added, so it is `Six passes …` today and the provenance clauses for the fifth and sixth passes must be preserved verbatim in the NEW string. If the count differs from Six, re-read the line and adjust the number rather than forcing this edit.

   **(b)** In the same file's report template, in the `## Rigor Passes` block — which ends with the `Premise fidelity:` line, *not* `MVP framing:` as it did when this prompt was written; `Citation fidelity:` and `Premise fidelity:` were added after it — add one line after `Premise fidelity:`:
   ```
   Verification-claim fidelity: [command re-run + its actual output, or "none asserted"]
   ```

   **(c)** In the same file's `### 16.5` block, immediately after its closing `Cite the command and the actual output in your report. Do not accept the file's own report of it.` line, add:
   ```
   ⚠️ Human-authored tasks only — `<pipeline_artifact_preflight>` skips this section for pipeline artifacts, which is intended.
   ```

3. **Confirm the insert landed where intended:**
   ```
   grep -n '^### 16.5 Verification-Claim Fidelity' agents/task-auditor.md   # exactly 1 hit
   grep -n '^### 17. Citation Fidelity' agents/task-auditor.md              # must come AFTER the 16.5 hit
   grep -cE '^### 1[0-9]\. ' agents/task-auditor.md                         # unchanged from before the edit (10)
   grep -c 'Six passes' agents/task-auditor.md                              # 0 after the edit
   ```
   The third command's count must equal its value before the edit — **record the before-count first and assert the after-count is identical**, rather than trusting the literal. It was 8 when this prompt was written and is **10** today (§ 10–§ 19 are the ten `### N.` sections), so a hardcoded number drifts by one with every added section. ⚠️ The trailing space inside the pattern is load-bearing: without it, `^### 1[0-9]\.` also matches the new `### 16.5` heading and the count reads one higher than the baseline, which looks like a displaced section but is not.

4. **Create the changelog's unreleased section and record the change.** `CHANGELOG.md` has no `## Unreleased` heading today. Add one directly above the **newest versioned heading** — resolve that at run time; it was `## v0.150.0` when this prompt was written and is thirteen releases further on now, so a hardcoded literal would insert the bullet mid-file and leave it silently unreleased. Under it add a single bullet with a conventional prefix:
   ```
   ## Unreleased

   - feat: add a verification-claim fidelity rigor pass to the task auditor
   ```
   ⚠️ If a `## Unreleased` section already exists when you run, append the bullet as the last bullet inside the **existing** section — never create a second `## Unreleased` heading. The releaser renames only the topmost section, so a second one would leave this bullet silently unreleased. Do NOT bump the plugin version strings and do NOT create a tag — the release agent owns both after merge.

5. **Run the repository's full gate** as final validation; see `<verification>`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- The edit is additive except for the two named repairs in requirement 2 — the intro-line count (a) and the one added `Verification-claim fidelity:` line in the report template (b): do NOT renumber § 17+, do NOT edit § 13 or § 15, and make no other change to the report template at the bottom of `agents/task-auditor.md`.
- Do NOT touch any Go source, `docs/task-writing.md`, or any `CHANGELOG.md` section other than adding the one bullet.
- Do NOT bump the plugin version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- If a `## Unreleased` section already exists when you run, append the bullet inside the existing one; never create a second `## Unreleased` heading.
</constraints>

<verification>
Run each; record the output verbatim in the report.

- `grep -n '^### 16.5 Verification-Claim Fidelity' agents/task-auditor.md` → exactly 1 hit
- `grep -n '^### 17. Citation Fidelity' agents/task-auditor.md` → its line number is greater than the 16.5 hit's
- `grep -cE '^### 1[0-9]\. ' agents/task-auditor.md` → identical to the before-count recorded first (10 today)
- `grep -c 'Six passes' agents/task-auditor.md` → 0
- `grep -c '^## Unreleased' CHANGELOG.md` → exactly 1
- `make precommit` → exit 0

If `make precommit` fails, STOP and report `"status":"failed"` with the exact failing command and its output. Do not attempt a partial fix of unrelated pre-existing failures — report them and stop.
</verification>
