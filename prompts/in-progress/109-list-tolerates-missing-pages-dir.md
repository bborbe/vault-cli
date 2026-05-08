---
status: committing
summary: Modified ListPages in pkg/storage/page.go to return nil,nil when the pages directory does not exist (fs.ErrNotExist), added io/fs import, and added 3 new Ginkgo test cases covering missing dir, permission denied, and ENOTDIR scenarios, plus CHANGELOG entry.
container: vault-cli-109-list-tolerates-missing-pages-dir
dark-factory-version: v0.156.1-1-g04f3863-dirty
created: "2026-05-08T12:35:00Z"
queued: "2026-05-08T12:41:26Z"
started: "2026-05-08T12:41:54Z"
---

<summary>
- `vault-cli {task,goal,theme,objective} list` currently errors when the configured pages directory does not exist
- A missing pages directory is a normal config state — vaults that don't manage that page-type don't need the directory
- After this change, list returns an empty list and exits 0 when the directory does not exist
- All other I/O errors (permission denied, broken symlinks, ENOTDIR, generic I/O) still error with the existing wrapped message
- Detection uses `errors.Is(err, fs.ErrNotExist)`, not string match
- All four list commands share the same code path (`pkg/storage.ListPages`) and inherit the new behavior together
- A `slog.Debug` line is emitted when the directory is missing, for traceability
</summary>

<objective>
In `pkg/storage.ListPages`, treat a missing pages directory as "no pages" (return `nil, nil`) instead of an error. Other I/O errors keep current behavior. Add unit tests covering missing dir, populated dir, permission error, and ENOTDIR. Update CHANGELOG.
</objective>

<context>
Read CLAUDE.md for project conventions and the `bborbe/errors` wrapping rules.

Read these files in full before making changes:
- `pkg/storage/page.go` — `ListPages` is the function to modify. The block to change is the `entries, err := os.ReadDir(targetDir)` immediately followed by an `if err != nil { return nil, errors.Wrap(...) }`. Anchor by that snippet, not by line number. The file already imports `log/slog` and uses `slog.Debug(...)` elsewhere.
- `pkg/storage/markdown_test.go` — existing Ginkgo/Gomega test suite. Tests for `ListPages` live inside the `Context("ListPages", ...)` block (search the file for that string). New cases go there as additional `It(...)` blocks. Use the package's existing `vaultPath`/`store`/`ctx` from suite-level `BeforeEach`.
- `pkg/storage/storage_suite_test.go` — Ginkgo suite bootstrap (for reference; do not modify).
- `pkg/ops/list.go` — caller of `ListPages` (uses the result for all four page-type list commands). Verify no behavior change is needed here.

Detection mechanism: `errors.Is(err, fs.ErrNotExist)` (import `io/fs`). Do NOT string-match on the error message.

Logging: use `slog.Debug(...)` (project uses `log/slog` exclusively — `page.go` already imports it). Do NOT introduce `glog`.
</context>

<requirements>
### 1. Modify `pkg/storage/page.go` `ListPages`

Locate the existing block:

```go
entries, err := os.ReadDir(targetDir)
if err != nil {
    return nil, errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", targetDir))
}
```

Replace it with:

```go
entries, err := os.ReadDir(targetDir)
if err != nil {
    if errors.Is(err, fs.ErrNotExist) {
        slog.Debug("pages directory does not exist; returning empty list", "dir", targetDir)
        return nil, nil
    }
    return nil, errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", targetDir))
}
```

Add the `io/fs` import at the top of the file (alphabetical order with other stdlib imports). `log/slog` is already imported. Do NOT introduce `glog`.

### 2. Add Ginkgo/Gomega test cases inside `pkg/storage/markdown_test.go`

Locate the existing `Context("ListPages", ...)` block. Add new `It(...)` blocks alongside the existing ones (do NOT create a new test file — `pkg/storage/page_test.go` must NOT exist). Match the package's existing style (`Expect(...).To(BeNil())`, `Expect(...).To(BeEmpty())`, `Expect(...).To(HaveLen(N))`, reuse suite-level `vaultPath`, `store`, `ctx`).

Add the following cases:

a. **Missing directory returns empty list, no error**: pass a `pagesDir` that does NOT exist (e.g., `"DoesNotExist"` under the tempdir-backed vault). Assert `Expect(err).To(BeNil())` and `Expect(pages).To(BeEmpty())`. Also explicitly verify the `errors.Is(err, fs.ErrNotExist)` boundary by attempting an `os.ReadDir` of a path under a non-existent ancestor directly in the test setup — confirms the detection branch fires.

b. **Empty existing directory returns empty list, no error**: a test like this likely already exists in the suite — confirm by reading the file. If yes, skip; if not, add it.

c. **Populated directory returns matching `.md` files**: a test like this likely already exists — confirm and skip if so.

d. **Permission denied still errors**: create the directory, `os.Chmod(dir, 0)`, call `ListPages`. `Expect(err).NotTo(BeNil())`, `Expect(err.Error()).To(ContainSubstring("read directory"))`. Restore `chmod` afterwards (`AfterEach` or inline `defer`). Skip with `Skip("...")` if `runtime.GOOS == "windows"` or `os.Geteuid() == 0`.

e. **Non-directory at the configured path returns a real error**: create a regular file at the `pagesDir` path. `Expect(err).NotTo(BeNil())`. Verify `errors.Is(err, fs.ErrNotExist)` is FALSE — this confirms ENOTDIR isn't accidentally caught by the new branch.

### 3. CHANGELOG entry

Add under `## Unreleased` in `CHANGELOG.md`. The file is structured by released version (`## v0.58.3`, etc.) without an existing `## Unreleased` section — create the `## Unreleased` block above the topmost released version heading:

```markdown
## Unreleased

- fix: `vault-cli {task,goal,theme,objective} list` now returns an empty list (exit 0) when the configured pages directory does not exist, instead of erroring. All other I/O errors (permission denied, broken symlinks, ENOTDIR) still error with the original wrapped message.
```
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT change the `ListPages` signature
- Do NOT add a new public API in `pkg/storage`
- Do NOT use string matching to detect the missing-directory case — `errors.Is(err, fs.ErrNotExist)` only
- Preserve the existing error wrap (`read directory %s`) for the non-missing-dir error path so callers parsing stderr stay green
- Do NOT change `pkg/ops/list.go` or any of the four list command entry points
- Do NOT modify `show`, `set`, `add`, `clear`, `complete`, `defer`, or any non-`list` commands
- Existing tests must remain green
- Follow the `bborbe/errors` wrapping convention (`errors.Wrap(ctx, err, "...")`) for any new error sites — but the new code path returns `nil, nil`, no wrap needed there
</constraints>

<verification>
Run `make precommit` — must pass.

Manual smoke test against a real vault that has no Goals directory:
```bash
vault-cli goal list --vault Brogrammers
echo $?    # expect: 0
vault-cli goal list --vault Brogrammers --output json
echo $?    # expect: 0
```

A vault that does have goals must still produce the existing output:
```bash
vault-cli goal list --vault Personal
# expect: existing list of goals, exit 0
```
</verification>
