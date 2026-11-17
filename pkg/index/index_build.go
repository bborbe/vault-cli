// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"context"
	"log/slog"
	"sort"

	"github.com/bborbe/errors"
)

// role describes what the caller that acquired a build has to do with it.
type role int

const (
	// roleRun means the caller owns the build and it is already in flight.
	roleRun role = iota
	// roleFollowUp means the caller owns a queued build and must wait until it
	// is promoted before running it.
	roleFollowUp
	// roleWait means the caller only waits for somebody else's build.
	roleWait
)

// ensure returns the key's published snapshot, resolving whatever work is pending for it.
//
// It returns the build error separately from a cancellation of the caller's own
// ctx: a failed build is not an error for a caller that can still serve a
// previous snapshot, while a cancelled caller must not be handed stale pages.
//
// With refresh set, the caller wants a stat-diff that started after this call
// and gets one; without it the caller blocks only while a mark is pending and
// is served from the current snapshot otherwise.
func (p *pageIndex) ensure(
	ctx context.Context,
	key Key,
	refresh bool,
) (published, error, error) {
	reason := ReadReasonWrite
	if refresh {
		reason = ReadReasonRescan
	}
	p.mu.Lock()
	e := p.entryLocked(key)
	if refresh {
		e.requestSeq++
	}
	// Only work marked before this call is resolved here: a mark recorded while
	// a read is in flight stays pending for the next reader.
	entryNeedSeq := e.requestSeq
	p.mu.Unlock()

	var last published
	for first := true; ; first = false {
		p.mu.Lock()
		e = p.entryLocked(key)
		kind, needSeq, pending := p.pendingLocked(e, refresh)
		if !pending || (!first && needSeq > entryNeedSeq) {
			snapshot := e.snapshot
			p.mu.Unlock()
			if first {
				return snapshot, nil, nil
			}
			return last, nil, nil
		}
		b, callerRole := p.acquireLocked(e, kind, reason, needSeq)
		p.mu.Unlock()

		snap, buildErr, cancelErr := p.await(ctx, key, b, callerRole)
		if cancelErr != nil {
			return published{}, nil, cancelErr
		}
		last = snap
		if buildErr != nil {
			// A failed listing leaves the mark pending for the next reader.
			return snap, buildErr, nil
		}
		if refresh {
			if b.covers(buildStatDiff, needSeq) {
				return snap, nil, nil
			}
			// The build that was in flight re-reads files without listing the
			// folder, so it cannot satisfy a refresh: wait for one that does.
			continue
		}
	}
}

// pendingLocked reports the work the key needs next: a cold build when it has
// no snapshot, otherwise the highest-priority pending mark. The caller must
// hold the mutex.
func (p *pageIndex) pendingLocked(e *entry, refresh bool) (buildKind, uint64, bool) {
	if !e.hasSnapshot {
		return buildCold, e.requestSeq, true
	}
	if refresh {
		return buildStatDiff, e.requestSeq, true
	}
	if e.reloadSeq > e.reloadResolvedSeq {
		return buildReload, e.reloadSeq, true
	}
	if len(e.writeMarked) > 0 {
		return buildFileReads, maxMarkedSeq(e.writeMarked), true
	}
	if e.fileReadsInflightLocked() {
		// A per-file re-read is already running: join it rather than serving a
		// snapshot the mark was recorded against.
		return buildFileReads, 0, true
	}
	if e.dirtySeq > e.dirtyResolvedSeq {
		return buildStatDiff, e.dirtySeq, true
	}
	return 0, 0, false
}

// fileReadsInflightLocked reports whether a per-file re-read is running or
// queued for the key. The caller must hold the mutex.
func (e *entry) fileReadsInflightLocked() bool {
	return (e.inflight != nil && e.inflight.kind == buildFileReads) ||
		(e.followUp != nil && e.followUp.kind == buildFileReads)
}

// maxMarkedSeq returns the highest requestSeq in a write-mark map.
func maxMarkedSeq(marked map[string]uint64) uint64 {
	var highest uint64
	for _, seq := range marked {
		if seq > highest {
			highest = seq
		}
	}
	return highest
}

// acquireLocked picks the build the caller waits on and starts one when none
// covers its need. The caller must hold the mutex.
func (p *pageIndex) acquireLocked(
	e *entry,
	kind buildKind,
	reason string,
	needSeq uint64,
) (*build, role) {
	if e.inflight != nil && e.inflight.covers(kind, needSeq) {
		return e.inflight, roleWait
	}
	if e.followUp != nil {
		return e.followUp, roleWait
	}
	if e.inflight != nil {
		return p.queueFollowUpLocked(e, kind, reason), roleFollowUp
	}
	b := newBuild(kind, reason, e.requestSeq)
	e.inflight = b
	return b, roleRun
}

// queueFollowUpLocked appends the one queued build a key may have. The caller
// must hold the mutex.
func (p *pageIndex) queueFollowUpLocked(e *entry, kind buildKind, reason string) *build {
	b := newBuild(kind, reason, e.requestSeq)
	e.followUp = b
	return b
}

// await waits for or runs the build the caller acquired.
func (p *pageIndex) await(
	ctx context.Context,
	key Key,
	b *build,
	callerRole role,
) (published, error, error) {
	if callerRole == roleWait {
		select {
		case <-b.done:
		case <-ctx.Done():
			return published{}, nil, errors.Wrap(ctx, ctx.Err(), "wait for page index build")
		}
		return b.snap, b.err, nil
	}
	if callerRole == roleFollowUp {
		// A follow-up must never be abandoned: it is already the key's queued
		// build, and nobody else would run it.
		<-b.started
	}
	p.execute(ctx, key, b)
	if err := ctx.Err(); err != nil {
		return published{}, b.err, errors.Wrap(ctx, err, "page index build")
	}
	return b.snap, b.err, nil
}

// execute runs the build's read and publishes its result. It runs on the
// goroutine of the caller that owns the build and finishes even when that
// caller's ctx is cancelled.
func (p *pageIndex) execute(ctx context.Context, key Key, b *build) {
	readCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rebuildTimeout)
	defer cancel()

	switch b.kind {
	case buildFileReads:
		p.executeFileReads(readCtx, key, b)
	default:
		p.executeListing(readCtx, key, b)
	}
	p.release(key, b)
}

// executeFileReads re-reads exactly the write-marked files. The marks are taken
// out under the lock as the read starts, so a mark recorded during the read
// stays pending for the next one.
func (p *pageIndex) executeFileReads(ctx context.Context, key Key, b *build) {
	p.mu.Lock()
	e := p.entryLocked(key)
	startSeq := e.requestSeq
	b.startSeq = startSeq
	names := make([]string, 0, len(e.writeMarked))
	for name, seq := range e.writeMarked {
		if seq <= startSeq {
			names = append(names, name)
			delete(e.writeMarked, name)
		}
	}
	p.mu.Unlock()

	var delta *storeDelta
	for _, name := range names {
		seq := p.takeReadSeq()
		read, fingerprint, readErr := p.reader.ReadEntry(
			ctx,
			key.VaultPath,
			key.PagesDir,
			name,
		)
		p.metrics.FileRead(ReadReasonWrite)

		p.mu.Lock()
		d := p.applyFileReadLocked(e, key, name, seq, read, fingerprint, readErr)
		p.mu.Unlock()
		delta = mergeStoreDeltas(delta, d)
	}

	p.mu.Lock()
	b.delta = delta
	p.mu.Unlock()
}

// mergeStoreDeltas returns a delta holding both inputs' puts and deletes; nil
// inputs contribute nothing and the result is nil when both are nil.
func mergeStoreDeltas(a, b *storeDelta) *storeDelta {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := &storeDelta{
		puts:    make([]StoredEntry, 0, len(a.puts)+len(b.puts)),
		deletes: make([]string, 0, len(a.deletes)+len(b.deletes)),
	}
	merged.puts = append(merged.puts, a.puts...)
	merged.puts = append(merged.puts, b.puts...)
	merged.deletes = append(merged.deletes, a.deletes...)
	merged.deletes = append(merged.deletes, b.deletes...)
	return merged
}

// executeListing lists the key's folder and reads the files the build's kind
// asks for, then merges the result into the published snapshot.
//
// A pass whose listing matches the recorded fingerprints, whose name set
// matches the recorded name set and that has no file to re-read is an
// unchanged pass: it publishes nothing, leaves the published snapshot and the
// recorded fingerprints exactly in place, and allocates nothing beyond the one
// listing it performs. A pass with work to do re-reads exactly the changed,
// new and forced names and updates the recorded fingerprints for those names
// in place.
func (p *pageIndex) executeListing(ctx context.Context, key Key, b *build) {
	p.mu.Lock()
	e := p.entryLocked(key)
	startSeq := e.requestSeq
	b.startSeq = startSeq
	baseline := e.hasSnapshot
	full := b.kind == buildCold || b.kind == buildReload || !baseline
	forced := takeWriteMarksLocked(e, startSeq, full)
	p.mu.Unlock()

	reason := b.reason
	switch {
	case !baseline, b.kind == buildCold:
		reason = ReadReasonBuild
	case b.kind == buildReload:
		reason = ReadReasonReload
	}

	listingSeq := p.takeReadSeq()
	entries, listErr := p.lister.ListFiles(ctx, key.VaultPath, key.PagesDir)
	if listErr != nil {
		p.failListing(key, b, listErr)
		return
	}

	var toRead map[string]bool
	if !full {
		p.mu.Lock()
		e = p.entryLocked(key)
		var unchanged bool
		toRead, unchanged = diffListingLocked(e, entries, forced)
		if unchanged {
			b.snap = e.snapshot
			e.dirtyResolvedSeq = startSeq
			p.mu.Unlock()
			return
		}
		p.mu.Unlock()
	}

	reads, collectErr := p.collect(ctx, key, listingSeq, entries, full, toRead, reason)
	if collectErr != nil {
		p.failListing(key, b, collectErr)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	e = p.entryLocked(key)
	b.delta = publishListingLocked(e, entries, reads, full)
	b.snap = e.snapshot
	e.dirtyResolvedSeq = startSeq
	if full {
		e.reloadResolvedSeq = startSeq
	}
}

// takeWriteMarksLocked takes out every write mark recorded at or before
// startSeq. A stat-diff returns them as forced names; a full listing re-reads
// every file anyway and returns nil. The caller must hold the mutex.
func takeWriteMarksLocked(e *entry, startSeq uint64, full bool) map[string]bool {
	var forced map[string]bool
	for name, seq := range e.writeMarked {
		if seq > startSeq {
			continue
		}
		delete(e.writeMarked, name)
		if full {
			continue
		}
		if forced == nil {
			forced = map[string]bool{}
		}
		forced[name] = true
	}
	return forced
}

// diffListingLocked compares a listing with the recorded fingerprints and
// returns the names to re-read, and whether the pass is unchanged. An
// unchanged pass requires a published snapshot to keep serving. The name set
// is equal exactly when the lengths match and every listed entry is present
// and equal, and the missing/empty state did not flip. The caller must hold
// the mutex.
func diffListingLocked(
	e *entry,
	entries []FileEntry,
	forced map[string]bool,
) (map[string]bool, bool) {
	var toRead map[string]bool
	for _, fe := range entries {
		recorded, ok := e.fingerprints[fe.Name]
		if !forced[fe.Name] && ok && recorded == fe.Fingerprint {
			continue
		}
		if toRead == nil {
			toRead = make(map[string]bool)
		}
		toRead[fe.Name] = true
	}
	unchanged := len(forced) == 0 &&
		len(toRead) == 0 &&
		e.hasSnapshot &&
		len(entries) == len(e.fingerprints) &&
		(entries == nil) == (e.snapshot.entries == nil)
	return toRead, unchanged
}

// publishListingLocked merges a listing's reads into the key's snapshot,
// publishes it when the entries moved and returns the listing's delta. The
// caller must hold the mutex.
func publishListingLocked(
	e *entry,
	entries []FileEntry,
	reads *snapshotReads,
	full bool,
) *storeDelta {
	merged, dropped := mergeSnapshot(e, entries, reads)
	delta := listingDelta(reads, dropped)
	if e.hasSnapshot && sameEntries(e.snapshot.entries, merged) {
		return delta
	}
	e.snapshot = publish(merged)
	e.revision++
	// A publication is attributable to a bounded set of single files only when
	// it merged into an already-published snapshot. A key's first publication
	// publishes the whole snapshot whatever the build's kind, so it stays
	// unbounded.
	bounded := !full && e.hasSnapshot
	e.recordChangeLocked(listingChangedNames(full, delta), bounded, unboundedListingReason(e, bounded))
	e.hasSnapshot = true
	return delta
}

// unboundedListingReason returns the unbounded cause of a listing's
// publication, or "" when it is bounded.
func unboundedListingReason(e *entry, bounded bool) string {
	switch {
	case bounded:
		return ""
	case !e.hasSnapshot:
		return UnboundedFirstPublication
	default:
		return UnboundedForcedReload
	}
}

// failListing records a listing or collection failure: nothing is published,
// the error is logged with the key, and the mark stays pending so the next
// read retries.
func (p *pageIndex) failListing(key Key, b *build, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	b.snap = published{}
	b.err = err
	slog.Error(
		"page index rebuild failed",
		"vault", key.VaultPath,
		"pagesDir", key.PagesDir,
		"error", err,
	)
}

// listingDelta returns the change one listing produced: a put for every
// entry it re-read (including an excluded entry, stored as fingerprint plus a
// nil page) and a delete for every name the entry held before the merge that
// the merged set no longer holds. dropped is exactly those names, captured by
// mergeSnapshot before its in-place update removed them.
func listingDelta(reads *snapshotReads, dropped []string) *storeDelta {
	delta := &storeDelta{}
	for name, read := range reads.reads {
		delta.puts = append(delta.puts, StoredEntry{
			Filename:    name,
			Entry:       read.entry,
			Fingerprint: read.fingerprint,
		})
	}
	delta.deletes = append(delta.deletes, dropped...)
	return delta
}

// listingChangedNames returns the page names a listing's publication can be
// attributed to: every name the listing re-read and every name it dropped. A
// full listing — a cold build or a reload — re-reads the whole folder, so it
// cannot be attributed to a bounded set and reports none; the caller records it
// as unbounded instead.
func listingChangedNames(full bool, delta *storeDelta) []string {
	if full {
		return nil
	}
	names := make([]string, 0, len(delta.puts)+len(delta.deletes))
	for _, put := range delta.puts {
		names = append(names, put.Filename)
	}
	return append(names, delta.deletes...)
}

// release clears the finished build and promotes the key's follow-up, so no
// caller can slip a parallel build into the gap.
func (p *pageIndex) release(key Key, b *build) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.entryLocked(key)
	if b.snap.entries == nil && b.err == nil && e.hasSnapshot {
		// A per-file re-read publishes by splicing; its callers still need the
		// snapshot that is current once it is done.
		b.snap = e.snapshot
	}
	if e.inflight == b {
		e.inflight = nil
	}
	if e.followUp != nil {
		next := e.followUp
		e.followUp = nil
		next.startSeq = e.requestSeq
		e.inflight = next
		close(next.started)
	}
	close(b.done)
}

// maxUnreadablePageWarnings is the number of per-file skip warnings a single
// listing emits before it stops naming files individually and prints one
// summary line instead. It bounds a folder full of unreadable pages.
const maxUnreadablePageWarnings = 10

// fileRead is one listed file's read result within a listing. A nil entry
// means the read failed, so the file is excluded.
type fileRead struct {
	seq         uint64
	entry       *Entry
	fingerprint FileFingerprint
}

// snapshotReads is one listing's collected reads: the sequence the listing
// itself started at, the K-sized map of each re-read file's result and whether
// the folder was missing. It holds no per-name order slice: the listing's
// []FileEntry supplies the order.
type snapshotReads struct {
	listingSeq uint64
	reads      map[string]fileRead
	// missing reports that the folder does not exist, which lists as nil.
	missing bool
}

// collect reads the listed entries that need it: every entry when full is set,
// otherwise only the names in toRead (the changed, new and forced names). It
// lists nothing and copies no recorded fingerprints, so its allocation is
// bounded by the entries it actually re-read. It returns the listing's
// sequence and the result and fingerprint of every entry it re-read, including
// the ones whose read failed.
func (p *pageIndex) collect(
	ctx context.Context,
	key Key,
	listingSeq uint64,
	entries []FileEntry,
	full bool,
	toRead map[string]bool,
	reason string,
) (*snapshotReads, error) {
	result := &snapshotReads{
		listingSeq: listingSeq,
		reads:      map[string]fileRead{},
		missing:    entries == nil,
	}
	skipped := 0
	for _, fe := range entries {
		if err := ctx.Err(); err != nil {
			return nil, errors.Wrap(ctx, err, "build page snapshot")
		}
		if !full && !toRead[fe.Name] {
			continue
		}
		seq := p.takeReadSeq()
		read, fingerprint, readErr := p.reader.ReadEntry(
			ctx,
			key.VaultPath,
			key.PagesDir,
			fe.Name,
		)
		p.metrics.FileRead(reason)
		result.reads[fe.Name] = fileRead{seq: seq, entry: read, fingerprint: fingerprint}
		if readErr != nil {
			if skipped < maxUnreadablePageWarnings {
				p.warnf(
					"skipping unreadable page %q in %q/%q: %v",
					fe.Name,
					key.VaultPath,
					key.PagesDir,
					readErr,
				)
			}
			skipped++
		}
	}
	if skipped >= maxUnreadablePageWarnings {
		p.warnf("skipping %d unreadable pages", skipped)
	}
	return result, nil
}

// sameEntries reports whether two snapshots hold the same entries in the same
// order, distinguishing a nil snapshot from an empty one.
func sameEntries(a, b []*Entry) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// snapshotMerge accumulates one listing's merge into a new snapshot and
// updates the entry's recorded fingerprints in place. The entry's current
// snapshot is the other input, so a read a later-started one has already
// superseded is never applied.
type snapshotMerge struct {
	entry   *entry
	reads   *snapshotReads
	entries []*Entry
	dropped []string
}

// listed takes a listed file: the listing's own read when no later-started read
// has already been applied to that name, else the page currently published. A
// re-read updates the recorded fingerprint in place; a name carried forward
// keeps the fingerprint it already has.
func (m *snapshotMerge) listed(name string, current *Entry) {
	read, ok := m.reads.reads[name]
	if ok && read.seq >= m.entry.fileReadSeq[name] {
		m.entry.fileReadSeq[name] = read.seq
		m.entry.fingerprints[name] = read.fingerprint
		if read.entry != nil {
			m.entries = append(m.entries, read.entry)
		}
		return
	}
	m.kept(current)
}

// unlisted drops a name the listing no longer holds, unless the listing started
// before that name's last applied read — then a newer read put it there. A
// dropped name has its recorded fingerprint deleted in place so the recorded
// name set never keeps a tombstone the next listing could not match.
func (m *snapshotMerge) unlisted(name string, current *Entry) {
	if m.reads.listingSeq > m.entry.fileReadSeq[name] {
		m.remove(name)
		return
	}
	m.kept(current)
}

// kept carries a currently published page forward. A name the listing excludes
// has no page but keeps its fingerprint, so an unchanged broken file is neither
// re-read nor re-warned on the next listing.
func (m *snapshotMerge) kept(current *Entry) {
	if current != nil {
		m.entries = append(m.entries, current)
	}
}

// remove deletes one recorded fingerprint in place, recording it for the store
// delta when the prior value was non-zero — a zero fingerprint is the "never
// recorded" sentinel and has no stored entry to delete.
func (m *snapshotMerge) remove(name string) {
	previous, ok := m.entry.fingerprints[name]
	if !ok {
		return
	}
	delete(m.entry.fingerprints, name)
	if previous != (FileFingerprint{}) {
		m.dropped = append(m.dropped, name)
	}
}

// pruneFingerprints deletes every recorded fingerprint that is neither listed
// nor currently published — an excluded file the listing no longer holds, whose
// fingerprint the in-place update would otherwise keep forever. A tombstone
// kept past the pass would leave the recorded name set larger than any listing,
// so the unchanged fast path could never fire again.
func (m *snapshotMerge) pruneFingerprints(entries []FileEntry, current []*Entry) {
	for name := range m.entry.fingerprints {
		if listedContains(entries, name) || publishedContains(current, name) {
			continue
		}
		m.remove(name)
	}
}

// listedContains reports whether the sorted listing holds name.
func listedContains(entries []FileEntry, name string) bool {
	i := sort.Search(len(entries), func(i int) bool { return entries[i].Name >= name })
	return i < len(entries) && entries[i].Name == name
}

// publishedContains reports whether the sorted snapshot holds name, whose
// ordering key is the page's file name plus ".md".
func publishedContains(entries []*Entry, name string) bool {
	i := sort.Search(len(entries), func(i int) bool {
		return entries[i].Page.Name+".md" >= name
	})
	return i < len(entries) && entries[i].Page.Name+".md" == name
}

// mergeSnapshot merges a listing's reads into the entry's current snapshot in
// one O(n) pass over both ordered name sequences and updates the entry's
// recorded fingerprints in place. It returns the new entries slice and the names
// whose recorded fingerprints it dropped. The caller must hold the mutex.
func mergeSnapshot(
	e *entry,
	entries []FileEntry,
	reads *snapshotReads,
) ([]*Entry, []string) {
	current := e.snapshot.entries
	if e.fingerprints == nil {
		e.fingerprints = map[string]FileFingerprint{}
	}
	m := &snapshotMerge{
		entry:   e,
		reads:   reads,
		entries: make([]*Entry, 0, len(entries)),
	}
	i, j := 0, 0
	for i < len(entries) && j < len(current) {
		listed := entries[i].Name
		published := current[j].Page.Name + ".md"
		switch {
		case listed < published:
			m.listed(listed, nil)
			i++
		case listed > published:
			m.unlisted(published, current[j])
			j++
		default:
			m.listed(listed, current[j])
			i++
			j++
		}
	}
	for ; i < len(entries); i++ {
		m.listed(entries[i].Name, nil)
	}
	for ; j < len(current); j++ {
		m.unlisted(current[j].Page.Name+".md", current[j])
	}
	m.pruneFingerprints(entries, current)
	if reads.missing && len(m.entries) == 0 {
		return nil, m.dropped
	}
	return m.entries, m.dropped
}
