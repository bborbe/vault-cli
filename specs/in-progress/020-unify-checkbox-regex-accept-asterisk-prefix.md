---
status: completed
tags:
    - dark-factory
    - spec
approved: "2026-06-28T10:51:17Z"
generating: "2026-06-28T10:51:17Z"
prompted: "2026-06-28T11:00:00Z"
verifying: "2026-06-28T11:11:20Z"
completed: "2026-06-28T13:12:00Z"
branch: dark-factory/unify-checkbox-regex-accept-asterisk-prefix
---

## Summary

- vault-cli's checkbox linter accepts both `- [ ]` and `* [ ]` as valid Markdown task lines, but the storage and ops parsers accept only the dash form.
- The mismatch is a silent-skip bug: `complete-goal` / `complete-task` / `update-task` / `defer-task` / `work-on-task` all ignore asterisk-prefixed checkboxes that the linter just declared fine.
- Real impact: a goal whose Success Criteria use `* [ ]` can be marked completed even though no boxes are checked, because the parser cannot see those lines.
- Fix is mechanical: unify 7 regexes in 6 files behind the pattern `[-*]`, with care that the one replacement site preserves whichever list marker the source line used.
- No change in behavior for dash-prefixed files (the dominant form); no new states; no migration of existing vault content.

## Problem

vault-cli treats Markdown checkboxes inconsistently across layers. The linter (`pkg/ops/lint.go`) recognizes both `- [ ]` and `* [ ]` as task lines, matching the CommonMark spec and Obsidian's rendering (which displays both identically). The storage parser (`pkg/storage/base.go`) and five operation handlers (`update`, `complete` ×2, `defer`, `workon`) match only `- [...]`. The result is a silent-skip: lines that pass lint are invisible to runtime operations. `complete-goal` can succeed on a goal whose Success Criteria are still `* [ ]`, because those criteria never enter the parsed model. Eight files in the user's vault use the asterisk form and are silently broken today. Because Obsidian renders both markers identically, the user has no visual signal that anything is wrong.

## Goal

Every vault-cli code path that reads or rewrites a Markdown checkbox line accepts both `- [ ]` and `* [ ]` (and their `[x]` / `[/]` variants) as equivalent inputs. Replacement operations preserve whichever list marker the source line used. The linter's existing tolerance for both markers is no longer a contradiction with runtime behavior.

## Non-goals

- Do NOT normalize existing vault files from `* [...]` to `- [...]` — this fix removes the silent-skip bug, after which normalization is optional, not required.
- Do NOT change the linter's checkbox character class from `[ xX]` to `[ x/]` — lint and runtime check different things on purpose.
- Do NOT add new checkbox states beyond the three already supported (`[ ]`, `[x]`, `[/]`).
- Do NOT add a config flag to toggle asterisk acceptance — invariant; if a future consumer demands variation, that's a separate spec.

## Acceptance Criteria

- [x] AC1: The seven occurrences of the dash-only checkbox regex are replaced. Evidence: `git grep -nE '\^\(\\s\*\)- \\\[' pkg/` returns zero matches inside the storage and ops files listed in Desired Behavior #1; `git grep -nE '\[-\*\] \\\[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go` returns the expected count of matches (one per updated site).
- [x] AC2: The single replacement site at `pkg/ops/complete.go` captures the list marker and reuses it in the replacement. Evidence: `git grep -nE '\(\[-\*\]\) \\\[\(\[ /\]\)\\\]' pkg/ops/complete.go` returns exactly one match, and `git grep -nE '"- \\\[x\\\]"' pkg/ops/complete.go` returns zero matches (the literal `- [x]` replacement string is gone).
- [x] AC3: Asterisk-prefixed checkboxes for all three states are parsed and round-tripped correctly. Evidence: new Ginkgo cases in `pkg/storage/markdown_test.go`, `pkg/ops/complete_test.go`, `pkg/ops/defer_test.go`, `pkg/ops/workon_test.go`, `pkg/ops/update_test.go` run and pass. `go test ./pkg/storage/... ./pkg/ops/...` exits 0.
- [x] AC4: Completing a `* [/]` line writes back `* [x]` (not `- [x]`). Evidence: a new test case in `pkg/ops/complete_test.go` asserts the exact post-write line content; the test passes.
- [x] AC5: Existing dash-prefixed behavior does not regress. Evidence: the pre-existing checkbox test cases in `pkg/storage/markdown_test.go` (including the `[/]` case) continue to pass without modification; `git diff` on those specific test assertions shows no semantic change to existing dash-form expectations.
- [x] AC6: `CHANGELOG.md` contains a new bullet under `## Unreleased` (creating the section if missing) prefixed `fix(parser):` that names the lint-vs-runtime mismatch. Evidence: `grep -nE '^- fix\(parser\):.*asterisk|^- fix\(parser\):.*list marker' CHANGELOG.md` returns at least one match under an `## Unreleased` heading.
- [x] AC7: `make precommit` exits 0. Evidence: exit code.

## Verification

```
make precommit
go test ./pkg/storage/... ./pkg/ops/...
git grep -nE '\^\(\\s\*\)- \\\[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
git grep -nE '\[-\*\] \\\[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
grep -nE '^- fix\(parser\):' CHANGELOG.md
```

Expected:
- `make precommit` exits 0.
- `go test` exits 0; new asterisk cases visible in test output.
- First `git grep` returns no matches in the listed files.
- Second `git grep` returns 7 matches (one per site).
- `grep` on CHANGELOG returns at least one line.

Manual smoke (post-merge, optional): create a Markdown file containing `* [ ] foo` in a goal Success Criteria block; run the relevant vault-cli verify/complete command; confirm the asterisk line is detected (no longer silently skipped).

## Desired Behavior

1. The dash-only checkbox detection regex `^(\s*)- \[([ x/])\] (.+)$` is replaced with `^(\s*)[-*] \[([ x/])\] (.+)$` at all seven sites: `pkg/storage/base.go`, `pkg/ops/update.go`, `pkg/ops/complete.go` (two sites), `pkg/ops/defer.go`, `pkg/ops/workon.go`, and the one replacement site in `pkg/ops/complete.go`.
2. The replacement-style regex in `pkg/ops/complete.go` (currently `- \[([ /])\]` paired with the literal replacement `- [x]`) is rewritten to capture the list marker and reuse it. The new shape is `([-*]) \[([ /])\]` with replacement `$1 [x]`, so a `* [/]` line becomes `* [x]` and a `- [/]` line stays `- [x]`.
3. The character class for checkbox state stays `[ x/]` for detection and `[ /]` for the in-flight-to-complete replacement — no new states.
4. The linter (`pkg/ops/lint.go`) is not modified — it already accepts both markers and intentionally uses a different state class.
5. For each modified file, the corresponding `_test.go` gains explicit Ginkgo cases that exercise the asterisk path: parse-only (storage), state transitions `[ ]` → `[/]` (workon), `[/]` → `[x]` (complete), `[ ]`/`[/]` → `[ ]` with date stamp or equivalent (defer), and subtask discovery (update).
6. The asterisk-form complete test asserts the written-back line preserves the `*` marker (guarding AC2 / AC4 against regression).

## Constraints

- Must NOT change the regex pattern at `pkg/ops/lint.go:460` or `pkg/ops/lint.go:771`.
- Must NOT add support for additional checkbox states (no `[?]`, `[!]`, etc.).
- Must NOT alter the meaning of `[/]` (in-progress) anywhere — replacement logic continues to treat `[/]` as eligible for completion.
- Existing dash-form test cases must continue to pass byte-for-byte without rewrite.
- Repository convention: code changes flow through dark-factory; branch `feat/checkbox-accept-asterisk` already exists for the implementation.
- Public API of storage and ops packages (exported type signatures, function names) does not change.

## Suggested Decomposition

Two prompts cleanly split the work along the regex-change vs CHANGELOG boundary; tests live with the code they cover:

| Prompt | Scope | Files |
|--------|-------|-------|
| 1 | Regex unification + matching tests | `pkg/storage/base.go` + `markdown_test.go`; `pkg/ops/update.go` + `update_test.go`; `pkg/ops/complete.go` (both detect + capture-group-replace sites) + `complete_test.go`; `pkg/ops/defer.go` + `defer_test.go`; `pkg/ops/workon.go` + `workon_test.go` |
| 2 | CHANGELOG bullet + final precommit | `CHANGELOG.md` |

If the daemon prefers single-prompt execution, the work is small enough to land in one prompt — the split is a suggestion, not a constraint.

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| Markdown file mixes `- [ ]` and `* [ ]` on different lines | Both lines are parsed; each round-trips with its original marker preserved | None needed — by design |
| Line uses tab indentation followed by `* [ ]` | Parsed as a checkbox; indentation captured in group 1 unchanged | None — `\s*` already matches tabs |
| Line uses `+ [ ]` (CommonMark also allows `+` as list marker) | Not parsed as a checkbox; treated as plain text (same as today for `+`) | Out of scope; file an issue if a real consumer surfaces |
| Replacement regex run on a `* [x]` line (already complete) | No match; line untouched | None — idempotent |
| Replacement regex run on a `- [/]` line | Becomes `- [x]` (existing behavior preserved) | Verified by AC5 |
| Single Success Criteria block contains both `- [ ]` and `* [ ]` lines | All lines parsed; goal does not complete until both markers' boxes are `[x]` | None — `complete-goal`'s "all checked" predicate now sees both lines instead of silently skipping the asterisk ones |

## Security / Abuse Cases

Not applicable. This change touches only in-process regex literals that operate on files already trusted by the caller. The new pattern is strictly broader than the old one (accepts an additional list marker character) and does not introduce backtracking risk (no nested quantifiers, fixed character class).

## Do-Nothing Option

If we don't fix this, the lint-vs-runtime mismatch remains a silent footgun: any user who writes Success Criteria with `*` (CommonMark's other allowed list marker, which Obsidian renders identically) can mark goals or tasks complete without any criteria actually being checked. Eight files in the active vault are affected today. Workarounds — telling users "always use `-`" or running a one-shot normalizer — leave the parser fragile to the same problem next time a file is authored or imported with `*`. The do-nothing cost is ongoing data-integrity risk on the goal/task completion surface, which is the single highest-stakes write path in vault-cli. Not acceptable.
