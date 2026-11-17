// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package index

// The reasons a single-file read happens, reported through Metrics.FileRead.
const (
	ReadReasonBuild  = "build"
	ReadReasonEvent  = "event"
	ReadReasonWrite  = "write"
	ReadReasonRescan = "rescan"
	ReadReasonReload = "reload"
)

// ReadReasons is every reason Metrics.FileRead can be called with.
var ReadReasons = []string{
	ReadReasonBuild,
	ReadReasonEvent,
	ReadReasonWrite,
	ReadReasonRescan,
	ReadReasonReload,
}

// The reasons a changed set cannot be bounded, reported through
// Metrics.ChangedPagesUnbounded.
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

// UnboundedReasons is every reason Metrics.ChangedPagesUnbounded can be called
// with.
var UnboundedReasons = []string{
	UnboundedUnknownKey,
	UnboundedRevisionAhead,
	UnboundedWindowNotRetained,
	UnboundedForcedReload,
	UnboundedFirstPublication,
	UnboundedFolderMarkExplicit,
	UnboundedFolderMarkWidenedName,
	UnboundedFolderMarkWidenedFile,
	UnboundedFolderMarkWidenedNameInvalid,
	UnboundedFolderMarkWidenedKey,
}

//counterfeiter:generate -o ../../mocks/index-metrics.go --fake-name IndexMetrics . Metrics

// Metrics receives the index's counters. A consumer adapts it to its own
// metrics stack; the package itself imports none.
type Metrics interface {
	// FileRead counts one single-file page read by reason.
	FileRead(reason string)
	// ChangedPagesUnbounded counts one refused changed-set bound by reason.
	ChangedPagesUnbounded(reason string)
}

// NewNoopMetrics returns a Metrics that records nothing.
func NewNoopMetrics() Metrics {
	return noopMetrics{}
}

type noopMetrics struct{}

func (noopMetrics) FileRead(string) {}

func (noopMetrics) ChangedPagesUnbounded(string) {}
