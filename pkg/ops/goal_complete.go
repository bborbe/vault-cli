// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-complete-operation.go --fake-name GoalCompleteOperation . GoalCompleteOperation
type GoalCompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		goalName string,
		vaultName string,
		force bool,
		reason string,
		gateSuccessor string,
	) (MutationResult, error)
}

// NewGoalCompleteOperation creates a new goal complete operation.
func NewGoalCompleteOperation(
	goalStorage storage.GoalStorage,
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) GoalCompleteOperation {
	return &goalCompleteOperation{
		goalStorage:     goalStorage,
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
	}
}

type goalCompleteOperation struct {
	goalStorage     storage.GoalStorage
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute marks a goal as completed, optionally blocking if open tasks exist.
func (g *goalCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	vaultName string,
	force bool,
	reason string,
	gateSuccessor string,
) (MutationResult, error) {
	goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"find goal",
		)
	}

	if goal.Status() == domain.GoalStatusCompleted {
		msg := fmt.Sprintf("goal %q is already completed", goalName)
		return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
	}

	if !force {
		if result, err := g.checkOpenTasks(ctx, vaultPath, goalName); err != nil {
			return result, err
		}
	}

	// One-step close-out: persist reason and successor before the status
	// transition so both land in a single WriteGoal. Fields are written only
	// when provided — the two-step close-out (fields already in the file) works
	// unchanged.
	if result, err := g.setCompletedStatus(ctx, goal, goalName, reason, gateSuccessor); err != nil {
		return result, err
	}
	goal.SetCompleted(libtime.ToDate(g.currentDateTime.Now().Time()).Ptr())

	if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"write goal",
		)
	}

	return MutationResult{Success: true, Name: goal.Name, Vault: vaultName}, nil
}

// setCompletedStatus persists the one-step close-out fields (when provided)
// and transitions the goal to completed. A guard rejection returns an error
// naming the missing fields and the succeeding command form.
//
//nolint:dupl // Structurally parallel to the task variant; frozen field names prevent dedup
func (g *goalCompleteOperation) setCompletedStatus(
	ctx context.Context,
	goal *domain.Goal,
	goalName string,
	reason, gateSuccessor string,
) (MutationResult, error) {
	if reason != "" {
		if err := goal.SetField(ctx, "aborted_reason", reason); err != nil {
			return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(ctx, err, "set aborted_reason")
		}
	}
	if gateSuccessor != "" {
		if err := goal.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
			return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(ctx, err, "set gate_successor")
		}
	}
	if err := goal.SetStatus(domain.GoalStatusCompleted); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "missing close-out field(s)") {
			msg = fmt.Sprintf(
				"%s\nTry: vault-cli goal complete \"%s\" --reason \"<text>\" --gate-successor \"<successor|none>\"",
				msg,
				goalName,
			)
		}
		return MutationResult{Success: false, Error: msg}, errors.Wrap(ctx, err, "set status")
	}
	return MutationResult{}, nil
}

func (g *goalCompleteOperation) checkOpenTasks(
	ctx context.Context,
	vaultPath string,
	goalName string,
) (MutationResult, error) {
	tasks, err := g.taskStorage.ListTasks(ctx, vaultPath)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"list tasks",
		)
	}

	var openTasks []*domain.Task
	for _, task := range tasks {
		if !taskLinkedToGoal(task, goalName) {
			continue
		}
		if task.Status() == domain.TaskStatusNext ||
			task.Status() == domain.TaskStatusTodo ||
			task.Status() == domain.TaskStatusInProgress {
			openTasks = append(openTasks, task)
		}
	}

	if len(openTasks) > 0 {
		taskList := joinTaskNames(openTasks)
		msg := fmt.Sprintf(
			"cannot complete goal: %d task(s) still open: %s",
			len(openTasks),
			taskList,
		)
		return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
	}

	return MutationResult{}, nil
}

func taskLinkedToGoal(task *domain.Task, goalName string) bool {
	for _, g := range task.Goals() {
		if g == goalName {
			return true
		}
	}
	return false
}

func joinTaskNames(tasks []*domain.Task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = fmt.Sprintf("%s (%s)", t.Name, t.Status())
	}
	return strings.Join(names, ", ")
}
