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
	// approved it, the instant of the transition, and the task's owner. Any task
	// that is not waiting in the inbox, and any task that already carries an
	// approval record, is refused with nothing written.
	//
	// The owner is resolved once, at approval time, by precedence: assignee when
	// non-empty, else the task's existing assignee, else currentUser. An empty
	// assignee means the --assignee flag was not given (a blank flag value is
	// never a request for an unowned task); an empty currentUser means no current
	// user could be resolved. When none of the three names an owner the approval
	// is refused and nothing is written, so a task can never enter planning
	// ownerless. An existing assignee that neither the flag nor currentUser
	// matches is preserved untouched and silently.
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		approvedBy string,
		assignee string,
		currentUser string,
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
	assignee string,
	currentUser string,
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
	// awaiting approval, and re-approving must not overwrite who approved it first.
	if err := refuseExistingApprovalRecord(ctx, taskName, task); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// Resolve the owner after every guard and before any mutation. Refusing here
	// — and only here — keeps the four earlier refusals' messages intact while
	// making approval the one moment an owner is fixed.
	owner, err := resolveOwner(ctx, taskName, task, assignee, currentUser)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All four keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	// The status is next, not in_progress: an approved row with no live session is
	// queued to be started, not active. The manager sweep's ready-to-start bucket
	// requires status next in both renderers, so writing in_progress was the write
	// that removed the row from the spawn offer.
	if err := task.SetStatus(domain.TaskStatusNext); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "set status")
	}
	task.SetPhase(domain.TaskPhasePlanning.Ptr())
	task.Set("approved_by", approvedBy)
	task.Set("approved_at", now.Time())
	// Only write the owner when it differs: a task already carrying the owner
	// the flag or the config would produce is not dirtied by an assignee write.
	if owner != task.Assignee() {
		task.SetAssignee(owner)
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write task")
	}

	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
}

// refuseExistingApprovalRecord returns the refusal error for a todo row that
// already carries an approved_by or an approved_at key, and nil otherwise. Both
// keys are checked independently so a row carrying either one is refused, and
// the message names every key that is present.
func refuseExistingApprovalRecord(ctx context.Context, taskName string, task *domain.Task) error {
	var present []string
	if task.Get("approved_by") != nil {
		present = append(present, "approved_by")
	}
	if task.Get("approved_at") != nil {
		present = append(present, "approved_at")
	}
	if len(present) == 0 {
		return nil
	}
	return errors.Errorf(
		ctx,
		"refusing to approve %q: task already carries an approval record (%s); re-approving is refused",
		taskName, strings.Join(present, ", "),
	)
}

// resolveOwner fixes the task's owner by the precedence the approval contract
// states: the --assignee flag, else the task's own assignee (preserved as it
// is), else the configured current user. It returns a refusal error when none
// of the three names an owner, so the caller writes nothing.
func resolveOwner(
	ctx context.Context,
	taskName string,
	task *domain.Task,
	assignee string,
	currentUser string,
) (string, error) {
	owner := assignee
	if owner == "" {
		owner = task.Assignee()
	}
	if owner == "" {
		owner = currentUser
	}
	if owner == "" {
		return "", errors.Errorf(
			ctx,
			"assignee must not be empty: task %q has no assignee, no --assignee was given, and no current_user is configured; refusing to approve an unowned task",
			taskName,
		)
	}
	return owner, nil
}
