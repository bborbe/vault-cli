# Page index

`pkg/index` keeps the metadata of each vault folder's pages in memory so a list
read can be answered without opening a vault file. It implements vault-cli's
`storage.PageStorage`, so it plugs in exactly where vault-cli reads a folder:
filtering, sorting and blocked-state logic stay in vault-cli's list operation
(`ops.NewListOperation(idx)` works unchanged). Single files are read through the
`PageReader` seam, which delegates to vault-cli's own
`storage.PageStorage.ReadPage`, so one parse path and one symlink-out-of-vault
guard serve both the index and vault-cli's own folder listing.

One immutable snapshot is held per key — a vault root plus a vault-relative
pages dir. A published snapshot is never mutated; concurrent readers share the
same `*Entry` and `*domain.Page` pointers. **Published entries and pages are
read-only for every consumer**: a consumer that needs to change one copies it
first, because mutating a shared page corrupts every other reader's view.

The package imports no glog and no prometheus. Warnings go to `log/slog` by
default (`SetWarnf` redirects them per index), and counters go through the
injected `Metrics` interface (`FileRead(reason)`,
`ChangedPagesUnbounded(reason)`); a nil `Metrics` records nothing.

## Metadata-only records

- Every indexed page keeps its full frontmatter map (every key), its name, its
  file path and its modification date exactly as vault-cli's folder listing
  produces them — and an **empty** `Content`.
- The body is read by the `PageReader` and dropped before the page is
  published. No body is held in memory and none is written to the store.
- `ListPages` over a folder equals `storage.NewPageStorage(nil).ListPages` over
  the same folder with only `Content` zeroed: same set, same order, same
  frontmatter, path and modification time — including files without
  frontmatter, invalid YAML, non-`.md` files, subdirectories, symlinks out of
  the vault, broken symlinks, in-vault symlinks and non-ASCII names.

## Entries and open questions

- `ListEntries` serves the same snapshot as `ListPages` — same order, same page
  pointers — where each `Entry` adds the page's `OpenQuestions` as
  `[]domain.OpenQuestion{Index, Text}`, projected from
  `storage.ParseOpenQuestions` (`Text` is the item's question, without any
  recorded answer).
- `OpenQuestions` is never nil: a page without an `Open Questions` section has
  an empty slice.
- Every retained `Text` is a copy (`strings.Clone`) that shares no memory with
  the body, so a retained question never pins a body in memory.
- An open-question parse error fails that read like any read error.

## Lazy body reads

- `ReadPage(ctx, vaultPath, pagesDir, name)` takes the bare base name (without
  `.md`) and always reads the file from disk through the `PageReader`,
  returning the full page including `Content`.
- It never caches the body, never mutates or republishes the snapshot and never
  moves the key's revision. A name that is empty, contains a path separator or
  `..` is rejected without a read.

## Staleness bounds

- A warm read never touches disk. `ListPages` / `ListEntries` return the
  published snapshot without any filesystem access.
- `Rescan` refreshes every known key once per `RescanInterval` (50 s) plus the
  1 s poll granularity plus one stat-diff — under 60 s. Each storage call is
  bounded by the rebuild timeout (2 min), so the worst case is the rescan
  interval plus that timeout. The bound holds for every change to a file's
  presence, size, modification time or status-change time.
- The first read after a mark resolves it before serving: a per-file mark
  re-reads exactly the marked files, a folder-level mark runs one stat-diff, and
  every concurrent reader waits on that same work. Marking a key the index has
  never seen is a no-op: its first read builds it anyway.
- Concurrent cold reads of one key share a single build.
- An event-driven (`RefreshFile`) or rescan-driven (`Refresh`) read never
  blocks `ListPages`: a reader is served from the current snapshot.
- `ForceReload` is the only path that ignores fingerprints: the next read of
  every key re-reads every file.
- A failed listing keeps serving the previous snapshot and logs the error with
  the key; the mark stays pending, so the next read retries. A failed first
  build returns an error.

## Incremental updates

- `RefreshFile(ctx, key, filename)` re-reads only that file and lists nothing.
  The file's current on-disk state decides the outcome: present and readable
  replaces or inserts its entry at its filename-ordered position, absent or
  unreadable removes it. A filename that is empty, contains a path separator or
  `..`, or does not end in `.md` is rejected and nothing is read.
- Every file read records a fingerprint — size, modification time and
  status-change time — from a stat that follows symlinks, including files that
  end up excluded.
- `Refresh` and the rescan are a **stat-diff**: list the folder, compare each
  entry's fingerprint with the one recorded at its last read (inequality, not
  "newer"), and re-read only what differs or is new. A body-only edit changes
  the fingerprint, so it is re-read and republished with a new page pointer and
  named by `ChangedPagesSince`.
- An unchanged stat-diff reads nothing, publishes nothing, keeps the snapshot
  and the recorded fingerprint set in place, and allocates nothing beyond the
  one folder listing it performs. An excluded file is warned about once per
  fingerprint change.
- `MarkDirty(keys...)` marks a folder stale; the next read resolves it by one
  stat-diff. `MarkFileDirty(key, id)` marks one file stale only when the id is
  exact (`[[`/`]]` stripped, no separator or `..`, a byte-equal page name in the
  snapshot, `<name>.md` present); anything else widens to a folder-level mark.
- `Revision(key)` advances on every publication and mark. `ChangedPagesSince`
  returns the `.md` filenames changed after a revision, with `ok == false` for
  an unknown key, a revision ahead of the key, a revision outside the retained
  change log (64 advances), a folder-level mark, a forced reload, or the key's
  first publication.

## Key derivation

A key is `NewKey(vault path, folder)`, which applies `filepath.Clean` to both
parts so equivalent paths produce identical keys. A consumer derives the read
side and the event side from the same vault config entry; a mismatch silently
disables event refresh and leaves the folder on its rescan cadence.
