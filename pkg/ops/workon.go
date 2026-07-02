// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/config"
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
		isInteractive bool,
		sessionDir string,
		vault *config.Vault,
	) (MutationResult, error)
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

// Execute marks a task as in_progress, advances phase to planning when entering the
// workflow (current phase nil/empty/"todo"), assigns it, and starts or resumes a Claude session.
// A mid-flight phase (in_progress, ai_review, human_review, done, ...) is preserved.
func (w *workOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	assignee string,
	vaultName string,
	isInteractive bool,
	sessionDir string,
	vault *config.Vault,
) (MutationResult, error) {
	var warnings []string

	task, err := w.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"find task",
			)
	}

	_ = task.SetStatus(domain.TaskStatusInProgress)

	if w := applyAssigneeMatrix(task, assignee); w != "" {
		warnings = append(warnings, w)
	}

	advancePhaseIfEntering(task)

	if err := w.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"write task",
			)
	}

	today := w.currentDateTime.Now().Format("2006-01-02")
	if err := w.updateDailyNote(ctx, vaultPath, today, task.Name); err != nil {
		warning := fmt.Sprintf("failed to update daily note: %v", err)
		warnings = append(warnings, warning)
		slog.Warn("workon warning", "warning", warning)
	}

	sessionID, sessionErr := w.handleClaudeSession(ctx, task, sessionDir, vault)
	if sessionErr != nil {
		if errors.Is(sessionErr, ErrStarterUnavailable) {
			// Soft failure — claude binary missing. Spec 014 Failure Modes table:
			// "Unchanged". Keep as warning, continue, CLI exits 0.
			warning := fmt.Sprintf("claude session: %v", sessionErr)
			warnings = append(warnings, warning)
			slog.Warn("workon warning", "warning", warning)
		} else {
			slog.Warn("workon session error", "error", sessionErr)
			return MutationResult{Success: false, Name: task.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()},
				errors.Wrap(ctx, sessionErr, "start work-on session")
		}
	}

	if isInteractive && w.resumer != nil && sessionID != "" {
		return MutationResult{
			Success:   true,
			Name:      task.Name,
			Vault:     vaultName,
			Warnings:  warnings,
			SessionID: sessionID,
		}, w.resumer.ResumeSession(ctx, sessionID, sessionDir)
	}

	return MutationResult{
		Success:   true,
		Name:      task.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}, nil
}

// advancePhaseIfEntering moves a task into the planning phase only when entering
// the workflow (current phase nil or "todo"). Resuming a mid-flight task
// (in_progress, ai_review, human_review, done, ...) must not reset progress backward.
func advancePhaseIfEntering(task *domain.Task) {
	if currentPhase := task.Phase(); currentPhase == nil || *currentPhase == domain.TaskPhaseTodo {
		task.SetPhase(domain.TaskPhasePlanning.Ptr())
	}
}

// applyAssigneeMatrix updates the task's assignee per the blank/equal/different rule
// so `task work-on` never silently overrides a teammate's assignment.
//
// Returns a warning string when the task already belongs to a different non-blank
// user (and the assignee is left unchanged); returns "" for the blank and
// already-self-assigned cases.
func applyAssigneeMatrix(task *domain.Task, assignee string) string {
	switch existing := task.Assignee(); existing {
	case "":
		task.SetAssignee(assignee)
		return ""
	case assignee:
		return ""
	default:
		return fmt.Sprintf(
			"assignee not updated: task owned by %s (current user: %s)",
			existing,
			assignee,
		)
	}
}

// handleClaudeSession starts or returns an existing Claude session for the task.
func (w *workOnOperation) handleClaudeSession(
	ctx context.Context,
	task *domain.Task,
	vaultPath string,
	vault *config.Vault,
) (string, error) {
	if existing := task.ClaudeSessionID(); existing != "" {
		return existing, nil
	}
	if w.starter == nil {
		return "", ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
	slog.Info("starting claude session", "task", task.Name)
	sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath, task.Name)
	if err != nil {
		return "", errors.Wrap(ctx, err, "start claude session")
	}
	task.SetClaudeSessionID(sessionID)
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
	for i, line := range lines {
		if matches := storage.CheckboxRegex.FindStringSubmatch(line); len(
			matches,
		) == 4 { //nolint:nestif
			taskText := matches[3]
			if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
				found = true
				state := matches[2]
				// Only update if currently [ ] (pending)
				if state == " " {
					marker := matches[1]
					lines[i] = strings.Replace(line, marker+" [ ]", marker+" [/]", 1)
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
