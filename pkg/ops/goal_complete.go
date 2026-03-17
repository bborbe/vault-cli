// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
		outputFormat string,
		force bool,
	) error
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

// GoalCompleteResult represents the JSON result of a goal complete operation.
type GoalCompleteResult struct {
	Success   bool   `json:"success"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Completed string `json:"completed,omitempty"`
	Vault     string `json:"vault,omitempty"`
	Error     string `json:"error,omitempty"`
}

func outputGoalCompleteError(outputFormat string, msg string) {
	if outputFormat == "json" {
		result := GoalCompleteResult{Success: false, Error: msg}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	}
}

// Execute marks a goal as completed, optionally blocking if open tasks exist.
func (g *goalCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	vaultName string,
	outputFormat string,
	force bool,
) error {
	goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		outputGoalCompleteError(outputFormat, err.Error())
		return errors.Wrap(ctx, err, "find goal")
	}

	if goal.Status == domain.GoalStatusCompleted {
		outputGoalCompleteError(outputFormat, fmt.Sprintf("goal %q is already completed", goalName))
		return fmt.Errorf("goal %q is already completed", goalName) //nolint:goerr113
	}

	if !force {
		if err := g.checkOpenTasks(ctx, vaultPath, goalName, outputFormat); err != nil {
			return err
		}
	}

	goal.Status = domain.GoalStatusCompleted
	goal.Completed = libtime.ToDate(g.currentDateTime.Now().Time()).Ptr()

	if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
		outputGoalCompleteError(outputFormat, err.Error())
		return errors.Wrap(ctx, err, "write goal")
	}

	if outputFormat == "json" {
		result := GoalCompleteResult{
			Success:   true,
			Name:      goal.Name,
			Status:    string(goal.Status),
			Completed: goal.Completed.Format("2006-01-02"),
			Vault:     vaultName,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("✅ Goal completed: %s\n", goal.Name)
	return nil
}

func (g *goalCompleteOperation) checkOpenTasks(
	ctx context.Context,
	vaultPath string,
	goalName string,
	outputFormat string,
) error {
	tasks, err := g.taskStorage.ListTasks(ctx, vaultPath)
	if err != nil {
		outputGoalCompleteError(outputFormat, err.Error())
		return errors.Wrap(ctx, err, "list tasks")
	}

	var openTasks []*domain.Task
	for _, task := range tasks {
		if !taskLinkedToGoal(task, goalName) {
			continue
		}
		if task.Status == domain.TaskStatusTodo || task.Status == domain.TaskStatusInProgress {
			openTasks = append(openTasks, task)
		}
	}

	if len(openTasks) > 0 {
		taskList := joinTaskNames(openTasks)
		outputGoalCompleteError(
			outputFormat,
			fmt.Sprintf(
				"cannot complete goal: %d task(s) still open: %s",
				len(openTasks),
				taskList,
			),
		)
		return fmt.Errorf(
			"cannot complete goal: %d task(s) still open: %s",
			len(openTasks),
			taskList,
		) //nolint:goerr113
	}

	return nil
}

func taskLinkedToGoal(task *domain.Task, goalName string) bool {
	for _, g := range task.Goals {
		if g == goalName {
			return true
		}
	}
	return false
}

func joinTaskNames(tasks []*domain.Task) string {
	names := make([]string, len(tasks))
	for i, t := range tasks {
		names[i] = fmt.Sprintf("%s (%s)", t.Name, t.Status)
	}
	return strings.Join(names, ", ")
}
