---
status: approved
spec: [055-remove-metrics-session]
created: "2026-09-27T09:26:40Z"
queued: "2026-09-27T10:04:31Z"
branch: dark-factory/remove-metrics-session
---

# Stop the auditor recommending a link it will then score as an orphan (spec 055, prompt 2 of 3)

<summary>
- Adds one guard sentence to the task auditor's goal-alignment section so the rule that judges a goal link and the advice that recommends one use the same predicate.
- Today the section flags as MAJOR any goal link that advances none of the goal's criteria, while the surrounding guidance can suggest linking a goal on family resemblance — so a link the auditor itself recommends can come back as a MAJOR orphan on the next run.
- That contradiction cost two extra audit cycles plus an operator ruling on 2026-09-15 to unblock a single task: the recommendation was taken, and the two following runs scored the very same link as an orphan.
- The new sentence sits between the orphan bullet and the implementation-level bullet, so it is read at the moment the auditor decides whether to recommend a link.
- The rule it states: if the goal can be marked complete without this task, do not recommend the link — recommend theme-only linkage and say so.
- Markdown-only change to an agent definition. No Go code, no tests, no version bump.
- Covers spec 055 AC 8. AC 9 is the post-deploy behavioural half and is not checkable before release.
- Independent of prompt 1 — this can land or be reverted without the Go change.
</summary>

<objective>
Remove the self-contradiction in `task-auditor`: a goal link the auditor recommends must never be one its own alignment check would then score as a MAJOR orphan. Covers spec 055 AC 8. Prompt 1 (the removal verb) and prompt 3 (the docs) are unaffected by this change.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

Read fully:
- `agents/task-auditor.md` § `## Task-Goal Alignment (per-goal-link check)` — the section starting at line 237. Read to at least the `### 9. Scope Appropriateness` heading at line 248. The two bullets that bracket the insertion point are:
  - line 243: `3. **Flag orphans as MAJOR** — task has a goal link but advances none of its criteria.`
  - line 244: `4. **Flag implementation-level tasks** — if title reads like a low-level code change ("Add field X to struct Y"), check whether a dark-factory spec or prompt is the right artifact instead.`
  Also read the note at line 246 (`**When \`goals:\` is absent but \`themes:\` is populated**`), which already carries the "theme linkage is acceptable" half of this rule, and line 133, which states the same threshold for the structure check. The new sentence must not contradict either.
- The `## Task-Goal Alignment` report template at line 501 — the table whose `ORPHAN — MAJOR` verdict is the check the new sentence has to agree with.

This is a Direct-layer markdown edit to an agent definition. No Go code is involved.
</context>

<requirements>
1. **Insert exactly one guard sentence** into § Task-Goal Alignment, as a sub-bullet of item 3, on its own line, positioned **after** the `**Flag orphans as MAJOR**` bullet (line 243) and **before** the `**Flag implementation-level tasks**` bullet (line 244). Leave the two bracketing bullets' text unchanged and do not renumber them.

   The inserted line MUST contain this exact substring, verbatim and in lowercase, because AC 8 greps for it:
   ```
   never recommend linking a goal the alignment check will then score as an orphan
   ```
   A capitalised `Never` at the start of the sentence will NOT satisfy the grep. Put the phrase mid-sentence, for example:
   ```markdown
   3. **Flag orphans as MAJOR** — task has a goal link but advances none of its criteria.
      - When the goal can be marked complete without this task, never recommend linking a goal the alignment check will then score as an orphan — recommend theme-only linkage instead and say so explicitly.
   4. **Flag implementation-level tasks** — ...
   ```
   Keep the wording close to that — the requirement is that the sentence states the rule (do not recommend a link the check would then flag) and the fallback (recommend theme-only linkage and say so).

2. **Make the two halves agree.** The sentence must be consistent with the existing note at line 246 and with the `ORPHAN — MAJOR` verdict in the report template at line 501. Do not weaken the orphan rule itself, and do not change the threshold at which a missing `goals:` link is flagged — this prompt adds a constraint on the *advice*, not on the *check*.

3. **Change nothing else.** No other section of `agents/task-auditor.md` is edited, no bullet is reordered, no heading is renamed, and the file's frontmatter is untouched. This is a one-sentence change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` only reads; never stage or commit.
- **Exactly one sentence is added.** A diff that restructures § Task-Goal Alignment, renumbers the list, or edits the line-246 note fails review.
- **Do NOT weaken the orphan check.** The `**Flag orphans as MAJOR**` bullet's threshold is unchanged; this prompt constrains what the auditor *recommends*, not what it *flags*.
- **Do NOT touch the plugin cache** at `~/.claude/plugins/cache/vault-cli/vault-cli/*` — that is an install artifact, clobbered by the next `claude plugin update`. Edit only `agents/task-auditor.md` in the repo.
- **No version bump.** This is an unreleased source change; the release is handled by the repo's `autoRelease` releaser after merge. Do not edit `CHANGELOG.md`, `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json` — prompt 3 owns the CHANGELOG bullet.
- This change ships in the plugin, so it reaches the installed `task-auditor` only after `claude plugin update vault-cli@vault-cli`. That is AC 9's post-deploy rung and is explicitly out of scope here.
</constraints>

<verification>
PRIMARY GATE — AC 8. The sentence must exist AND sit strictly between the two bracketing bullets, checked section-scoped rather than file-wide. Run and record:

```
grep -n 'never recommend linking a goal the alignment check will then score as an orphan' agents/task-auditor.md
```
Must return ≥ 1 line. Let `<guard>`, `<orphan>` and `<impl>` be the line numbers of the guard sentence, `**Flag orphans as MAJOR**`, and `**Flag implementation-level tasks**` respectively — all three inside § Task-Goal Alignment (the section beginning at `## Task-Goal Alignment (per-goal-link check)`):

```
grep -n 'Flag orphans as MAJOR' agents/task-auditor.md              # the orphan bullet — line <orphan>
grep -n 'Flag implementation-level tasks' agents/task-auditor.md    # the impl bullet — line <impl>
```
Assert `<orphan> < <guard> < <impl>`. A guard line equal to `<orphan>` or `<impl>` fails — it must be strictly between. A match elsewhere in the file (the report template at line 501 also discusses orphans) does NOT satisfy this; confirm the guard's line number falls inside the section's line range.

NEGATIVE EVIDENCE — the two bracketing bullets' text is unchanged:
```
git diff -- agents/task-auditor.md   # must show exactly one added line, no other hunks
```
Confirm the diff is a single insertion with no deletions other than the `4.` line's re-emission if your editor rewrote it; a hunk touching any other section fails.

SCOPE — ⚠️ this checkout is shared and is routinely dirty with other sessions' and the daemon's in-flight work, so scope the check to the file rather than asserting a clean tree:
```
git diff --name-only -- agents/task-auditor.md   # must list agents/task-auditor.md
```
Do NOT assert that `git diff --stat` lists only this file — unrelated paths from sibling work will appear and the assertion fails spuriously, which could push an implementer to "fix" files outside this prompt's scope. The evidence that nothing else was touched is the single-insertion diff above plus the unchanged bracketing bullets.

FULL GATE — `make precommit` at the repo root must exit 0. This change adds no Go code, so the gate should be unaffected by it; a failure here means either a pre-existing issue or an accidental edit outside `agents/task-auditor.md`. If it fails on something this prompt introduced, fix it and re-run only the failing target, then `make precommit` once more.
</verification>
