---
status: prompted
approved: "2026-10-19T17:49:55Z"
generating: "2026-10-19T17:51:03Z"
prompted: "2026-10-19T18:03:01Z"
branch: dark-factory/bug-rollup-aborts-on-frontmatterless-task-file
---

## Summary

- The weekly rollup aborts on a single task file it cannot parse, instead of skipping it — one malformed file blocks the whole weekly measurement.
- The abort is deliberate today: the rollup asks for a strict listing, and that listing fails loudly on the first unreadable file.
- Failing loudly is the right instinct and the wrong outcome here: the rollup can name the file it skipped and still report the figures for every file it read.
- The fix reads what it can, reports the figures for it, and names the files it left out — in both the plain report and the JSON output.
- The strict listing keeps its current contract untouched; the rollup stops being one of its callers.

## Problem

The rollup's job is to answer "what happened in this vault in this ISO week". It reads the task population with the strict listing, which aborts on the first unparseable file. A vault can accumulate a malformed task file from an unrelated failure — the live case is a 72-byte file whose entire body is `Failed to authenticate: OAuth session expired and could not be refreshed`, written by a producer that failed mid-write — and from that moment every weekly rollup for that vault fails, for every week, until a human repairs or deletes the file.

The failure mode is worse than a missing figure: it is an unusable command. The operator cannot measure the week at all, and the error names a file whose repair is somebody else's task. `vault-cli` already has the right shape for this in the tolerant sibling listing (`pkg/storage/task.go:80-101`), which skips and logs — but it logs at `slog.Debug` with the filename only in that debug record, so nothing user-visible names what it skipped. Neither existing path is the right one: one is too loud, the other too quiet.

The correct behaviour is the third option: read what is readable, report the figures for it, and name the files that were left out. That is the rule the rollup's own figures already follow — an absent measurement is never rendered as a measurement of nothing — applied to the population instead of to a median.

## Assumptions

- The malformed file is a *task* file under the configured `tasks_dir`, not a page, goal or theme. The rollup walks only tasks.
- "Unparseable" means the per-file read/parse error that `walkTaskFiles` hands to its callback (`pkg/storage/task.go:134-160`). A walk-level error — a missing or unreadable `tasks_dir` — stays fatal; a missing vault is a different failure and must not be reported as "everything was skipped".
- The tasks directory supports subdirectories, so a skipped name must be the path **relative to `tasks_dir`**, not the bare basename: two files named `Broken.md` in different subdirectories must not render as one entry.
- `.dark-factory.yaml` is `workflow: direct`, `pr: false`, `autoRelease: false`; `.maintainer.yaml` is `release.autoRelease: true`. The daemon commits to the current branch; the github-releaser owns the tag and the four version strings post-merge.

## Reproduction

Reproduced 2026-10-19 against the installed binary and the live vault.

1. The vault `private-personal` holds a task file with no frontmatter block: `25 Tasks/Analyze Sentry issue 11fec2f87fcbddd8ad06ea8264a170c4 - 2026-10-17.md`, 72 bytes, body `Failed to authenticate: OAuth session expired and could not be refreshed`.
2. Run `vault-cli rollup weekly --vault private-personal --week 2026-W42` (installed `vault-cli version v0.167.1`).
3. Observed — exit **1**, no figures printed:

   ```
   Error: list tasks of vault private-personal: walk tasks directory /Users/bborbe/Documents/Obsidian/private-personal/25 Tasks: read task file /Users/bborbe/Documents/Obsidian/private-personal/25 Tasks/Analyze Sentry issue 11fec2f87fcbddd8ad06ea8264a170c4 - 2026-10-17.md: parse frontmatter: no frontmatter found
   ```

4. The same command against a vault whose task files all parse exits 0 and prints the report — the abort is caused by the one file, not by the week token (the week selector is never reached; the abort precedes it).

### Fixture

The storage's tasks directory defaults to `Tasks` when the vault config does not override it (`pkg/config/config.go`), and the existing rollup tests write there. The fixture therefore uses `Tasks/`; the live `private-personal` vault config sets `tasks_dir: 25 Tasks`, which is why the reproduction names that path.

```bash
VAULT=/tmp/rollup-skip-fixture
mkdir -p "$VAULT/Tasks"
# One healthy completed task with a recorded zero: a recorded zero is a measurement,
# so it contributes 0 human interactions and exactly 1 unattended delivery.
printf -- '---\nstatus: completed\nmetrics_completed_at: "2026-10-14T10:00:00Z"\nmetrics_interaction_count: 0\n---\n# Healthy\n' \
  > "$VAULT/Tasks/Healthy.md"
# One task file with no frontmatter at all.
printf 'Failed to authenticate: OAuth session expired and could not be refreshed\n' \
  > "$VAULT/Tasks/Broken.md"
```

Expected figures for this fixture: `Human interactions: 0`, `Unattended deliveries: 1`, one skipped entry `Broken.md`.

### Mechanism (verified against source at `origin/master`, 2026-10-19)

- `pkg/ops/rollup_weekly.go:155` — `tasks, err := o.taskStorage.ListTasksStrict(ctx, vaultPath)`; on a non-nil error the operation returns `RollupWeeklyResult{}` wrapped as `list tasks of vault <name>` (`:157`).
- `pkg/storage/task.go:107-127` — `ListTasksStrict` calls `walkTaskFiles` and, on any per-file error, returns `errors.Wrapf(ctx, err, "read task file %s", path)` — aborting the walk.
- `pkg/storage/task.go:134-160` — `walkTaskFiles` walks `tasks_dir` recursively and hands each per-file error to the callback rather than handling it, which is what lets the two callers differ.
- `pkg/storage/task.go:80-101` — `ListTasks` is the tolerant sibling, but logs at `slog.Debug` with no filename in any user-visible output.
- `pkg/ops/rollup_weekly.go:104-115` — `RollupWeeklyResult` has no skipped field; the plain renderer is `pkg/cli/rollup.go:114-122`, and the JSON branch calls `PrintJSON(result)` at `pkg/cli/rollup.go:100` (`PrintJSON` itself is defined in `pkg/cli/output.go`).
- `pkg/ops/rollup_weekly.go:262-276` — `rollupFigures`: a task counts toward the human-interaction sum by its `metrics_interaction_count` (read via `MetricsInteractionCount` in `pkg/domain/task_frontmatter_metrics.go`), and an unattended delivery is a *recorded zero* on a completed task. Unattended is derived, never stored.

## Expected vs Actual

| | Expected | Actual |
|---|---|---|
| Exit code, one frontmatter-less task file | 0 | 1 |
| Figures for the readable files | printed | not printed |
| The skipped file | named in a `skipped:` line | only named inside the error, with no figures |
| Effect of an unrelated malformed file | bounded to that file | blocks every week of the vault |
| JSON output | a `skipped` array | no field, no output at all |

## Why this is a bug

The rollup's contract is "report one ISO week's task metrics for a single vault". A file the command cannot read is not a reason to refuse to report on the files it can read — and `vault-cli` already made that judgement elsewhere in the same package family: `ListTasks` skips, and the page loader warns and continues (`pkg/storage/page.go`). The strict method's doc comment (`pkg/storage/task.go:103-106`) states its purpose correctly — "so a caller that must not report a silently smaller set fails loudly" — and the rollup satisfies the letter of that by failing, but violates its own purpose by producing no report at all. Failing loudly and reporting loudly are both available; only the second keeps the measurement.

## Goal

`vault-cli rollup weekly` reports every week it can measure: it reads the task files it can parse, computes the figures from them, and names each file it skipped on a `skipped:` line — in both plain and JSON output — while the strict listing and every other command keep their current behaviour.

## Non-goals

- Fixing the malformed task file itself (`25 Tasks/Analyze Sentry issue … - 2026-10-17.md`) — separate, in the vault.
- Changing how any other command treats a frontmatter-less file. `task list`, `goal`, `theme` and the page loader are untouched.
- Weakening the strict listing. It keeps aborting for any caller that wants the loud contract; the rollup simply stops being such a caller.
- Repairing or migrating malformed files automatically. The rollup reports; it does not write.

## Do-Nothing Option

Doing nothing leaves the vault's weekly measurement unusable until an unrelated file is repaired or deleted — and the failure recurs for every future week, since the walk aborts before the week is ever resolved. The cost is not one missing figure: it is a command whose whole contract (report a week) is denied by a file that belongs to a different concern. The alternative fixes are both wrong in a way this spec already rules out: the strict path (status quo) loses the report, and the silent tolerant path loses the disclosure. The tolerant-and-reporting path is the only one that keeps both.

## Acceptance Criteria

Each AC declares an evidence shape and the fixture it is measured on. Expected values are pinned so a test cannot pass by asserting `no data`.

- [ ] **AC1 — the rollup skips and lists.** One frontmatter-less task file plus one healthy completed task with `metrics_interaction_count: 0` in the target week yields: no error, the pinned figures, and exactly one skipped entry naming the bad file.
  - fixture: the `# Reproduction` fixture.
  - evidence: a Ginkgo test over `RollupWeeklyOperation.Execute` (reusing `writeRollupTask` / `rollupTaskFrontmatter` from `pkg/ops/rollup_weekly_test.go`) asserts `err == nil`, `HumanInteractions == "0"`, `UnattendedDeliveries == "1"`, and exactly one skipped name equal to `Broken.md`. Pre-fix `Execute` returns an error and no result → fails.
- [ ] **AC2 — the skipped name is the path relative to `tasks_dir`.** A frontmatter-less file in a subdirectory of `tasks_dir` is reported as `<subdir>/<name>.md`, and two same-named files in different subdirectories appear as two distinct entries.
  - fixture: `Tasks/sub-a/Broken.md` and `Tasks/sub-b/Broken.md`, both frontmatter-less, plus the healthy task.
  - evidence: a Ginkgo test asserts the result's skipped list is exactly `["sub-a/Broken.md", "sub-b/Broken.md"]` in that order (sorted). Pre-fix no such list exists → fails.
- [ ] **AC3 — the plain report prints one `skipped:` line naming every file, as the last line.** With ≥1 skipped file the plain output contains a line beginning `skipped:` that names each file, and that line is the final line of the output; with 0 skipped files no `skipped:` line is printed at all.
  - fixture: the AC1 fixture; and a clean vault holding only a completed task with `metrics_interaction_count: 2`.
  - evidence: a test over `formatRollupWeeklyPlain` asserts the output contains exactly one line matching `^skipped: `, that the line contains the bad filename, and that it is the last line (`strings.TrimRight(out, "\n")` ends with that line); and, for the clean vault, asserts `strings.Contains(output, "skipped:")` is false. Pre-fix the function is unreachable (Execute errors) → fails.
- [ ] **AC4 — JSON carries a `skipped` array, empty when nothing was skipped.** `vault-cli rollup weekly --output json` emits `"skipped":["Broken.md"]` for the fixture and `"skipped":[]` for the clean vault — an array, never `null`.
  - fixture: both above.
  - evidence: a test marshals the result with `json.Marshal` (not `PrintJSON`, which writes to `os.Stdout`) and asserts `skipped` is a JSON array of length 1 (fixture) / 0 (clean), and that the raw JSON contains `"skipped":[]` rather than `"skipped":null` for the clean case. Pre-fix the field is absent → fails.
- [ ] **AC5 — the strict listing is unchanged (negative).** A frontmatter-less task file still makes `ListTasksStrict` return an error naming the file.
  - evidence: a `pkg/storage` test asserts `ListTasksStrict` returns a non-nil error whose message contains the bad filename; `git diff "$(git merge-base HEAD origin/master)" -- pkg/storage/task.go` shows additions only (a new method), never an edit inside `ListTasksStrict`'s body.
- [ ] **AC6 — a walk-level failure stays fatal (negative).** A vault whose `tasks_dir` does not exist still returns an error from the rollup rather than an empty report.
  - evidence: a test asserts `Execute` returns an error for a vault path with no `Tasks/` directory. Pre-fix and post-fix both pass — this is the invariant the skip must not swallow.
- [ ] **AC7 — regression lock fails pre-fix.** AC1–AC4 fail against the pre-fix implementation.
  - evidence: with the fix's source changes reverted (tests kept), `make test` reports the AC1/AC2/AC3/AC4 specs failing; restored, `make test` passes. Record the pre-fix run's output in the PR body.
- [ ] **AC8 — `make precommit` green, a `## Unreleased` bullet, and the README's rollup section updated.** `make precommit` exits 0, `CHANGELOG.md` carries a `fix:` bullet under `## Unreleased`, and `README.md` § rollup documents the `skipped:` line and the `skipped` JSON field.
  - evidence: `make precommit` exits 0; the new bullet is pinned by `awk '/^## /{sec=$0} /^- fix:.*rollup.*skipped/ {print sec}' CHANGELOG.md | sort -u`, which must print exactly `## Unreleased` (a bare `/rollup/` match would also print the historical `## v0.149.0`, `## v0.148.0` and `## v0.139.0` sections); `sed -n '/^### rollup/,/^## /p' README.md | grep -n 'skipped'` returns ≥1 line, scoping the check to the rollup section. If the repo has no `## Unreleased` section at implementation time, the prompt creates it below the preamble and above the newest versioned section.
- [ ] **Post-Deploy (Rung-2):** the installed binary runs the original reproduction against the live vault and exits 0, printing the figures and a `skipped:` line naming the bad file.
  - evidence: `vault-cli rollup weekly --vault private-personal --week 2026-W42` exits 0, the output contains the three headline figures, and the `skipped:` line contains `Analyze Sentry issue 11fec2f87fcbddd8ad06ea8264a170c4 - 2026-10-17.md`. This live run is the authoritative check — a `deploy_check:` match alone does not satisfy it, because before the release the newest tag and the installed binary both predate the fix.
  - `deploy_check:` `vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git describe --tags --abbrev=0 origin/master)`

## Verification

### Container-executable (runs inside the YOLO container at prompt time)

- `make precommit` — lint / format / generate / test / checks clean.
- `make test` — unit + integration suite passes, including the new `pkg/storage`, `pkg/ops` and `pkg/cli` specs.
- `git status --short mocks/` — clean after `make precommit`, proving the counterfeiter fakes were regenerated rather than left stale.
- `grep -n 'skipped' pkg/ops/rollup_weekly.go` — the result field exists.
- `grep -n 'skipped:' pkg/cli/rollup.go` — the plain line exists.

### Operator-executable (runs on the host after PR merge, spec verification ladder)

- Build the post-merge tree — run these as separate commands in the main checkout (`~/Documents/workspaces/vault-cli`): `git fetch origin`, then `git merge origin/master`, then `go build -o /tmp/new-vault-cli .`.
- Run the fixture walk and the live check (see `# Desired Behavior` DB5) against `/tmp/new-vault-cli`.
- After the github-releaser tags: release gate per `docs/releasing-vault-cli.md`, `make install`, and re-run the live command from the installed binary.

## Desired Behavior

1. **Tolerant listing, with the skip boundary fixed.** The task storage gains a way to list tasks that also returns the names it skipped. A per-file read/parse error is skipped and recorded; a walk error (missing or unreadable `tasks_dir`) is returned. The boundary is exactly the one `walkTaskFiles` already draws. It is an addition; the existing listings keep their current bodies and contracts.
2. **Skipped names are `tasks_dir`-relative paths**, sorted, so subdirectory files are unambiguous and two same-named files stay distinct.
3. **Result carries the skipped names.** The rollup result serializes them as an array under `"skipped"`, always an array — empty when nothing was skipped, never `null`.
4. **Plain line.** When at least one file was skipped, the plain report prints exactly one line beginning `skipped:` that names every skipped path. The line is the **last line of the report**, after the delta block, so the figures block's layout is unchanged. When none was skipped, the line is absent — a clean run prints no skip noise.
5. **Runtime confirmation (the mandatory bug check — the original reproduction).** Against the live vault, `vault-cli rollup weekly --vault private-personal --week 2026-W42` exits **0**, prints the figures, and prints a `skipped:` line naming `Analyze Sentry issue 11fec2f87fcbddd8ad06ea8264a170c4 - 2026-10-17.md`. Against the fixture vault, exit 0 with `Human interactions: 0`, `Unattended deliveries: 1`, and one skipped entry.
6. **Figures exclude skipped files.** The skipped files contribute nothing to any figure, and the figures are not rendered as `no data` merely because something was skipped — the readable population is measured on its own terms.

## Constraints

- **Frozen interface shape.** The tolerant listing returns `(tasks []*domain.Task, skipped []string, err error)`. It is added to the `TaskStorage` interface in `pkg/storage/storage.go`; `Storage` embeds `TaskStorage`, so both counterfeiter fakes (`mocks/task-storage.go`, `mocks/storage.go`) must be regenerated with `make generate`. The result field is `Skipped []string` with JSON tag `skipped`.
- **`pkg/ops/` stays output-free.** The operation returns structured data; all formatting stays in `pkg/cli/`. The rollup's plain/JSON contract is documented in `README.md` § rollup; `docs/output-formatting.md` covers only the list JSON read model, not this report.
- **Do not edit `ListTasksStrict`.** AC5 pins its behaviour and its body; the tolerant path is a new method.
- **Do not change `walkTaskFiles`'s callback contract.** It already passes `(task, name, path, err)`, which is everything the tolerant method needs — the relative name comes from `filepath.Rel` against `tasks_dir`, not from a new callback parameter. The two existing callers depend on this signature; a change here is a defect, not a judgement call.
- **Skip only per-file errors.** A walk error must still abort (AC6). Swallowing a missing directory would turn "the vault is misconfigured" into "the vault has no tasks", which is the absent-is-not-zero rule inverted.
- **One physical line per report.** A skipped name containing a newline or other control character is rendered with each control character replaced by a single space, so the `skipped:` line stays one line and AC3's "exactly one line" invariant holds.
- **No `skipped: 0` line and no `null` array.** Zero skipped files means the line is absent and the JSON array is `[]`.
- **No hand-bump of version strings or tags.** `.maintainer.yaml: release.autoRelease: true` — the github-releaser owns the tag and bumps all four version strings post-merge. Add a `## Unreleased` bullet and let it run. (`.dark-factory.yaml: autoRelease: false` is the dark-factory driver, deliberately off; it does not contradict this.)

## Security / Abuse Cases

Not a security fix. The skipped names are a new **output** surface: they are `tasks_dir`-relative paths printed to stdout, already derivable from the vault the command is pointed at. The only new risk is report integrity — a filename carrying a newline could forge extra lines — which the control-character replacement in `## Constraints` closes. No new input surface; the command still reads only the configured vault.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---|---|---|
| One frontmatter-less task file | skipped, named on the `skipped:` line, figures printed, exit 0 | none needed |
| Several unparseable files | all named on the one `skipped:` line | none needed |
| Same-named files in different subdirectories | two distinct `tasks_dir`-relative entries | none needed |
| Filename containing a newline | rendered on one line, control characters replaced by spaces | none needed |
| `tasks_dir` missing | error returned, no report | fix the vault config — never reported as "all skipped" |
| Every task file unparseable | exit 0, all named, figures rendered as `no data` (an empty readable population is the existing no-data path) | repair the files |
| Tolerant path accidentally edits `ListTasksStrict` | (must not happen) the loud contract silently weakens | AC5's diff and error test fail; revert the stray edit |
| Skip list rendered as `null` in JSON | (must not happen) consumers cannot distinguish "none" from "absent" | AC4's array assertion fails |

## Suggested Decomposition

Prompts should be generated in this order — each row is a single prompt with a clear scope.

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Tolerant `TaskStorage` method returning `(tasks, skipped, err)`: the interface entry in `pkg/storage/storage.go`, the `mocks/task-storage.go` + `mocks/storage.go` regeneration (`make generate`), skip boundary, relative sorted names, control-character replacement; unit tests incl. the subdirectory fixture and the missing-`tasks_dir` invariant | 1, 2 | 2, 5, 6 | — |
| 2 | `RollupWeeklyResult.Skipped`, the plain `skipped:` line (last line), the JSON array; renderer + operation tests with the pinned figures | 3, 4, 6 | 1, 3, 4 | 1 |
| 3 | Regression lock (AC1–AC4 fail pre-fix), `## Unreleased` `fix:` bullet, README rollup section, `make precommit` | — | 7, 8 | 1, 2 |

Rationale: prompt 1 establishes the tolerant read and its boundary; prompt 2 surfaces it through the result and both renderers; prompt 3 adds the pre-fix-fails evidence and the changelog/README entries once both layers exist. AC6's invariant and AC5's negative both belong to prompt 1, since they constrain the boundary it introduces. The interface and both counterfeiter fakes are prompt 1's, not an afterthought: `Storage` embeds `TaskStorage` (`pkg/storage/storage.go`), so adding a method without regenerating breaks `make precommit`.

**DB5 is not a prompt.** It runs on the host after the PR merges and the github-releaser tags — container-executable prompts cannot read the live `private-personal` vault. It is the spec-verification phase's check, mapped to the `Post-Deploy (Rung-2)` AC above.
