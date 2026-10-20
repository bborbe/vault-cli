// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-approve-operation.go --fake-name GoalApproveOperation . GoalApproveOperation
type GoalApproveOperation interface {
	// Execute approves the named goal: it moves a goal waiting in the approval
	// inbox (phase todo) to phase planning and records, in the same write, who
	// approved it and the instant of the transition. A goal that is not waiting
	// in the inbox, or that already carries an approval record, is refused with
	// nothing written.
	//
	// Goals are not required to have an owner, and this verb does not assign
	// one.
	Execute(
		ctx context.Context,
		vaultPath string,
		goalName string,
		vaultName string,
		approvedBy string,
	) (MutationResult, error)
}

// NewGoalApproveOperation creates a new goal approve operation.
func NewGoalApproveOperation(
	goalStorage storage.GoalStorage,
	currentDateTime libtime.CurrentDateTime,
) GoalApproveOperation {
	return &goalApproveOperation{
		goalStorage:     goalStorage,
		currentDateTime: currentDateTime,
	}
}

type goalApproveOperation struct {
	goalStorage     storage.GoalStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute approves a goal waiting in the approval inbox.
func (o *goalApproveOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	vaultName string,
	approvedBy string,
) (MutationResult, error) {
	// Refuse an empty approver before reading the goal: a present-but-blank
	// approved_by is worse than no record at all, and nothing is written here.
	if approvedBy == "" {
		err := errors.Errorf(
			ctx,
			"approved_by must not be empty: refusing to record a blank approver on goal %q",
			goalName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find goal")
	}

	// Read and validate the clock before any guard: a zero instant would be
	// recorded as a placeholder rather than a measurement, so it is refused.
	now := o.currentDateTime.Now()
	if now.Time().IsZero() {
		err := errors.Errorf(
			ctx,
			"approved_at is not a valid instant: refusing to record a zero timestamp on goal %q",
			goalName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The inbox guard: only a goal at phase todo is waiting for approval. The
	// message names both the current phase and the expected one so the operator
	// can see what was found and what was required.
	if phase := goal.Phase(); phase == nil || *phase != domain.GoalPhaseTodo {
		current := "(none)"
		if phase != nil {
			current = string(*phase)
		}
		err := errors.Errorf(
			ctx,
			"refusing to approve %q: goal is at phase %q, not %q; only a goal in the approval inbox (phase %q) can be approved",
			goalName, current, string(domain.GoalPhaseTodo), string(domain.GoalPhaseTodo),
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The existing-record guard: a todo row carrying an approval record is not
	// awaiting approval, and re-approving must not overwrite who approved it first.
	if err := refuseExistingEntityApprovalRecord(ctx, "goal", goalName, goal.Get); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All three keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	goal.SetPhase(domain.GoalPhasePlanning.Ptr())
	goal.Set("approved_by", approvedBy)
	goal.Set("approved_at", now.Time())

	if err := o.goalStorage.WriteGoal(ctx, goal); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write goal")
	}

	return MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil
}
