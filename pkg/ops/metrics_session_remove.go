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

// RemoveMetricsSessionOperation removes one metrics entry from a task by session id.
//
//counterfeiter:generate -o ../../mocks/remove-metrics-session-operation.go --fake-name RemoveMetricsSessionOperation . RemoveMetricsSessionOperation
type RemoveMetricsSessionOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, sessionID string) error
}

// NewRemoveMetricsSessionOperation creates a new remove-metrics-session operation.
func NewRemoveMetricsSessionOperation(
	taskStorage storage.TaskStorage,
) RemoveMetricsSessionOperation {
	return &removeMetricsSessionOperation{
		taskStorage: taskStorage,
	}
}

type removeMetricsSessionOperation struct {
	taskStorage storage.TaskStorage
}

// Execute validates the session id, then removes every metrics_sessions entry carrying
// it, preserving every other entry. The generic write verbs (set / add / remove) refuse
// metrics_sessions outright — their scalar and comma-split shapes cannot express a map
// entry — so this dedicated verb is the only path that removes one entry by id.
//
// The id is validated before the task is read, so an id this verb cannot honour is
// refused with nothing read and nothing written. When no entry carries the id the task is
// not written at all and the call fails: a silent no-op is the exact defect this verb
// exists to fix, so a typo'd or already-cleared id must be visible, never reported as
// success.
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
	if removed := task.RemoveMetricsSession(sessionID); removed == 0 {
		return errors.Errorf(ctx,
			"no metrics_sessions entry with session id %q on %q — nothing removed",
			sessionID, taskName,
		)
	}
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}
	return nil
}
