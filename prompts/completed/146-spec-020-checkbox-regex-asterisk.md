---
status: completed
spec: [020-unify-checkbox-regex-accept-asterisk-prefix]
execution_id: vault-cli-checkbox-asterisk-exec-146-spec-020-checkbox-regex-asterisk
dark-factory-version: v0.187.11
created: "2026-06-28T10:51:17Z"
queued: "2026-06-28T11:05:59Z"
started: "2026-06-28T11:06:01Z"
completed: "2026-06-28T11:10:05Z"
branch: dark-factory/unify-checkbox-regex-accept-asterisk-prefix
---
<summary>
- vault-cli's storage and runtime ops now accept Markdown checkboxes prefixed with `*` as well as `-`, so `* [ ]`, `* [/]`, and `* [x]` lines parse and round-trip the same as their dash counterparts.
- Eight vault files that were silently invisible to goal-completion and task-completion paths (because they used `* [...]`) are now correctly seen and rewritable.
- The linter already accepted both markers and stays unchanged — this only fixes the runtime/storage mismatch.
- The single replacement site (in `pkg/ops/complete.go` daily-note path) now captures the original list marker and writes back the same marker, so a `* [/]` line becomes `* [x]`, not `- [x]`.
- Existing dash-prefixed behavior is byte-for-byte preserved: no public API change, no exported signature change, no migration of vault files.
- New Ginkgo tests in the five affected `_test.go` files pin the asterisk path so the regex cannot silently regress.
- Behavior is invariant (no new states, no new flags) per spec Non-goals.

</summary>

<objective>
Unify the seven checkbox regex sites across the storage and ops packages so that both `-` and `*` are accepted as Markdown list markers before a checkbox, and make the one replacement site capture and preserve whichever marker the source line used. The lint-vs-runtime mismatch is eliminated: lint passes, runtime cannot silently skip.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read /home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md for Ginkgo v2 / Gomega patterns (Describe/Context/It, ContainSubstring assertions, BeforeEach fixture setup).

Read these source files fully before editing (each is the load-bearing contract):
- pkg/storage/base.go — detection regex at line 25 (the var declaration used by `isCheckboxLine`).
- pkg/ops/update.go — detection regex inside the update function (line 151).
- pkg/ops/complete.go — TWO detection regex sites (lines 272 and 357) AND the single replacement site at line 364 (`regexp.MustCompile(`- \[([ /])\]`).ReplaceAllString(line, "- [x]")`).
- pkg/ops/defer.go — detection regex at line 206.
- pkg/ops/workon.go — detection regex at line 244.
- pkg/ops/lint.go — DO NOT TOUCH. Lines 460 and 771 use a different shape (`(?m)^[\s]*[-*]\s+\[([ xX])\]`) and intentionally check a different state class. They already accept both markers.

Read the corresponding `_test.go` files to anchor new test cases in their existing patterns:
- pkg/storage/markdown_test.go — find the existing checkbox-state cases (parse / round-trip).
- pkg/ops/complete_test.go — read the `updateDailyNote with in-progress checkbox` Context (around line 217) for the `ContainSubstring` assertion style.
- pkg/ops/defer_test.go — locate the in-progress test cases.
- pkg/ops/workon_test.go — locate the `[ ]` → `[/]` state-transition test.
- pkg/ops/update_test.go — locate the subtask-discovery test.

Existing test pattern to mirror (from `complete_test.go` line 217):
```go
Context("updateDailyNote with in-progress checkbox", func() {
    BeforeEach(func() {
        dailyContent := `# 2026-03-02
## Tasks
- [/] [[my-task]]
`
        mockDailyNoteStorage.ReadDailyNoteReturns(dailyContent, nil)
        mockDailyNoteStorage.WriteDailyNoteReturns(nil)
    })
    It("changes [/] to [x] in daily note", func() {
        Expect(err).To(BeNil())
        if mockDailyNoteStorage.WriteDailyNoteCallCount() > 0 {
            _, _, _, updatedContent := mockDailyNoteStorage.WriteDailyNoteArgsForCall(0)
            Expect(updatedContent).To(ContainSubstring("- [x] [[my-task]]"))
            Expect(updatedContent).NotTo(ContainSubstring("- [/]"))
        }
    })
})
```

The seven existing regex literals to replace (verified by grep before this prompt was written):
```
pkg/storage/base.go:25       ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/update.go:151        ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/workon.go:244        ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/complete.go:272      ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/complete.go:357      ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/defer.go:206         ^(\s*)- \[([ x/])\] (.+)$
pkg/ops/complete.go:364      - \[([ /])\]   (replacement style; literal "- [x]")
```
</context>

<requirements>

## Regex unification

1. In `pkg/storage/base.go` (line 25), change the var `checkboxRegex` literal from
   ```go
   ^(\s*)- \[([ x/])\] (.+)$
   ```
   to
   ```go
   ^(\s*)[-*] \[([ x/])\] (.+)$
   ```
   Keep the variable name and the surrounding package-level declaration intact. Do not touch any other line in this file.

2. In `pkg/ops/update.go` (line 151), apply the same literal change to the local `checkboxRegex` variable. Do not touch any other line in this file.

3. In `pkg/ops/workon.go` (line 244), apply the same literal change to the local `checkboxRegex` variable. Do not touch any other line in this file.

4. In `pkg/ops/defer.go` (line 206), apply the same literal change to the local `checkboxRegex` variable. Do not touch any other line in this file.

5. In `pkg/ops/complete.go` apply the same literal change to BOTH detection sites (lines 272 and 357). Do not touch any other line at these two sites.

## Replacement site (the only non-mechanical change)

6. In `pkg/ops/complete.go` (line 364), rewrite the replacement site so the list marker is captured and reused. The current code is:
   ```go
   lines[i] = regexp.MustCompile(`- \[([ /])\]`).ReplaceAllString(line, "- [x]")
   ```
   Replace it with:
   ```go
   lines[i] = regexp.MustCompile(`([-*]) \[([ /])\]`).ReplaceAllString(line, "$1 [x]")
   ```
   Effect: a `* [/]` line becomes `* [x]`; a `- [/]` line becomes `- [x]`; a `* [ ]` line becomes `* [x]`; a `- [ ]` line becomes `- [x]`. Idempotency for already-complete `[x]` lines is preserved (no match, line untouched).

   Do NOT introduce a `regexp.MustCompile` at package level — the local form is fine; the existing code already inlines this regex and that style is preserved.

## Tests — add explicit asterisk-path cases (one per affected file)

7. In `pkg/storage/markdown_test.go`, add a new Ginkgo `Context` block (named to make intent obvious, e.g. `Context("asterisk-prefixed checkboxes", func() { ... })`) that exercises parse + round-trip for all three states:
   - `* [ ] foo` parses to a checkbox line with text `foo` and state unchecked.
   - `* [/] foo` parses to in-progress.
   - `* [x] foo` parses to complete.
   Use `DescribeTable` if the existing file already uses it for checkbox states; otherwise use parallel `It` blocks following the existing style. The test MUST assert that the text after the marker is recovered (group 3) and the state is recovered (group 2). Mirror the naming and assertion style of the pre-existing dash-form cases — do not duplicate their assertions, only add the asterisk variants.

8. In `pkg/ops/complete_test.go`, add a new `Context` block (suggested name: `Context("updateDailyNote with asterisk-prefixed in-progress checkbox", func() { ... })`) whose `BeforeEach` sets the daily-note content to:
   ```
   # 2026-03-02

   ## Tasks
   * [/] [[my-task]]
   ```
   Add at least one `It` that asserts `updatedContent`:
   - `ContainSubstring("* [x] [[my-task]]")`
   - `Not(ContainSubstring("* [/]"))`
   - `Not(ContainSubstring("- [x]"))`  — guards AC4 against marker rewrite.

   Do NOT modify the existing `updateDailyNote with in-progress checkbox` Context at line 217 — that test guards AC5 (dash-form preservation) and must remain byte-identical.

9. In `pkg/ops/defer_test.go`, add a new `Context` block whose `BeforeEach` uses an asterisk-prefixed checkbox in the relevant content, and add an `It` that asserts the post-write content preserves the `*` marker while the in-progress transition is reflected.

10. In `pkg/ops/workon_test.go`, add a new `Context` block whose `BeforeEach` uses a `* [ ]` line in the relevant content, and add an `It` that asserts the `[ ]` → `[/]` transition preserves the `*` marker in the written-back content.

11. In `pkg/ops/update_test.go`, add a new `Context` block (or `It`) that exercises subtask discovery on a `* [ ]` subtask line — the regex change must let asterisk-prefixed subtasks be enumerated. Mirror the style of the existing subtask-discovery case.

## Quality gates

12. After all edits, run `make test` (fast feedback) and confirm:
    - `pkg/storage/...` and `pkg/ops/...` pass.
    - All seven updated files compile.
    - New asterisk cases appear in the test output and pass.

13. Then run `make precommit` ONCE at the very end. If it fails, fix the failing target (e.g. `make lint`) and re-run only that target until it passes, then re-run `make precommit` once.

## Acceptance-criteria evidence

14. After `make precommit` passes, run the spec's verification grep commands and confirm:
    ```
    grep -nE '\^\(\s*\)- \[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
    # Expected: no matches

    grep -nE '\[-\*\] \[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
    # Expected: 6 matches (one per detection site; the replacement site uses a different shape)

    grep -nE '\(\[-\*\]\) \[' pkg/ops/complete.go
    # Expected: 1 match (the new capture-group replacement site)

    grep -nE '"- \[x\]"' pkg/ops/complete.go
    # Expected: no matches (the literal "- [x]" replacement string is gone)
    ```

</requirements>

<constraints>
- Copied from spec 020:
  - Must NOT change the regex pattern at `pkg/ops/lint.go:460` or `pkg/ops/lint.go:771`. Different shape, different state class, intentional.
  - Must NOT add support for additional checkbox states (no `[?]`, `[!]`, etc.). State class stays `[ x/]` for detection and `[ /]` for the replacement site.
  - Must NOT alter the meaning of `[/]` (in-progress) anywhere — replacement logic continues to treat `[/]` as eligible for completion.
  - Existing dash-form test cases MUST continue to pass byte-for-byte without rewrite. AC5 guard.
  - Repository convention: code changes flow through dark-factory; branch `feat/checkbox-accept-asterisk` already exists.
  - Public API of storage and ops packages (exported type signatures, function names) does not change.
- Do NOT normalize existing vault files from `* [...]` to `- [...]` — this fix removes the silent-skip bug; normalization is out of scope.
- Do NOT add a config flag to toggle asterisk acceptance — invariant per spec Non-goal.
- Do NOT commit — dark-factory handles git.
- Use `make test` iteratively for fast feedback; reserve `make precommit` for the very end.
- Tests use Ginkgo v2 / Gomega in `package _test`. No Counterfeiter mocks needed for these cases — the storage and ops tests use direct construction (storage) or the existing mocks (ops), so follow each file's existing style.

</constraints>

<verification>
Run `make precommit` from the repo root — must exit 0.

Targeted checks (each MUST hold after edits):

```bash
# 1. AC1: no dash-only detection regex left in the six listed files
grep -nE '\^\(\s*\)- \[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
# Expected: no matches

# 2. AC1: six detection sites carry the unified pattern
grep -nE '\[-\*\] \[' pkg/storage/base.go pkg/ops/update.go pkg/ops/complete.go pkg/ops/defer.go pkg/ops/workon.go
# Expected: 6 matches

# 3. AC2: capture-group replacement site present, literal "- [x]" gone
grep -nE '\(\[-\*\]\) \[' pkg/ops/complete.go
# Expected: 1 match
grep -nE '"- \[x\]"' pkg/ops/complete.go
# Expected: no matches

# 4. AC3: tests pass and asterisk cases ran
go test ./pkg/storage/... ./pkg/ops/...
# Expected: ok

# 5. AC5: existing dash-form test cases unchanged (visual review of git diff for the dash-form Context blocks)
git diff pkg/storage/markdown_test.go pkg/ops/complete_test.go pkg/ops/defer_test.go pkg/ops/workon_test.go pkg/ops/update_test.go
# Expected: only ADDITIONS in new Context blocks; pre-existing dash-form Context blocks have no semantic changes

# 6. linter file untouched
git diff pkg/ops/lint.go
# Expected: no output

# 7. coverage of changed packages ≥80% per docs/definition-of-done.md
go test -coverprofile=/tmp/cover.out -mod=mod ./pkg/storage/... ./pkg/ops/... && go tool cover -func=/tmp/cover.out | tail -1
# Expected: total ≥80%
```
</verification>

<!-- DARK-FACTORY-REPORT -->