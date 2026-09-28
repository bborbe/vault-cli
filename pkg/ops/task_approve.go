// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/task-approve-operation.go --fake-name TaskApproveOperation . TaskApproveOperation
type TaskApproveOperation interface {
	// Execute approves the named task: it moves a task waiting in the approval
	// inbox (phase todo) to phase planning and records, in the same write, who
	// approved it and the instant of the transition. Any task that is not
	// waiting in the inbox, and any task that already carries an approval
	// record, is refused with nothing written.
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		approvedBy string,
	) (MutationResult, error)
}

// NewTaskApproveOperation creates a new task approve operation.
func NewTaskApproveOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) TaskApproveOperation {
	return &taskApproveOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
	}
}

type taskApproveOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute approves a task waiting in the approval inbox.
func (o *taskApproveOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	approvedBy string,
) (MutationResult, error) {
	// Refuse an empty approver before reading the task: a present-but-blank
	// approved_by is worse than no record at all, and nothing is written here.
	if approvedBy == "" {
		err := errors.Errorf(
			ctx,
			"approved_by must not be empty: refusing to record a blank approver on task %q",
			taskName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find task")
	}

	// Read and validate the clock before any guard: a zero instant would be
	// recorded as a placeholder rather than a measurement, so it is refused.
	now := o.currentDateTime.Now()
	if now.Time().IsZero() {
		err := errors.Errorf(
			ctx,
			"approved_at is not a valid instant: refusing to record a zero timestamp on task %q",
			taskName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The inbox guard: only a task at phase todo is waiting for approval. The
	// message names both the current phase and the expected one so the operator
	// can see what was found and what was required.
	if phase := task.Phase(); phase == nil || *phase != domain.TaskPhaseTodo {
		current := "(none)"
		if phase != nil {
			current = string(*phase)
		}
		err := errors.Errorf(
			ctx,
			"refusing to approve %q: task is at phase %q, not %q; only a task in the approval inbox (phase %q) can be approved",
			taskName, current, string(domain.TaskPhaseTodo), string(domain.TaskPhaseTodo),
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The existing-record guard: a todo row carrying an approval record is not
	// awaiting approval, and re-approving must not overwrite who approved it
	// first. Both keys are checked independently so a row carrying either one
	// is refused, and the message names every key that is present.
	var present []string
	if task.Get("approved_by") != nil {
		present = append(present, "approved_by")
	}
	if task.Get("approved_at") != nil {
		present = append(present, "approved_at")
	}
	if len(present) > 0 {
		err := errors.Errorf(
			ctx,
			"refusing to approve %q: task already carries an approval record (%s); re-approving is refused",
			taskName, strings.Join(present, ", "),
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All four keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	if err := task.SetStatus(domain.TaskStatusInProgress); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "set status")
	}
	task.SetPhase(domain.TaskPhasePlanning.Ptr())
	task.Set("approved_by", approvedBy)
	task.Set("approved_at", now.Time())

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write task")
	}

	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
}
