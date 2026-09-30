---
status: approved
spec: [058-bug-approve-status-removes-row-from-spawn-offer]
created: "2026-09-30T07:10:08Z"
queued: "2026-09-30T07:20:47Z"
---

# Record the corrected approval status in the changelog (spec 058, prompt 2 of 2)

<summary>
- The changelog records the approval fix as an unreleased bug fix.
- A reader learns what the command wrote before, what it writes now, and why the old value mattered.
- The entry names the consequence the fix removes: an approved row was never offered to the fleet.
- The entry states what did not change, so nobody has to read the code to find out.
- The entry lands in the correct unreleased section, above the newest released section.
- The entry names the files the fix touched, so the change set is legible from the changelog alone.
- No version number is hand-written — the repository's release automation owns the bump and the release cut.
- The changelog's structure check stays green, which is part of the repository's pre-commit gate.
- Nothing else changes: no code, no documentation page, no README.
</summary>

<objective>
Add one `## Unreleased` `fix:` bullet to `CHANGELOG.md` describing the corrected approval write that prompt 1 of this spec makes, so the change is recorded under the repository's Definition of Done. Covers spec 058 Desired Behavior 6 and Acceptance Criterion 7. It depends on prompt 1, whose files the bullet's `Change set:` clause names; if prompt 1 is rejected at audit, reject this one too — the bullet would otherwise document behaviour the code does not have.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` § Documentation for the CHANGELOG placement rule.

Read fully (in this order):

- `CHANGELOG.md` — lines 1-20 only. At HEAD the file reads `# Changelog` (line 1), a blank line, the preamble ending with `* PATCH version when you make backwards-compatible bug fixes.` (line 9), a blank line, and `## v0.155.1` (line 11). **There is no `## Unreleased` section**, so this prompt creates one; the following sections are `## v0.155.0` (line 15) and `## v0.154.2` (line 20). Read the `## v0.155.1`, `## v0.155.0` and `## v0.154.2` bullets in full — they are the style this bullet must match, and `## v0.154.2`'s `- fix:` bullet is the shape to imitate.
- `docs/dod.md` § Documentation — the required order: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z` (newest first), with `## Unreleased` below the preamble and never between the `# Changelog` title and the preamble.
- `scripts/check-changelog.sh` — read the whole file (~40 lines). It fails the build when a `## ` section appears *above* the preamble line, so `## Unreleased` goes immediately below the preamble and immediately above `## v0.155.1`. This is the check `make precommit` runs via the `check-changelog` target.
- `specs/in-progress/058-bug-approve-status-removes-row-from-spawn-offer.md` — the `## Problem`, `## Why this is a bug`, `## Goal` and `## Non-goals` sections. They are the source of the facts the bullet must convey; quote them rather than re-deriving the story.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` placement rules, the frozen-preamble rule (`changelog/preamble-frozen`), and the list of recognised conventional prefixes (`fix:` is the right one here).
</context>

<requirements>
1. **Create the `## Unreleased` section in the right place.** In `CHANGELOG.md`, insert the heading and the bullet so the file reads:

   ```
   # Changelog

   All notable changes to this project will be documented in this file.

   Please choose versions by [Semantic Versioning](http://semver.org/).

   * MAJOR version when you make incompatible API changes,
   * MINOR version when you add functionality in a backwards-compatible manner, and
   * PATCH version when you make backwards-compatible bug fixes.

   ## Unreleased

   - fix: <the bullet from requirement 2>

   ## v0.155.1
   ```

   The heading goes **below** the preamble's last line (`* PATCH version when you make backwards-compatible bug fixes.`) and **above** `## v0.155.1`. Placing it between the `# Changelog` title and the preamble makes `scripts/check-changelog.sh` fail, which fails `make precommit`. Do not add a date suffix to the heading, do not add a `### Fixed` category heading, and do not reorder or edit any existing section. The preamble is frozen: do not move, delete or edit any of it.

   If a `## Unreleased` section already exists (a sibling change or the releaser created one), add the bullet to it instead of creating a second section — the file must never carry two. If the releaser has already renamed the section to a `## vX.Y.Z` on this branch, put the bullet under the newest `## vX.Y.Z` section instead: same bullet, same `fix:` prefix, and `<verification>`'s section walk accepts either.

2. **Write one `- fix:` bullet.** One bullet, starting with `- fix: `, matching the density and shape of the surrounding entries: a single long line that says what was wrong, why it mattered, and what changed, ending with a `Change set:` clause. It must convey all five of these, in the entry's own voice:

   - **What was written, and what is written now.** `vault-cli task approve` wrote `status: in_progress` together with `phase: planning`, `approved_by` and `approved_at`; it now writes `status: next` with those three unchanged. The approval records the operator's decision in the same single write as before — only the status value moves.
   - **Why the old value mattered — the consequence the fix removes.** The manager sweep's `ready-to-start` bucket requires `status: next` in **both** renderers: the vault's model-free sweep gate returns `🚀 ready-to-start` only inside `if status == "next"` and renders `🔄 progressing` for `status == "in_progress"`, and the claude-supervisor plugin's manager-sweep reader states the same rule in words. So the write that was supposed to release a row for spawn was the same write that disqualified it: **an approved row was never offered to the fleet at any point in its life** — before approval it sat at `phase: todo` held as waiting on the operator, and after approval it read as already owned. Measured 2026-09-30: two operator-approved rows had been swept for hours by two live managers and drew no action, the `To open` set was empty for both, and the only remaining route — the orphan/auto-resume gate — refused terminally on clause 11 (`mode=interactive`). State this as the observable consequence, not as a restatement of the value change.
   - **Why `next` is the correct value.** An approved row with no live session is *queued to be started*, not *active*: the vault's own status vocabulary defines `next` as queued and `in_progress` as active, and the plugin's own manager-sweep reader documents the `status: in_progress`-with-no-real-session shape as a missed dispatch rather than a false alarm. Writing `in_progress` asserted an owner that did not exist.
   - **What did not change.** `phase: planning`, `approved_by` and `approved_at` keep their exact spellings and formats and still travel with the status in a single write; the command's stdout is byte-identical in both the plain and the `--output json` forms; and all four refusals are unchanged — a row past the inbox, a row already carrying an approval key, an empty `--by`, and a row with no resolvable owner all still exit non-zero with their files untouched. Also state the scope: this changes what the command writes from now on, not existing data — rows approved before the change keep `status: in_progress` and stay out of the offer.
   - **The change set.** Close with a `Change set:` clause naming the files prompt 1 actually touched, in the form the neighbouring bullets use. Prompt 1's scope was widened at audit to include `commands/plan-task.md`, so the clause names **five** files: `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `integration/cli_test.go`, `docs/task-writing.md`, `commands/plan-task.md`: `` Change set: `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `integration/cli_test.go`, `docs/task-writing.md`, `commands/plan-task.md`. ``

   A shape that satisfies this (wording may differ; the five facts and the `Change set:` clause may not):

   ```
   - fix: `vault-cli task approve` now writes `status: next` instead of `status: in_progress`, so an approved row reaches the fleet's spawn offer. The approval transition had been writing the one status value the manager sweep reads as *someone already owns this*: the `ready-to-start` bucket requires `status: next` in both renderers — the vault's model-free sweep gate returns `🚀 ready-to-start` only inside `if status == "next"` and renders `🔄 progressing` for `status == "in_progress"`, and the claude-supervisor plugin's manager-sweep reader states the same rule in words — so the write meant to release a row for spawn was the same write that disqualified it. The net effect was that an approved row was never offered at any point in its life: before approval it sat at `phase: todo` held as waiting on the operator, and after approval it read as already in flight. Measured 2026-09-30: two operator-approved rows had been swept for hours by two live managers and drew no action, the `To open` set was empty for both, and the only remaining route — the orphan/auto-resume gate — refused terminally on clause 11 (`mode=interactive`). An approved row with no live session is *queued to be started*, not *active*: the vault's own status vocabulary defines `next` as queued and `in_progress` as active, and the plugin's reader documents the `in_progress`-with-no-real-session shape as a missed dispatch rather than a false alarm. Nothing else in the transition moves: `phase: planning`, `approved_by`, `approved_at` and the resolved assignee keep their exact spellings and formats and still travel in the same single write, the plain and `--output json` output are byte-identical, and all four refusals (a row past the inbox, a row already carrying an approval key, an empty `--by`, and a row with no resolvable owner) still exit non-zero with their files untouched. Rows approved before this change keep `status: in_progress` and stay out of the offer — this changes the writer, not the data. Change set: `pkg/ops/task_approve.go`, `pkg/ops/task_approve_test.go`, `integration/cli_test.go`, `docs/task-writing.md`, `commands/plan-task.md`.
   ```

3. **No version bump and no other file.** Do not create a `## vX.Y.Z` section, do not hand-bump any version string, and do not run `make release-check` or `make check-versions`. `.maintainer.yaml` sets `release.autoRelease: true`, so the releaser classifies the bump from the `## Unreleased` prefixes (`fix:` → patch), renames the section to `## vX.Y.Z`, and bumps the version strings in lockstep. Hand-bumping races it. Do not touch `README.md` either — the change alters no usage, configuration or setup, so `docs/dod.md`'s README criterion is met by not editing it. Do not touch `docs/`, `commands/`, `agents/`, `scenarios/`, `specs/`, `prompts/`, or any `.go` file: prompt 1 already made the code and doc changes and this prompt must not re-edit them.

4. **Do not hunt for artifacts outside this repository.** Spec 058 cites `sweep-gate.py` (the Personal vault's model-free sweep gate) and `agents/manager-sweep-reader.md` (the claude-supervisor plugin) as evidence for the bug. They are not in this repository. Naming them in the bullet's prose is fine and is how the surrounding bullets describe external mechanisms; opening, editing or searching for them is not — write the bullet from spec 058's own `## Problem` and `## Why this is a bug` sections.

5. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, including the `awk` section walk and the structure check. Then confirm the bullet is the **first** `- fix:` line in the file: `## Unreleased` sits above every `## vX.Y.Z`, and spec 058 AC 7's assertion inspects that first match, so a bullet appended to the bottom of an existing released section would leave the assertion reading a pre-existing bullet and report a false positive.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit. Note that in-container `git` does not work in this project directory (it is a git worktree, so `.git` is a `gitdir:` pointer file): any `git` command fails with `fatal: not a git repository`, and the daemon records a pass regardless of exit code. Use the non-git checks in `<verification>`.
- **The `## Unreleased` section goes below the preamble**, never between the `# Changelog` title and the preamble — `make precommit` runs `check-changelog`, which fails the build on that shape. The preamble (the `All notable changes…` line, the SemVer link and the three `* MAJOR / MINOR / PATCH` bullets) is frozen: do not move, delete or edit any of it.
- **No version bump.** The repository's release automation owns the version strings; this prompt writes a bullet under `## Unreleased` and nothing else. Do not create a `## vX.Y.Z` section and do not run `make release-check` or `make check-versions`.
- **One bullet, flat list.** No `### Fixed` category heading, no nested bullets, no multiple entries. The prefix must be exactly `fix:` — spec 058 corrects a bug, and the releaser classifies the bump from that prefix.
- **The bullet must be the first `- fix:` line in the file.** `## Unreleased` sits above `## v0.155.1`, so a correct placement makes it the first match; an entry appended to an existing released section does not satisfy spec 058 AC 7's assertion.
- **This prompt changes `CHANGELOG.md` and nothing else.** `pkg/ops/`, `integration/`, `docs/`, `commands/`, `agents/`, `scenarios/`, `README.md` and `.dark-factory.yaml` are prompt 1's or nobody's scope.
- **The facts in the bullet are frozen by the code change prompt 1 made.** The bullet must not describe a behaviour change beyond the status value: `phase`, the two approval keys, the resolved assignee, the single-write property, the stdout shape and all four refusals are unchanged, and existing approved rows are not repaired.
- **Do not name the artifact that consumes the classification as a modification target.** The vault's sweep gate and the plugin's manager-sweep reader are external evidence; the bullet describes the rule they apply, not a change to them.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…`, which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
</constraints>

<verification>
**Structure check — the gate `make precommit` runs:**

```
bash scripts/check-changelog.sh
```

must print `CHANGELOG structure OK` and exit 0.

**The section exists, and only once:**

```
grep -n '^## Unreleased' CHANGELOG.md
```
must print exactly one line (at HEAD it prints nothing — this prompt creates the section). On the fallback path — a releaser that already renamed the section on this branch — expect zero lines here and rely on the section walk below for placement.

**Placement — the section order:**

```
grep -n '^## ' CHANGELOG.md | head -3
```
must print three lines in this order: `## Unreleased`, then `## v0.155.1`, then `## v0.155.0`. (On a branch where the releaser has already renamed `## Unreleased` to a `## vX.Y.Z`, expect the newest `## vX.Y.Z` first and no `## Unreleased` line at all — the section walk below accepts either.)

**Section walk — the bullet sits under `## Unreleased`, not under a released heading.** Use the section-walking form, never a line-window `grep -A`, which swallows a neighbouring section and reports a false positive. At HEAD the first `- fix:` sits under `## v0.154.2`, so this check is non-vacuous:

```
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec; exit}' CHANGELOG.md
```
must print exactly `sits under: ## Unreleased` — or, on a branch where the releaser has already renamed the section, `sits under: ## vX.Y.Z` (same bullet, same `fix:` prefix; spec 058 AC 7).

**Bullet content:**

```
awk '/^## Unreleased/{u=1;next} u&&/^## /{u=0} u&&NF{c++} END{print c+0}' CHANGELOG.md   # 1 — the section holds exactly one non-blank line: the bullet, unwrapped
grep -c '^- fix:' CHANGELOG.md                                              # 166 — 165 at HEAD plus exactly one
awk '/^## /{sec=$0} /Change set:/{print sec; exit}' CHANGELOG.md              # ## Unreleased — the clause sits in the new section, not a released one
```

**Full gate:**

```
make precommit
```
must exit 0. It runs `ensure format generate test check addlicense`, and `check` includes `check-changelog`. If it fails on something other than this prompt's change, report the exact failure rather than editing an unrelated file. `generate` wipes and regenerates `mocks/` and `addlicense` re-adds the copyright header, so `mocks/mocks.go` normally ends clean — restore it only if it shows as a diff.

**Scope check (secondary).** In-container `git` does not work here, so confirm the scope from the file itself rather than from `git diff --name-only`:

```
grep -c '^## Unreleased' CHANGELOG.md   # 1 — this prompt did not create a second section
head -11 CHANGELOG.md                    # the preamble is byte-identical to HEAD; line 11 is the ## Unreleased heading
```

**SELF-CHECK before finishing:** re-run the steps above and confirm each printed line, then confirm the new bullet is the first `- fix:` line in the file and that the bullet names the corrected status write, the consequence it removes (an approved row never reaching the spawn offer), what did not change, and the four-file `Change set:`.
</verification>
