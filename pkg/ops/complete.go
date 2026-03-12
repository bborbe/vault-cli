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
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/complete-operation.go --fake-name CompleteOperation . CompleteOperation
type CompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		outputFormat string,
	) error
}

// NewCompleteOperation creates a new complete operation.
func NewCompleteOperation(
	storage storage.Storage,
	currentDateTime libtime.CurrentDateTime,
) CompleteOperation {
	return &completeOperation{
		storage:         storage,
		currentDateTime: currentDateTime,
	}
}

type completeOperation struct {
	storage         storage.Storage
	currentDateTime libtime.CurrentDateTime
}

// MutationResult represents the result of a mutation operation.
type MutationResult struct {
	Success   bool     `json:"success"`
	Name      string   `json:"name,omitempty"`
	Vault     string   `json:"vault,omitempty"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

// IncompleteResult represents the result when a task has incomplete subtasks.
type IncompleteResult struct {
	Success    bool   `json:"success"`
	Reason     string `json:"reason"`
	Pending    int    `json:"pending"`
	InProgress int    `json:"inprogress"`
	Completed  int    `json:"completed"`
	Total      int    `json:"total"`
}

// Execute marks a task as complete and updates the associated goal.
func (c *completeOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	outputFormat string,
) error {
	var warnings []string

	// Find and read the task
	task, err := c.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		if outputFormat == "json" {
			result := MutationResult{
				Success: false,
				Error:   err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "find task")
	}

	// Handle recurring tasks differently
	if task.Recurring != "" {
		return c.handleRecurringTask(ctx, task, vaultPath, vaultName, outputFormat, warnings)
	}

	// Check subtask completion for non-recurring tasks
	if shouldBlock, err := c.checkSubtaskCompletion(task, outputFormat); shouldBlock {
		return err
	}

	// Update task status to completed
	task.Status = domain.TaskStatusCompleted

	// Write updated task
	if err := c.storage.WriteTask(ctx, task); err != nil {
		if outputFormat == "json" {
			result := MutationResult{
				Success: false,
				Error:   err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "write task")
	}

	// Update associated goals
	for _, goalName := range task.Goals {
		if err := c.markGoalCheckbox(ctx, vaultPath, goalName, task.Name); err != nil {
			warning := fmt.Sprintf("failed to update goal %s: %v", goalName, err)
			warnings = append(warnings, warning)
			if outputFormat == "plain" {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
			}
		}
	}

	// Update today's daily note
	today := c.currentDateTime.Now().Format("2006-01-02")
	if err := c.updateDailyNote(ctx, vaultPath, today, task.Name, true); err != nil {
		warning := fmt.Sprintf("failed to update daily note: %v", err)
		warnings = append(warnings, warning)
		if outputFormat == "plain" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
	}

	if outputFormat == "json" {
		result := MutationResult{
			Success:  true,
			Name:     task.Name,
			Vault:    vaultName,
			Warnings: warnings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("✅ Task completed: %s\n", task.Name)
	return nil
}

// checkSubtaskCompletion checks if all subtasks are complete.
// Returns (true, error) if task should not be completed (json mode with incomplete items).
// Returns (false, nil) if task can proceed to completion.
func (c *completeOperation) checkSubtaskCompletion(
	task *domain.Task,
	outputFormat string,
) (bool, error) {
	completed, inProgress, pending := countCheckboxStates(task.Content)
	total := completed + inProgress + pending

	// If no checkboxes or all complete, proceed normally
	if total == 0 || (pending == 0 && inProgress == 0) {
		return false, nil
	}

	// JSON mode: return incomplete status without completing
	if outputFormat == "json" {
		result := IncompleteResult{
			Success:    false,
			Reason:     "incomplete_items",
			Pending:    pending,
			InProgress: inProgress,
			Completed:  completed,
			Total:      total,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return true, enc.Encode(result)
	}

	// Plain mode: warn but continue
	fmt.Fprintf(
		os.Stderr,
		"⚠️  Warning: %d/%d subtasks incomplete (%d pending, %d in-progress). Completing anyway.\n",
		pending+inProgress,
		total,
		pending,
		inProgress,
	)
	return false, nil
}

// RecurringMutationResult represents the result of a recurring task mutation.
type RecurringMutationResult struct {
	Success   bool     `json:"success"`
	Name      string   `json:"name,omitempty"`
	Vault     string   `json:"vault,omitempty"`
	Recurring bool     `json:"recurring"`
	NextDate  string   `json:"next_date,omitempty"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// handleRecurringTask handles completion of a recurring task.
func (c *completeOperation) handleRecurringTask(
	ctx context.Context,
	task *domain.Task,
	vaultPath string,
	vaultName string,
	outputFormat string,
	warnings []string,
) error {
	now := c.currentDateTime.Now().Time()
	today := now.Format("2006-01-02")

	// 1. Reset all checkboxes in content
	task.Content = resetCheckboxes(task.Content)

	// 2. Set last_completed to today
	task.LastCompleted = today

	// 3. Bump defer_date based on recurring interval
	newDeferDate := calculateNextDeferDate(task.Recurring, now)
	task.DeferDate = newDeferDate.Ptr()

	// 4. If planned_date exists and < new defer_date, clear it
	if task.PlannedDate != nil && (*task.PlannedDate).Before(newDeferDate) {
		task.PlannedDate = nil
	}

	// 5. Status remains as-is (do NOT set to completed)

	// Write updated task
	if err := c.storage.WriteTask(ctx, task); err != nil {
		if outputFormat == "json" {
			result := RecurringMutationResult{
				Success:   false,
				Vault:     vaultName,
				Recurring: true,
				Error:     err.Error(),
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "write recurring task")
	}

	// Update today's daily note (mark checkbox as checked for completion)
	if err := c.updateDailyNote(ctx, vaultPath, today, task.Name, true); err != nil {
		warning := fmt.Sprintf("failed to update daily note: %v", err)
		warnings = append(warnings, warning)
		if outputFormat == "plain" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
	}

	nextDateStr := newDeferDate.Format("2006-01-02")

	if outputFormat == "json" {
		result := RecurringMutationResult{
			Success:   true,
			Name:      task.Name,
			Vault:     vaultName,
			Recurring: true,
			NextDate:  nextDateStr,
			Warnings:  warnings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("🔄 Recurring task reset: %s (next: %s)\n", task.Name, nextDateStr)
	return nil
}

// calculateNextDeferDate calculates the next defer date based on recurring interval.
func calculateNextDeferDate(recurring string, now time.Time) libtime.Date {
	// weekdays is a special case: check before ParseRecurringInterval
	if recurring == "weekdays" {
		next := now.AddDate(0, 0, 1) // tomorrow
		switch next.Weekday() {
		case time.Saturday:
			return libtime.ToDate(now.AddDate(0, 0, 3)) // Saturday → Monday
		case time.Sunday:
			return libtime.ToDate(now.AddDate(0, 0, 2)) // Sunday → Monday
		default:
			return libtime.ToDate(next)
		}
	}

	interval, err := domain.ParseRecurringInterval(recurring)
	if err != nil {
		// Unknown recurring type, treat as daily
		fmt.Fprintf(
			os.Stderr,
			"Warning: unknown recurring interval %q, treating as daily\n",
			recurring,
		)
		return libtime.ToDate(now.AddDate(0, 0, 1))
	}
	return libtime.ToDate(interval.AddTo(now))
}

// resetCheckboxes resets all checked checkboxes in content to unchecked.
func resetCheckboxes(content string) string {
	// Replace all "- [x]" with "- [ ]"
	return strings.ReplaceAll(content, "- [x]", "- [ ]")
}

// countCheckboxStates counts checkbox states in content.
func countCheckboxStates(content string) (completed, inProgress, pending int) {
	lines := strings.Split(content, "\n")
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)

	for _, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			state := matches[2]
			switch state {
			case "x":
				completed++
			case "/":
				inProgress++
			case " ":
				pending++
			}
		}
	}

	return completed, inProgress, pending
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
		return errors.Wrap(ctx, err, "find goal")
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
		return errors.Wrap(ctx, err, "write goal")
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
		return errors.Wrap(ctx, err, "read daily note")
	}

	if content == "" {
		return nil // No daily note exists, skip
	}

	// Find and update checkbox for this task
	lines := strings.Split(content, "\n")
	modified := false

	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)
	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 { //nolint:nestif
			taskText := matches[3]
			if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
				if checked {
					// Replace any checkbox state with [x]
					lines[i] = regexp.MustCompile(`- \[([ /])\]`).ReplaceAllString(line, "- [x]")
				} else {
					// Replace [x] with [ ]
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
		return errors.Wrap(ctx, err, "write daily note")
	}

	return nil
}
