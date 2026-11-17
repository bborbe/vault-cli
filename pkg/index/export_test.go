// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	libtime "github.com/bborbe/time"
)

// CountingMetrics records every counter call, so a test can assert deltas.
type CountingMetrics struct {
	mu        sync.Mutex
	reads     map[string]int
	unbounded map[string]int
}

// NewCountingMetrics returns an empty CountingMetrics.
func NewCountingMetrics() *CountingMetrics {
	return &CountingMetrics{reads: map[string]int{}, unbounded: map[string]int{}}
}

// FileRead counts one read.
func (m *CountingMetrics) FileRead(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads[reason]++
}

// ChangedPagesUnbounded counts one refused bound.
func (m *CountingMetrics) ChangedPagesUnbounded(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unbounded[reason]++
}

// Reads returns the read count for reason.
func (m *CountingMetrics) Reads(reason string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reads[reason]
}

// TotalReads returns the read count over every reason.
func (m *CountingMetrics) TotalReads() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := 0
	for _, count := range m.reads {
		total += count
	}
	return total
}

// TestMetrics is shared by every test index, mirroring vault-ui's
// package-global counter: callers assert deltas, never absolutes.
var TestMetrics = NewCountingMetrics()

// FilesReadTotal returns the shared read counter for reason.
func FilesReadTotal(reason string) float64 {
	return float64(TestMetrics.Reads(reason))
}

// NewPageIndexWithWarnf builds a page index whose warnings go to warnf.
func NewPageIndexWithWarnf(
	reader PageReader,
	lister DirectoryLister,
	currentDateTimeGetter libtime.CurrentDateTimeGetter,
	waiter libtime.WaiterDuration,
	warnf func(format string, args ...any),
) PageIndex {
	idx := NewPageIndex(reader, lister, currentDateTimeGetter, waiter, TestMetrics)
	SetWarnf(idx, warnf)
	return idx
}

// FingerprintSetIdentity returns an identity token for the key's recorded
// fingerprint set. An unknown key reports "".
func FingerprintSetIdentity(idx PageIndex, key Key) string {
	pi, ok := idx.(*pageIndex)
	if !ok {
		return ""
	}
	pi.mu.Lock()
	defer pi.mu.Unlock()
	e, ok := pi.entries[key]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%p", e.fingerprints)
}

// SnapshotIdentity returns an identity token for the key's published snapshot.
// An unknown or unpublished key reports "".
func SnapshotIdentity(idx PageIndex, key Key) string {
	pi, ok := idx.(*pageIndex)
	if !ok {
		return ""
	}
	pi.mu.Lock()
	defer pi.mu.Unlock()
	e, ok := pi.entries[key]
	if !ok || !e.hasSnapshot {
		return ""
	}
	return fmt.Sprintf("%p", e.snapshot.entries)
}
