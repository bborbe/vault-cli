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
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/workon-operation.go --fake-name WorkOnOperation . WorkOnOperation
type WorkOnOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		assignee string,
		vaultName string,
		outputFormat string,
	) error
}

// NewWorkOnOperation creates a new work-on operation.
func NewWorkOnOperation(
	storage storage.Storage,
	currentDateTime libtime.CurrentDateTime,
) WorkOnOperation {
	return &workOnOperation{
		storage:         storage,
		currentDateTime: currentDateTime,
	}
}

type workOnOperation struct {
	storage         storage.Storage
	currentDateTime libtime.CurrentDateTime
}

// Execute marks a task as in_progress and assigns it to the given user.
func (w *workOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	assignee string,
	vaultName string,
	outputFormat string,
) error {
	var warnings []string

	// Find and read the task
	task, err := w.storage.FindTaskByName(ctx, vaultPath, taskName)
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

	// Update task status to in_progress and set assignee
	task.Status = domain.TaskStatusInProgress
	task.Assignee = assignee

	// Write updated task
	if err := w.storage.WriteTask(ctx, task); err != nil {
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

	// Update today's daily note
	today := w.currentDateTime.Now().Format("2006-01-02")
	if err := w.updateDailyNote(ctx, vaultPath, today, task.Name); err != nil {
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

	fmt.Printf("✅ Now working on: %s (assigned to %s)\n", task.Name, assignee)
	return nil
}

// updateDailyNote updates the daily note to mark the task as in-progress.
func (w *workOnOperation) updateDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	taskName string,
) error {
	content, err := w.storage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return errors.Wrap(ctx, err, "read daily note")
	}

	if content == "" {
		return nil // No daily note exists, skip
	}

	lines := strings.Split(content, "\n")
	found, modified := findAndUpdateTaskCheckbox(lines, taskName)

	if !found {
		lines = appendTaskToDaily(lines, taskName)
		modified = true
	}

	if !modified {
		return nil // Nothing to update
	}

	// Write updated daily note
	updatedContent := strings.Join(lines, "\n")
	if err := w.storage.WriteDailyNote(ctx, vaultPath, date, updatedContent); err != nil {
		return errors.Wrap(ctx, err, "write daily note")
	}

	return nil
}

// findAndUpdateTaskCheckbox searches for a task checkbox and updates it to in-progress if pending.
func findAndUpdateTaskCheckbox(lines []string, taskName string) (found, modified bool) {
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)
	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 { //nolint:nestif
			taskText := matches[3]
			if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
				found = true
				state := matches[2]
				// Only update if currently [ ] (pending)
				if state == " " {
					lines[i] = strings.Replace(line, "- [ ]", "- [/]", 1)
					modified = true
				}
				// If already [/] or [x], skip (already in-progress or completed)
				break
			}
		}
	}
	return found, modified
}

// appendTaskToDaily appends a task to the daily note, preferring the Must section.
func appendTaskToDaily(lines []string, taskName string) []string {
	mustIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "## Must") {
			mustIndex = i
			break
		}
	}

	newLine := fmt.Sprintf("- [/] [[%s]]", taskName)
	if mustIndex >= 0 {
		// Insert after Must header
		return append(
			lines[:mustIndex+1],
			append([]string{newLine}, lines[mustIndex+1:]...)...)
	}
	// Append to end
	return append(lines, newLine)
}
