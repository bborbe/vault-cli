---
status: completed
spec: [056-task-approve-command]
summary: Documented the `todo → planning` approval verb in docs/task-writing.md, realigned commands/plan-task.md onto it (refusing to plan an unapproved row), and added the `## Unreleased` feat bullet naming the verb, its four recorded values, `--by` default and refusals.
execution_id: vault-cli-approve-cmd-exec-240-spec-056-docs-changelog
dark-factory-version: v0.196.0
created: "2026-09-28T07:54:18Z"
queued: "2026-09-28T09:12:46Z"
started: "2026-09-28T09:25:38Z"
completed: "2026-09-28T09:29:21Z"
---

# Document the approval transition and record it in the CHANGELOG (spec 056, prompt 3 of 5)

<summary>
- The task-writing guide now names the command that performs the `todo → planning` transition, so an operator reading the phase table is pointed at a real verb instead of having to infer one.
- The guide states the rule the gate exists for: a phase set without an approval record is not an approval.
- The guide names the two fields the approval records, so a reader knows what evidence to look for in a file.
- The CHANGELOG carries an unreleased `feat:` bullet describing the new verb, its recorded fields and its refusals.
- No version string is hand-bumped — the repository's releaser owns the bump and the release cut.
- No new documentation page is introduced; the phase lifecycle's existing home is updated in place.
- The planning command is realigned onto the new verb alongside the guide: it no longer moves an unapproved row by setting the phase directly, and now refuses to plan such a row until the operator has run the approval command.
- Covers spec 056's docs AC.
</summary>

<objective>
Update `docs/task-writing.md` § Phase transitions so it names `vault-cli task approve` as the command that performs `todo → planning` and states that a phase set without an approval record is not an approval, and add the `## Unreleased` `feat:` bullet to `CHANGELOG.md`. This prompt covers spec 056's docs AC and its plan-task AC, and depends on prompts 1–2, whose surface it describes.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` § Documentation for the CHANGELOG placement rule.

Read fully (in this order):
- `docs/task-writing.md` — § Phase transitions (the heading is near line 427; the phase table follows it and the `todo` row is near line 433). Read the table in full, plus the two paragraphs below it ("The split between `/work-on-task` …" near line 439 and "**`/create-task` does not chain into planning.**" near line 441). This section already owns the phase lifecycle and is the only doc this prompt changes.
- `commands/plan-task.md` — step 3 "Entry contract — flip if needed" (near lines 66-73) and the two § Notes entries near lines 225-226. Requirement 5 rewrites both; read them before editing.
- `CHANGELOG.md` — lines 1-14 only. The preamble ends with the three `* MAJOR / MINOR / PATCH` lines; `## v0.152.0` begins near line 12. **There is no `## Unreleased` section at HEAD**, so this prompt creates one.
- `docs/dod.md` § Documentation — the exact required order: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z` (newest first).
- `scripts/check-changelog.sh` — it fails the build when a `## ` section appears *above* the preamble, so `## Unreleased` goes immediately below the preamble and immediately above `## v0.152.0`.
- `.maintainer.yaml` — `release.autoRelease: true`. The `github-releaser` owns the version bump and renames `## Unreleased` to `## vX.Y.Z` when it cuts a release, so nothing in this prompt touches a version string.
- `docs/releasing-vault-cli.md` — read § Version alignment only, to confirm that four version strings must stay in lockstep and that this prompt is not a release.

NOTE: git IS available in this container (`.dark-factory.yaml` is `workflow: direct`, no `hideGit`), so the `git diff --exit-code` / `git diff --name-only` / `git checkout --` guards in `<verification>` genuinely run here. If the repo is ever switched to `hideGit: true` or `workflow: worktree`, those commands fail with `fatal: not a git repository` and the daemon does not check verification exit codes for failure — a false-positive pass. Prompts 1 and 2 carry this same note.
</context>

<requirements>
1. **Update the `todo` row of the phase table** in `docs/task-writing.md` § Phase transitions so its trigger cell names the command. The row currently reads (near line 433):
   ```
   | `todo` | The operator's approval inbox — filed, not yet approved | `/vault-cli:create-task` writes it, and stops there; the operator's `todo → planning` flip is the approval. No command advances it on its own |
   ```
   Change the third cell so it names `vault-cli task approve` and states that the command records the approval. Keep the row's meaning cell ("The operator's approval inbox — filed, not yet approved") unchanged, and keep the other table rows' structure intact. A row in the form below satisfies the requirement (wording may differ, the named command and the two recorded fields may not):
   ```
   | `todo` | The operator's approval inbox — filed, not yet approved | `/vault-cli:create-task` writes it, and stops there. The `todo → planning` flip is the approval and is performed by `vault-cli task approve <task-name>`, which records `approved_by` and `approved_at` in the same write |
   ```

2. **Add the rule as prose** in § Phase transitions, immediately after the existing "**`/create-task` does not chain into planning.**" paragraph (near line 441). Add the new content there; line 441's own closing sentence ("The `todo → planning` flip is the operator's approval, and nothing else advances a `todo` row") now states the same rule the new paragraph states, so relabel or fold that one sentence into the new paragraph rather than leaving the rule stated twice. Do not delete the `/create-task`-does-not-chain point itself. It must state, in the section's own voice:
   - `vault-cli task approve <task-name>` is the command that performs `todo → planning`;
   - **a phase set without an approval record is not an approval** — `task set <name> phase planning` writes the phase and nothing else, so a row moved that way is indistinguishable from one that was moved by accident, which is the whole reason the gate needs its own verb;
   - the approval writes `status: in_progress`, `phase: planning`, `approved_by` and `approved_at` together in one write, so a row can never sit at `planning` without a record beside it;
   - the command refuses any row that is not at `phase: todo`, and any row that already carries `approved_by` or `approved_at`, writing nothing in either case;
   - the approver defaults to `operator` and `--by <approver>` names another.
   The `todo` row and this paragraph are the two places spec 056's docs AC counts: `grep -cE 'task approve' docs/task-writing.md` must be ≥ 2, and the two occurrences must be the two-word verb, never the bare substring `approve` (which already occurs twice at HEAD inside unrelated words).

3. **Name the entering command in the `planning` row too.** That row's trigger cell (near line 434) currently reads `` `/vault-cli:plan-task` (runs `task-auditor` + 5 hard non-negotiables loop; on score ≥ 8 + gates pass, reports ready and hands off to `/execute-task` — **never flips phase itself**) ``. Prefix the cell with the entering step so the table names exactly one enterer: `entered by `vault-cli task approve`; ` followed by the existing `/vault-cli:plan-task` text unchanged. Do not delete the `plan-task` half of that cell and do not change its meaning cell — the planning work is still driven by `plan-task`; only the *entering* step moved to the approval. This is a deliberate exception to requirement 1's "keep the other table rows' structure intact" (structure, not content), and the resulting count of three `task approve` occurrences still satisfies requirement 2's `≥ 2`.

4. **Add the CHANGELOG bullet.** In `CHANGELOG.md`, create the `## Unreleased` section **below the preamble and above `## v0.152.0`** (so the order reads `# Changelog` → preamble → `## Unreleased` → `## v0.152.0`), and add one bullet beginning `- feat:` that describes the new verb. The bullet must name the verb as the two-word form `task approve` and should describe: the `todo → planning` approval, the four values written together (`status: in_progress`, `phase: planning`, `approved_by`, `approved_at`), the `--by` flag with its `operator` default, and the refusals (a row not at `todo`, a row that already carries an approval record, and an empty approver). Match the surrounding entries' style: one long, dense bullet that says what changed and why, ending with a `Change set:` clause naming the touched files (`pkg/ops/task_approve.go`, `pkg/ops/workon.go`, `pkg/ops/frontmatter.go`, `pkg/cli/cli.go`, `docs/task-writing.md`, `commands/plan-task.md`). The fourth prompt changes `pkg/ops/workon.go` (the `work-on` bypass) and the fifth changes `pkg/ops/frontmatter.go`, so the bullet's `Change set` names both to be complete.

5. **Touch nothing else, with one required exception.** No new page under `docs/`, no change to any other section of `docs/task-writing.md`, no change to `README.md`, and no change to `pkg/`, `mocks/` or `integration/`.

   **The exception is `commands/plan-task.md` and it is load-bearing.** Its step 3 ("Entry contract — flip if needed", near lines 70-71) still tells the agent to move a `todo` row with `vault-cli task set "<name>" phase planning` — in **two** bullets, the `next`/`todo`/`backlog` case (near line 70) and the `status: in_progress` + `phase: todo`/empty case (near line 71); `grep -cE 'task set.*phase planning' commands/plan-task.md` returns 2 at HEAD and both must be gone. Its § Notes carries **two** entries stating the same bypass: near line 225 ("**Entry contract.** … flips to `in_progress, planning` itself") and near line 226 ("**No phase flip.** … Entry-contract flips (`next` → `in_progress` + `planning`) still happen in step 3"); update both. Both now describe the exact bypass spec 056 exists to close — a row at `planning` with no `approved_by`/`approved_at`. Change step 3 so a `todo` row is no longer moved by `task set`: plan-task must **refuse to plan a `todo` row** and tell the operator to run `vault-cli task approve <name>` first, then re-run `/vault-cli:plan-task`. Do **not** have plan-task call `vault-cli task approve` itself — that would record `approved_by: operator` for an approval the operator never gave, which spec 056's Assumptions treat as a false claim. Update the § Notes entry near line 225 to match, and leave the `status`-only promotions (`next` → `in_progress`) and the `planning → execution` hand-off to `/vault-cli:execute-task` unchanged. Touch no other command file. `commands/plan-goal.md` carries the identical unrecorded move (`goal set "<name>" phase planning`) and is deliberately out of scope — spec 056's Non-goals leave the goal-side gate to its own spec — so leave it untouched and do not report it as a blocker. The refusal stops in both modes and never calls `AskUserQuestion` under `--non-interactive`.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- **No version bump.** `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser` bumps `.claude-plugin/plugin.json`, both `.claude-plugin/marketplace.json` version fields, and renames `## Unreleased` to `## vX.Y.Z` post-merge. Do not hand-bump any of the four version strings, do not create a `## vX.Y.Z` section, and do not run `make release-check` — hand-bumping races the releaser. (`docs/task-writing.md` ships as part of the Claude Code plugin, so this change *will* be released — by the releaser, from the `## Unreleased` bullet you add here.)
- **The `## Unreleased` section goes below the preamble**, never between `# Changelog` and the preamble — `make precommit` runs `check-changelog`, which fails the build on that shape.
- **The bullet must be the first `- feat:` line in the file.** `## Unreleased` sits above `## v0.152.0`, and the AC's assertion (`awk '/^## /{s=$0} /^- feat:/{print s" | "$0}' CHANGELOG.md | head -1 | grep -c 'task approve'`) inspects only the first `- feat:` line. A bullet appended to the bottom of an existing released section would leave that assertion reading the pre-existing `## v0.152.0` bullet and return 0.
- **The `todo` row and the new prose are the only required mentions.** `grep -cE 'task approve' docs/task-writing.md` must reach ≥ 2 on the two-word verb; do not pad the count with the bare substring `approve`, which already matches twice at HEAD inside unrelated words and would make the assertion vacuous.
- **`approved_by` and `approved_at` must be named in `docs/task-writing.md`.** `grep -cE 'approved_(by|at)' docs/task-writing.md` must reach ≥ 1; this string occurs zero times at HEAD, so the assertion is only satisfiable by actually documenting the recorded fields.
- No new `docs/` page — § Phase transitions already owns the phase lifecycle and is the doc this spec updates.
- Existing tests must still pass; no Go file changes in this prompt, so `make test` should be unaffected. Note that `make precommit` runs `test`, and prompt 2 documents a known pre-existing integration failure (the `topic defer writes defer_date for a relative and an absolute date` spec, ~2 h each day when the local date leads the UTC date). Do not "fix" it opportunistically and do not let it block this prompt — a `make precommit` run failing on exactly that named spec and nothing else is a pass for this prompt.
</constraints>

<verification>
PRIMARY GATE — the spec's own container-executable evidence greps. Run each, record the count, and confirm it against the expectation:

```
grep -cE 'task approve' docs/task-writing.md                                          # >= 2 (the todo table row and the prose)
awk -F'|' '/^\| `todo`/ {print $0}' docs/task-writing.md | grep -c 'task approve'      # >= 1 (the table row itself names the verb)
grep -cE 'approved_(by|at)' docs/task-writing.md                                       # >= 1 (the recorded fields are named; 0 at HEAD)
awk '/^## /{s=$0} /^- feat:/{print s" | "$0}' CHANGELOG.md | head -1 | grep -cE '^## Unreleased \|.*task approve'   # >= 1 (the newest feat bullet names the verb under the ## Unreleased heading; 0 at HEAD)
grep -cE 'task set.*phase planning' commands/plan-task.md                       # == 0 (spec 056's plan-task AC: the bypass is gone, not discouraged; 2 at HEAD — both bullets)
grep -cE 'task approve' commands/plan-task.md                                   # >= 1 (spec 056's plan-task AC: the replacement names the verb; 0 at HEAD)
grep -cE 'task set "<name>" status in_progress' commands/plan-task.md           # >= 1 (spec 056's plan-task AC: the status-only promotion survives; 1 at HEAD — without this row a wholesale step-3 deletion passes the gate)
grep -A2 'Entry contract' commands/plan-task.md | grep -c 'task approve'         # >= 1 (spec 056's plan-task AC: step-3 heading area or the § Notes entry names it; 0 at HEAD)
```

⚠️ **Read the last row's intermediate output, not just its count.** The `awk … | head -1` must print a line whose section is `## Unreleased` (or, if the releaser has already cut a release, the newest `## vX.Y.Z`). If it prints `## v0.152.0 | …` with no `task approve` in it, the bullet landed in the wrong section — fix the placement rather than editing an older release's text.

STRUCTURE:
```
bash scripts/check-changelog.sh     # prints "CHANGELOG structure OK" and exits 0
grep -n '^## ' CHANGELOG.md | head -3    # first line must be "## Unreleased", second "## v0.152.0"
```

VERSION STRINGS UNTOUCHED — this prompt is not a release:
```
git diff --name-only                # must list only: docs/task-writing.md, CHANGELOG.md, commands/plan-task.md (assumes prompts 1 and 2 were committed first, which the sequential container run guarantees; if prompt 1 or 2's files also appear, stop and report rather than reverting them)
git diff --exit-code -- .claude-plugin/
```

FULL GATE — `make precommit` at the repo root must exit 0 (it runs `check-changelog` among its checks). If it fails on something this prompt introduced, fix it and re-run only the failing target, then `make precommit` once more.

⚠️ **`make precommit` runs `generate` (`rm -rf mocks` + `echo "package mocks" > mocks/mocks.go` + `go generate ./...`) as its third prerequisite (after `ensure` and `format`), before `test`, `check` and `addlicense`; `addlicense` then re-adds the copyright header, so `mocks/mocks.go` normally ends clean — restore it only if it shows as a diff.** If that file shows up as a diff, restore it as the **last** action, after the final `make precommit`:
```
git checkout -- mocks/mocks.go
git diff --exit-code -- mocks/mocks.go
```
</verification>
