---
spec: ["068-pkg-index-metadata-page-index"]
status: draft
created: "2026-11-18T23:16:38Z"
---

# Add the metadata-only bolt store and store hydration to the page index

<summary>
- A restarted process can hydrate the page index from an optional on-disk cache instead of re-parsing unchanged files.
- The cache holds filenames, frontmatter, file metadata, Open Questions and fingerprints, and never a page body.
- A cache written under another store format or another parser version is recognised and discarded rather than read.
- A discarded cache deletes its entry buckets and warns once with the file path and a distinct reason.
- A cache that is locked by another process is given up on quickly, and the index still serves from a full parse.
- Every publication writes exactly the changed and vanished entries in one transaction, outside the index lock.
- A hydrated key always resolves one stat-diff before it is served, so stored records are never served as written.
- The package documentation and changelog state the store contract.
</summary>

<objective>
Add an optional metadata-only bolt cache and its hydration path to the page index, so a restart skips re-parsing unchanged files without ever persisting or serving a page body.
</objective>

<context>
Read `CLAUDE.md` for project conventions and `docs/dod.md` for the definition of done this change is graded against.

Read these files before writing anything:

- `pkg/index/index.go` — the `pageIndex` struct, the build kinds, `pendingLocked`, `execute`, `executeListing`, `publishListingLocked`, `release` and `writeThrough` hooks. This prompt adds the hydrate build kind and the store-backed constructor.
- `pkg/index/index_build.go` — `collect`, `mergeSnapshot`, `listingDelta` and `diffListingLocked`; the store delta is computed from the same publication.
- `pkg/index/index_file.go` — `applyFileReadLocked` returns the per-file delta a `RefreshFile` publication produces.
- `pkg/index/seams.go` — `Entry`, `FileFingerprint`, `PageReader`.
- `pkg/index/export_test.go` — `CountingMetrics`, `TestMetrics`, `NewPageIndexWithWarnf`.
- `pkg/domain/page.go` — `Page` embeds `FrontmatterMap`, `FileMetadata` and `Content`; `FrontmatterMap.RawMap()` and `domain.NewPage` are the only safe codec entry points. A JSON codec on `domain.Page` silently drops every frontmatter key because the embedded map's field is unexported.
- `pkg/domain/file_metadata.go`, `pkg/domain/open_question.go`.
- `pkg/index/` — this package already exists in the working tree from an earlier pass, and `pkg/index` is named in the released `## v0.197.0` CHANGELOG section. Read every existing file before editing. Keep code and tests that already satisfy the contract below; add or correct what does not. Do not delete passing tests and do not recreate a file that already matches.
- `docs/page-index.md` — the six sections written by the previous prompt. This prompt appends the seventh.

Library APIs (verified against the module cache; use these exact forms):

- `github.com/bborbe/boltkv` v1.17.0: `func OpenFile(ctx context.Context, path string, fn ...ChangeOptions) (DB, error)` where `type ChangeOptions func(opts *bolt.Options)`; `boltkv.DB` embeds `github.com/bborbe/kv`'s `DB` and exposes `DB() *bolt.DB`, `View`, `Update`, `Close`.
- `github.com/bborbe/kv` v1.21.30: `type Tx interface { Bucket; CreateBucket; CreateBucketIfNotExists; DeleteBucket; ListBucketNames }`, `type Bucket interface { Put; Get; Delete; Iterator; IteratorReverse }`, `type Item interface { Exists; Key; Value(fn func(val []byte) error) error }`, `type BucketName []byte`, `func NewBucketName(name string) BucketName`, `func ForEach(ctx, bucket, fn) error`, and `var ErrBucketNotFound` (with the deprecated alias `BucketNotFoundError`).
- `go.etcd.io/bbolt` v1.5.0: `bolt.Open(path string, mode os.FileMode, options *bolt.Options) (*bolt.DB, error)`, `bolt.Options{Timeout time.Duration}`. The zero `Timeout` waits forever, so it must always be set.

Coding conventions to follow:

- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `errors.Wrap` / `errors.Wrapf` from `github.com/bborbe/errors`; `errors.Is` for sentinels.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-security-linting.md` — directory mode `0700`, file mode `0600`; no `#nosec` without a reason.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen 80, nestif 4, golines 100, license headers.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` rules.
</context>

<requirements>
1. Create `pkg/index/store.go` with these frozen constants:

   ```go
   const StoreFormatVersion = 2
   const storeOpenTimeout = 5 * time.Second
   const storeMetaBucketName = "vault-ui-page-index-meta"
   const storeBucketPrefix = "vault-ui-page-index\x00"
   const storeIdentityKey = "identity"
   const vaultCLIModulePath = "github.com/bborbe/vault-cli"
   const unknownParserVersion = "(unknown)"
   ```

   Document: version 2 records carry no body and a store under another version is discarded, never converted; the meta bucket name has no NUL prefix so it can never collide with an entry bucket; the NUL separators cannot appear in a path so two keys cannot produce one bucket; the names are byte-identical to vault-ui's v1 store so that file is recognised and discarded.

2. Add the public store surface:

   ```go
   type WriterIdentity struct {
       StoreFormat   int
       ParserVersion string
   }

   type Store interface {
       Load(ctx context.Context, key Key) (entries []StoredEntry, ok bool, err error)
       Write(ctx context.Context, key Key, puts []StoredEntry, deletes []string) error
       Path() string
       Close() error
   }

   func DefaultStorePath(app string) (string, error)
   func CurrentWriterIdentity(ctx context.Context) WriterIdentity
   func NewBoltStore(
       ctx context.Context,
       path string,
       identity WriterIdentity,
       warnf func(format string, args ...any),
       vlogf func(format string, args ...any),
   ) Store
   ```

   - `StoredEntry` is defined in `pkg/index/index.go` or `store.go` as `{Filename string; Entry *Entry; Fingerprint FileFingerprint}`. `Entry` is nil for a file that was excluded but whose fingerprint is recorded.
   - `Store.Load` reports `ok == false` when there is nothing usable for the key; `err` is non-nil only for context cancellation. Document that a store is a cache, never a source of truth.
   - `Store.Write` applies one publication's delta in a single transaction: `puts` replaces or inserts by `Filename`, `deletes` removes.
   - `DefaultStorePath` is a pure path helper returning `<user cache dir>/<app>/page-index.bolt` via `os.UserCacheDir()`; it creates nothing and returns the stdlib error unwrapped (it has no context parameter by contract).
   - `CurrentWriterIdentity` returns `WriterIdentity{StoreFormat: StoreFormatVersion, ParserVersion: vaultCLIParserVersion()}`.
   - `vaultCLIParserVersion()` uses `debug.ReadBuildInfo()`: return the `Deps` entry whose `Path` is `github.com/bborbe/vault-cli`; otherwise the `Main` module's version when `Main.Path` is the same; otherwise `unknownParserVersion`.
   - `NewBoltStore` never returns nil and never fails. On any open or reconcile failure it returns a `discardedStore` that serves nothing and warns.
   - Add the counterfeiter directive `//counterfeiter:generate -o ../../mocks/index-store.go --fake-name IndexStore . Store`.

3. Implement `NewBoltStore`:
   - `os.MkdirAll(filepath.Dir(path), 0700)`; on error return a discarded store with reason `"create store directory"`.
   - `boltkv.OpenFile(ctx, path, func(opts *bolt.Options) { opts.Timeout = storeOpenTimeout })`; on error return a discarded store with reason `"open store"`. The 5-second timeout is the contract: a second process on the same cache path must never hang startup.
   - Build a `boltStore{db, path, identity, warnf, vlogf, empty atomic.Bool}`, run `reconcile`; on error close the DB and return a discarded store with reason `"record writer identity"`.
   - Count keys and entries once and store `empty = (keys == 0)`; call `vlogf("page index store %s: %d keys, %d entries", path, keys, entries)`.

4. Implement `reconcile` in one `db.Update` transaction:
   - create or open the meta bucket with `tx.CreateBucketIfNotExists(ctx, kv.NewBucketName(storeMetaBucketName))`;
   - read `identity` with `meta.Get` and copy the value out inside `item.Value`;
   - compute the discard reason with `discardReason(raw)`; an empty reason means keep the store;
   - when discarding: call `warnf("page index store discarded (%s): %s", reason, path)`, delete every bucket except the meta bucket, and write this process's identity JSON into the meta bucket;
   - `discardReason` checks in this order and returns: `"store file missing or empty"` when the raw value is empty; `"damaged"` when `json.Unmarshal` into `WriterIdentity` fails; `fmt.Sprintf("store format mismatch (stored %d, current %d)", existing.StoreFormat, s.identity.StoreFormat)` when the format differs; `"parser version mismatch"` when only the parser version differs; otherwise `""`. Format is checked before parser version so a file differing in both reports a format mismatch.

5. Implement `Load`:
   - return `(nil, false, wrapped ctx error)` when `ctx.Err()` is set;
   - return `(nil, false, nil)` immediately when the store is empty;
   - in one `db.View` transaction get the key's bucket and iterate it with `kv.ForEach`; decode every value with `decodeRecord`; **append** decoded entries to a nil slice — never preallocate from a stored count or length, so a crafted value cannot drive an unbounded allocation;
   - a decode or transaction error warns `"page index store discarded (damaged): %s"` and returns `(nil, false, nil)`; a `kv.ErrBucketNotFound` (accept the deprecated alias too) warns `"page index store discarded (no stored entries for key): %s"` and returns `(nil, false, nil)`; a context cancellation returns the wrapped error;
   - return the entries with `ok == true`.

6. Implement `Write`:
   - one `db.Update` transaction: write this process's identity into the meta bucket, `CreateBucketIfNotExists` for the key's bucket, then put each entry encoded with `encodeRecord` under `[]byte(entry.Filename)` and delete each name in `deletes`;
   - wrap failures with `errors.Wrap(ctx, err, "write page index store")`; a failed write leaves the previous content intact and the caller owns the warning;
   - set `empty` to false after a successful write.
   - `storeBucketName(key)` returns `kv.NewBucketName(storeBucketPrefix + key.VaultPath + "\x00" + key.PagesDir)`.

7. Implement the `discardedStore`:
   - `Load` warns once with `warnf("page index store discarded (%s): %s", reason, path)` and returns `(nil, false, nil)`;
   - `Write` returns `errors.Errorf(ctx, "page index store discarded (%s): %s", reason, path)`;
   - `Path` returns the intended path; `Close` is a no-op.

8. Create `pkg/index/store_codec.go`:
   - `type storeRecord struct { Filename string; HasPage bool; Frontmatter map[string]any; Metadata domain.FileMetadata; OpenQuestions []domain.OpenQuestion; Fingerprint FileFingerprint }` — no `Content` field at any depth, and a field named `OpenQuestions`.
   - Use `encoding/gob`, not JSON. Register every concrete type a YAML frontmatter map can hold in `init()`: `""`, `false`, `int(0)`, `int64(0)`, `float64(0)`, `time.Time{}`, `[]any{}`, `map[string]any{}`, `map[any]any{}`. Document that `libtime.DateOrDateTime` is deliberately not registered because it implements `BinaryMarshaler` but not `BinaryUnmarshaler`; `encodeValue` converts it to `time.Time` instead.
   - `encodeRecord(ctx, StoredEntry) ([]byte, error)`: encode `entry.Entry.Page.RawMap()` through `encodeValue` and require the result to be `map[string]any`; set `HasPage`, `Frontmatter`, `Metadata` and `OpenQuestions` from the entry; gob-encode the record.
   - `decodeRecord(ctx, []byte) (StoredEntry, error)`: gob-decode; rebuild the page with `domain.NewPage(record.Frontmatter, record.Metadata, "")` when `HasPage`; normalise a nil `OpenQuestions` to an empty non-nil slice.
   - `encodeValue` rewrites `libtime.DateOrDateTime` and `*libtime.DateOrDateTime` to `time.Time`, recurses into `map[string]any`, `map[any]any` and `[]any`, and returns every other value unchanged. `encodeMap` is the generic map helper.

9. Wire the store into the engine in `pkg/index/index.go` and `pkg/index/index_build.go`:
   - Add `store Store` to `pageIndex` and a `storeLoaded bool` and `storedBaseline bool` to `entry`.
   - Add the frozen constructor:

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

     `NewPageIndex` delegates to it with a nil store. A nil store disables persistence; a nil metrics becomes `NewNoopMetrics()`.
   - Add a `buildHydrate` build kind. In `pendingLocked`, a key with no snapshot consults the store first when `p.store != nil && !e.storeLoaded`, then falls back to a cold build.
   - `executeHydrate`: call `p.store.Load(ctx, key)` outside the mutex; on error treat the store as absent (`ok = false`); under the mutex seed the baseline from the stored entries when `ok`, set `storeLoaded = true`, then run the ordinary listing path. `seedStoredBaseline` fills the snapshot slice (stored order is byte-ascending filenames, which equals `os.ReadDir` order) and the fingerprint map, leaves `hasSnapshot == false` and sets `storedBaseline = true`.
   - The stored baseline is an input to the stat-diff, never a served value. A failed listing while `storedBaseline` is set clears the snapshot, the fingerprints and `hasSnapshot` so a stored snapshot can never be served without its stat-diff.
   - `execute` writes the publication's delta through **after** `release`, outside the index mutex, so no reader waits on store I/O. `writeThrough(ctx, key, delta)` is a no-op for a nil store, a nil delta, or an empty delta; a failed write warns once with `warnf("page index store write failed for %q/%q: %v", key.VaultPath, key.PagesDir, err)` and does not affect what is served.
   - Publication deltas: a listing puts every entry it re-read (including an excluded entry, as `StoredEntry{Filename, Entry: nil, Fingerprint}`) and deletes every name the merge dropped; an unchanged pass produces no delta; a per-file read produces one put or one delete and merges into the build's delta. A hydrate's stat-diff writes only what it re-read.

10. Extend `pkg/index/export_test.go` with `NewPageIndexWithStoreAndWarnf(reader PageReader, lister DirectoryLister, currentDateTimeGetter libtime.CurrentDateTimeGetter, waiter libtime.WaiterDuration, store Store, warnf func(format string, args ...any)) PageIndex` and `PersistedRecordType() reflect.Type` returning `reflect.TypeOf(storeRecord{})`.

11. Add the module requirements. `github.com/bborbe/boltkv` v1.17.0, `github.com/bborbe/kv` v1.21.30 and `go.etcd.io/bbolt` v1.5.0 must be direct requirements in `go.mod`. Write the importing code first, then run `go mod tidy`; never run `go mod vendor`, and do not add a `replace` or `exclude` directive.

12. Write the store tests in `package index_test`, over real temp directories and real bolt files:

    - `Describe("Page index store")` containing:
      - `Describe("codec")` with `It("AC1 round-trips every awkward file losslessly")` (the awkward-files fixture: every frontmatter key, nested maps, lists, dates survive encode/decode and re-encode to a page equal to the reader's entry except `Content`) and `It("converts a DateOrDateTime frontmatter value to time.Time")`.
      - `Describe("discard on mismatch")` covering: a store file that never existed is absent; a zero-byte file opens empty without panicking; garbage bytes are discarded as damaged; a directory that cannot be created yields a discarded store; a read-permission failure is discarded; a different store format is discarded; a different parser version is discarded; one key serves while another full-parses.
      - `Describe("write")` covering: puts and deletes in one transaction; a failed write leaves the previous content intact; a written delta is observable through a second store opened on the same file.
      - `Describe("location")` covering: `DefaultStorePath` resolves `<user cache dir>/<app>/page-index.bolt`; the default path opens; a missing store directory is created with mode 0700; the store is never written under a vault directory.
      - `Describe("reporting")` with `It("AC7 warns nothing on a hit and reports keys and entries at V(2)")`.
      - `Describe("bounded open")` with `It("AC-safety times out instead of hanging on a locked store")` (`NodeTimeout(30*time.Second)`).

    - `Describe("Store format version 2")` with these exact specs:
      - `It("persists no Content field in the store record")` — reflection walk over `index.PersistedRecordType()` (recurse into struct fields, including embedded and pointer-to-struct fields) asserts no field named `Content` at any depth and a field named `OpenQuestions`.
      - `It("writes no body bytes to the store file")` — build a fixture whose body contains the sentinel `BODY-SENTINEL-7f3a` outside frontmatter and outside any Open Questions item; cold-build with a store; close it; read the bolt file's raw bytes and assert the sentinel does not appear.
      - `It("discards a v1 store, deletes its entry buckets and warns store format mismatch with the path")` — write the bolt file with **literal** bytes (not package constants): meta bucket `vault-ui-page-index-meta`, key `identity`, value `{"StoreFormat":1,"ParserVersion":"v0.171.0"}`, and one entry bucket `vault-ui-page-index\x00<vault>\x00<dir>`. Open with `NewBoltStore`; assert the entry bucket is gone (only the meta bucket remains) and exactly one warning contains `store format mismatch`, `stored 1, current 2`, and the path.
      - `It("reports parser version mismatch as a distinct reason")` — literal identity `{"StoreFormat":2,"ParserVersion":"v0.0.0-stale"}`; assert a warning contains `parser version mismatch` and the path, and no warning contains `store format mismatch`.
      - `It("reports a damaged identity as a distinct reason")` — literal `{not json`; assert a warning contains `damaged`.
      - `It("serves without a store when the bolt file is locked by another process")` — hold the file open with a second `bolt.Open` handle, construct the store, assert construction returns in under 10 seconds, a discard warning was emitted, and `ListPages` serves the fixture. Use `NodeTimeout(30*time.Second)` and `SpecContext`.
      - `It("resolves the store path under the user cache directory per app")` and `It("creates the store directory with mode 0700")`.

    - `Describe("Store hydration")` (the exact container name — the verifier greps for it) with:
      - `It("restores open questions from the store without re-reading the file")` — index A over a fixture with an Open Questions section publishes to the store and closes it; index B opens the same store over unchanged files with a fresh `index.NewCountingMetrics()`; B's `ListEntries` `OpenQuestions` equal A's and `metricsB.TotalReads() == 0`.
      - `It("AC1: the store round-trip is lossless")`, `It("AC2: a stored snapshot is never served before its stat-diff")`, `It("AC2 control: a key with no stored entries blocks on the full parse")`, `It("AC3(a): an unchanged folder lists once and reads nothing")`, `It("AC3(b): re-reads exactly the K changed files")`, `It("AC3(c): re-reads only the added file and drops the removed one")`, `It("AC3(d): detects a same-size rewrite with a restored modification time")`, `It("AC4: falls back to a full parse when the store supplies nothing")`, `It("AC4: serves correctly when the store read fails")`, `It("AC2/failure: a failed stat-diff of a store-loaded key publishes nothing")`.

    - `Describe("Store write-through")` with: a cold build writes N entries in one transaction; a per-file update writes exactly one put; a deletion writes exactly one delete; a stat-diff that drops a removed file writes exactly one delete; a failed write leaves serving and the store intact; a blocking write does not block a reader; an unchanged rescan writes nothing; a hydrate's stat-diff writes only what it re-read.

    Test helpers you may add in `store_test.go`: a `warnSink` and `vlogSink` capturing formatted lines, `newStoreAt(path, identity, warns, vlogs)`, `newStore(path, warns, vlogs)` using `index.CurrentWriterIdentity`, `closeStore`, and a `writeLiteralStore` that opens the file with `bolt.Open` and writes the literal meta/entry bytes.

13. Append `## Store format version 2` to `docs/page-index.md`, after `## Key derivation`. State: one record per indexed file (filename, has-page flag, full frontmatter, file metadata, Open Questions, fingerprint, no body); the frozen bucket and identity names; `StoreFormat` is 2 and `ParserVersion` comes from build info; the four discard reasons and the format-before-parser order; the 5-second open timeout and that any open/reconcile failure yields a discarded store that never fails construction; each publication writes exactly its changed and vanished entries in one transaction outside the index lock, and a failed write leaves the previous content intact; `DefaultStorePath` and the 0700 directory; the codec rule (raw frontmatter map in, `domain.NewPage` out, no preallocation from stored counts, any decode failure discards the store).

14. Add a bullet to the existing `## Unreleased` section of `CHANGELOG.md` naming `pkg/index` and the optional bolt cache: store format version 2, metadata-only records, hydration then one stat-diff, distinct discard reasons, and write-through of changed entries. Append to the section; do not replace it.

15. Before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the change.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Do NOT change `pkg/cli`, `pkg/config`, `pkg/domain`, `pkg/ops` or `pkg/storage`
- Do not cache page bodies anywhere, in memory or in the store, not even an opt-in body cache
- Do not shrink, compact or migrate the bolt file — a mismatching store is discarded, never converted
- Do not add configuration for the store location, rescan interval, timeouts or change-log size
- The store lives under the user cache directory, never inside a vault or the repository tree; create its directory with mode 0700
- Treat the store as untrusted input: never preallocate from stored counts or lengths, and discard on any decode failure rather than serving partial content
- Nothing may retry forever: store open waits at most 5 s, storage calls are bounded by 2 min, rescan runs on a fixed 50 s interval
- `pkg/index` must import neither `github.com/golang/glog` nor any prometheus package (direct imports). Transitive imports through `github.com/bborbe/kv` / `boltkv` are unavoidable and allowed.
- New module dependencies are limited to `github.com/bborbe/boltkv`, `go.etcd.io/bbolt` and `github.com/bborbe/kv`; do not add others
- Builds for darwin and linux with `CGO_ENABLED=0`
- Every new file carries the project's BSD license header, matching `pkg/storage/page.go`
- Do not run `go mod vendor`; `vendor/` is a build-time artifact
- Existing tests must still pass
</constraints>

<verification>
make precommit

go test ./pkg/index/... -ginkgo.v

go doc ./pkg/index PageIndex
go doc ./pkg/index Entry
go doc ./pkg/index

grep -cE '^## (Metadata-only records|Entries and open questions|Lazy body reads|Store format version 2|Staleness bounds|Incremental updates|Key derivation)$' docs/page-index.md

awk '/^## /{sec=$0} /pkg\/index/{print sec}' CHANGELOG.md | grep -q '^## Unreleased$'

! go list -f '{{join .Imports "\n"}}' ./pkg/index | grep -qE 'golang/glog|prometheus'
</verification>
