// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/bborbe/validation"
	"github.com/google/uuid"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// AppendMetricsSessionOperation appends one metrics entry to a task.
//
//counterfeiter:generate -o ../../mocks/append-metrics-session-operation.go --fake-name AppendMetricsSessionOperation . AppendMetricsSessionOperation
type AppendMetricsSessionOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, sessionID string) error
}

// NewAppendMetricsSessionOperation creates a new append-metrics-session operation.
func NewAppendMetricsSessionOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) AppendMetricsSessionOperation {
	return &appendMetricsSessionOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
	}
}

type appendMetricsSessionOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute validates the session id, then appends exactly one entry to the task's
// metrics_sessions, preserving every entry already present. started_at is stamped
// here from the injected clock; no caller supplies it.
//
// The id is validated before the task is read, so an id this verb cannot honour is
// refused with nothing read and nothing written. The entry goes through the domain
// append (domain.TaskFrontmatter.AppendMetricsSession) — the same writer the
// work-on path uses — so the two producers emit one shape.
//
// There is deliberately no re-read, no lock and no dedup: the spec accepts the same
// last-write-wins race the work-on path has, and the accumulator must never suppress
// a duplicate session id.
func (o *appendMetricsSessionOperation) Execute(
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
	task.AppendMetricsSession(domain.MetricsSession{
		SessionID: sessionID,
		StartedAt: libtime.DateOrDateTime(o.currentDateTime.Now().Time()),
	})
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}
	return nil
}
