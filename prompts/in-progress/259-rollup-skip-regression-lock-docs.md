---
status: approved
spec: [060-bug-rollup-aborts-on-frontmatterless-task-file]
created: "2026-10-19T17:57:00Z"
queued: "2026-10-19T18:14:28Z"
branch: dark-factory/bug-rollup-aborts-on-frontmatterless-task-file
---

# Lock the regression and document the skip behaviour

<!--
Decomposition note (for the auditor): the spec's row 3 owns AC7 and AC8. AC7's
wording assumes the fix's source changes can be reverted with git and the failing
run recorded in a PR body. This repo runs `workflow: direct` with `pr: false`, and
a git-based revert is neither needed nor used here, so this prompt neutralises the
two behaviour-bearing edits in place (keeping the post-fix API and every import
used, so the tests still compile and the failure is a spec failure rather than a
build error), records the failing run, restores the files from byte-identical
copies, and puts the recorded evidence in the dark-factory completion report for
the operator to carry into the eventual PR.
-->

<summary>
- Proves the new skip tests actually fail when the skip behaviour is removed.
- Restores the implementation and confirms the suite is green again.
- Records the pre-fix failure output as the evidence of record for the change.
- Adds an Unreleased changelog entry describing the fix.
- Documents the skipped line and the JSON skipped array in the README rollup section.
- Leaves every version string and tag untouched — the releaser owns them.
- The full precommit gate passes.
</summary>

<objective>
Prove the fix is load-bearing (the new specs fail without it), then document the new behaviour for users and for the changelog, so the change ships with evidence and a written contract rather than only passing tests.
</objective>

<context>
Read `CLAUDE.md` for project conventions. This repo is a single Go module at the repo root; run `make test` and `make precommit` at the repo root.

The previous two prompts run before this one; their changes must be present in the working tree. If any of the symbols below is missing from the working tree, stop and report a missing dependency — do not re-implement it:

- `pkg/storage/task.go` has `ListTasksTolerant`, which returns `(tasks []*domain.Task, skipped []string, err error)`.
- `pkg/ops/rollup_weekly.go` calls `ListTasksTolerant`, carries `Skipped []string` with `json:"skipped"`, and initialises it to `[]string{}`.
- `pkg/cli/rollup.go` has `writeRollupWeeklySkipped`, called last by `formatRollupWeeklyPlain`.

Read before editing:

- `pkg/storage/task.go` — the `ListTasksTolerant` method body (the first neutralisation target).
- `pkg/cli/rollup.go` — `formatRollupWeeklyPlain` (the second neutralisation target).
- `pkg/ops/rollup_weekly.go` — `Execute`, for orientation only; it is not neutralised.
- `CHANGELOG.md` — the preamble ends with the three SemVer bullets; the newest section is `## v0.171.1`. There is currently no `## Unreleased` section.
- `README.md` — the `### rollup` section (starts at the `### rollup` heading and ends at the next `## ` heading, which is `## Claude Code Plugin`).
- `scripts/check-changelog.sh` — the structure rule `make precommit` enforces: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z`.

Coding guides (in-container paths):

- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/documentation-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/definition-of-done.md`

The pre-fix behaviour this spec is about: a single frontmatter-less task file under the vault's tasks directory makes `rollup weekly` exit 1 with an error naming that file and print no figures at all, so one malformed file blocks every week of that vault.
</context>

<requirements>
1. **Record a green baseline first.** Run `make test` and confirm it exits 0. Do not proceed until the suite is green with the fix in place.

2. **Save the two files you will neutralise.**

   ```bash
   cp pkg/storage/task.go /tmp/task.go.fixed
   cp pkg/cli/rollup.go /tmp/rollup.go.fixed
   ```

3. **Neutralise the storage behaviour while keeping the post-fix API and every import used.** Replace only the body of `ListTasksTolerant` in `pkg/storage/task.go` with the strict behaviour expressed under the tolerant signature, keeping the method name, signature and doc comment:

   ```go
   func (t *taskStorage) ListTasksTolerant(
       ctx context.Context,
       vaultPath string,
   ) ([]*domain.Task, []string, error) {
       var tasks []*domain.Task
       skipped := make([]string, 0)
       if err := t.walkTaskFiles(
           ctx,
           vaultPath,
           func(task *domain.Task, _, path string, err error) error {
               if err != nil {
                   return errors.Wrapf(ctx, err, "read task file %s", path)
               }
               tasks = append(tasks, task)
               return nil
           },
       ); err != nil {
           return nil, nil, err
       }
       sort.Strings(skipped)
       return tasks, skipped, nil
   }
   ```

   This is the pre-fix behaviour — a per-file error aborts the listing, no skipped names are collected — while the signature the tests compile against is unchanged. Do not declare `tasksDir`: nothing in this body uses it, and an unused local is a compile error that would turn the expected spec failures into a build error. Keep `sort.Strings(skipped)` in place so the `sort` import stays used; an unused import is a compile error for the same reason. Leave the `skippedTaskName` helper in place (it may become unused; Go permits unused functions, and it keeps the `unicode` import used).

4. **Neutralise the renderer.** In `pkg/cli/rollup.go`, remove the `writeRollupWeeklySkipped(&builder, result.Skipped)` call from `formatRollupWeeklyPlain`. Leave the `writeRollupWeeklySkipped` function defined (unused functions compile); the report must end at the delta block, exactly as before the fix.

5. **Run the suite and capture the failure.**

   ```bash
   make test > /tmp/rollup-prefix-failure.txt 2>&1; echo "make test exit=$?"
   ```

   This run is expected to exit non-zero. Confirm it is a spec failure, not a compile error, and that the fixture-driven specs fail: the AC1 rollup spec, the AC2 storage tolerant-listing specs, the AC3 plain `skipped:`-line spec, and the AC4 fixture-vault `"skipped":["Broken.md"]` spec. The clean-vault cases (no `skipped:` line, `"skipped":[]`) are invariants and are expected to keep passing — do not read their pass as a broken lock. If every fixture-driven spec still passes, the tests are not load-bearing — stop and report that finding rather than continuing.

6. **Restore both files and re-run the suite.**

   ```bash
   cp /tmp/task.go.fixed pkg/storage/task.go
   cp /tmp/rollup.go.fixed pkg/cli/rollup.go
   cmp -s pkg/storage/task.go /tmp/task.go.fixed
   cmp -s pkg/cli/rollup.go /tmp/rollup.go.fixed
   make test
   ```

   `cmp -s` exits 0 only when the files are byte-identical. `make test` must exit 0. Do not re-type the restored bodies — copy them back.

7. **Write the changelog entry.** Create `## Unreleased` below the preamble (after the three `* MAJOR/MINOR/PATCH` bullet lines) and above the newest `## v0.171.1` section. Add exactly one bullet, on one physical line, starting with `- fix:` and containing both the word `rollup` and the word `skipped`, for example:

   ```markdown
   ## Unreleased

   - fix: `rollup weekly` skips task files it cannot read or parse and names them on a final `skipped:` line and a `skipped` JSON array, instead of aborting the whole week's report; a walk-level failure such as a missing `tasks_dir` still errors

   ## v0.171.1
   ```

   Do not add a version heading, do not bump any version string, and do not create a tag. `scripts/check-changelog.sh` requires the preamble to stay above every `## ` section.

8. **Document the behaviour in the README.** At the end of the `### rollup` section — after the final paragraph, whose last sentence names `vault-cli config set-baseline <vault> <path>`, and before the `## Claude Code Plugin` heading — add a paragraph covering:

   - A task file the rollup cannot read or parse (for example a file with no frontmatter block) is skipped rather than failing the whole report.
   - The figures are computed from the files that did parse; the skipped files contribute nothing.
   - Each skipped file is named on one final line beginning `skipped:`, as a path relative to the vault's `tasks_dir`; a clean run prints no such line.
   - `--output json` carries the same names in a `skipped` array — an array, empty when nothing was skipped, never `null`.
   - A missing or unreadable tasks directory is still an error and is never reported as a vault with no tasks.

   Keep the prose consistent with the section's existing voice (plain sentences, no bullet lists unless the section already uses one). Do not edit `docs/output-formatting.md` — the spec states it covers only the list JSON read model, not this report.

9. **Report the evidence.** In the dark-factory completion report, include the neutralised run's failing spec names and the first failure line verbatim from `/tmp/rollup-prefix-failure.txt`, labelled as the AC7 pre-fix evidence. The completion report is the evidence of record; this workflow creates no PR, so no PR body is written.

10. Do not change any version string, do not run `git tag`, and do not edit the three plugin JSON version fields. `.maintainer.yaml` sets `release.autoRelease: true`; the github-releaser owns the tag and the four version strings post-merge.
</requirements>

<constraints>
- Copied from the spec: no hand-bump of version strings or tags. `.maintainer.yaml: release.autoRelease: true` — the github-releaser owns the tag and bumps all four version strings post-merge. Add a `## Unreleased` bullet and let it run. (`.dark-factory.yaml: autoRelease: false` is the dark-factory driver, deliberately off; it does not contradict this.)
- The restoration in step 6 must leave `ListTasksTolerant` as the real tolerant implementation and `formatRollupWeeklyPlain` ending with the skipped line. The neutralised state must not survive this prompt.
- Do not edit `ListTasksStrict`'s method body, `walkTaskFiles`'s signature, or any test assertion written by the previous prompts.
- `pkg/ops/` stays output-free; all formatting stays in `pkg/cli/`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- Do not write a separate evidence or summary markdown file into the repo; the evidence goes in the completion report.
</constraints>

<verification>
The final state must be restored, documented and green. Run at the repo root:

```bash
make precommit
```

It must exit 0. `make precommit` includes `check-changelog`, so a mis-ordered changelog fails here.

Confirm the restoration is byte-identical to the saved copies and that the fix is live:

```bash
cmp -s pkg/storage/task.go /tmp/task.go.fixed
cmp -s pkg/cli/rollup.go /tmp/rollup.go.fixed
grep -q 'skipped = append(skipped, skippedTaskName(tasksDir, path))' pkg/storage/task.go
grep -q 'writeRollupWeeklySkipped(&builder, result.Skipped)' pkg/cli/rollup.go
```

All four must exit 0. The two `cmp` checks prove byte-identity only within this same run — if `/tmp` was cleared between steps they will fail for that reason alone; in that case confirm the two `grep` checks pass and restore the fix by re-applying the tolerant callback from the previous prompt rather than leaving the neutralised body in place.

Confirm the changelog bullet sits under `## Unreleased` and nowhere else:

```bash
awk '/^## /{sec=$0} /^- fix:.*rollup.*skipped/ {print sec}' CHANGELOG.md | sort -u
```

This must print exactly one line: `## Unreleased`.

Confirm the README section documents the skip behaviour:

```bash
sed -n '/^### rollup/,/^## /p' README.md | grep -n 'skipped'
```

This must print at least one line.

Before finishing, re-run the verification block and confirm it passes. Confirm the recorded pre-fix failure names the AC1, AC2, AC3 and AC4 specs, and that it is quoted in the completion report.
</verification>
