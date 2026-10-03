---
status: completed
summary: Added a tested domain.SanitizeFilename function, exposed it as `vault-cli filename sanitize` (plain + JSON), and replaced the prose filename rule in both creator agents with a call to that command.
execution_id: vault-cli-windows-safe-filenames-exec-250-sanitize-windows-invalid-filenames
dark-factory-version: v0.196.0
created: "2026-10-03T12:20:00Z"
queued: "2026-10-03T13:17:39Z"
started: "2026-10-03T13:30:48Z"
completed: "2026-10-03T13:34:59Z"
---

# Sanitize Windows-invalid characters in generated filenames

<summary>
- Task and goal creation can no longer produce a filename that Windows refuses to check out.
- The rule becomes a tested function the creator agents call, instead of a sentence they are asked to follow.
- The same title still appears in the file's body and frontmatter — only the filename changes.
- Names that are already safe are returned unchanged, so nothing existing is renamed.
- The characters Windows refuses are handled, along with trailing dots and spaces and the Windows reserved device names.
- Nothing else changes: no new required flag, no change to how a vault is resolved, no version bump.
</summary>

<objective>
A title containing `:` or `?` yields a task or goal filename that checks out on Windows, because both creator agents derive that filename by running one tested command instead of applying a prose rule they can quietly ignore.
</objective>

<context>
Read `CLAUDE.md` for project conventions and the prompt flow, and read `docs/dod.md` — this repo's Definition of Done, which dark-factory passes as `validationPrompt`. It names the CHANGELOG `## Unreleased` placement rule, the README-on-usage-change rule, the ≥80% coverage target, and — the one this prompt turns on — the `integration/cli_test.go` command-registration table that every new CLI command must be added to.

**Why this is a code change and not a wording change.** `agents/task-creator.md` has carried the line `- Strip filesystem-illegal characters: / \ : * ? " < > |` since 2026-04-27 (commit `60fe7d5`, release v0.58.0). It did not work. On 2026-09-13 — four and a half months later — the vault gained `25 Tasks/Fix Build-Fix Planning: Model Prose Breaks parseFixPlan JSON Unmarshal.md`, and by 2026-10-03 `origin/master` held ~145 such paths. A prose instruction is not enforcement, and a rule with no test cannot be checked. That is the whole reason this becomes a function with a table test and a command the agent runs, rather than a better sentence.

**What the rule must produce.** These are the actual before/after pairs from the 2026-10-03 cleanup of those 145 paths — the transformation below is the one that was applied by hand and verified, so match it:

| Input | Output |
|---|---|
| `Fix Build-Fix Planning: Model Prose` | `Fix Build-Fix Planning - Model Prose` |
| `Fix A : B` | `Fix A - B` (rule 1 alone leaves two spaces where the colon was; rule 5 is what collapses them) |
| `Vault-cli Resolve Stops At First Vault On found:false` | `Vault-cli Resolve Stops At First Vault On found -false` |
| `vault-cli topic defer Test Fails Between Midnight and 02:00 CEST` | `vault-cli topic defer Test Fails Between Midnight and 02 -00 CEST` |
| `The Board Offers No Yes/No Control` | `The Board Offers No Yes-No Control` |
| `Settle Whether the *.needs.json Store Is Retired` | `Settle Whether the x.needs.json Store Is Retired` |
| `Support Repeated ?scope= Parameters` | `Support Repeated scope= Parameters` |

So: `:` becomes ` -`, `/` and `\` become `-`, `*` becomes `x`, and `? " < > |` are removed. Em-dashes, umlauts and emoji are legal in a Windows filename and must pass through untouched — the vault is full of them.

**The reserved-device rule is about the stem, not the whole name.** Microsoft documents that "NUL.txt and NUL.tar.gz are both equivalent to NUL", so a name whose text before the first dot is a reserved device is invalid however many extensions follow. It is also every path component, though this function only ever sees one.

Read fully before changing anything:

- `pkg/domain/` — pick an existing **stdlib-only** file of the same shape (`pkg/domain/content.go` or `pkg/domain/file_metadata.go`) and copy its idiom: a short doc comment, no dependencies outside the standard library, an external `domain_test` package for the test. For the `DescribeTable` idiom specifically, `pkg/domain/goal_phase_test.go` is the exemplar — but `goal_phase.go` itself imports `bborbe/collection`, `bborbe/errors` and `bborbe/validation`, so do not take it as the stdlib-only model. There is no `pkg/domain/id.go`.
- `pkg/cli/cli.go` — the root command wiring. Every command group is a `create<Name>Commands(ctx, …)` constructor registered with `rootCmd.AddCommand(...)` around lines 175–202. Add yours in the same style; do not restructure the file.
- `agents/task-creator.md` — § 5 "Compose the title" ends with the strip line and `Final filename: {JIRA_KEY }{Title}.md`. This is the call site you replace.
- `agents/goal-creator.md` — § "Compose the title" carries the same strip line; its `Final filename` line is `{Title}.md`.

Verified against source 2026-10-03 (do not re-derive):

- There is no existing sanitizer, slug helper, or `filename` command anywhere in `pkg/` — `grep -rn 'func.*[Ss]lug\|func.*Sanitiz' pkg/` returns nothing.
- `CHANGELOG.md` has **no** `## Unreleased` section today; the topmost heading is a released version.
- The binary is built from the repo root: `go build -mod=mod -o bin/vault-cli main.go` (`Makefile` § build).
- `make precommit` runs `ensure format generate test check addlicense`. It is the gate.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` enforces (funlen 80, nestif 4, golines 100).
</context>

<requirements>
1. **Add `pkg/domain/filename.go`.** Export exactly one function:

   ```go
   // SanitizeFilename returns name with the characters Windows forbids in a
   // filename replaced or removed, so the result can be checked out on Windows.
   // name is a filename stem — the caller appends the extension.
   func SanitizeFilename(name string) string
   ```

   It must apply, in this order:
   1. `:` → ` -`
   2. `/` and `\` → `-`
   3. `*` → `x`
   4. `?`, `"`, `<`, `>`, `|` → removed
   5. every run of whitespace → one space
   6. leading and trailing whitespace, and trailing dots, trimmed
   7. if the text before the first `.` equals a reserved device name — `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, compared case-insensitively — append `_` to that stem, leaving any remainder intact (`NUL.txt` → `NUL_.txt`)
   8. if the result is empty, return `Untitled`

   Keep the reserved names in a package-level slice or map with a doc comment naming its source, so the list is visible in one place. The function must be pure: no `context`, no error return, no I/O, no logging.

2. **Add `pkg/domain/filename_test.go`.** Use Ginkgo v2 with an external `domain_test` package and a `DescribeTable`, one `Entry` per rule and per literal character — not a loop over a slice inside a single `It`, which the testing guide rejects. At minimum: each of the **nine** characters above — `:` `/` `\` `*` `?` `"` `<` `>` `|` — with its expected output (for the five that are *removed*, embed the character in a surrounding string, e.g. `a?b` → `ab`, since a lone `?` is removed and the name then falls through to `Untitled` rather than yielding a replacement); a safe name returned unchanged; an em-dash name, a name with an umlaut, and a name with an emoji all returned unchanged; `NUL`, `nul`, `NUL.txt`, `CON.md`, `COM1`, `LPT9` each gaining the `_`; `Console`, `COM0`, `LPT0` and `NULx` **not** gaining it; a name made only of *removed* characters (e.g. `???`) returning `Untitled` — note that a name of only `:` does **not** reach that branch, since rule 1 substitutes ` -` rather than deleting — `::` yields `- -` and `:::` yields `- - -`, one dash per colon; a whitespace-collapse case mirroring the `<context>` row `Fix A : B` → `Fix A - B`, which pins rule 5 independently of rule 1; and a name with a leading or trailing space and one with a trailing dot, both trimmed. Also add a command-level test that drives the real dispatch path — `cli.Run(ctx, []string{"filename", "sanitize", "Fix Build-Fix Planning: Model Prose"})` — asserting both the plain output and the `--output json` shape. Take the dispatch pattern from `pkg/cli/config_test.go` and `pkg/cli/rollup_test.go`, but for the output assertions capture `os.Stdout` the way `pkg/cli/resolve_test.go` does (`os.Pipe()`, `os.Stdout = w`, restore afterwards): `PrintJSON` writes to `os.Stdout` directly, so cobra's `SetOut` will not capture it, and neither cited dispatch exemplar captures stdout. Put that command-level test in a new `pkg/cli/filename_test.go` (package `cli_test`) so it runs under the existing `cli_suite_test.go`. Registration in `cli.go` proves the command exists; only this proves the dispatcher finds it, and the JSON output is a serialization boundary with no other coverage.

3. **Add the `filename` command in a new `pkg/cli/filename.go`** — the `createFilenameCommands(ctx, …)` constructor plus its subcommand, in the style of its siblings. `pkg/cli/cli.go` changes by exactly one line, `rootCmd.AddCommand(createFilenameCommands(...))`, so the `<constraints>` rule about not restructuring `cli.go` holds. Exposes `vault-cli filename sanitize "<name>"`:
   - `sanitize` takes exactly one positional argument and errors with a clear message when it is missing, wrapped with `errors.Errorf(ctx, …)` from `github.com/bborbe/errors` — this project does not use `fmt.Errorf`.
   - Plain output prints the sanitized name and nothing else — no label, no quotes — so it can be used in a shell substitution.
   - `--output json` prints a `{"filename": "<sanitized>"}` object through the existing `PrintJSON` helper, so it is 2-space indented like every other command's JSON, not the compact form.
   - It must not load, resolve, or require a vault: this command is pure and must work with no vault configured.
   - Add `Entry("filename sanitize", "filename", "sanitize")` to the `Describe("command registration")` / `DescribeTable("exits 0 for --help", …)` table in `integration/cli_test.go`, alongside the existing `Entry("task list", "task", "list")` rows. `docs/dod.md` line 19 names this table as a Definition-of-Done requirement for every new CLI command, and the table is not exhaustive — nothing else fails when a row is missing, so its absence ships silently.
   - Add a `### filename` subsection to `README.md` § Usage, matching the existing `### rollup` / `### config` blocks (a fenced `bash` block, one inline `#` comment per example). `docs/dod.md` requires a README update when a change affects usage, and this adds a command.

4. **Replace the prose rule at both call sites.** In `agents/task-creator.md` § 5, replace the `Strip filesystem-illegal characters:` line with an instruction to run `vault-cli filename sanitize "<Title>"` and use its stdout verbatim as the filename stem, keeping the existing `Final filename: {JIRA_KEY }{Title}.md` line and noting that the sanitized value is what `{Title}` means there. Make the same replacement in `agents/goal-creator.md`. Delete the old character list from both — leaving it beside the new instruction gives the agent two rules to choose between.

5. **Add the changelog bullet.** Create a `## Unreleased` section in `CHANGELOG.md` **below the preamble** — the `All notable changes…` line and the `* MAJOR / MINOR / PATCH` lines — and immediately above the topmost `## vX.Y.Z` heading. Never place it between the `# Changelog` title and the preamble; `scripts/check-changelog.sh` rejects that shape. One bullet: `- Sanitize Windows-invalid characters in filenames generated for tasks and goals, and expose it as `vault-cli filename sanitize`.`

6. **Self-check before you finish.** Re-run every command in `<verification>` and walk the list one line at a time against your actual working tree. Do not report a line as passing from memory or from an earlier run — the tree changed since then. If any line fails, fix it and re-run the whole block; if you cannot make one pass, say which and stop rather than reporting success.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump the version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- If a `## Unreleased` section already exists when you run, append the bullet inside the existing one; never create a second `## Unreleased` heading. The releaser renames only the topmost section, so a second one would leave this bullet silently unreleased.
- Do NOT rename, move, or otherwise touch any existing file in any vault. This change affects only names generated from now on.
- Do NOT change any other command's output, flags, or behaviour, and do not restructure `cli.go` beyond adding the one `AddCommand` line.
- Do NOT add a dependency outside the Go standard library.
- Keep `SanitizeFilename` free of `context`, logging, and I/O so it stays testable as a pure function.
</constraints>

<verification>
Run each; record the output verbatim in the report.

- `make precommit` → exit 0
- `go test ./pkg/domain/... 2>&1 | tail -3` → contains `ok`
- `grep -c 'SanitizeFilename' pkg/domain/filename_test.go` → ≥1 (this package's tests pass before your change, so the line above cannot tell you whether requirement 2 was done at all — this one can)
- `grep -c 'func SanitizeFilename' pkg/domain/filename.go` → 1
- `go run main.go filename sanitize 'Fix Build-Fix Planning: Model Prose'` → prints exactly `Fix Build-Fix Planning - Model Prose`
- `go run main.go filename sanitize 'Support Repeated ?scope= Parameters'` → prints exactly `Support Repeated scope= Parameters`
- `go run main.go filename sanitize 'NUL.txt'` → prints exactly `NUL_.txt`
- `go run main.go filename sanitize 'emdash — ok'` → prints exactly `emdash — ok`
- `grep -c '^## Unreleased' CHANGELOG.md` → 1
- `! grep -q 'filesystem-illegal' agents/task-creator.md` → exit 0 (the prose rule is gone)
- `! grep -q 'filesystem-illegal' agents/goal-creator.md` → exit 0
- `grep -c 'vault-cli filename sanitize' agents/task-creator.md agents/goal-creator.md` → one `path:count` line per file with a count of at least 1 (a multi-file `grep -c` prints `path:count` per file and never a bare total; the count is a floor, not an equality, since a correct edit may name the command twice. Deleting the old rule without adding the replacement would satisfy the two lines above, so assert the replacement is present as well)
- `grep -c 'filename sanitize' integration/cli_test.go` → ≥1 (the command-registration row `docs/dod.md` requires; no other check catches its absence)
- `grep -c '^### filename' README.md` → 1

If `make precommit` fails, STOP and report `"status":"failed"` with the exact failing command and its output. Do not attempt a partial fix of unrelated pre-existing failures — report them and stop.
</verification>
