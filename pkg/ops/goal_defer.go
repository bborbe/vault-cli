// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-defer-operation.go --fake-name GoalDeferOperation . GoalDeferOperation
type GoalDeferOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		goalName string,
		dateStr string,
		vaultName string,
	) (MutationResult, error)
}

// NewGoalDeferOperation creates a new goal defer operation.
func NewGoalDeferOperation(
	goalStorage storage.GoalStorage,
	currentDateTime libtime.CurrentDateTime,
) GoalDeferOperation {
	return &goalDeferOperation{
		goalStorage:     goalStorage,
		currentDateTime: currentDateTime,
	}
}

type goalDeferOperation struct {
	goalStorage     storage.GoalStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute sets defer_date on a goal without updating daily notes.
func (g *goalDeferOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	dateStr string,
	vaultName string,
) (MutationResult, error) {
	now := g.currentDateTime.Now().Time()

	targetDate, err := parseDeferDate(ctx, dateStr, now)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "parse date")
	}

	if isDeferDateInPast(targetDate, now) {
		return MutationResult{
				Success: false,
				Error: fmt.Sprintf(
					"cannot defer to past date: %s",
					targetDate.Time().Format("2006-01-02"),
				),
			}, errors.Errorf(
				ctx,
				"cannot defer to past date: %s",
				targetDate.Time().Format("2006-01-02"),
			)
	}

	goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find goal")
	}

	goal.SetDeferDate(targetDate.Ptr())

	if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write goal")
	}

	formattedDate := targetDate.Time().Format("2006-01-02")
	return MutationResult{
		Success: true,
		Name:    goal.Name,
		Vault:   vaultName,
		Message: formattedDate,
	}, nil
}
