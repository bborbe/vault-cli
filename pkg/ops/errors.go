// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	stderrors "errors"
)

// ErrStarterUnavailable indicates the claude session starter is nil
// (typically because the configured claude script is not on PATH).
// This is intentionally a soft failure — work-on still marks the task
// in_progress on disk; the CLI exits 0 with the error recorded as a warning.
var ErrStarterUnavailable = stderrors.New(
	"claude session starter unavailable — claude script not found in PATH",
)

// ErrSessionBusy indicates another claude process already holds the per-session
// lock for the requested session id (a second work-on or resume raced a live
// session). Hard failure — the caller spawns no second writer.
var ErrSessionBusy = stderrors.New(
	"claude session is busy — another process is already working on this session",
)
