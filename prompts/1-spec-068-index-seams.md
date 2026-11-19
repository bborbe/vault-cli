---
status: draft
spec: [068-pkg-index-metadata-page-index]
created: "2026-11-19T20:45:00Z"
branch: dark-factory/pkg-index-metadata-page-index
---

# Add the page index seams: keys, fingerprints, folder listing and metadata-only page reads

<summary>
- vault-cli gains a new exported `pkg/index` package with no implementation of its own yet, only the seams the later prompts build on.
- A folder is identified by a key that cleans both of its path parts, so equivalent paths share one index entry.
- A page is read through the same single-page read vault-cli's own folder listing uses, so a page the index serves is byte-identical to the one storage serves.
- The read drops the file body before returning, so an indexed page never carries content.
- Each indexed page also carries its pre-parsed Open Questions, with every retained question text copied so it cannot pin a file body in memory.
- A folder listing is read once and yields each page file with its size, modification time and status-change time, which is the only input the later change detection needs.
- Two counters (one for single-file reads, one for refused changed-page bounds) are defined as an interface the consumer injects, so the package itself imports no metrics library and no glog.
- Reader behaviour is proven by tests that cover the body drop, the frontmatter/name/path/mtime match, the Open Questions projection, the empty-but-non-nil slice and the no-shared-memory rule.
</summary>

<objective>
Create the exported seams of `pkg/index` — `Key`, `FileFingerprint`, `FileEntry`, `Entry`, `PageReader`, `DirectoryLister`, `Metrics` and their production constructors — so the snapshot engine (prompt 2) and the bolt store (prompt 3) can be written against a fixed contract. This is spec 068 prompt 1 of 3: Desired Behavior 1 (read side), 2 (entries) and 5 (library hygiene), plus Acceptance Criteria 7 and 13.
</objective>

<context>
Read `CLAUDE.md`, `docs/development-patterns.md` and `docs/dod.md` first.

Read these files fully before writing anything:

- `specs/in-progress/068-pkg-index-metadata-page-index.md` — Summary, Goal, Non-goals, Desired Behavior 1, 2 and 5, Constraints "Decisions (frozen for this spec)", "Must not change", Assumptions, Failure Modes, Security, AC2 (Entry), AC7, AC13.
- `pkg/storage/page.go` — `pageStorage.ListPages`, `pageStorage.ReadPage`, `isBarePageName`, `readPageFromPath`. This is the single parse path and the symlink-out-of-vault guard the new reader must delegate to.
- `pkg/storage/base.go` — `ParseOpenQuestions(ctx context.Context, content string) ([]OpenQuestionItem, error)`; read the `OpenQuestionItem` type so the projection is exact.
- `pkg/storage/storage.go` — the `PageStorage` interface (`ListPages`, `ReadPage`). `NewPageStorage(nil)` is the production constructor used in tests.
- `pkg/domain/page.go` — `Page` (embeds `FrontmatterMap`, `FileMetadata`, `Content`) and `NewPage(data map[string]any, meta FileMetadata, content Content) *Page`.
- `pkg/domain/file_metadata.go` — `FileMetadata{Name, FilePath string; ModifiedDate *time.Time}`.
- `pkg/domain/open_question.go` — `OpenQuestion{Index int; Text string}`.
- `pkg/domain/content.go` — `Content string`.
- `pkg/index/index_suite_test.go`-style suite files elsewhere in the repo, e.g. `pkg/storage/storage_suite_test.go` — the Ginkgo v2 suite shape and the `//go:generate` counterfeiter directive.
- `pkg/ops/export_test.go` — the pattern for exposing package internals to an external `_test` package.
- `Makefile` targets `generate`, `test`, `precommit`, `addlicense`, and `.golangci.yml` (funlen 80 lines / 50 statements, nestif 4, gocognit 20, depguard denies `github.com/pkg/errors`).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`
</context>

<requirements>
1. **Create the package documentation** at `pkg/index/doc.go`. Package clause `index`; the doc comment states in two or three sentences that the package keeps each vault folder's page metadata in memory so a list read needs no file open, that it never holds a page body (listed pages carry empty `Content`, entries carry pre-parsed Open Questions, a body is read on request only), and it ends with the sentence `See docs/page-index.md for the full contract.` The literal string `docs/page-index.md` must appear in the package doc comment — AC14 greps `go doc ./pkg/index` for it.

2. **Create `pkg/index/key.go`** with exactly:

   ```go
   type Key struct {
       VaultPath string
       PagesDir  string
   }

   func NewKey(vaultPath, pagesDir string) Key
   ```

   `NewKey` applies `filepath.Clean` to both parts (import `path/filepath`) so the read side and an event side derive identical keys from equivalent paths. Document that in the GoDoc.

3. **Create `pkg/index/seams.go`** holding the shared shapes and the two read seams:

   ```go
   type FileFingerprint struct {
       Size             int64
       ModTime          time.Time
       StatusChangeTime time.Time
   }

   type FileEntry struct {
       Name        string
       Fingerprint FileFingerprint
   }

   type Entry struct {
       Page          *domain.Page
       OpenQuestions []domain.OpenQuestion
   }
   ```

   `Entry`'s GoDoc must say that `Page.Content` is always empty, that `OpenQuestions` is never nil and every `Text` shares no memory with the file body, and that a published `Entry` and its `Page` are read-only for every consumer.

   Then the two interfaces, each with a `//counterfeiter:generate -o ../../mocks/index-<name>.go --fake-name Index<Name> . <Name>` directive on the line directly above the type (mirror the directive style already used in `pkg/storage/storage.go`):

   ```go
   type PageReader interface {
       ReadEntry(ctx context.Context, vaultPath string, pagesDir string, filename string) (*Entry, FileFingerprint, error)
       ReadPage(ctx context.Context, vaultPath string, pagesDir string, name string) (*domain.Page, error)
   }

   type DirectoryLister interface {
       ListFiles(ctx context.Context, vaultPath string, pagesDir string) ([]FileEntry, error)
   }
   ```

   `ReadEntry` takes the file's base name **including** the `.md` suffix and returns the metadata-only entry plus the pre-read fingerprint. A non-nil error means the file is excluded (absent, unreadable, unparsable, symlink out of the vault); the returned fingerprint is then still the best available value and is the zero value when the stat itself failed. `ReadPage` takes the bare base name **without** `.md` and returns the full page including `Content`; it is never cached. State both in the interface's GoDoc.

   Also add the production constructors in this file:

   ```go
   func NewPageReader(pages storage.PageStorage) PageReader
   func NewDirectoryLister() DirectoryLister
   ```

   (`storage` is `github.com/bborbe/vault-cli/pkg/storage`.)

4. **Create `pkg/index/reader.go`** with the private `pageReader` struct (one field: the `storage.PageStorage`), plus `ReadEntry`, `ReadPage` and `projectOpenQuestions`.

   - `ReadEntry` stats the file first (via `statFingerprint` from requirement 7), then calls `pages.ReadPage(ctx, vaultPath, pagesDir, strings.TrimSuffix(filename, ".md"))`, then projects the Open Questions from `string(page.Content)`, then returns `&Entry{Page: metadataOnly, OpenQuestions: questions}` where `metadataOnly` is a **copy** of the returned page with `Content` set to `""`. Copy the struct value (`metadataOnly := *page; metadataOnly.Content = ""`) — never mutate the page the storage layer returned.
   - On a stat failure return `(nil, FileFingerprint{}, wrapped error)`; on a read or Open Questions failure return `(nil, fingerprint, wrapped error)`. Wrap with `github.com/bborbe/errors` (`errors.Wrapf(ctx, err, "read page %s", filePath)` / `errors.Wrap(ctx, err, "parse open questions")`); never `fmt.Errorf`.
   - `ReadPage` delegates to `pages.ReadPage` and wraps a failure as `read page %s` with the bare name.
   - `projectOpenQuestions(ctx context.Context, content string) ([]domain.OpenQuestion, error)` calls `storage.ParseOpenQuestions`, then builds `make([]domain.OpenQuestion, 0, len(items))` and appends `domain.OpenQuestion{Index: item.Index, Text: strings.Clone(item.Question)}`. The `strings.Clone` is mandatory: the retained text must share no memory with `content`. A parse error is returned wrapped; a page with no section yields the empty non-nil slice.

5. **Create `pkg/index/lister.go`** with the private `directoryLister` struct and `ListFiles`. It must mirror `pageStorage.ListPages`'s walk exactly, because prompt 2's equivalence test compares the two listings:

   - `targetDir := filepath.Join(vaultPath, pagesDir)`; `os.ReadDir(targetDir)`.
   - `fs.ErrNotExist` returns `(nil, nil)` — a missing folder is an empty listing, not an error (use `errors.Is(err, fs.ErrNotExist)`).
   - Skip directories (`entry.IsDir()`) and names that do not end in `.md` (`strings.HasSuffix(entry.Name(), ".md")`).
   - For each kept entry, `statFingerprint(filepath.Join(targetDir, entry.Name()))`; if the stat fails, keep the entry with a zero `FileFingerprint` (so the reader's exclusion path still runs and a later stat-diff does not re-read the file every pass).
   - Preserve `os.ReadDir` order; return `make([]FileEntry, 0, len(entries))`.
   - Other `ReadDir` errors are wrapped with `errors.Wrapf(ctx, err, "read directory %s", targetDir)`.

6. **Create `pkg/index/metrics.go`** with:

   - Read-reason constants `ReadReasonBuild = "build"`, `ReadReasonEvent = "event"`, `ReadReasonWrite = "write"`, `ReadReasonRescan = "rescan"`, `ReadReasonReload = "reload"` and a `ReadReasons []string` listing all five in that order.
   - Unbounded-reason constants `UnboundedUnknownKey = "unknown_key"`, `UnboundedRevisionAhead = "revision_ahead"`, `UnboundedWindowNotRetained = "window_not_retained"`, `UnboundedForcedReload = "forced_reload"`, `UnboundedFirstPublication = "first_publication"`, `UnboundedFolderMarkExplicit = "folder_mark_explicit"`, `UnboundedFolderMarkWidenedName = "folder_mark_widened_name_not_in_snapshot"`, `UnboundedFolderMarkWidenedFile = "folder_mark_widened_file_absent"`, `UnboundedFolderMarkWidenedNameInvalid = "folder_mark_widened_invalid_name"`, `UnboundedFolderMarkWidenedKey = "folder_mark_widened_key_unpublished"` and an `UnboundedReasons []string` listing all ten in that order.
   - The injected counter interface, with a counterfeiter directive producing `mocks/index-metrics.go` / fake name `IndexMetrics`:

     ```go
     type Metrics interface {
         FileRead(reason string)
         ChangedPagesUnbounded(reason string)
     }

     func NewNoopMetrics() Metrics
     ```

     `NewNoopMetrics` returns an unexported empty struct whose two methods are no-ops. The package must import no metrics library and no glog (AC13): this file imports nothing.

7. **Create the per-OS fingerprint stat** in `pkg/index/fingerprint_linux.go` (`//go:build linux`) and `pkg/index/fingerprint_darwin.go` (`//go:build darwin`), each with:

   ```go
   func statFingerprint(path string) (FileFingerprint, error)
   ```

   Use `os.Stat` (never `os.Lstat`) so symlinks are followed — that is what makes an in-vault symlink and an out-of-vault symlink behave exactly as they do in `pageStorage.ListPages`. Fill `Size` from `info.Size()` and `ModTime` from `info.ModTime().UTC()`. Fill `StatusChangeTime` from the platform stat when the type assertion succeeds:
   - linux: `info.Sys().(*syscall.Stat_t)` → `time.Unix(stat.Ctim.Unix()).UTC()`
   - darwin: `info.Sys().(*syscall.Stat_t)` → `time.Unix(stat.Ctimespec.Unix()).UTC()`

   When the assertion fails, leave `StatusChangeTime` zero. Do not return an error for that case.

8. **Create the Ginkgo suite** at `pkg/index/index_suite_test.go` in package `index_test`, mirroring `pkg/storage/storage_suite_test.go`: `TestIndex(t *testing.T)`, `time.Local = time.UTC`, `format.TruncatedDiff = false`, `RegisterFailHandler(Fail)`, `RunSpecs(t, "Index Suite", ...)` with `suiteConfig.Timeout = 60 * time.Second`, and the directive line

   ```go
   //go:generate go run github.com/maxbrunsfeld/counterfeiter/v6@v6.12.2 -generate
   ```

   above the test function so `make generate` produces the three fakes from this package's annotations.

9. **Create `pkg/index/reader_test.go`** (package `index_test`, real temp vault via `GinkgoT().TempDir()`, production reader `index.NewPageReader(storage.NewPageStorage(nil))`) containing a `Describe("PageReader", ...)` with at least these four specs, with these exact texts (AC3 and AC7 grep them):

   - `It("projects open questions from ParseOpenQuestions", ...)` — write a page whose body has `## Open Questions` with two items (one carrying `→ answer`), assert `entry.OpenQuestions` equals the projection of `storage.ParseOpenQuestions` with `Text = item.Question`, and that it has length 2.
   - `It("returns an empty non-nil slice when the page has no Open Questions section", ...)` — a page with only `## Notes` yields `entry.OpenQuestions` that `BeEmpty()` and is `NotTo(BeNil())`.
   - `It("drops the body but keeps frontmatter, name, path and modification date", ...)` — `entry.Page.Content` is `""`, `entry.Page.GetString("status")` (or another frontmatter key) is the file's value, and `Name` / `FilePath` / `ModifiedDate` equal the file's (`FilePath` equal to `filepath.Join(vaultDir, folder, filename)`, `ModifiedDate` non-nil).
   - `It("reads the full page including its body on ReadPage", ...)` — `ReadPage` with the bare name returns `Content` byte-for-byte equal to the file's contents, and a subsequent `ReadEntry` still reports `Content == ""`.

10. **Create `pkg/index/reader_internal_test.go`** (package `index`) containing `ginkgo.Describe("projectOpenQuestions", ...)` with the spec text `does not retain open-question text that shares memory with the file body` (AC7 greps it). Implementation: call `projectOpenQuestions(context.Background(), body)`; compute the body's address range with `unsafe.Pointer(unsafe.StringData(body))` and `len(body)`; assert each `question.Text` is non-empty and lies entirely outside that range (its start plus length is `<= bodyStart`, or its start is `>= bodyEnd`). Use `unsafe` deliberately and only in this test file.

11. **Run `ROOTDIR=/workspace make generate`** once after the annotations exist, then confirm `mocks/index-page-reader.go`, `mocks/index-directory-lister.go` and `mocks/index-metrics.go` exist with the fake names above.

12. **Self-check before finishing**: re-run the `<verification>` block, confirm every command passes, and walk requirements 1–11 against the files on disk.
</requirements>

<constraints>
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage` (spec AC16). This prompt only adds `pkg/index` files and generated mocks.
- Do NOT switch any vault-cli CLI command to the index — the CLI keeps reading files directly (spec Non-goal).
- Do NOT cache a page body anywhere, in memory or otherwise; `Entry.Page.Content` is always `""` and `ReadPage` always goes to storage (spec Non-goal).
- Do NOT project or intern frontmatter keys — the full frontmatter map is retained.
- `pkg/index` must import neither `github.com/golang/glog` nor any `github.com/prometheus/*` package directly (AC13). Transitive imports via `github.com/bborbe/kv` are allowed.
- Every new `.go` file needs the repository license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` + the BSD two-line notice); `make precommit` runs `addlicense` and fails without it.
- Wrap errors with `github.com/bborbe/errors`; never `fmt.Errorf`, never `github.com/pkg/errors` (depguard denies it), never `context.Background()` inside the package for wrapping.
- Respect the linter: functions under 80 lines / 50 statements, nestif under 4, gocognit under 20. Split files rather than growing one past ~600 lines.
- Use Ginkgo v2 / Gomega only; external test package `index_test` except where `unsafe`/private access is required (`reader_internal_test.go`).
- Do NOT add configuration for store location, rescan interval, timeouts or change-log size — ported constants are invariant (spec Non-goal).
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must exit 0.

Then confirm the package compiles and its tests pass with named specs visible:

```
GOFLAGS=-buildvcs=false go test ./pkg/index/... -ginkgo.v
```

Then confirm the public surface exists:

```
go doc ./pkg/index Entry          # must print "Page" and "OpenQuestions"
go doc ./pkg/index NewPageReader
go doc ./pkg/index NewDirectoryLister
go doc ./pkg/index | grep -c 'docs/page-index.md'   # must print 1 or more
```

Then confirm the import hygiene of AC13:

```
go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -E 'golang/glog|prometheus'
```

This command must print zero lines (a non-zero grep exit here is the expected "no match" result).
</verification>
