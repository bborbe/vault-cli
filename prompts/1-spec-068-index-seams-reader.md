---
spec: ["068-pkg-index-metadata-page-index"]
status: draft
created: "2026-11-18T23:16:38Z"
---

# Build the page index seams and the metadata-only page reader

<summary>
- A new shared library can name a vault folder and read its pages' metadata without keeping any page body in memory.
- Each read page keeps its full frontmatter, name, file path and modification date.
- Each page also carries its pre-parsed Open Questions, and every retained question is a copy that cannot pin a page body in memory.
- Folder listing matches the existing CLI listing: markdown files only, directory order, a missing folder reads as empty, an unreadable file is excluded.
- A per-file fingerprint records size, modification time and status-change time from a stat that follows symlinks.
- Warnings go through a replaceable sink that defaults to structured logging.
- Counters go through an injected interface with a no-op default, so the library imports no metrics client and no legacy logger.
- Unit tests cover the reader, the lister, key normalization and the Open Questions memory rule.
</summary>

<objective>
Establish the page index's public data types and read seams so the snapshot engine and the on-disk store can be built on a read path that provably holds no page bodies.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/dod.md` for the definition of done this change is graded against.

Read these existing files before writing anything:

- `pkg/storage/page.go` — `(*pageStorage).ListPages` and `(*pageStorage).ReadPage` are the one parse path. `ReadPage` takes a bare base name WITHOUT the `.md` suffix and rejects a name containing a path separator. `ListPages` skips directories and non-`.md` files, and skips an unreadable page with a warning.
- `pkg/storage/storage.go` — `PageStorage` interface (`ListPages`, `ReadPage`) and `NewPageStorage(*Config)` (a nil config means `DefaultConfig()`).
- `pkg/storage/base.go` — `ParseOpenQuestions(ctx context.Context, content string) ([]OpenQuestionItem, error)` and `OpenQuestionItem{Index int; Question string; Answer string; Marker string; Line int}`. Only `Index` and `Question` are used here.
- `pkg/domain/page.go` — `Page` embeds `FrontmatterMap`, `FileMetadata` and `Content`; `NewPage(data map[string]any, meta FileMetadata, content Content) *Page`.
- `pkg/domain/file_metadata.go` — `FileMetadata{Name string; FilePath string; ModifiedDate *time.Time}`.
- `pkg/domain/open_question.go` — `OpenQuestion{Index int; Text string}`.
- `pkg/index/` — this package already exists in the working tree from an earlier pass, and `pkg/index` is named in the released `## v0.197.0` CHANGELOG section. Read every existing file before editing. Keep code and tests that already satisfy the contract below; add or correct what does not. Do not delete passing tests and do not recreate a file that already matches.

Coding conventions to follow:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md` — public interface, private struct, `New*` constructor, counterfeiter annotation.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — wrap with `errors.Wrap` / `errors.Wrapf` from `github.com/bborbe/errors`; never `fmt.Errorf`.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test package, suite file, counterfeiter mocks.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md` — GoDoc comments start with the name and state behaviour.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80, nestif 4, golines 100, license headers.
</context>

<requirements>
1. Create `pkg/index/doc.go` with the package doc. It must state, in this order: the package keeps each vault folder's page metadata in memory so a list read can be answered without opening a vault file; it never holds a page body (listed pages carry empty `Content`, each entry carries its pre-parsed Open Questions, and a body is read from disk on request only); and it points to `docs/page-index.md` for the full contract. The package clause is `package index`.

2. Create `pkg/index/key.go` with exactly this public shape:

   ```go
   type Key struct {
       VaultPath string
       PagesDir  string
   }

   func NewKey(vaultPath, pagesDir string) Key
   ```

   `NewKey` applies `filepath.Clean` to both parts, so equivalent paths (for example `Tasks/` and `Tasks`) produce identical keys. GoDoc states why.

3. Create `pkg/index/seams.go` with these types, interfaces and constructors:

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

   type PageReader interface {
       ReadEntry(ctx context.Context, vaultPath string, pagesDir string, filename string) (*Entry, FileFingerprint, error)
       ReadPage(ctx context.Context, vaultPath string, pagesDir string, name string) (*domain.Page, error)
   }

   type DirectoryLister interface {
       ListFiles(ctx context.Context, vaultPath string, pagesDir string) ([]FileEntry, error)
   }

   func NewPageReader(pages storage.PageStorage) PageReader
   func NewDirectoryLister() DirectoryLister
   ```

   Contract details to document on each declaration:
   - `Entry.OpenQuestions` is never nil, and every `Text` is a copy that shares no memory with the file body.
   - `Entry.Page` and the `Entry` itself are read-only for every consumer once published.
   - `PageReader.ReadEntry` takes the filename WITH its `.md` suffix. A non-nil error means the file is excluded (absent, unreadable, unparsable, or a symlink out of the vault) and the returned fingerprint is still the best available (zero when the stat failed).
   - `PageReader.ReadPage` takes the bare base name WITHOUT `.md` and returns the full page including its body; it is never cached.
   - `DirectoryLister.ListFiles` returns `.md` files only, no directories, in `os.ReadDir` order.

   Add the counterfeiter directives exactly:

   ```go
   //counterfeiter:generate -o ../../mocks/index-page-reader.go --fake-name IndexPageReader . PageReader
   //counterfeiter:generate -o ../../mocks/index-directory-lister.go --fake-name IndexDirectoryLister . DirectoryLister
   ```

4. Create `pkg/index/fingerprint_linux.go` (`//go:build linux`) and `pkg/index/fingerprint_darwin.go` (`//go:build darwin`), each with the unexported function:

   ```go
   func statFingerprint(path string) (FileFingerprint, error)
   ```

   It uses `os.Stat` (follows symlinks, unlike `os.Lstat`) and returns `Size` from `info.Size()`, `ModTime` from `info.ModTime().UTC()`, and `StatusChangeTime` from the platform's status-change field when `info.Sys()` is `*syscall.Stat_t`: `stat.Ctim` on linux, `stat.Ctimespec` on darwin, both converted with `time.Unix(...).UTC()`. A stat error returns the zero `FileFingerprint` and the error. Keep the two files otherwise identical so the per-OS difference is only the status-change field name.

5. Create `pkg/index/lister.go` with the private `directoryLister struct{}` and `ListFiles` on it:
   - join `vaultPath` and `pagesDir`, then `os.ReadDir`;
   - an `fs.ErrNotExist` error returns `(nil, nil)` — a missing folder lists as empty, matching `storage.ListPages`;
   - any other read error returns `errors.Wrapf(ctx, err, "read directory %s", targetDir)`;
   - skip entries where `entry.IsDir()` is true and names that do not end in `.md`;
   - for each kept entry call `statFingerprint`; on a stat error keep the entry with a zero `FileFingerprint`. The zero fingerprint is deliberate: it keeps the reader's exclusion path reachable and stops a later stat-diff from re-reading a permanently broken file on every pass.
   - return a slice preallocated with `make([]FileEntry, 0, len(entries))`.

6. Create `pkg/index/reader.go` with the private `pageReader struct{ pages storage.PageStorage }` and the two `PageReader` methods:
   - `ReadEntry`: build `filePath := filepath.Join(vaultPath, pagesDir, filename)`; stat it first with `statFingerprint` and on error return `(nil, FileFingerprint{}, errors.Wrapf(ctx, err, "stat %s", filePath))`; call `r.pages.ReadPage(ctx, vaultPath, pagesDir, strings.TrimSuffix(filename, ".md"))` and on error return `(nil, fingerprint, errors.Wrapf(ctx, err, "read page %s", filePath))`; project the Open Questions from `string(page.Content)`; copy the page value (`metadataOnly := *page; metadataOnly.Content = ""`) so the caller's page is metadata-only and the body is dropped; return the `*Entry` and the pre-read fingerprint.
   - `ReadPage`: delegate to `r.pages.ReadPage` and wrap the error with `errors.Wrapf(ctx, err, "read page %s", name)`.
   - Add `projectOpenQuestions(ctx context.Context, content string) ([]domain.OpenQuestion, error)`: call `storage.ParseOpenQuestions`, wrap a parse error with `errors.Wrap(ctx, err, "parse open questions")`, allocate `make([]domain.OpenQuestion, 0, len(items))`, and append `domain.OpenQuestion{Index: item.Index, Text: strings.Clone(item.Question)}`. The clone is load-bearing: a substring of the body would keep the whole body reachable from the retained entry.

7. Create `pkg/index/metrics.go` with:
   - the `Metrics` interface and its counterfeiter directive:

     ```go
     //counterfeiter:generate -o ../../mocks/index-metrics.go --fake-name IndexMetrics . Metrics

     type Metrics interface {
         FileRead(reason string)
         ChangedPagesUnbounded(reason string)
     }
     ```

   - `func NewNoopMetrics() Metrics` returning a private `noopMetrics struct{}` whose two methods do nothing.
   - The reason constants the engine will pass, so the vocabulary lives with the interface:

     ```go
     const (
         ReadReasonBuild  = "build"
         ReadReasonEvent  = "event"
         ReadReasonWrite  = "write"
         ReadReasonRescan = "rescan"
         ReadReasonReload = "reload"
     )

     var ReadReasons = []string{ReadReasonBuild, ReadReasonEvent, ReadReasonWrite, ReadReasonRescan, ReadReasonReload}

     const (
         UnboundedUnknownKey                   = "unknown_key"
         UnboundedRevisionAhead                = "revision_ahead"
         UnboundedWindowNotRetained            = "window_not_retained"
         UnboundedForcedReload                 = "forced_reload"
         UnboundedFirstPublication             = "first_publication"
         UnboundedFolderMarkExplicit           = "folder_mark_explicit"
         UnboundedFolderMarkWidenedName        = "folder_mark_widened_name_not_in_snapshot"
         UnboundedFolderMarkWidenedFile        = "folder_mark_widened_file_absent"
         UnboundedFolderMarkWidenedNameInvalid = "folder_mark_widened_invalid_name"
         UnboundedFolderMarkWidenedKey         = "folder_mark_widened_key_unpublished"
     )

     var UnboundedReasons = []string{
         UnboundedUnknownKey, UnboundedRevisionAhead, UnboundedWindowNotRetained,
         UnboundedForcedReload, UnboundedFirstPublication, UnboundedFolderMarkExplicit,
         UnboundedFolderMarkWidenedName, UnboundedFolderMarkWidenedFile,
         UnboundedFolderMarkWidenedNameInvalid, UnboundedFolderMarkWidenedKey,
     }
     ```

   `ReadReasons` and `UnboundedReasons` exist so a consumer adapter can pre-register a metric for every label value instead of creating one lazily.

8. Do not add any glog or prometheus import to `pkg/index`. Warnings must be emitted only through a `func(format string, args ...any)` sink (the engine adds the sink and the default in the next prompt); counters only through `Metrics`.

9. Create the Ginkgo test infrastructure:
   - `pkg/index/index_suite_test.go` in `package index_test`: set `time.Local = time.UTC`, set `format.TruncatedDiff = false`, register the fail handler, configure a 60-second suite timeout, run `"Index Suite"`, and add the mock-generation directive `//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6@v6.12.2 -generate`.
   - `pkg/index/reader_test.go` in `package index_test`, driving the production reader over `storage.NewPageStorage(nil)` and real temp folders. It must contain these specs, with these exact descriptions:
     - `Describe("PageReader")` with `It("projects open questions from ParseOpenQuestions")` (compare against `storage.ParseOpenQuestions` on the same body, mapping `Index` and `Question` to `domain.OpenQuestion{Index, Text}`), `It("returns an empty non-nil slice when the page has no Open Questions section")`, `It("drops the body but keeps frontmatter, name, path and modification date")`, `It("reads the full page including its body on ReadPage")`, and `It("reports an error and a zero fingerprint for a missing file")`.
     - `Describe("DirectoryLister")` with `It("lists .md files only, skipping directories, in directory order")` and `It("returns nil for a missing folder")`.
     - `Describe("NewKey")` with `It("cleans both parts so equivalent paths share a key")`.
     - `Describe("Metrics")` with `It("accepts calls on the no-op implementation")`.
   - `pkg/index/reader_internal_test.go` in `package index` (so it can call the unexported `projectOpenQuestions`), with `Describe("projectOpenQuestions")` and the exact spec `It("does not retain open-question text that shares memory with the file body")`. Assert it with address ranges: take `uintptr(unsafe.Pointer(unsafe.StringData(body)))` and `+ len(body)` as the body range, and require every retained `Text` to start outside that range. This is the boundary guard for the clone rule; a shape-only assertion (`Text` non-empty) does not satisfy it.

10. These tests traverse the real boundaries this layer crosses: `storage.ParseOpenQuestions` (the Open Questions contract), `storage.PageStorage.ReadPage` (the single parse path and the symlink-out-of-vault guard) and the filesystem stat. Do not replace them with fakes.

11. Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage`; this prompt adds a new package only
- `pkg/index` must import neither `github.com/golang/glog` nor any prometheus package (direct imports). Transitive imports via another dependency are allowed.
- Do not keep a page body anywhere: no field on a public or private type may hold a page's `Content`
- Open Questions text must be cloned; retaining a substring of the body is a contract violation
- New module dependencies are limited to `github.com/bborbe/boltkv`, `go.etcd.io/bbolt` and `github.com/bborbe/kv`, and only the store prompt may add them — this prompt needs none
- Builds for darwin and linux with `CGO_ENABLED=0`
- Every new file carries the project's BSD license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` plus the two following lines), matching `pkg/storage/page.go`
- Do not run `go mod vendor`; `vendor/` is a build-time artifact
- Existing tests must still pass
</constraints>

<verification>
make precommit

go test ./pkg/index/... -ginkgo.v

go doc ./pkg/index Entry

! go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -qE 'golang/glog|prometheus'
</verification>
