---
status: approved
spec: [059-bug-bound-skip-warning-output]
created: "2026-10-01T18:28:55Z"
queued: "2026-10-01T19:02:01Z"
branch: dark-factory/bug-bound-skip-warning-output
---

# Lock the skip-warning bounds end-to-end and record the fix (spec 059, prompt 3 of 3)

<summary>
- A real command run against a vault holding a badly damaged file is proven to still succeed: it exits 0, prints its rows, and writes only a small amount to stderr.
- The bound is proven end-to-end through the actual built binary, not only through the storage layer.
- The discriminating tests are shown to fail against the old behaviour and pass against the new one, so they are known to test the fix rather than restate it.
- The change is recorded in the changelog as an unreleased bug fix, so a reader learns what was wrong and what changed.
- No version number is hand-written — the repository's release automation owns the bump and the tag.
- Nothing outside the change is touched.
</summary>

<objective>
Close spec 059 with the two pieces that need both code paths in place: an integration test proving `vault-cli task list` survives a file with thousands of duplicate keys (exit 0, rows printed, bounded stderr), and a `## Unreleased` `fix:` bullet in `CHANGELOG.md`. Also perform the regression-lock check — confirm the new discriminating specs fail against the pre-fix implementation and pass against the fixed one. Depends on prompts 1 and 2.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` § Documentation for the CHANGELOG placement rule.

**This prompt depends on prompts 1 and 2.** Prompt 1 changed the warn site in `pkg/storage/page.go` to `truncateCause(errors.Cause(err).Error())`; prompt 2 added the `maxUnreadablePageWarnings = 10` cap and the `skipping N unreadable pages` summary. If either is missing — `grep -n 'unreadable pages' pkg/storage/page.go` prints nothing, or `grep -n 'errors.Cause(err).Error()' pkg/storage/page.go` prints nothing — stop and report `Status: failed` with `"prompt 1/2 (skip-warning bounds) not yet deployed"`; do not re-implement them here.

Read fully (in this order):

- `integration/cli_test.go` — the top of the file (the imports and the `createTempVault` / `createTempVaultWithGoals` / `createTempVaultWithCurrentUser` helpers, roughly lines 1-120), and the `Describe("vault-cli list", …)` block. That block is the pattern to copy: `BeforeEach` calls `createTempVault(map[string]string{…})` and assigns `configPath, cleanup`; `AfterEach` calls `cleanup()`; each `It` runs `gexec.Start(exec.Command(binPath, "--config", configPath, "--vault", "test", "task", "list"), GinkgoWriter, GinkgoWriter)`, waits with `Eventually(session).Should(gexec.Exit(0))`, and reads `session.Out` (a `*gbytes.Buffer`) and `session.Err`. `os/exec`, `gbytes`, `gexec`, `strings` and `fmt` are already imported.
- `integration/integration_suite_test.go` — `BeforeSuite` builds the binary with `gexec.Build("github.com/bborbe/vault-cli")` into `binPath`. That is the binary your spec runs.
- `pkg/storage/page.go` — the `ListPages` warn site and summary, as prompts 1 and 2 left them. Read it so the pre-fix shape you revert to in requirement 3 is exactly the original.
- `CHANGELOG.md` — lines 1-20 only. At HEAD the file reads `# Changelog` (line 1), a blank line, the preamble ending with `* PATCH version when you make backwards-compatible bug fixes.` (line 9), a blank line, and `## v0.158.2` (line 11). **There is no `## Unreleased` section**, so this prompt creates one. Note that released history contains one bullet carrying the exact phrase `skipping unreadable page` (line 754, under an old `## vX.Y.Z` section) and a neighbouring bullet at line 753 that mentions an unreadable page — the section-walking `awk` in `<verification>` uses `head -1`, so the first match is the new bullet under `## Unreleased`.
- `scripts/check-changelog.sh` — the whole file (~40 lines). It fails the build when a `## ` section appears *above* the preamble, so `## Unreleased` goes immediately below the preamble and immediately above `## v0.158.2`. This is the check `make precommit` runs via the `check-changelog` target.
- `docs/dod.md` § Documentation — the required order: `# Changelog` → preamble → `## Unreleased` → `## vX.Y.Z` (newest first).

Verified against source 2026-10-01 (do not re-derive):

- `createTempVault` writes every entry of its `map[string]string` into `<vaultPath>/Tasks` and configures `tasks_dir: Tasks`, so a task file placed there is read by `vault-cli task list`.
- `slog`'s text handler is installed at `Warn` by `NewRootCommand`'s `PersistentPreRunE`, so the skip warning reaches stderr without `--verbose`.
- A file with 2,992 duplicate `task_identifier` keys produces a single warning record; pre-fix that record was 332,280,825 bytes on one line, post-fix it is well under 1 KB (its `error` field is capped at 200 bytes).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, the `integration/` harness.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — the `## Unreleased` placement rules and the recognised conventional prefixes (`fix:` is the right one here).
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs.
</context>

<requirements>
1. **Add the end-to-end integration spec to `integration/cli_test.go`** (spec 059 AC4). Add a helper beside `createTempVault`:

   ```go
   // duplicateKeyTaskContent returns a task file whose frontmatter declares
   // task_identifier n times, so the YAML parse raises n(n-1)/2 duplicate-key
   // errors — the shape that produced a 332 MB warning record before spec 059.
   func duplicateKeyTaskContent(n int) string {
   	var sb strings.Builder
   	sb.WriteString("---\n")
   	for i := 0; i < n; i++ {
   		sb.WriteString("task_identifier: 11111111-1111-4111-a111-111111111111\n")
   	}
   	sb.WriteString("---\n# Broken\n")
   	return sb.String()
   }
   ```

   Then add a new `Describe` block (place it directly after the existing `Describe("vault-cli list", …)` block) following that block's structure exactly:

   ```go
   Describe("vault-cli task list with an unreadable page", func() {
   	var configPath string
   	var cleanup func()

   	BeforeEach(func() {
   		_, configPath, cleanup = createTempVault(map[string]string{
   			"healthy-task": `---
   status: todo
   priority: 2
   ---
   # Healthy Task
   `,
   			"broken-task": duplicateKeyTaskContent(2992),
   		})
   	})

   	AfterEach(func() {
   		cleanup()
   	})

   	It("AC4 (059): task list survives a file with thousands of duplicate keys", func() {
   		cmd := exec.Command(binPath, "--config", configPath, "--vault", "test", "task", "list")
   		session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
   		Expect(err).NotTo(HaveOccurred())
   		Eventually(session).Should(gexec.Exit(0))
   		Expect(session.Out).To(gbytes.Say("healthy-task"))
   		Expect(len(session.Err.Contents())).To(BeNumerically("<=", 1024))
   	})
   })
   ```

   The three assertions are the acceptance criterion: exit code 0, the healthy row still printed, and stderr bounded at 1 KB. Pre-fix the same command writes 332,280,825 bytes to stderr, so the third assertion is the discriminating one. The `(059)` qualifier follows the file's existing `AC4 (nil phase)` / `AC2 (058)` convention and keeps the label unambiguous across specs.

2. **Record the change in `CHANGELOG.md`** (spec 059 AC8). The repository has **no** `## Unreleased` section today, so create it below the preamble's last line (`* PATCH version when you make backwards-compatible bug fixes.`) and above the newest `## vX.Y.Z` heading (currently `## v0.158.2`; if the releaser has already cut a release on this branch, place it above whatever the newest `## vX.Y.Z` heading is). Add exactly one `- fix:` bullet, matching the density and shape of the surrounding entries — a single long line that says what was wrong, why it mattered, and what changed, ending with a `Change set:` clause:

   ```
   ## Unreleased

   - fix: Bound the `skipping unreadable page` warning so one malformed vault file can no longer take a read-only listing out of action. The warning's `error` field is now the plain cause (`errors.Cause(err).Error()`) truncated to at most 200 bytes on a UTF-8 rune boundary with a `…` suffix and the head kept — so the leading `already defined` survives and no Go stack is rendered — and a directory of unreadable pages now stops the per-file lines at ten and emits one `skipping N unreadable pages` summary instead. `vault-cli task list` against a file with 2,992 duplicate `task_identifier` keys wrote ≈332 MB to stderr before this change and now writes ≤1 KB while still exiting 0 with its rows. Change set: `pkg/storage/page.go`, `pkg/storage/page_test.go`, `integration/cli_test.go`.
   ```

   Do not add a date suffix to the heading, do not add a `### Fixed` category heading, do not reorder or edit any existing section, and do not touch the preamble. Wording of the bullet may be adjusted, but it must carry all of: the literal per-file message `skipping unreadable page` (singular — `<verification>`'s `awk` keys on this exact phrase, and it is what distinguishes the new bullet from the older `## vX.Y.Z` history), the plain-cause change, the ≤200-byte UTF-8-safe head-keeping truncation, the removal of the Go stack, the ten-line cap with the `skipping N unreadable pages` summary, the before/after stderr magnitude, and the `Change set:` clause.

3. **Perform the regression-lock check** (spec 059 AC6) — the new discriminating specs must fail against the pre-fix implementation. Use filesystem copies, not git. The container's git state is not guaranteed, and a `git` command that dies still leaves a false pass.

   a. Back up the fixed file: `cp pkg/storage/page.go /tmp/page.go.fixed`.
   b. Revert `pkg/storage/page.go` to its pre-fix shape by editing it: restore the warn site to the single original call `slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))`, remove the `if skipped < maxUnreadablePageWarnings { … }` guard, remove the `skipped := 0` declaration and the `skipped++` line, and remove the trailing `if skipped >= maxUnreadablePageWarnings { … }` summary block. Leave the `maxCauseBytes` / `truncationSuffix` / `maxUnreadablePageWarnings` constants and the `truncateCause` helper in place — an unused function is legal Go and keeps the `unicode/utf8` import used, so the package still compiles.
   c. Run the discriminating specs and confirm they **FAIL** (non-zero exit):
      ```
      go test ./pkg/storage/... -count=1 -ginkgo.focus='ListPages diagnostics'
      ```
      The size/stack/cap assertions flip; a passing run means the tests do not discriminate and the prompt is not done. The non-zero exit here is expected and is not a prompt failure — do not weaken the assertions to make it pass; steps 3d-3f restore the file and re-verify.
   d. Restore the fixed file: `cp /tmp/page.go.fixed pkg/storage/page.go`.
   e. Confirm the restore: `grep -n 'errors.Cause(err).Error()' pkg/storage/page.go` must match, and `grep -n 'unreadable pages' pkg/storage/page.go` must match.
   f. Re-run the focused specs and confirm they now **PASS**.
   g. Run `make precommit` last. **The restored file is what gets committed** — a tree left in the reverted state is a failed prompt.

4. **Do not touch `pkg/storage/task.go`.** `ListTasks` (Debug-level) and the `ListTasksStrict` split are a different call path and are explicitly out of scope (spec Non-goals). Spec 059 AC7 — that the `ListTasks` path is unchanged — is out of scope here: no requirement edits that file, and the file's byte-identity is asserted on the operator side, not in this prompt.

5. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, then walk spec 059's AC4 (the integration spec exits 0, prints `healthy-task`, and stderr is ≤1 KB), AC6 (the focused unit specs failed against the reverted warn site and passed once restored), and AC8 (`make precommit` green and the `## Unreleased` `fix:` bullet present) against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit. The container's git state is not guaranteed: do not use `git` for the regression-lock revert — use the `/tmp` backup and `cp` restore described in requirement 3.
- **This prompt depends on prompts 1 and 2.** It assumes `truncateCause(errors.Cause(err).Error())`, `maxUnreadablePageWarnings = 10` and the `skipping N unreadable pages` summary already exist in `pkg/storage/page.go`. If they are missing, report `Status: failed` with `"prompt 1/2 (skip-warning bounds) not yet deployed"` — never `needs_input`.
- **The regression-lock revert is temporary and the restore is mandatory.** `pkg/storage/page.go` must be back to the fixed form before the prompt ends; `<verification>` greps for it and runs `make precommit` last.
- **Do not fold in `pkg/storage/task.go`.** `ListTasks` (Debug-level) is a different call path and stays out of scope (spec Non-goals; spec AC7 is operator-side).
- **The `## Unreleased` section goes below the preamble**, never between the `# Changelog` title and the preamble — `make precommit` runs `check-changelog`, which fails the build on that shape. The preamble (the `All notable changes…` line, the SemVer link and the three `* MAJOR / MINOR / PATCH` bullets) is frozen: do not move, delete or edit any of it.
- **One bullet, flat list, prefix exactly `fix:`.** No `### Fixed` category heading, no nested bullets, no multiple entries. Spec 059 is a bug fix, not a new capability.
- **No version bump and no tag.** `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser-agent` classifies the bump from the `## Unreleased` prefixes (`fix:` → patch), renames the section to `## vX.Y.Z`, and bumps `CHANGELOG.md`, `.claude-plugin/plugin.json` and both `.claude-plugin/marketplace.json` version fields in lockstep. Do not create a `## vX.Y.Z` section, do not edit `.claude-plugin/*.json` version fields, do not run `git tag`, and do not run `make release-check` or `make check-versions`.
- **README.md is not updated.** The change alters no usage, configuration or setup; only the size and content of a stderr warning change.
- **The command's stdout and exit code are frozen.** `vault-cli task list` still exits 0 and prints its rows; only stderr reporting changed. Do not change `pkg/cli/` or `pkg/ops/`.
- **`pkg/ops/` stays output-free.** No formatting moves into `pkg/ops/`.
- **Tests follow repository convention:** Ginkgo v2 / Gomega against the `integration/` harness's temp vault and the `binPath` built in `BeforeSuite`.
- **Scope is `integration/cli_test.go` and `CHANGELOG.md`.** `pkg/storage/page.go` is touched only by the temporary revert in requirement 3, which must be undone. `pkg/storage/page_test.go`, `pkg/storage/task.go`, `pkg/ops/`, `pkg/cli/`, `mocks/`, `docs/`, `commands/`, `scenarios/` and `.dark-factory.yaml` are nobody's scope in this prompt.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…` and the `/tmp` scratch backup used by the regression-lock step, both of which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every spec already in `integration/cli_test.go` and `pkg/storage/page_test.go`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. Absence assertions are written as `! grep -q` because `grep -c` exits 1 when it prints `0`.

**PRIMARY GATE — the fix is in place and restored after the regression-lock revert.**

```
grep -n 'errors.Cause(err).Error()' pkg/storage/page.go     # exactly 1 line — the fixed warn site is back
grep -n 'unreadable pages' pkg/storage/page.go              # the summary line is back
! grep -q 'errors.Cause(err))' pkg/storage/page.go          # the pre-fix Any-valued form is not present
grep -n 'duplicateKeyTaskContent' integration/cli_test.go    # the helper definition plus its single call
grep -n 'AC4 (059)' integration/cli_test.go                 # the integration spec is present
```

**CHANGELOG — structure and placement.**

```
bash scripts/check-changelog.sh                             # prints "CHANGELOG structure OK" and exits 0
grep -n '^## ' CHANGELOG.md | head -2                       # "## Unreleased" then "## vX.Y.Z"
awk '/^## /{sec=$0} /skipping unreadable page/ {print sec}' CHANGELOG.md | head -1   # "## Unreleased"
```

The `awk` walks sections top-down and `head -1` takes the first match, so the released-history bullets further down the file do not interfere. Do **not** use a line-window `grep -A` form, which swallows a neighbouring section.

**TESTS.**

```
go test ./pkg/storage/... -count=1
go test ./integration/... -count=1
make test
make precommit
```

All must exit 0.

⚠️ **Guard the integration run against a false pass.** `make test` runs without `-ginkgo.v`, so a suite that was skipped rather than run also looks green. Confirm the new spec actually ran and the focus matched something:

```
go test ./integration/... -count=1 -ginkgo.focus='task list with an unreadable page' -ginkgo.v 2>&1 | grep -c 'AC4 (059): task list survives a file with thousands of duplicate keys'
```

must be `>= 1`.

**SYNTAX.**

```
gofmt -e -l integration/cli_test.go                         # must list NO files
```

**SELF-CHECK before finishing:** re-run the PRIMARY GATE, the CHANGELOG checks and the TESTS. Walk spec 059's AC4 (the integration spec exits 0, prints `healthy-task`, stderr ≤1 KB), AC6 (the focused `pkg/storage` specs failed against the reverted warn site in requirement 3c and passed once restored in 3f) and AC8 (`make precommit` exit 0 and the `## Unreleased` `fix:` bullet present) against the change. Confirm `pkg/storage/page.go` is in its fixed form — the regression-lock revert must not be left in the tree.
</verification>
