// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"strings"

	"github.com/bborbe/errors"
	"github.com/bborbe/validation"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/frontmatter-get-operation.go --fake-name FrontmatterGetOperation . FrontmatterGetOperation
type FrontmatterGetOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) (string, error)
}

// NewFrontmatterGetOperation creates a new frontmatter get operation.
func NewFrontmatterGetOperation(taskStorage storage.TaskStorage) FrontmatterGetOperation {
	return &frontmatterGetOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterGetOperation struct {
	taskStorage storage.TaskStorage
}

// Execute retrieves the value of a frontmatter field from a task.
func (o *frontmatterGetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) (string, error) {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return "", errors.Wrap(ctx, err, "find task")
	}

	return task.GetField(key), nil
}

//counterfeiter:generate -o ../../mocks/frontmatter-set-operation.go --fake-name FrontmatterSetOperation . FrontmatterSetOperation
type FrontmatterSetOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key, value, reason, gateSuccessor string, force bool) error
}

// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
	}
}

type frontmatterSetOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
}

// Execute sets the value of a frontmatter field on a task.
func (o *frontmatterSetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key, value, reason, gateSuccessor string, force bool,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// Read the assignee before the mutation. `task set <task> assignee ""` leaves
	// the key present and empty, so a read taken after the write can no longer
	// tell a cleared assignee from one that was never set.
	previousAssignee := task.Assignee()

	// One-step close-out: when this invocation sets a close-out status, persist
	// reason and successor first so both land in a single WriteTask. Fields are
	// written only when provided; non-close-out targets never receive them.
	if err := writeTaskCloseOutFieldsIfCloseOut(ctx, task, value, reason, gateSuccessor); err != nil {
		return err
	}

	// Phase-regression guard: an in-progress task may not silently move
	// backward out of execution into todo (observed 2026-09-02: a bulk
	// `task set phase todo` loop regressed 80 active tasks). --force overrides.
	if err := checkPhaseRegression(ctx, task, key, value, force); err != nil {
		return err
	}

	if err := task.SetField(ctx, key, value); err != nil {
		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
			err = errors.Errorf(ctx,
				"%s\nTry: vault-cli task set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
				err.Error(), taskName, value)
		}
		return errors.Wrap(ctx, err, "set field")
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee)

	return nil
}

// writeTaskCloseOutFieldsIfCloseOut writes the one-step close-out fields when
// the target status is a close-out (aborted or completed), returning the first
// write error. Fields are written only when provided; non-close-out targets
// never receive them.
func writeTaskCloseOutFieldsIfCloseOut(
	ctx context.Context,
	task *domain.Task,
	value, reason, gateSuccessor string,
) error {
	target, ok := domain.NormalizeTaskStatus(value)
	if !ok || (target != domain.TaskStatusAborted && target != domain.TaskStatusCompleted) {
		return nil
	}
	if reason != "" {
		if err := task.SetField(ctx, "aborted_reason", reason); err != nil {
			return errors.Wrap(ctx, err, "set aborted_reason")
		}
	}
	if gateSuccessor != "" {
		if err := task.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
			return errors.Wrap(ctx, err, "set gate_successor")
		}
	}
	return nil
}

//counterfeiter:generate -o ../../mocks/frontmatter-clear-operation.go --fake-name FrontmatterClearOperation . FrontmatterClearOperation
type FrontmatterClearOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) error
}

// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
}

// Execute clears (removes) the value of a frontmatter field on a task.
func (o *frontmatterClearOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// Read the assignee before the deletion. ClearField removes the key outright,
	// so a read taken after the write always yields "".
	previousAssignee := task.Assignee()

	task.ClearField(key)

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee)

	return nil
}

// checkPhaseRegression rejects setting phase "todo" on a task whose status is
// in_progress and whose current phase is execution, ai_review, or human_review.
// Such a write silently regresses the lifecycle and stalls the execution
// pipeline (observed 2026-09-02: a bulk `task set phase todo` loop regressed 80
// active tasks in the Personal vault). A deliberate reset must pass force=true.
func checkPhaseRegression(ctx context.Context, task *domain.Task, key, value string, force bool) error {
	if force || key != "phase" || task.Status() != domain.TaskStatusInProgress {
		return nil
	}
	canonical, ok := domain.NormalizeTaskPhase(value)
	if !ok || canonical != domain.TaskPhaseTodo {
		return nil
	}
	current := task.Phase()
	if current == nil {
		return nil
	}
	switch *current {
	case domain.TaskPhaseExecution, domain.TaskPhaseAIReview, domain.TaskPhaseHumanReview:
		return errors.Wrapf(ctx, validation.Error,
			"refusing to set phase %q on %q: task is in_progress with phase %q; this regression would silently stall the execution pipeline. Pass --force to override",
			canonical, task.Name, *current)
	}
	return nil
}

// publishAssigneeClearEscalation emits one agent-escalation notification when a
// frontmatter write cleared a non-empty assignee. It is the single transition
// rule both clear paths share.
//
// previousAssignee MUST be read before the mutation: `task set <task> assignee ""`
// leaves the key present and empty while `task clear <task> assignee` deletes it,
// so a read taken after the mutation yields "" on both paths and the notification
// would lose the one value it exists to carry. value is always "" on the clear
// path, which has no value argument.
func publishAssigneeClearEscalation(
	ctx context.Context,
	publisher EscalationPublisher,
	task *domain.Task,
	key, value, previousAssignee string,
) {
	if key != "assignee" || value != "" || previousAssignee == "" {
		return
	}
	publisher.PublishEscalation(ctx, Escalation{
		TaskIdentifier:   task.TaskIdentifier(),
		TaskName:         task.Name,
		PreviousAssignee: previousAssignee,
	})
}
