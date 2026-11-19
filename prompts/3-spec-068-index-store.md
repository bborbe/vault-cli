---
status: draft
spec: [068-pkg-index-metadata-page-index]
created: "2026-11-19T20:45:00Z"
branch: dark-factory/pkg-index-metadata-page-index
---

# Add the metadata-only bolt store and hydrate the page index from it

<summary>
- A restarted process can skip re-parsing unchanged files by loading the index from a local cache file.
- The cache holds page metadata only: no file body is ever written to it, and the raw cache file contains no body bytes.
- Each cached page also carries its Open Questions, so a hydrated index serves them without opening a single vault file.
- A cache written by an older vault-ui version, by a different parser version, or a damaged one is discarded rather than converted, and the warning says which of those happened and where the file is.
- A cache file locked by another process never hangs startup: the index waits a bounded five seconds, then serves from a full parse.
- Every publication writes exactly what changed and what vanished, in one transaction, without making readers wait for disk.
- The cache lives under the user cache directory, never inside a vault, and its directory is created private.
- The contract document gains its store section and the package keeps importing no logging or metrics library.
</summary>

<objective>
Add the optional on-disk page index store to `pkg/index`: the `Store` seam, the bolt-backed implementation with its writer-identity reconcile and distinct discard reasons, the metadata-only codec, and the engine wiring that hydrates a key from the store and writes each publication through. This is spec 068 prompt 3 of 3: Desired Behaviors 6 and 7, plus Acceptance Criteria 9 (`Store hydration`), 10, 11, 12 and 14 (`Store format version 2`). It depends on prompts 1 and 2.
</objective>

<context>
Read `CLAUDE.md`, `docs/development-patterns.md` and `docs/dod.md` first.

Read these files fully before writing anything:

- `specs/in-progress/068-pkg-index-metadata-page-index.md` — Desired Behaviors 6 and 7, Constraints "Decisions (frozen for this spec)" (especially the frozen store names and `ParserVersion`), Assumptions (the codec trap), Failure Modes (every store row), Security, AC9, AC10, AC11, AC12, AC14.
- `pkg/index/index.go`, `pkg/index/index_build.go`, `pkg/index/index_file.go`, `pkg/index/export_test.go` — prompt 2's engine. You will add the store field, the hydrate build kind, the delta computation and the write-through hook; the ported refresh contract must not change.
- `pkg/index/seams.go` — `Entry`, `FileFingerprint`, `FileEntry`; the stored record is built from these.
- `pkg/index/reader.go` — `ReadEntry` returns the metadata-only entry and the fingerprint, which is exactly one stored record.
- `pkg/domain/page.go` — `domain.NewPage(data map[string]any, meta FileMetadata, content Content) *Page`.
- `pkg/domain/frontmatter_map.go` — `RawMap() map[string]any` (the only lossless way out of the frontmatter map).
- `pkg/domain/file_metadata.go`, `pkg/domain/open_question.go`, `pkg/domain/content.go`.
- `pkg/ops/export_test.go` — the `_test` helper pattern.
- `docs/page-index.md` — prompt 2's contract document; you add the seventh section.
- `Makefile` (`generate`, `ensure`, `test`, `precommit`) and `.golangci.yml`.

Library APIs already verified for this prompt (all three are already direct dependencies in `go.mod`):
- `github.com/bborbe/boltkv` v1.17.0 — `boltkv.OpenFile(ctx context.Context, path string, fn ...ChangeOptions) (DB, error)` where `ChangeOptions` is `func(opts *bbolt.Options)`; `DB` has `Update(ctx, func(ctx context.Context, tx kv.Tx) error) error`, `View(...)`, `Close() error`.
- `github.com/bborbe/kv` v1.21.30 — `kv.NewBucketName(name string) BucketName`; `Tx.CreateBucketIfNotExists(ctx, kv.BucketName) (kv.Bucket, error)`, `Tx.Bucket(ctx, kv.BucketName) (kv.Bucket, error)`, `Tx.DeleteBucket(ctx, kv.BucketName) error`, `Tx.ListBucketNames(ctx) ([]kv.BucketName, error)`; `kv.Bucket` has `Get`, `Put`, `Delete`, `Iterator`, `Name`; `kv.Item.Value(func([]byte) error) error`; `kv.ForEach(ctx, bucket, func(kv.Item) error) error`; `kv.BucketNotFoundError`.
- `go.etcd.io/bbolt` v1.5.0 — `bolt.Options{Timeout time.Duration}`; `bolt.Open(path string, mode os.FileMode, opts *bolt.Options) (*bolt.DB, error)`.
- `github.com/bborbe/time` (as `libtime`) — `libtime.DateOrDateTime` has `Time() time.Time`; it implements `BinaryMarshaler` but **not** `BinaryUnmarshaler`, so gob cannot round-trip it.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-patterns.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-mocking-guide.md`
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-doc-best-practices.md`
</context>

<requirements>
1. **Declare the store seam in `pkg/index/store.go`.** Frozen constants (byte-identical to vault-ui's v1 store so an old file is recognised, never left behind):

   ```go
   const StoreFormatVersion = 2
   const storeOpenTimeout = 5 * time.Second
   const storeMetaBucketName = "vault-ui-page-index-meta"
   const storeBucketPrefix = "vault-ui-page-index\x00"
   const storeIdentityKey = "identity"
   const vaultCLIModulePath = "github.com/bborbe/vault-cli"
   const unknownParserVersion = "(unknown)"
   ```

   ```go
   type WriterIdentity struct {
       StoreFormat   int
       ParserVersion string
   }
   ```

   No JSON tags on those fields: the persisted identity JSON must use the literal keys `StoreFormat` and `ParserVersion` (AC11 writes and reads them literally).

   The store interface, with a counterfeiter directive producing `mocks/index-store.go` / fake name `IndexStore`:

   ```go
   type Store interface {
       Load(ctx context.Context, key Key) (entries []StoredEntry, ok bool, err error)
       Write(ctx context.Context, key Key, puts []StoredEntry, deletes []string) error
       Path() string
       Close() error
   }
   ```

   `Load` reports `ok == false` whenever it cannot supply usable entries (that is the full-parse path) and returns a non-nil `err` only when the ctx was cancelled. `Write` applies one publication's delta in a single transaction: `puts` replaces or inserts entries keyed by `Filename`, `deletes` removes them. Document both.

   Add `StoredEntry` next to it (exported; the store API exposes it):

   ```go
   type StoredEntry struct {
       Filename    string
       Entry       *Entry // nil when the file was excluded
       Fingerprint FileFingerprint
   }
   ```

2. **Path and identity helpers.**

   ```go
   func DefaultStorePath(app string) (string, error)
   func CurrentWriterIdentity(ctx context.Context) WriterIdentity
   ```

   - `DefaultStorePath` is a pure path helper: `os.UserCacheDir()`, then `filepath.Join(cacheDir, app, "page-index.bolt")`. It creates nothing. It has no ctx parameter by contract, so return the stdlib error unwrapped.
   - `CurrentWriterIdentity` returns `{StoreFormat: StoreFormatVersion, ParserVersion: vaultCLIParserVersion()}`.
   - `vaultCLIParserVersion()` reads `runtime/debug.ReadBuildInfo()`: the `Deps` entry whose `Path` equals `vaultCLIModulePath` when vault-cli is a dependency; otherwise `info.Main.Path == vaultCLIModulePath && info.Main.Version != ""` when vault-cli is itself main; otherwise `unknownParserVersion`. A failed `ReadBuildInfo` also yields `unknownParserVersion`.

3. **`NewBoltStore`** with vault-ui's signature and the guarantee that it never returns nil and never fails:

   ```go
   func NewBoltStore(
       ctx context.Context,
       path string,
       identity WriterIdentity,
       warnf func(format string, args ...any),
       vlogf func(format string, args ...any),
   ) Store
   ```

   - `os.MkdirAll(filepath.Dir(path), 0700)`; on failure return `&discardedStore{path: path, reason: "create store directory", warnf: warnf}`.
   - `boltkv.OpenFile(ctx, path, func(opts *bolt.Options) { opts.Timeout = storeOpenTimeout })`. The explicit timeout is mandatory: bolt's zero `Timeout` waits forever, so a second process on the same cache path would hang startup (Failure Modes: locked bolt file). On error return `&discardedStore{path: path, reason: "open store", warnf: warnf}`.
   - Build the live `boltStore` and run `reconcile`. If reconcile fails, close the DB (log a close failure at `slog.Debug`) and return `&discardedStore{path: path, reason: "record writer identity", warnf: warnf}`.
   - Count the entry buckets and entries once, set the `empty` flag, and emit `vlogf("page index store %s: %d keys, %d entries", path, keys, entries)`.

   `boltStore` fields: `db boltkv.DB`, `path string`, `identity WriterIdentity`, `empty atomic.Bool`, `warnf`, `vlogf`.

4. **Reconcile with distinct reasons.** `reconcile(ctx)` runs one `db.Update` transaction:

   - `tx.CreateBucketIfNotExists(ctx, kv.NewBucketName(storeMetaBucketName))`; read `storeIdentityKey` into a copied `[]byte` (the value is only valid inside `item.Value`'s callback — copy it).
   - `discardReason(raw []byte) string` returns `""` when the stored identity matches, otherwise, **in this order**:
     - `len(raw) == 0` → `store file missing or empty`
     - JSON unmarshal error → `damaged`
     - `existing.StoreFormat != s.identity.StoreFormat` → `fmt.Sprintf("store format mismatch (stored %d, current %d)", existing.StoreFormat, s.identity.StoreFormat)`
     - `existing.ParserVersion != s.identity.ParserVersion` → `parser version mismatch`
   - On a non-empty reason: `s.warnf("page index store discarded (%s): %s", reason, s.path)` — the wording `discarded` and the reason string and the path must all appear in one warning (AC11 and AC12 grep them); then `deleteEntryBuckets`, then write the current identity JSON back.
   - `deleteEntryBuckets` lists every bucket name and deletes each one whose name is not `storeMetaBucketName`.

5. **Load, Write, Path, Close on `boltStore`.**

   - `Load`: return a wrapped error when `ctx.Err() != nil`; return `(nil, false, nil)` immediately when `empty` is set; otherwise `db.View` over `tx.Bucket(storeBucketName(key))` and `kv.ForEach`, decoding each value with `decodeRecord`. Append decoded entries — **never** preallocate from a stored count or length (Security: the store is untrusted input). On `kv.BucketNotFoundError`, warn `page index store discarded (no stored entries for key): <path>` and return `(nil, false, nil)`; on any other error (or a decode error), warn `page index store discarded (damaged): <path>` and return `(nil, false, nil)`. A non-nil error is reserved for ctx cancellation.
   - `Write`: `db.Update`, call `writeIdentity`, `tx.CreateBucketIfNotExists(ctx, storeBucketName(key))`, then `applyDelta` (put each encoded record by `Filename`, delete each name). Wrap a failure as `write page index store`. On success clear `empty`.
   - `storeBucketName(key Key) kv.BucketName` returns `kv.NewBucketName(storeBucketPrefix + key.VaultPath + "\x00" + key.PagesDir)`.
   - `Path()` returns the path; `Close()` closes the DB.

6. **`discardedStore`** stands in for a store that could not be opened: fields `path`, `reason`, `warnf`. `Load` warns `page index store discarded (<reason>): <path>` and returns `(nil, false, nil)`; `Write` returns `errors.Errorf(ctx, "page index store discarded (%s): %s", reason, path)`; `Path` returns the path; `Close` is a nil-returning no-op. The caller owns the write-failure warning.

7. **Create the codec at `pkg/index/store_codec.go`.**

   ```go
   type storeRecord struct {
       Filename      string
       HasPage       bool
       Frontmatter   map[string]any
       Metadata      domain.FileMetadata
       OpenQuestions []domain.OpenQuestion
       Fingerprint   FileFingerprint
   }
   ```

   `domain.Page` is deliberately **not** encoded directly: its embedded `FrontmatterMap` has only an unexported field, so both gob and JSON silently drop every frontmatter key (spec Assumption). Encode `entry.Entry.Page.RawMap()` and rebuild with `domain.NewPage(record.Frontmatter, record.Metadata, "")`.

   - `init()` registers every concrete type that can appear as a value inside a decoded YAML frontmatter map, because gob encodes interface values by concrete type name and fails on an unregistered one: `""`, `false`, `int(0)`, `int64(0)`, `float64(0)`, `time.Time{}`, `[]any{}`, `map[string]any{}`, `map[any]any{}`. Do **not** register `libtime.DateOrDateTime`: it implements `BinaryMarshaler` but not `BinaryUnmarshaler`, so gob encodes it and fails to decode. Convert it instead (below).
   - `encodeRecord(ctx, entry StoredEntry) ([]byte, error)`: always set `Filename` and `Fingerprint`; when `entry.Entry != nil`, run `entry.Entry.Page.RawMap()` through `encodeValue`, require the result to be `map[string]any` (otherwise `errors.Errorf`), and set `HasPage`, `Frontmatter`, `Metadata` (`entry.Entry.Page.FileMetadata`) and `OpenQuestions`. gob-encode into a `bytes.Buffer`.
   - `decodeRecord(ctx, data []byte) (StoredEntry, error)`: gob-decode; on failure return the zero value and a wrapped error (never a partially-filled record). When `HasPage`, set `Entry = &Entry{Page: domain.NewPage(record.Frontmatter, record.Metadata, ""), OpenQuestions: questions}` where a nil `record.OpenQuestions` becomes `[]domain.OpenQuestion{}` (the entry's slice is never nil).
   - `encodeValue(ctx, value any) (any, error)` converts `libtime.DateOrDateTime` and `*libtime.DateOrDateTime` to the `time.Time` they wrap, walks `map[string]any`, `map[any]any` and `[]any` recursively, and returns every other value unchanged. `encodeMap[K comparable](ctx, map[K]any) (map[K]any, error)` does the per-value conversion.

8. **Wire the store into the engine.** These edits touch prompt 2's files; the ported refresh contract must not change.

   - `pageIndex` gains `store Store`. `entry` gains `storeLoaded bool` (the store is consulted at most once per key per process) and `storedBaseline bool` (the snapshot and fingerprints were seeded from the store and must never be served before a stat-diff).
   - `build` gains `delta *storeDelta`, written under the mutex before `done` is closed and read only by the goroutine that ran the build. Add:

     ```go
     type storeDelta struct {
         puts    []StoredEntry
         deletes []string
     }
     ```

   - Add the hydrate build kind: `buildHydrate` (the key has no snapshot and its store has not been consulted). Extend `covers` so `buildHydrate` covers `buildCold`, `buildStatDiff` and `buildHydrate`.
   - `pendingLocked`: when a key has no snapshot, `!storeLoaded` and `store != nil`, return `buildHydrate` before falling through to `buildCold`.
   - `executeHydrate`: `store.Load(ctx, key)`; mark `storeLoaded = true`; when `ok`, seed the snapshot and the fingerprint map from the stored entries (in `Filename` order) and set `storedBaseline = true`, then run exactly one listing pass (the stat-diff) against that baseline; when `!ok`, fall through to the cold build. A stored record is **never** served as written (Failure Modes / AC9). A failed stat-diff of a store-loaded key publishes nothing.
   - `NewPageIndexWithStore` (frozen signature, added here) and the rewiring:

     ```go
     func NewPageIndexWithStore(
         reader PageReader,
         lister DirectoryLister,
         currentDateTimeGetter libtime.CurrentDateTimeGetter,
         waiter libtime.WaiterDuration,
         metrics Metrics,
         store Store,
     ) PageIndex
     ```

     `NewPageIndex` becomes a one-line delegation passing `nil` for the store. A nil store disables persistence; a nil metrics still becomes `NewNoopMetrics()`.
   - `writeThrough(ctx, key Key, delta *storeDelta)`: return immediately when the store is nil or the delta is nil; otherwise call `store.Write` under a bounded context and, on failure, emit exactly one warning through `p.warnf` naming the path and the error. It is called **after** the index mutex is released, so no reader waits on store I/O.
   - Delta computation: a listing publication's delta holds the entries it re-read (`puts`, carrying their fingerprints) and the names it dropped (`deletes`). A `RefreshFile` publication's delta holds exactly one put (a present file) or one delete (a vanished file). An unchanged stat-diff produces a nil delta, so nothing is written.
   - `applyFileReadLocked` returns the `*storeDelta` it produced (nil when a later-started read already applied), and `RefreshFile` calls `writeThrough` with it.

9. **Extend `pkg/index/export_test.go`** with:

   - `func NewPageIndexWithStoreAndWarnf(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, store Store, warnf func(format string, args ...any)) PageIndex` — builds with `TestMetrics` and applies `SetWarnf`.
   - `func PersistedRecordType() reflect.Type` returning `reflect.TypeOf(storeRecord{})`, so the AC10 reflection test can walk the persisted shape from the external test package.

10. **Write the store tests.**

    `pkg/index/store_v2_test.go` (external package `index_test`) must contain `Describe("Store format version 2")` with at least these exact spec texts:

    - `It("persists no Content field in the store record", ...)` — recursively walk `index.PersistedRecordType()` and assert no field named `Content` exists at any depth while a field named `OpenQuestions` does. Guard the walk with a visited-type set and skip non-struct kinds, so `time.Time`'s unexported fields terminate.
    - `It("writes no body bytes to the store file", ...)` — a fixture whose body contains the sentinel `BODY-SENTINEL-7f3a` outside the frontmatter and outside every Open Questions item; cold-build with a store, close it, read the bolt file with `os.ReadFile`, and assert `bytes.Contains(raw, []byte("BODY-SENTINEL-7f3a"))` is false while a retained Open Questions string (for example `kept question`) is present.
    - `It("discards a v1 store, deletes its entry buckets and warns store format mismatch with the path", ...)` — write the bolt file **directly with `bolt.Open`** using **literal byte strings, never package constants**: bucket `vault-ui-page-index-meta` with key `identity` and value `{"StoreFormat":1,"ParserVersion":"v0.171.0"}` (both format and parser differ — the real vault-ui case), plus one entry bucket named `"vault-ui-page-index\x00" + vaultPath + "\x00" + pagesDir`. Open it through `index.NewBoltStore`, close it, then assert the entry bucket is gone (only `vault-ui-page-index-meta` remains) and the captured warning contains `store format mismatch`, `stored 1, current 2` and the file path.
    - `It("reports parser version mismatch as a distinct reason", ...)` — identity `{"StoreFormat":2,"ParserVersion":"v0.0.0-stale"}`; assert a warning containing `parser version mismatch` and the path, and that no warning contains `store format mismatch`.
    - `It("reports a damaged identity as a distinct reason", ...)` — identity `{not json`; assert a warning containing `damaged`.
    - `It("serves without a store when the bolt file is locked by another process", NodeTimeout(30*time.Second), func(specCtx SpecContext) { ... })` — hold the file open with a second `bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})`, then construct the store and assert construction returns in under 10 s, a discard warning naming the path was emitted, and `ListPages` still serves the fixture's expected names.
    - `It("resolves the store path under the user cache directory per app", ...)` — `DefaultStorePath("vault-api")` equals `filepath.Join(cacheDir, "vault-api", "page-index.bolt")`.
    - `It("creates the store directory with mode 0700", ...)` — after construction, the parent directory's permission is `0700`.

    `Describe("Store hydration")` with `It("restores open questions from the store without re-reading the file", ...)`: index A over a fixture holding an Open Questions section publishes to the store and closes it; index B opens the same store over the unchanged files; B's `ListEntries` `OpenQuestions` equal A's, and B's injected `Metrics` records zero file reads.

    `pkg/index/store_test.go` must cover the remaining store contract: `Describe("Page index store")` with `Describe("codec")` (a lossless round-trip over the awkward-files fixture, and a `libtime.DateOrDateTime` frontmatter value converted to `time.Time`), `Describe("discard on mismatch")` (a never-existing file, a zero-byte file, garbage bytes, an uncreatable directory, a read-permission-removed file, a different store format, a different parser version, and one key served while another full-parses), `Describe("write")` (put and delete in one transaction; a failed write leaves the previous content intact; a written delta is observable through a second store), `Describe("location")` (fixed cache path, default store opened at it, missing directory created, never under a vault), `Describe("reporting")` (no warning on a hit; keys and entries reported at V(2)), and `Describe("bounded open")` (`AC-safety times out instead of hanging on a locked store`).

    `pkg/index/store_writethrough_test.go` must contain `Describe("Store write-through")` with: `AC5: a cold build writes N entries in one transaction`, `AC5: a per-file update writes exactly one put`, `AC5: a deletion writes exactly one delete`, `AC5: a stat-diff that drops a removed file writes exactly one delete`, `AC5: a failed write leaves serving and the store intact`, `AC5: a blocking write does not block a reader`, `AC5: an unchanged rescan writes nothing`, `AC5: a hydrate's stat-diff writes only what it re-read`. Use a fake `Store` (the generated `mocks.IndexStore` or a small in-test recorder) for the counting assertions and a real bolt store for the blocking and failure cases.

    Test helpers may live in `store_test.go`: a `warnSink` capturing `warnf` output with a `naming(substring) []string` accessor, a `vlogSink`, `newStore` / `newStoreAt`, `bucketNames(path) []string` (open with `bolt.Open` read-only and list buckets), `writeLiteralStore`, `closeStore`, and `cacheDirEnv` (set both `XDG_CACHE_HOME` and `HOME` to a temp dir so the real user cache is never touched).

11. **Add the seventh contract section.** Append to `docs/page-index.md`:

    ```
    ## Store format version 2
    ```

    Content, from Desired Behavior 6 and 7: the store is an optional cache, never a source of truth; one record per indexed file holding filename, has-page flag, full frontmatter, file metadata, Open Questions and fingerprint and **no body** (an excluded file stores its fingerprint and no page); the frozen names (meta bucket `vault-ui-page-index-meta`, identity key `identity`, identity JSON fields `StoreFormat` / `ParserVersion`, entry bucket `vault-ui-page-index\x00<VaultPath>\x00<PagesDir>`) so a v1 file is recognised and discarded with its entry buckets deleted; the identity values (format `2`; parser version from build info — dependency entry, main module when vault-cli is main, else `(unknown)`); the four discard reasons with their exact wording, format checked before parser version, the 5 s lock wait and the rule that an open or reconcile failure yields a discarded store that serves nothing and never fails construction; writes of exactly the changed and vanished entries in one transaction outside the index mutex, a failed write leaving the previous content intact with one warning; the location (`<user cache dir>/<app>/page-index.bolt`, directory created `0700`, never inside a vault); and the codec rule (`RawMap()` in, `domain.NewPage` out, no preallocation from stored counts, any decode failure discards the store).

    Also add the one store sentence to the `## Staleness bounds` section: with a store, a key is hydrated from it and resolves exactly one stat-diff before it is served, and the stored records are never served as written.

12. **Dependencies.** `github.com/bborbe/boltkv`, `github.com/bborbe/kv` and `go.etcd.io/bbolt` are already direct requirements in `go.mod`; confirm with `grep -E 'boltkv|bborbe/kv|bbolt' go.mod`. If any is missing, write the importing code first, then run `ROOTDIR=/workspace make ensure` (which runs `go mod tidy`) — never `go mod vendor`, never `-mod=vendor`.

13. **Self-check before finishing**: re-run the `<verification>` block, confirm it passes, and walk AC9 (`Store hydration`), AC10, AC11, AC12 and AC14 (all seven headings) against the result.
</requirements>

<constraints>
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage` (AC16). This prompt adds `pkg/index` files, tests, one doc section and the store wiring only.
- Do NOT cache a body anywhere — not in memory, not in the store, not as an opt-in (spec Non-goal). The persisted record has no `Content` field at any depth.
- Do NOT shrink, compact or migrate the bolt file: a mismatching store is discarded, its entry buckets deleted, and never converted (spec Non-goal).
- Do NOT add configuration for the store location, rescan interval, timeouts or change-log size; the ported constants are invariant (spec Non-goal).
- Do NOT project or intern frontmatter keys — the full map is persisted.
- Do NOT change the ported refresh semantics from prompt 2: hydration adds one stat-diff before serving; it does not alter stat-diff, mark, revision or change-log behaviour.
- Do NOT add new module dependencies beyond `github.com/bborbe/boltkv`, `go.etcd.io/bbolt` and `github.com/bborbe/kv`.
- `pkg/index` must import neither `github.com/golang/glog` nor any `github.com/prometheus/*` package directly (AC13); transitive imports through `boltkv` / `kv` are allowed.
- The store lives under the user cache directory, never inside a vault or the repository tree, with directory mode `0700` (Security).
- Every new `.go` file needs the repository license header (`// Copyright (c) 2026 Benjamin Borbe All rights reserved.` + the BSD two-line notice).
- Wrap errors with `github.com/bborbe/errors`; never `fmt.Errorf`, never `github.com/pkg/errors`, never `context.Background()` inside the package for wrapping.
- Respect the linter (funlen 80/50, nestif 4, gocognit 20, dupl, maintidx 20); split `store.go` and `store_codec.go` further if a function grows past the limit.
- Use Ginkgo v2 / Gomega only; external package `index_test` except `export_test.go`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must exit 0.

Then confirm the named specs are visible and passing:

```
GOFLAGS=-buildvcs=false go test ./pkg/index/... -ginkgo.v
```

The output must contain the passing containers `Stat-diff rescan`, `Allocation`, `Per-file write marks`, `ChangedPagesSince`, `Cold read`, `Failed rebuild`, `Store hydration`, and the passing specs `persists no Content field in the store record`, `writes no body bytes to the store file`, `discards a v1 store, deletes its entry buckets and warns store format mismatch with the path`, `reports parser version mismatch as a distinct reason`, `serves without a store when the bolt file is locked by another process`, and `restores open questions from the store without re-reading the file`.

Confirm the full public surface (AC2):

```
go doc ./pkg/index PageIndex
go doc ./pkg/index Entry
go doc ./pkg/index | grep -E 'NewPageIndex|NewPageIndexWithStore|NewBoltStore|DefaultStorePath|CurrentWriterIdentity'
```

Confirm all seven doc headings (AC14):

```
grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Store format version 2|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md
```

This must print `7`.

Confirm import hygiene (AC13):

```
go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -E 'golang/glog|prometheus'
```

This must print zero lines.

Confirm the changelog bullet is still in an unreleased section (AC15):

```
awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md
```

This must print `## Unreleased` at least once.
</verification>
