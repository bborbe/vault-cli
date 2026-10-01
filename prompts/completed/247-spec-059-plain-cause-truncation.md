---
status: completed
spec: [059-bug-bound-skip-warning-output]
summary: Bounded the per-file skip warning in ListPages by logging errors.Cause(err).Error() truncated to 200 bytes on a UTF-8 rune boundary, removing the multi-megabyte YAML duplicate-key dump and the Go stack trace from frontmatter-less pages
execution_id: vault-cli-bound-skip-warning-exec-247-spec-059-plain-cause-truncation
dark-factory-version: v0.196.0
created: "2026-10-01T18:28:55Z"
queued: "2026-10-01T19:02:01Z"
started: "2026-10-01T19:02:03Z"
completed: "2026-10-01T19:04:45Z"
branch: dark-factory/bug-bound-skip-warning-output
---

# Bound the per-file skip warning: plain-string cause, truncated to 200 bytes (spec 059, prompt 1 of 3)

<summary>
- A read-only listing command stays usable when one file in the vault is malformed.
- Each skipped file still produces exactly one warning line, but the line now names the human-readable reason instead of the whole parse dump.
- No Go stack trace appears in that warning any more.
- The reason embedded in the line is capped at 200 bytes, cut on a character boundary, and keeps the beginning of the message — so the phrase `already defined` still shows.
- A file carrying thousands of duplicate keys no longer floods stderr with hundreds of megabytes.
- A page with no frontmatter yields one short line instead of a screenful of stack frames.
- Files that parse are listed exactly as before, and the listing still exits 0.
- The skip-and-continue behaviour is unchanged: the file is still skipped and the command still succeeds.
</summary>

<objective>
Make the per-file skip warning in `ListPages` bounded and stack-free: log `errors.Cause(err).Error()` — a plain `string` — truncated to at most 200 bytes on a UTF-8 rune boundary, keeping the head, with a `…` suffix when any bytes were cut. This removes the unbounded `error` field (a 2,992-duplicate-key file currently produces a single ≈332 MB warning record) and the Go stack that the frontmatter-less path renders. It is the precondition for prompt 2, which adds the global per-directory cap on top of this warn site.
</objective>

<context>
Read `CLAUDE.md` for project conventions.

**The defect in one paragraph, because it is what makes requirement 2 the whole fix.** `pkg/storage/page.go`'s `ListPages` emits one `slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))` per unreadable file. `errors.Cause(err)` returns the *deepest* wrapped error, and `slog`'s text handler renders a `KindAny` attribute with `%+v` — confirmed at `log/slog/text_handler.go`, the `case KindAny` branch ending in `s.appendString(fmt.Sprintf("%+v", v.Any()))`. For a duplicate-key YAML file the deepest cause is a `*yaml.TypeError` whose `Error()` embeds one entry per duplicate-key *pair* (quadratic in the corruption), so the single warning line reached 332,280,825 bytes on the 2,992-key reproduction. For a frontmatter-less page the deepest cause is a `*pkg/errors.fundamental` (from `errors.Errorf`), whose `%+v` renders the full frame list (~2,457 bytes). Logging `.Error()` — a `string` — makes `slog` render it verbatim (`KindString`), so no stack; truncating that string is what bounds the size. Neither change alone fixes both symptoms.

Read fully before changing anything:

- `pkg/storage/page.go` — the whole file. This is the file you change. The warn site is the `slog.Warn("skipping unreadable page", …)` call inside the `for _, entry := range entries` loop, immediately after `p.readPageFromPath` returns an error, followed by `continue`. Note the `select { case <-ctx.Done(): … }` guard at the top of the loop and the `errors.Is(err, fs.ErrNotExist)` early return — both are unchanged by this prompt. `fmt` is already imported; you will add `unicode/utf8`.
- `pkg/storage/base.go` — `ParseFrontmatterMap` (the `errors.Errorf(ctx, "no frontmatter found")` path, and the `errors.Wrap(ctx, err, "unmarshal yaml frontmatter")` path) and `readEntityComponentsFromPath` (which wraps again with `"parse frontmatter"`). Together they are why `errors.Cause` unwraps to a `*fundamental` on one path and a `*yaml.TypeError` on the other.
- `pkg/storage/page_test.go` — the whole file. Copy its idiom: external `storage_test` package, `os.MkdirTemp("", "vault-test")`, and a `slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})` installed in `BeforeEach` / restored in `AfterEach`. The spec `It("warns with full path and parse error when skipping a corrupt page", …)` at the `Expect(log).To(ContainSubstring("already defined"))` assertion is the one the spec requires you to keep green — do not weaken, move or delete it.
- `pkg/cli/cli.go` — `NewRootCommand`'s `PersistentPreRunE` installs `slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))`. That is the production logger; it is the reason the fix must produce a plain `string`, not merely a shorter `error`.
- `pkg/storage/storage.go` — `PageStorage` interface (`ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Page, error)`) and `NewPageStorage`. Read only; the interface does not change.

Verified against source 2026-10-01 (do not re-derive; these are the numbers the assertions are calibrated to):

- `github.com/bborbe/errors` v1.6.1 — `func Cause(err error) error` (`errors_cause.go`), `func Errorf(ctx context.Context, format string, args ...interface{}) error` (`errors_new.go`). `Cause` delegates to `github.com/pkg/errors.Cause`, which loops on the `Cause() error` interface; `*dataError` implements `Cause()` (`errors_data-error.go`), so the unwrap goes past the wrappers to the deepest error.
- A frontmatter-less page (`ParseFrontmatterMap` on `[]byte("# No Frontmatter\n")`): `errors.Cause(err).Error()` == `no frontmatter found` — exactly 20 bytes.
- A duplicate-key file with N identical `task_identifier` keys: `errors.Cause(err).Error()` starts `yaml: unmarshal errors:\n  line 2: mapping key "task_identifier" already defined at line 1\n  line 3: …` and its length grows quadratically — N=50 → 82,882 bytes; N=500 → 8,678,631 bytes; N=2,992 → ≈318 MB.
- The head-keeping requirement is real: for every duplicate-key fixture the phrase `already defined` begins well inside the first 200 bytes (at N=50 it sits at byte ~65), so a head-truncated value still contains it.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — the `bborbe/errors` API and wrapping idiom.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — what `make precommit` runs (funlen 80, nestif 4, golines 100).
</context>

<requirements>
1. **Add the bound and the truncation helper to `pkg/storage/page.go`.** Add `unicode/utf8` to the import block and add these declarations near the top of the file (after the `import` block, before `type pageStorage struct`):

   ```go
   const (
   	// maxCauseBytes bounds the cause string embedded in a skip warning so one
   	// corrupt file cannot flood stderr. It is a hard bound: the logged value —
   	// including the truncationSuffix when the cause was cut — is never longer
   	// than this.
   	maxCauseBytes = 200

   	// truncationSuffix marks a cause that was cut at maxCauseBytes.
   	truncationSuffix = "…"
   )

   // truncateCause bounds cause to maxCauseBytes bytes. When the cause is longer
   // it is cut on a UTF-8 rune boundary and truncationSuffix is appended, so the
   // result is always valid UTF-8 and never longer than maxCauseBytes. The head
   // of the message is kept: the leading "yaml: unmarshal errors:\n  line N: …
   // already defined" survives for a duplicate-key file.
   func truncateCause(cause string) string {
   	if len(cause) <= maxCauseBytes {
   		return cause
   	}
   	cut := maxCauseBytes - len(truncationSuffix)
   	for cut > 0 && !utf8.RuneStart(cause[cut]) {
   		cut--
   	}
   	return cause[:cut] + truncationSuffix
   }
   ```

   `truncationSuffix` is the single-rune string `…` (U+2026, three bytes in UTF-8), so `maxCauseBytes - len(truncationSuffix)` is `197` and the returned value is at most `197 + 3 = 200` bytes. The `utf8.RuneStart` walk steps back to a rune boundary so a cut that lands mid-rune (a multibyte key name) still yields valid UTF-8. Do not use `[]rune` conversion — it would re-encode and change the byte budget.

2. **Change the warn site to log the truncated plain-string cause.** In `ListPages`, replace

   ```go
   slog.Warn("skipping unreadable page", "file", filePath, "error", errors.Cause(err))
   ```

   with

   ```go
   slog.Warn(
   	"skipping unreadable page",
   	"file", filePath,
   	"error", truncateCause(errors.Cause(err).Error()),
   )
   ```

   The literal call `errors.Cause(err).Error()` must appear — `<verification>` greps for it. Nothing else about the loop moves: the `continue` after the warn stays, the `select` context guard stays, the `fs.ErrNotExist` early return stays, and `ListPages` still returns `pages, nil` (a partial list plus a nil error — the skip-and-continue contract). Do not add a counter or a summary line in this prompt; that is prompt 2.

3. **Add the discriminating unit specs to `pkg/storage/page_test.go`.** Add them inside the existing `Describe("pageStorage.ListPages diagnostics", …)` block, keeping its `BeforeEach`/`AfterEach` (text handler into `logBuf`, restored after). Add the imports `encoding/json`, `fmt`, `strings` and `unicode/utf8` as needed. Add one small fixture helper beside the specs:

   ```go
   // duplicateKeyPage returns a page whose frontmatter declares task_identifier n
   // times, so the YAML parse raises n(n-1)/2 duplicate-key errors.
   func duplicateKeyPage(n int) string {
   	var sb strings.Builder
   	sb.WriteString("---\n")
   	for i := 0; i < n; i++ {
   		sb.WriteString("task_identifier: 11111111-1111-4111-a111-111111111111\n")
   	}
   	sb.WriteString("---\n# Broken\n")
   	return sb.String()
   }
   ```

   **3a — AC1, one bounded record per file, independent of the error count.** One spec writes three files into the pages dir: `Short.md` = `duplicateKeyPage(3)`, `Huge.md` = `duplicateKeyPage(2992)`, and `Healthy.md` with valid frontmatter. Call `store.ListPages(ctx, vaultPath, "<pagesDir>")` and assert:
   - `Expect(err).To(BeNil())` and the healthy page is returned (`HaveLen(1)`, name `Healthy`);
   - `strings.Count(logBuf.String(), "skipping unreadable page")` equals `2` — exactly one record per unreadable file;
   - every record is bounded: split the buffer with `strings.Split(strings.TrimRight(logBuf.String(), "\n"), "\n")` and assert each line's length is `<= 1024`. The 2,992-key line is the discriminating one — pre-fix it is 332,280,825 bytes;
   - `Expect(logBuf.String()).ToNot(ContainSubstring("Healthy.md"))`.

   **3b — AC2, the `error` field is ≤200 bytes, valid UTF-8, head-keeping.** Read the field value without slog's quoting/escaping, by installing a **JSON handler** for this spec (override the `BeforeEach` handler inside the `It`): `slog.SetDefault(slog.New(slog.NewJSONHandler(jsonBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))`. Then, in two arms:
   - `duplicateKeyPage(2992)`: unmarshal the single record line (`json.Unmarshal([]byte(strings.TrimRight(jsonBuf.String(), "\n")), &rec)` where `rec` is a `map[string]any`), read `errField, ok := rec["error"].(string)` and guard it with `Expect(ok).To(BeTrue())` (`forcetypeassert` is enabled in `.golangci.yml` and applies to `_test.go` files; the repo uses the comma-ok form everywhere), then assert `len(errField) <= 200`, `utf8.ValidString(errField)`, `strings.HasSuffix(errField, "…")`, and `strings.Contains(errField, "already defined")` — the last one is what proves the **head** was kept rather than the tail.
   - the mid-rune fixture: a duplicate-key page whose key names carry multibyte runes, built so the 200-byte cut lands mid-rune. Use a key of **100** `ü` runes — verified 2026-10-01: `"k" + strings.Repeat("ü", 100)` makes the raw cause 275 bytes with byte offset 197 a UTF-8 continuation byte (`0xBC`), so the cut is forced onto a rune boundary:

     ```go
     key := "k" + strings.Repeat("ü", 100)
     midRune := fmt.Sprintf("---\n%s: 1\n%s: 2\n---\n# MidRune\n", key, key)
     ```

     Assert `len(errField) <= 200`, `utf8.ValidString(errField)`, and `strings.HasSuffix(errField, "…")`.
     Do **not** use 60 `ü` runes: verified, that fixture's cause is 195 bytes — under the bound — so nothing is cut and there is no `…`; the assertion would fail on a correct implementation.

   **3c — AC3, no stack, one short line for a frontmatter-less page.** One spec writes `NoFm.md` = `"# No Frontmatter\n"` (no frontmatter block) and calls `ListPages`. Using the default text handler, assert:
   - `strings.Count(logBuf.String(), "\n")` equals `1` — a single physical line;
   - `len(logBuf.String()) <= 300`;
   - `Expect(logBuf.String()).To(ContainSubstring("no frontmatter found"))`;
   - `Expect(logBuf.String()).ToNot(ContainSubstring("runtime.goexit"))`, `…ToNot(ContainSubstring("errors_new.go"))`, `…ToNot(ContainSubstring("errors_wrap.go"))`, and `…ToNot(ContainSubstring("\n\t"))`.

   The existing spec at `pkg/storage/page_test.go`'s `Expect(log).To(ContainSubstring("already defined"))` stays byte-identical and green — its `broken` fixture's cause is 110 bytes, so it is never truncated and the head is intact.

4. **Do not touch `pkg/storage/task.go`.** `ListTasks` (which logs at `slog.Debug`) and the `ListTasksStrict` split are a different call path and are explicitly out of scope (spec Non-goals; spec AC7). This prompt changes `pkg/storage/page.go` and `pkg/storage/page_test.go` and nothing else.

5. **Self-check before finishing.** Re-run `<verification>` and confirm each printed line against its expectation, then walk spec 059's AC1, AC2 and AC3 against the change: one record per unreadable file with each record ≤1 KB; the `error` field ≤200 bytes, valid UTF-8, ending in `…`, and still containing `already defined`; and the frontmatter-less page logging one ≤300-byte line with no stack marker. Confirm the existing `already defined` spec is still present and green.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git. Never stage or commit.
- **Preserve the existing assertion.** The spec requires that the `pkg/storage/page_test.go` assertion on `already defined` still passes; truncation must keep the **head**, not the tail. Do not weaken, move or delete that assertion or its spec.
- **Do not fold in `pkg/storage/task.go`.** `ListTasks` (Debug-level) is a different call path and stays out of scope.
- **The pinned bounds are load-bearing.** ≤200 bytes for the `error` field (including the `…`), head-keeping truncation, and valid UTF-8 are what make the criteria falsifiable — do not loosen them.
- **Skip-and-continue is unchanged.** The `continue` after the warn stays; `ListPages` still returns the pages it could read plus a `nil` error. Do not turn a skipped file into a returned error.
- **`pkg/ops/` stays output-free.** No formatting moves into `pkg/ops/`; the warn site remains in `pkg/storage/`.
- **No counter and no summary line in this prompt.** The global cap and its `skipping N unreadable pages` summary belong to prompt 2; adding them here would collide with prompt 2's edit to the same loop.
- **Do not try to avoid materialising the parse error string.** `errors.Cause(err).Error()` still builds the full multi-megabyte string before it is truncated; that transient allocation is expected and is not part of this fix (the fix bounds the *logged output*). Do not add a byte-limited reader or a custom error walker.
- **No hand-bump of version strings or tags.** `.maintainer.yaml` sets `release.autoRelease: true`, so the `github-releaser-agent` owns the version bump and the tag post-merge. Do not edit `.claude-plugin/*.json` version fields and do not run `git tag`. Do not add a `CHANGELOG.md` entry in this prompt — prompt 3 owns it.
- **Tests follow repository convention:** Ginkgo v2 / Gomega, external `storage_test` package, `os.MkdirTemp` vault. Counterfeiter mocks are not needed here.
- **Scope is `pkg/storage/page.go` and `pkg/storage/page_test.go`.** `pkg/storage/task.go`, `pkg/storage/base.go`, `pkg/storage/storage.go`, `pkg/ops/`, `pkg/cli/`, `integration/`, `mocks/`, `docs/`, `commands/`, `scenarios/`, `CHANGELOG.md` and `.dark-factory.yaml` are nobody's scope in this prompt.
- All repository paths in this prompt are repo-relative. The only absolute paths are the in-container coding-plugin doc paths under `/home/node/.claude/…`, which resolve inside the container; never use a host absolute path (`/Users/…`, `/home/<user>/…`) or a `~/` path.
- Existing tests must still pass, including every spec already in `pkg/storage/page_test.go`.
</constraints>

<verification>
Run each of these and confirm the printed result against its expectation. Absence assertions are written as `! grep -q` because `grep -c` exits 1 when it prints `0`.

**PRIMARY GATE — the warn site logs the truncated plain-string cause.**

```
grep -n 'errors.Cause(err).Error()' pkg/storage/page.go     # exactly 1 line — the warn site logs the string form
grep -n 'truncateCause' pkg/storage/page.go                 # the helper definition plus its single call site
grep -n 'maxCauseBytes' pkg/storage/page.go                 # the named 200-byte bound and its uses
grep -n 'unicode/utf8' pkg/storage/page.go                  # the import is present
! grep -q 'errors.Cause(err))' pkg/storage/page.go          # the old Any-valued form is gone
```

**TESTS.**

```
go test ./pkg/storage/... -count=1
make test
make precommit
```

All three must exit 0.

⚠️ **Guard the unit run against a false pass.** A focus that matches nothing exits 0 while running nothing, so confirm the new specs actually ran:

```
go test ./pkg/storage/... -count=1 -ginkgo.focus='ListPages diagnostics' -ginkgo.fail-on-empty
```

must exit 0. (`-ginkgo.fail-on-empty` fails the run when the focus matches nothing. Do not grep for `skipping unreadable page`: that string is written to the specs' in-memory `logBuf`, never to stdout, so the grep always returns 0.)

**SYNTAX.**

```
gofmt -e -l pkg/storage/page.go pkg/storage/page_test.go    # must list NO files
```

**SELF-CHECK before finishing:** re-run the PRIMARY GATE and the TESTS, and walk spec 059's AC1 (two unreadable files → exactly two records, each ≤1 KB, healthy file unmentioned), AC2 (the JSON-captured `error` field is ≤200 bytes, `utf8.ValidString`, ends in `…`, and still contains `already defined`; the 100-`ü` mid-rune fixture ends in `…`) and AC3 (frontmatter-less page → one line, ≤300 bytes, contains `no frontmatter found`, no `runtime.goexit` / `errors_new.go` / `errors_wrap.go` / `\n\t`) against the change. Confirm the pre-existing `already defined` spec is untouched and green.
</verification>
