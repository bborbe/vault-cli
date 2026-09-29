---
spec: ["057-bug-topic-defer-local-basis"]
status: draft
created: "2026-09-29T06:19:28Z"
---

# Record the topic defer test-basis repair in the CHANGELOG (spec 057, prompt 2 of 2)

<summary>
- The changelog records the repaired date basis as an unreleased bug fix.
- A reader learns what disagreed, on which side, and that the command's output is unchanged.
- The entry lands in the correct unreleased section, above the newest released section.
- The entry names the files the repair touched, so the change set is legible from the changelog alone.
- No version number is hand-written — the repository's release automation owns the bump and the release cut.
- The changelog's structure check stays green, which is part of the repository's pre-commit gate.
- Nothing else changes: no code, no documentation page, no README.
</summary>

<objective>
Add one `## Unreleased` `fix:` bullet to `CHANGELOG.md` describing the test-basis repair prompt 1 of this spec makes, so the change is recorded under the repository's Definition of Done. Covers spec 057 Acceptance Criterion 8. It depends on prompt 1, whose files the bullet's `Change set:` clause names.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` § Documentation for the CHANGELOG placement rule.

Read fully (in this order):
- `CHANGELOG.md` — lines 1-30 only. At HEAD the file reads `# Changelog` (line 1), a blank line, the preamble ending with `* PATCH version when you make backwards-compatible bug fixes.` (line 9), a blank line, and `## v0.153.1` (line 11). **There is no `## Unreleased` section**, so this prompt creates one; the following sections are `## v0.153.0` (line 15), `## v0.152.0` (line 22), `## v0.151.2` (line 26). Read the `## v0.153.0` and `## v0.153.1` bullets in full — they are the style this bullet must match.
- `docs/dod.md` § Documentation — the required order: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z` (newest first).
- `scripts/check-changelog.sh` — read the whole file (~40 lines). It fails the build when a `## ` section appears *above* the preamble line, so `## Unreleased` goes immediately below the preamble and immediately above `## v0.153.1`. This is the check `make precommit` runs via the `check-changelog` target.
- `docs/releasing-vault-cli.md` — § Version alignment (near line 16) and § Binary release (near line 80) only. They establish that `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser-agent` owns the version bump: it classifies the semver bump from the `## Unreleased` bullet prefixes, rewrites `## Unreleased` → `## vX.Y.Z`, and bumps the four version strings in lockstep. Nothing in this prompt touches a version string.

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

   ## v0.153.1
   ```

   The heading goes **below** the preamble's last line (`* PATCH version when you make backwards-compatible bug fixes.`) and **above** `## v0.153.1`. Placing it between the `# Changelog` title and the preamble makes `scripts/check-changelog.sh` fail, which fails `make precommit`. Do not add a date suffix to the heading, do not add a `### Fixed` category heading, and do not reorder or edit any existing section.

2. **Write one `- fix:` bullet.** One bullet, starting with `- fix: `, matching the density and shape of the surrounding entries: a single long line that says what was wrong, why it mattered, and what changed, ending with a `Change set:` clause. It must convey all four of these, in the entry's own voice:
   - the `topic defer` integration spec computed its expected `defer_date` from the **UTC** calendar date while the CLI resolves it from the process's **local** calendar date — `libtime.ToDate` takes the calendar date in the time's own location — so the two disagreed by exactly one day whenever the local date differed from the UTC date;
   - the consequence that made it load-bearing: `make test` went red on a tree with no changes during that window, and because `.dark-factory.yaml` sets `preflightCommand: "make precommit"` and a preflight failure is terminal, dark-factory could not start any prompt on this repository for the duration;
   - the second half of the defect: `integration/integration_suite_test.go` assigned `time.Local = time.UTC` in the **test process**, which made the parent agree with the UTC expectation but never reached the spawned binary — and overrode `TZ` for the parent, which is why pinning `TZ` alone moved only one side;
   - what changed: the expectation now reads on the local calendar date and is asserted against both candidate bases, the parent-process zone assignment is gone so a `TZ` selected for a run governs the test process and the binary it spawns alike, the local-basis decision is documented where it is implemented, and a unit case pins the basis without depending on the host zone.

   Close with a `Change set:` clause naming the four files prompt 1 touched, in the form the neighbouring bullets use: `` Change set: `integration/cli_test.go`, `integration/integration_suite_test.go`, `pkg/ops/defer_date_parser.go`, `pkg/ops/topic_defer_test.go`. ``

   State explicitly in the bullet that the CLI's user-visible output is unchanged — `defer_date` was already the local-basis date and stays that way, so no user sees a different date after this change. That is the fact a reader most needs, and it is what keeps the entry a `fix:` rather than a `feat:`.

   A shape that satisfies this (wording may differ; the four facts, the unchanged-output statement and the `Change set:` clause may not):

   ```
   - fix: Repair the date basis in the `topic defer` integration spec, which computed its expected `defer_date` from the UTC calendar date while the CLI resolves it from the process's local calendar date (`libtime.ToDate` takes the calendar date in the time's own location), so the two disagreed by exactly one day whenever the local date differed from the UTC date and `make test` went red on an unchanged tree. Because `.dark-factory.yaml` sets `preflightCommand: "make precommit"` and a preflight failure is terminal, that window blocked every dark-factory run on this repository. The suite's own `time.Local = time.UTC` assignment hid the divergence: it made the test process agree with the UTC expectation but never reached the spawned binary, and it overrode `TZ` for the parent, which is why pinning `TZ` alone moved only one side. The expectation now reads on the local calendar date and is asserted against both candidate bases, the parent-process zone assignment is gone so a `TZ` selected for a run governs the test process and the binary it spawns alike, the local-basis decision is documented above the resolution in `pkg/ops/defer_date_parser.go`, and a unit case pins the basis with an explicit non-UTC location, independently of the host zone. No user-visible behaviour changes: `defer_date` was already the local-basis date and stays that way. Change set: `integration/cli_test.go`, `integration/integration_suite_test.go`, `pkg/ops/defer_date_parser.go`, `pkg/ops/topic_defer_test.go`.
   ```

3. **No version bump and no other file.** Do not create a `## vX.Y.Z` section, do not hand-bump any version string, and do not run `make release-check` or `make check-versions`. `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser-agent` classifies the bump from the `## Unreleased` prefixes (`fix:` → patch), renames the section to `## vX.Y.Z`, and bumps `CHANGELOG.md`, `.claude-plugin/plugin.json` and both `.claude-plugin/marketplace.json` version fields in lockstep. Hand-bumping races it. Do not touch `README.md` either — the change is test-side and alters no usage, configuration or setup. Do not touch `docs/`, `commands/`, `scenarios/`, or any `.go` file.

4. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, including the `awk` section walk and the structure check. Then confirm the bullet is the **first** `- fix:` line in the file: `## Unreleased` sits above every `## vX.Y.Z`, and spec 057 AC 8's assertion inspects that first match, so a bullet appended to the bottom of an existing released section would leave the assertion reading a pre-existing bullet and report a false positive.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. `git diff` and `git diff --name-only` only read; never stage or commit.
- **The `## Unreleased` section goes below the preamble**, never between the `# Changelog` title and the preamble — `make precommit` runs `check-changelog`, which fails the build on that shape. The preamble (the `All notable changes…` line, the SemVer link and the three `* MAJOR / MINOR / PATCH` bullets) is frozen: do not move, delete or edit any of it.
- **No version bump.** The repository's release automation owns the version strings; this prompt writes a bullet under `## Unreleased` and nothing else. Do not create a `## vX.Y.Z` section and do not run `make release-check` or `make check-versions`.
- **One bullet, flat list.** No `### Fixed` category heading, no nested bullets, no multiple entries. The prefix must be exactly `fix:` — spec 057 is a test-basis repair, not a new capability.
- **The bullet must be the first `- fix:` line in the file.** `## Unreleased` sits above `## v0.153.1`, so a correct placement makes it the first match; an entry appended to an existing released section does not satisfy spec 057 AC 8's assertion.
- This prompt changes `CHANGELOG.md` and nothing else. `integration/`, `pkg/`, `mocks/`, `docs/`, `commands/`, `scenarios/` and `.dark-factory.yaml` are prompt 1's or nobody's scope.
- The CLI's `defer_date` semantics are frozen: relative offsets resolve from the local calendar date, exactly as today. The bullet must not describe a behaviour change, because there is none.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…`, which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
</constraints>

<verification>
**Structure check — the gate `make precommit` runs:**

```
bash scripts/check-changelog.sh
```
must print `CHANGELOG structure OK` and exit 0.

**Placement — the section order:**

```
grep -n '^## ' CHANGELOG.md | head -3
```
must print three lines in this order: `## Unreleased`, then `## v0.153.1`, then `## v0.153.0`. (If the releaser has already cut a release on this branch, the newest `## vX.Y.Z` takes the second line instead — the heading itself must still be `## Unreleased`.)

**Section walk — the bullet sits under `## Unreleased`, not under a released heading.** Use the section-walking form, never a line-window `grep -A`, which swallows a neighbouring section and reports a false positive:

```
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec}' CHANGELOG.md | head -1
```
must print exactly `sits under: ## Unreleased` — or, on a branch where the releaser has already cut a release, `sits under: ## vX.Y.Z` (same bullet, same `fix:` prefix; spec 057 AC 8).

**Bullet content:**

```
awk '/^## /{sec=$0} /^- fix:/{print "sits under: " sec}' CHANGELOG.md | head -1   # ## Unreleased
awk '/^## Unreleased/{u=1} u&&/^- fix:/{c++} END{print c+0}' CHANGELOG.md          # 1 — exactly one bullet under ## Unreleased, and it is a single line
grep -n 'Change set:' CHANGELOG.md | head -1                                       # the new bullet's clause, in the ## Unreleased region
grep -c '^- fix:' CHANGELOG.md                                                      # 162 — 161 at HEAD plus exactly one
```

**Full gate:**

```
make precommit
```
must exit 0. It runs `ensure format generate test check addlicense`, and `check` includes `check-changelog`. If it fails on something other than this prompt's change, report the exact failure rather than editing an unrelated file. `generate` wipes and regenerates `mocks/` and `addlicense` re-adds the copyright header, so `mocks/mocks.go` normally ends clean — restore it only if it shows as a diff.

**Scope check (secondary; git is available here, `.dark-factory.yaml` is `workflow: direct` with no `hideGit`):**

```
git diff --name-only
```
must list exactly `CHANGELOG.md`. Any second file means scope leaked — fix it rather than reporting it.

Before finishing, re-run the steps above and confirm each printed line, then confirm the new bullet is the first `- fix:` line in the file.
</verification>
