// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"
	"github.com/bborbe/validation"
	"github.com/google/uuid"

	"github.com/bborbe/vault-cli/pkg/storage"
)

// RemoveMetricsSessionOperation removes every metrics entry carrying one session
// id from a task.
//
//counterfeiter:generate -o ../../mocks/remove-metrics-session-operation.go --fake-name RemoveMetricsSessionOperation . RemoveMetricsSessionOperation
type RemoveMetricsSessionOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, sessionID string) error
}

// NewRemoveMetricsSessionOperation creates a new remove-metrics-session operation.
func NewRemoveMetricsSessionOperation(
	taskStorage storage.TaskStorage,
) RemoveMetricsSessionOperation {
	return &removeMetricsSessionOperation{taskStorage: taskStorage}
}

type removeMetricsSessionOperation struct {
	taskStorage storage.TaskStorage
}

// Execute validates the session id, then removes every entry carrying it from the
// task's metrics_sessions, preserving every other entry in order. When the last
// entry goes, the key is deleted rather than left as an empty list.
//
// The id is validated before the task is read, so an id this verb cannot honour is
// refused with nothing read and nothing written.
//
// A call whose id matches no entry fails loudly and writes nothing at all: WriteTask
// is never called, so the file is byte-identical by construction rather than by a
// rewrite that happens to be stable. The removal goes through the domain
// (domain.TaskFrontmatter.RemoveMetricsSession) — the same reader every other
// consumer uses — so no string-level rewrite of the frontmatter block is possible.
//
// The verb stamps nothing, takes no clock, never writes claude_session_id, and adds
// no --force / --all / --started-at flag: removal is by session id only.
func (o *removeMetricsSessionOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, sessionID string,
) error {
	if err := uuid.Validate(sessionID); err != nil {
		return errors.Wrapf(ctx, validation.Error,
			"invalid session id %q: expected a well-formed UUID", sessionID)
	}
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}
	removed := task.RemoveMetricsSession(sessionID)
	if removed == 0 {
		return errors.Errorf(ctx,
			"no metrics_sessions entry with session id %q on task %q: nothing removed",
			sessionID, taskName,
		)
	}
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}
	return nil
}
