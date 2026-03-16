// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
		isInteractive bool,
	) error
}

// NewWorkOnOperation creates a new work-on operation.
func NewWorkOnOperation(
	taskStorage storage.TaskStorage,
	dailyNoteStorage storage.DailyNoteStorage,
	currentDateTime libtime.CurrentDateTime,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) WorkOnOperation {
	return &workOnOperation{
		taskStorage:      taskStorage,
		dailyNoteStorage: dailyNoteStorage,
		currentDateTime:  currentDateTime,
		starter:          starter,
		resumer:          resumer,
	}
}

type workOnOperation struct {
	taskStorage      storage.TaskStorage
	dailyNoteStorage storage.DailyNoteStorage
	currentDateTime  libtime.CurrentDateTime
	starter          ClaudeSessionStarter
	resumer          ClaudeResumer
}

// Execute marks a task as in_progress, assigns it, and starts or resumes a Claude session.
func (w *workOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	assignee string,
	vaultName string,
	outputFormat string,
	isInteractive bool,
) error {
	var warnings []string

	task, err := w.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		if outputFormat == "json" {
			result := MutationResult{Success: false, Error: err.Error()}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "find task")
	}

	task.Status = domain.TaskStatusInProgress
	task.Assignee = assignee

	if err := w.taskStorage.WriteTask(ctx, task); err != nil {
		if outputFormat == "json" {
			result := MutationResult{Success: false, Error: err.Error()}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		}
		return errors.Wrap(ctx, err, "write task")
	}

	today := w.currentDateTime.Now().Format("2006-01-02")
	if err := w.updateDailyNote(ctx, vaultPath, today, task.Name); err != nil {
		warning := fmt.Sprintf("failed to update daily note: %v", err)
		warnings = append(warnings, warning)
		slog.Warn("workon warning", "warning", warning)
	}

	sessionID, sessionErr := w.handleClaudeSession(ctx, task, vaultPath)
	if sessionErr != nil {
		warning := fmt.Sprintf("claude session: %v", sessionErr)
		warnings = append(warnings, warning)
		slog.Warn("workon warning", "warning", warning)
	}

	if isInteractive && w.resumer != nil && sessionID != "" {
		return w.resumer.ResumeSession(sessionID, vaultPath)
	}

	if outputFormat == "json" {
		result := MutationResult{
			Success:   true,
			Name:      task.Name,
			Vault:     vaultName,
			Warnings:  warnings,
			SessionID: sessionID,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("✅ Now working on: %s (assigned to %s)\n", task.Name, assignee)
	if sessionID != "" {
		fmt.Printf("session_id: %s\n", sessionID)
	}
	return nil
}

// handleClaudeSession starts or returns an existing Claude session for the task.
func (w *workOnOperation) handleClaudeSession(
	ctx context.Context,
	task *domain.Task,
	vaultPath string,
) (string, error) {
	if w.starter == nil {
		return task.ClaudeSessionID, nil
	}
	if task.ClaudeSessionID != "" {
		return task.ClaudeSessionID, nil
	}
	prompt := fmt.Sprintf(`/work-on-task "%s"`, task.FilePath)
	slog.Info("starting claude session", "task", task.Name)
	sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath)
	if err != nil {
		return "", errors.Wrap(ctx, err, "start claude session")
	}
	task.ClaudeSessionID = sessionID
	if err := w.taskStorage.WriteTask(ctx, task); err != nil {
		return sessionID, errors.Wrap(ctx, err, "save session id to task")
	}
	return sessionID, nil
}

// updateDailyNote updates the daily note to mark the task as in-progress.
func (w *workOnOperation) updateDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	taskName string,
) error {
	content, err := w.dailyNoteStorage.ReadDailyNote(ctx, vaultPath, date)
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
	if err := w.dailyNoteStorage.WriteDailyNote(ctx, vaultPath, date, updatedContent); err != nil {
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
