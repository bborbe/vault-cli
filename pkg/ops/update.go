// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/update-operation.go --fake-name UpdateOperation . UpdateOperation
type UpdateOperation interface {
	Execute(ctx context.Context, vaultPath string, taskName string) error
}

// NewUpdateOperation creates a new update operation.
func NewUpdateOperation(
	storage storage.Storage,
) UpdateOperation {
	return &updateOperation{
		storage: storage,
	}
}

type updateOperation struct {
	storage storage.Storage
}

// Execute syncs checkbox progress from the task content.
func (u *updateOperation) Execute(ctx context.Context, vaultPath string, taskName string) error {
	// Find and read the task
	task, err := u.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	// Parse checkboxes from task content
	checkboxes := u.parseCheckboxes(task.Content)
	if len(checkboxes) == 0 {
		fmt.Printf("No checkboxes found in task: %s\n", task.Name)
		return nil
	}

	// Count completed vs total
	completed := 0
	total := len(checkboxes)
	for _, cb := range checkboxes {
		if cb.Checked {
			completed++
		}
	}

	// Update task status based on progress
	if completed == total {
		task.Status = domain.TaskStatusDone
	} else if completed > 0 {
		task.Status = domain.TaskStatusInProgress
	} else {
		task.Status = domain.TaskStatusTodo
	}

	// Write updated task
	if err := u.storage.WriteTask(ctx, task); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	// Update associated goals
	for _, goalName := range task.Goals {
		if err := u.syncGoalCheckboxes(ctx, vaultPath, goalName, checkboxes); err != nil {
			fmt.Printf("Warning: failed to sync goal %s: %v\n", goalName, err)
		}
	}

	fmt.Printf(
		"✅ Task updated: %s (%d/%d completed, status: %s)\n",
		task.Name,
		completed,
		total,
		task.Status,
	)
	return nil
}

// parseCheckboxes extracts checkbox items from markdown content.
func (u *updateOperation) parseCheckboxes(content string) []domain.CheckboxItem {
	var items []domain.CheckboxItem
	lines := strings.Split(content, "\n")
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x])\] (.+)$`)

	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			items = append(items, domain.CheckboxItem{
				Line:    i,
				Checked: matches[2] == "x",
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
	goal, err := u.storage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return fmt.Errorf("find goal: %w", err)
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
	if err := u.storage.WriteGoal(ctx, goal); err != nil {
		return fmt.Errorf("write goal: %w", err)
	}

	return nil
}
