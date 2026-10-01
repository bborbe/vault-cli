---
status: approved
spec: [059-bug-bound-skip-warning-output]
created: "2026-10-01T18:28:55Z"
queued: "2026-10-01T19:02:01Z"
branch: dark-factory/bug-bound-skip-warning-output
---

# Cap per-file skip warnings at 10 with a summary line (spec 059, prompt 2 of 3)

<summary>
- A directory holding many unreadable pages no longer prints one warning line per file.
- The first ten skipped files are still named individually, so the operator can see which files were skipped.
- From the eleventh skipped file onward the per-file lines stop, and one line at the end reports how many pages were skipped in total.
- A directory with fewer than ten unreadable pages behaves exactly as before — one line per file and no summary.
- The total warning output for a directory of unreadable pages is bounded, regardless of how many files it holds.
- The listing still returns the pages it could read and still exits 0.
- The skip-and-continue behaviour is unchanged.
</summary>

<objective>
Add a global cap to `ListPages`'s skip reporting: count every skipped page, emit the per-file warning only for the first ten, and — once ten or more pages have been skipped — emit exactly one `skipping N unreadable pages` summary line after the walk. This bounds a directory-level listing whose directory has accumulated many malformed files, which the per-file truncation from prompt 1 alone does not bound. It builds on prompt 1's warn site, which must already be in place.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**This prompt is the second half of spec 059 and depends on prompt 1.** Prompt 1 changed the warn site in `pkg/storage/page.go` to `slog.Warn("skipping unreadable page", "file", filePath, "error", truncateCause(errors.Cause(err).Error()))` and added the `maxCauseBytes` / `truncationSuffix` constants plus the `truncateCause` helper. If those are not present when you start — `grep -n 'truncateCause' pkg/storage/page.go` prints nothing — stop and report `Status: failed` with `"prompt 1 (truncateCause warn site) not yet deployed"`; do not re-implement them here.

**Why the cap is needed on top of the truncation.** The producer of the corrupt files — tracked separately as `Fix Agent-task-controller Task_identifier Accumulator Loop` — writes *many* malformed files, not one, so a directory can accumulate several. Prompt 1 bounds a *single* record; it does not bound the *number* of records. Ten per-file lines at ~185 bytes plus one summary is the budget the acceptance criterion measures.

Read fully before changing anything:

- `pkg/storage/page.go` — the whole file, as prompt 1 left it. This is the file you change. The warn site is inside the `for _, entry := range entries` loop in `ListPages`, immediately after `p.readPageFromPath` returns an error, followed by `continue`. The loop opens with a `select { case <-ctx.Done(): return pages, errors.Wrap(ctx, ctx.Err(), "context cancelled") default: }` guard, and `ListPages` ends with `return pages, nil` — neither moves. `fmt` is already imported.
- `pkg/storage/page_test.go` — the whole file, as prompt 1 left it. Copy its idiom: external `storage_test` package, `os.MkdirTemp("", "vault-test")`, and a `slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})` installed in `BeforeEach` / restored in `AfterEach`. Add your new spec inside the same `Describe("pageStorage.ListPages diagnostics", …)` block.
- `pkg/storage/task.go` — read only, to confirm you are **not** touching it. `ListTasks` logs at `slog.Debug` and is a different call path (spec Non-goals; spec AC7).

Verified against source 2026-10-01 (do not re-derive):

- The per-file message is the singular `skipping unreadable page`; the summary message is the plural `skipping N unreadable pages`. The plural phrase does **not** occur inside the singular message (`…page"` is followed by a quote, not `s`), so `strings.Count(log, "skipping unreadable page")` counts only the per-file lines and `strings.Count(log, "unreadable pages")` counts only the summary.
- A page with no frontmatter block produces exactly one `no frontmatter found` warning, which is the cheapest fixture for the ≥10 arm.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs (funlen 80, nestif 4, golines 100).
</context>

<requirements>
1. **Add the named threshold constant to `pkg/storage/page.go`.** Extend the `const ( … )` block prompt 1 added (the one holding `maxCauseBytes` and `truncationSuffix`) with:

   ```go
   // maxUnreadablePageWarnings is the number of per-file skip warnings a single
   // ListPages walk emits before it stops naming files individually and prints
   // one summary line instead. It bounds a directory full of unreadable pages.
   maxUnreadablePageWarnings = 10
   ```

   The value is `10` and it is a named constant — do not inline the literal at the comparison sites.

2. **Count every skipped page and cap the per-file warnings in `ListPages`.** Declare a counter before the loop (`skipped := 0`), wrap the existing warn call in a guard, and increment the counter for **every** unreadable file — including the ones whose warning was suppressed. After the loop, before `return pages, nil`, emit the summary when the cap was reached. The loop body's skip branch becomes:

   ```go
   page, err := p.readPageFromPath(ctx, filePath, fileName, vaultPath)
   if err != nil {
   	// Warn and continue: the operator must be told the file was skipped,
   	// otherwise the page silently disappears from listings that still exit 0.
   	// The per-file line is capped at maxUnreadablePageWarnings so a directory
   	// full of unreadable pages cannot flood stderr; the total is reported in
   	// the summary line below.
   	if skipped < maxUnreadablePageWarnings {
   		slog.Warn(
   			"skipping unreadable page",
   			"file", filePath,
   			"error", truncateCause(errors.Cause(err).Error()),
   		)
   	}
   	skipped++
   	continue
   }
   ```

   and the tail of `ListPages` becomes:

   ```go
   if skipped >= maxUnreadablePageWarnings {
   	slog.Warn(fmt.Sprintf("skipping %d unreadable pages", skipped))
   }

   return pages, nil
   ```

   The summary message carries the total in its text (`skipping 12 unreadable pages`) — that exact rendered form is what the acceptance criterion asserts, so `fmt.Sprintf` into the message is the required shape; do not move the count into a structured attribute (a `count=12` attribute would not render as the required string). `fmt` is already imported.

   Nothing else in `ListPages` moves: the `select` context guard, the `entry.IsDir()` and `.md` suffix filters, the `fs.ErrNotExist` early return, and the `return pages, nil` (partial list plus nil error) are unchanged.

3. **Add the acceptance-criterion spec to `pkg/storage/page_test.go`.** Add it inside the existing `Describe("pageStorage.ListPages diagnostics", …)` block, as a `DescribeTable` (the repo's idiom for a value matrix — see `pkg/ops/blocked_by_write_test.go`). Ginkgo runs the block's `BeforeEach` once per entry, so each entry gets a fresh `vaultPath`, a fresh `logBuf` and the text handler already installed; do not re-create them inside the entry body. Add a small fixture helper beside the table:

   ```go
   // frontmatterlessPage returns a page with no frontmatter block, which
   // ListPages skips with the "no frontmatter found" warning.
   func frontmatterlessPage(i int) string {
   	return fmt.Sprintf("# No Frontmatter %d\n", i)
   }
   ```

   Four entries cover the boundary at 10 — below the cap, exactly at the cap, and above it:

   ```go
   DescribeTable("caps per-file skip warnings at ten and summarises the rest",
   	func(pageCount, expectedPerFile int, expectedSummary string) {
   		pagesDir := filepath.Join(vaultPath, "UnreadablePages")
   		Expect(os.MkdirAll(pagesDir, 0755)).To(Succeed())
   		for i := 0; i < pageCount; i++ {
   			Expect(os.WriteFile(
   				filepath.Join(pagesDir, fmt.Sprintf("NoFm%d.md", i)),
   				[]byte(frontmatterlessPage(i)),
   				0600,
   			)).To(Succeed())
   		}

   		pages, err := store.ListPages(ctx, vaultPath, "UnreadablePages")

   		Expect(err).To(BeNil())
   		Expect(pages).To(BeEmpty())
   		log := logBuf.String()
   		Expect(strings.Count(log, "skipping unreadable page")).To(Equal(expectedPerFile))
   		if expectedSummary == "" {
   			Expect(log).ToNot(ContainSubstring("unreadable pages"))
   		} else {
   			Expect(strings.Count(log, expectedSummary)).To(Equal(1))
   		}
   		Expect(len(log)).To(BeNumerically("<=", 3072))
   	},
   	Entry("below the cap: six pages, one line each, no summary", 6, 6, ""),
   	Entry("exactly at the cap: ten pages, ten lines plus a summary", 10, 10, "skipping 10 unreadable pages"),
   	Entry("above the cap: twelve pages, ten lines plus a summary", 12, 10, "skipping 12 unreadable pages"),
   	Entry("well above the cap: fifteen pages, ten lines plus a summary naming 15", 15, 10, "skipping 15 unreadable pages"),
   )
   ```

   The three entries are the acceptance criterion: fewer than ten unreadable pages gives one line each and no summary; twelve gives exactly ten per-file lines and exactly one `skipping 12 unreadable pages` line within a ≤3 KB budget; fifteen proves the summary names the true total (15) rather than the cap. `strings.Count(log, "skipping unreadable page")` counts only the per-file lines — the plural summary phrase is not a substring of the singular message.

   `path/filepath`, `os` and `bytes` are already imported in this file; `fmt` and `strings` were added by prompt 1. Add nothing else unless the compiler asks.

4. **Do not touch `pkg/storage/task.go`.** `ListTasks` (Debug-level) and the `ListTasksStrict` split are a different call path and are explicitly out of scope (spec Non-goals; spec AC7). This prompt changes `pkg/storage/page.go` and `pkg/storage/page_test.go` and nothing else.

5. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, then walk spec 059's AC5 against the change: the 6-page fixture gives six per-file lines and no summary; the 12-page fixture gives exactly ten per-file lines, exactly one `skipping 12 unreadable pages` line, and ≤3 KB of output; the 15-page fixture's summary names 15. Confirm prompt 1's specs (the `already defined` head-keeping assertion, the ≤200-byte field, the stack-free frontmatter-less line) are still green.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit.
- **This prompt depends on prompt 1.** It assumes `truncateCause`, `maxCauseBytes` and `truncationSuffix` already exist in `pkg/storage/page.go` and that the warn site already logs `truncateCause(errors.Cause(err).Error())`. If they are missing, report `Status: failed` with `"prompt 1 (truncateCause warn site) not yet deployed"` — never `needs_input`.
- **Do not fold in `pkg/storage/task.go`.** `ListTasks` (Debug-level) is a different call path and stays out of scope.
- **The threshold is exactly 10 and it is a named constant.** The pinned bound is load-bearing — do not loosen it and do not inline the literal.
- **The summary string is `skipping N unreadable pages`, with N the true total** (not the cap). The acceptance criterion greps for the rendered text, so the count belongs in the message via `fmt.Sprintf`.
- **The counter counts every skipped page, not only the warned ones.** If the increment moves inside the `if skipped < maxUnreadablePageWarnings` guard, the summary reports 10 for a 12-page directory and the criterion fails.
- **Skip-and-continue is unchanged.** The `continue` after the warn stays; `ListPages` still returns the pages it could read plus a `nil` error. Do not turn a skipped file into a returned error.
- **`pkg/ops/` stays output-free.** No formatting moves into `pkg/ops/`; the warn site and the summary remain in `pkg/storage/`.
- **No hand-bump of version strings or tags.** `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser-agent` owns the version bump and the tag post-merge. Do not edit `.claude-plugin/*.json` version fields and do not run `git tag`. Do not add a `CHANGELOG.md` entry in this prompt — prompt 3 owns it.
- **Tests follow repository convention:** Ginkgo v2 / Gomega, external `storage_test` package, `os.MkdirTemp` vault.
- **Scope is `pkg/storage/page.go` and `pkg/storage/page_test.go`.** `pkg/storage/task.go`, `pkg/storage/base.go`, `pkg/storage/storage.go`, `pkg/ops/`, `pkg/cli/`, `integration/`, `mocks/`, `docs/`, `commands/`, `scenarios/`, `CHANGELOG.md` and `.dark-factory.yaml` are nobody's scope in this prompt.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…`, which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every spec already in `pkg/storage/page_test.go`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. Absence assertions are written as `! grep -q` because `grep -c` exits 1 when it prints `0`.

**PRIMARY GATE — the cap and the summary exist.**

```
grep -n 'maxUnreadablePageWarnings' pkg/storage/page.go     # the named threshold constant plus its two uses
grep -n 'unreadable pages' pkg/storage/page.go              # the summary line exists
grep -n 'errors.Cause(err).Error()' pkg/storage/page.go     # prompt 1's warn site is still in place
```

**TESTS.**

```
go test ./pkg/storage/... -count=1
make test
make precommit
```

All three must exit 0.

⚠️ **Guard the unit run against a false pass.** A focus that matches nothing exits 0 while running nothing, so confirm the new spec actually ran:

```
go test ./pkg/storage/... -count=1 -ginkgo.focus='ListPages diagnostics' -ginkgo.fail-on-empty
```

must exit 0. (`-ginkgo.fail-on-empty` fails the run when the focus matches nothing. Do not grep for `skipping unreadable page`: that string is written to the specs' in-memory `logBuf`, never to stdout, so the grep always returns 0.)

**SYNTAX.**

```
gofmt -e -l pkg/storage/page.go pkg/storage/page_test.go    # must list NO files
```

**SELF-CHECK before finishing:** re-run the PRIMARY GATE and the TESTS, and walk spec 059's AC5 against the change — the 6-page fixture (six per-file lines, no summary), the 12-page fixture (exactly ten per-file lines, exactly one `skipping 12 unreadable pages` line, total output ≤3072 bytes) and the 15-page fixture (exactly one `skipping 15 unreadable pages` line). Confirm prompt 1's specs are still green.
</verification>
