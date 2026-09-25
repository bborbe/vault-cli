// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"
)

// metricsSessionsWriteRefusal returns an error when a generic write verb
// (set / add / remove) targets metrics_sessions on a task.
//
// The field's entries are maps (session_id + started_at). `set` stores whatever
// string it is handed, and `add` / `remove` comma-split their value into a list of
// scalars; the dedicated list reader discards both shapes by design, so either verb
// would report success while recording nothing visible. The append goes through
// `vault-cli task append-metrics-session <task> <session-id>`.
//
// Nothing is written on the refusal path: the check runs before any mutation, so the
// file on disk is byte-identical. There is no --force bypass — --force is scoped to
// the phase-regression guard, and this refusal is deliberately absolute.
func metricsSessionsWriteRefusal(ctx context.Context, taskName, key string) error {
	if key != "metrics_sessions" {
		return nil
	}
	return errors.Errorf(ctx,
		"cannot write %q on %q with this verb: metrics_sessions entries are maps (session_id + started_at) and every reader discards a scalar or a comma-split list. Append an entry with `vault-cli task append-metrics-session %q <session-id>`",
		key, taskName, taskName,
	)
}
