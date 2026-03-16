// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/update-operation.go --fake-name UpdateOperation . UpdateOperation
type UpdateOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		outputFormat string,
	) error
}

// NewUpdateOperation creates a new update operation.
func NewUpdateOperation(
	taskStorage storage.TaskStorage,
	goalStorage storage.GoalStorage,
) UpdateOperation {
	return &updateOperation{
		taskStorage: taskStorage,
		goalStorage: goalStorage,
	}
}

type updateOperation struct {
	taskStorage storage.TaskStorage
	goalStorage storage.GoalStorage
}

// Execute syncs checkbox progress from the task content.
func (u *updateOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	outputFormat string,
) error {
	var warnings []string

	task, err := u.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		u.outputErrorJSON(outputFormat, err)
		return errors.Wrap(ctx, err, "find task")
	}

	checkboxes := u.parseCheckboxes(task.Content)
	if len(checkboxes) == 0 {
		return u.handleNoCheckboxes(task.Name, vaultName, outputFormat)
	}

	completed, total := u.countCompleted(checkboxes)
	task.Status = u.statusFromProgress(completed, total)

	if err := u.taskStorage.WriteTask(ctx, task); err != nil {
		u.outputErrorJSON(outputFormat, err)
		return errors.Wrap(ctx, err, "write task")
	}

	warnings = u.syncGoals(ctx, vaultPath, task.Goals, checkboxes, outputFormat, warnings)

	return u.outputUpdateResult(
		outputFormat,
		task.Name,
		vaultName,
		completed,
		total,
		task.Status,
		warnings,
	)
}

func (u *updateOperation) outputErrorJSON(outputFormat string, err error) {
	if outputFormat != "json" {
		return
	}
	result := MutationResult{Success: false, Error: err.Error()}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

func (u *updateOperation) handleNoCheckboxes(taskName, vaultName, outputFormat string) error {
	warning := "No checkboxes found in task"
	if outputFormat == "plain" {
		fmt.Printf("%s: %s\n", warning, taskName)
		return nil
	}
	result := MutationResult{
		Success:  true,
		Name:     taskName,
		Vault:    vaultName,
		Warnings: []string{warning},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func (u *updateOperation) countCompleted(checkboxes []domain.CheckboxItem) (int, int) {
	completed := 0
	for _, cb := range checkboxes {
		if cb.Checked {
			completed++
		}
	}
	return completed, len(checkboxes)
}

func (u *updateOperation) statusFromProgress(completed, total int) domain.TaskStatus {
	if completed == total {
		return domain.TaskStatusCompleted
	}
	if completed > 0 {
		return domain.TaskStatusInProgress
	}
	return domain.TaskStatusTodo
}

func (u *updateOperation) syncGoals(
	ctx context.Context,
	vaultPath string,
	goals []string,
	checkboxes []domain.CheckboxItem,
	outputFormat string,
	warnings []string,
) []string {
	for _, goalName := range goals {
		if err := u.syncGoalCheckboxes(ctx, vaultPath, goalName, checkboxes); err != nil {
			warning := fmt.Sprintf("failed to sync goal %s: %v", goalName, err)
			warnings = append(warnings, warning)
			if outputFormat == "plain" {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
		}
	}
	return warnings
}

func (u *updateOperation) outputUpdateResult(
	outputFormat string,
	taskName string,
	vaultName string,
	completed int,
	total int,
	status domain.TaskStatus,
	warnings []string,
) error {
	if outputFormat == "json" {
		result := MutationResult{
			Success:  true,
			Name:     taskName,
			Vault:    vaultName,
			Warnings: warnings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Printf(
		"✅ Task updated: %s (%d/%d completed, status: %s)\n",
		taskName,
		completed,
		total,
		status,
	)
	return nil
}

// parseCheckboxes extracts checkbox items from markdown content.
func (u *updateOperation) parseCheckboxes(content string) []domain.CheckboxItem {
	var items []domain.CheckboxItem
	lines := strings.Split(content, "\n")
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)

	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			state := matches[2]
			items = append(items, domain.CheckboxItem{
				Line:    i,
				Checked: state == "x",
				Text:    matches[3],
				RawLine: line,
			})
		}
	}

	return items
}

// syncGoalCheckboxes updates checkboxes in the goal based on task progress.
func (u *updateOperation) syncGoalCheckboxes(
	ctx context.Context,
	vaultPath string,
	goalName string,
	taskCheckboxes []domain.CheckboxItem,
) error {
	goal, err := u.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return errors.Wrap(ctx, err, "find goal")
	}

	lines := strings.Split(goal.Content, "\n")
	modified := false

	// For each checkbox in the task, try to find matching checkbox in goal
	for _, taskCb := range taskCheckboxes {
		for i, line := range lines {
			if strings.Contains(line, "- [") &&
				strings.Contains(strings.ToLower(line), strings.ToLower(taskCb.Text)) {
				// Update if different
				if taskCb.Checked && strings.Contains(line, "- [ ]") {
					lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
					modified = true
				} else if !taskCb.Checked && strings.Contains(line, "- [x]") {
					lines[i] = strings.Replace(line, "- [x]", "- [ ]", 1)
					modified = true
				}
				break
			}
		}
	}

	if !modified {
		return nil
	}

	// Update goal content and write
	goal.Content = strings.Join(lines, "\n")
	if err := u.goalStorage.WriteGoal(ctx, goal); err != nil {
		return errors.Wrap(ctx, err, "write goal")
	}

	return nil
}
