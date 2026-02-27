// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/complete-operation.go --fake-name CompleteOperation . CompleteOperation
type CompleteOperation interface {
	Execute(ctx context.Context, vaultPath string, taskName string) error
}

// NewCompleteOperation creates a new complete operation.
func NewCompleteOperation(
	storage storage.Storage,
) CompleteOperation {
	return &completeOperation{
		storage: storage,
	}
}

type completeOperation struct {
	storage storage.Storage
}

// Execute marks a task as complete and updates the associated goal.
func (c *completeOperation) Execute(ctx context.Context, vaultPath string, taskName string) error {
	// Find and read the task
	task, err := c.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	// Update task status to done
	task.Status = domain.TaskStatusDone

	// Write updated task
	if err := c.storage.WriteTask(ctx, task); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	// Update associated goals
	for _, goalName := range task.Goals {
		if err := c.markGoalCheckbox(ctx, vaultPath, goalName, task.Name); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to update goal %s: %v\n", goalName, err)
		}
	}

	// Update today's daily note
	today := time.Now().Format("2006-01-02")
	if err := c.updateDailyNote(ctx, vaultPath, today, task.Name, true); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Warning: failed to update daily note: %v\n", err)
	}

	fmt.Printf("✅ Task completed: %s\n", task.Name)
	return nil
}

// markGoalCheckbox marks the checkbox for a task in the goal file.
func (c *completeOperation) markGoalCheckbox(
	ctx context.Context,
	vaultPath string,
	goalName string,
	taskName string,
) error {
	goal, err := c.storage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return fmt.Errorf("find goal: %w", err)
	}

	// Find checkbox that matches task name
	lines := strings.Split(goal.Content, "\n")
	modified := false

	for i, line := range lines {
		// Match checkbox with task name (case-insensitive)
		if strings.Contains(line, "- [ ]") &&
			strings.Contains(strings.ToLower(line), strings.ToLower(taskName)) {
			lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
			modified = true
			break
		}
	}

	if !modified {
		return fmt.Errorf("checkbox not found for task %s in goal %s", taskName, goalName)
	}

	// Update goal content
	goal.Content = strings.Join(lines, "\n")

	// Write updated goal
	if err := c.storage.WriteGoal(ctx, goal); err != nil {
		return fmt.Errorf("write goal: %w", err)
	}

	return nil
}

// updateDailyNote updates the daily note to mark the task as complete.
func (c *completeOperation) updateDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	taskName string,
	checked bool,
) error {
	content, err := c.storage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return fmt.Errorf("read daily note: %w", err)
	}

	if content == "" {
		return nil // No daily note exists, skip
	}

	// Find and update checkbox for this task
	lines := strings.Split(content, "\n")
	modified := false

	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x])\] (.+)$`)
	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 { //nolint:nestif
			taskText := matches[3]
			if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
				if checked {
					lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
				} else {
					lines[i] = strings.Replace(line, "- [x]", "- [ ]", 1)
				}
				modified = true
				break
			}
		}
	}

	if !modified {
		return nil // Task not found in daily note, that's ok
	}

	// Write updated daily note
	updatedContent := strings.Join(lines, "\n")
	if err := c.storage.WriteDailyNote(ctx, vaultPath, date, updatedContent); err != nil {
		return fmt.Errorf("write daily note: %w", err)
	}

	return nil
}
