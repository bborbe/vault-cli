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
	uuidGenerator func() string,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) WorkOnOperation {
	return &workOnOperation{
		taskStorage:      taskStorage,
		dailyNoteStorage: dailyNoteStorage,
		currentDateTime:  currentDateTime,
		uuidGenerator:    uuidGenerator,
		starter:          starter,
		resumer:          resumer,
	}
}

type workOnOperation struct {
	taskStorage      storage.TaskStorage
	dailyNoteStorage storage.DailyNoteStorage
	currentDateTime  libtime.CurrentDateTime
	uuidGenerator    func() string
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

	sessionID, sessionWarnings, sessionErr := w.handleClaudeSession(ctx, task, vaultPath, sessionDir, vault, isInteractive)
	warnings = append(warnings, sessionWarnings...)
	if sessionErr != nil {
		if errors.Is(sessionErr, ErrStarterUnavailable) {
			warnings = appendSessionWarning(warnings, sessionErr)
		} else {
			return sessionFailureResult(task, vaultName, warnings, sessionID, sessionErr),
				errors.Wrap(ctx, sessionErr, "start work-on session")
		}
	}

	if isInteractive && w.resumer != nil && sessionID != "" {
		// Turn 1 ran headless with --non-interactive. Since v0.109.0 that turn
		// auto-chains plan-task -> execute-task under a NO-ASK contract, so it can
		// end anywhere from phase: planning (a gate needed an answer it could not
		// ask for) to phase: execution. Turn 2 is interactive, so re-invoke the same
		// command WITHOUT the flag: it resumes the chain from whatever phase turn 1
		// left on disk and can ask the questions turn 1 had to skip.
		continuation := fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)
		return MutationResult{
			Success:   true,
			Name:      task.Name,
			Vault:     vaultName,
			Warnings:  warnings,
			SessionID: sessionID,
		}, w.resumer.ResumeSession(ctx, sessionID, sessionDir, continuation)
	}

	return MutationResult{
		Success:   true,
		Name:      task.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}, nil
}

// appendSessionWarning records a non-fatal session-start warning (claude binary
// missing). Spec 014 Failure Modes table: "Unchanged". Keep as warning, continue,
// CLI exits 0.
func appendSessionWarning(warnings []string, sessionErr error) []string {
	warning := fmt.Sprintf("claude session: %v", sessionErr)
	warnings = append(warnings, warning)
	slog.Warn("workon warning", "warning", warning)
	return warnings
}

// sessionFailureResult builds the hard-failure MutationResult for a session-start
// error: Success=false, the accumulated warnings (including any compensating-clear
// warning), and the spawn error message. The caller wraps the returned error.
func sessionFailureResult(
	task *domain.Task,
	vaultName string,
	warnings []string,
	sessionID string,
	sessionErr error,
) MutationResult {
	slog.Warn("workon session error", "error", sessionErr)
	return MutationResult{Success: false, Name: task.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()}
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

// persistSessionAndMetrics re-reads the task from disk and writes back the session id
// and one metrics_sessions entry in a single write. The re-read is load-bearing on the
// interactive branch and the cached-session path: the headless turn may mutate the
// file before the post-return persist, so writing the stale in-memory copy would
// revert the session's own frontmatter changes. On the non-interactive branch the
// persist runs before the child exists, so the session's own read-modify-write reads a
// file that already contains the id. Used on both the fresh-start path (the session id
// is new) and the cached-session path (the id already exists and is preserved).
func persistSessionAndMetrics(
	ctx context.Context,
	vaultPath string,
	taskName string,
	sessionID string,
	startedAt libtime.DateOrDateTime,
	taskStorage storage.TaskStorage,
) (string, error) {
	refreshed, err := taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return sessionID, errors.Wrap(ctx, err, "re-read task after claude session")
	}
	if refreshed.ClaudeSessionID() == "" {
		refreshed.SetClaudeSessionID(sessionID)
	}
	refreshed.AppendMetricsSession(domain.MetricsSession{
		SessionID: sessionID,
		StartedAt: startedAt,
	})
	if err := taskStorage.WriteTask(ctx, refreshed); err != nil {
		return sessionID, errors.Wrap(ctx, err, "save session id to task")
	}
	return sessionID, nil
}

// handleClaudeSession starts or returns an existing Claude session for the task.
// On the non-interactive branch the session id and its metrics entry are persisted
// BEFORE the child is spawned, so the session's own read-modify-write always reads a
// file that already contains the id. A spawn failure inside the liveness window
// triggers a compensating re-read-based clear that removes the id and this run's
// metrics entry while preserving any frontmatter the child wrote before dying; a
// failed clear is surfaced as a warning rather than masking the spawn error.
func (w *workOnOperation) handleClaudeSession(
	ctx context.Context,
	task *domain.Task,
	vaultPath string,
	sessionDir string,
	vault *config.Vault,
	isInteractive bool,
) (string, []string, error) {
	if existing := task.ClaudeSessionID(); existing != "" {
		startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
		sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, existing, startedAt, w.taskStorage)
		return sessionID, nil, err
	}
	if w.starter == nil {
		return "", nil, ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
	sessionID := w.uuidGenerator()
	slog.Info("starting claude session", "task", task.Name)
	if isInteractive {
		// TTY branch, unchanged: block through the headless turn, then re-read and
		// persist so frontmatter the session itself wrote survives.
		if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
			return "", nil, errors.Wrap(ctx, err, "start claude session")
		}
		startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
		sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage)
		return sessionID, nil, err
	}
	// Non-interactive branch: persist id + metrics BEFORE the child exists, so the
	// session's own read-modify-write always reads a file that already contains it.
	startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
	if _, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage); err != nil {
		return "", nil, errors.Wrap(ctx, err, "persist claude session before spawn")
	}
	if err := w.starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
		// Compensating clear: the child may have written frontmatter before dying
		// inside the window (e.g. phase: planning). Re-read and clear only the id and
		// this run's metrics entry, preserving every other field on disk.
		if clearErr := w.clearSessionAndMetrics(ctx, vaultPath, task.Name, sessionID); clearErr != nil {
			return "", []string{fmt.Sprintf("failed to clear claude session id after spawn failure: %v", clearErr)},
				errors.Wrap(ctx, err, "start claude session")
		}
		return "", nil, errors.Wrap(ctx, err, "start claude session")
	}
	return sessionID, nil, nil
}

// clearSessionAndMetrics re-reads the task after a spawn failure and clears only
// the claude_session_id and the metrics_sessions entry for this run, preserving
// any frontmatter the child wrote before dying. The re-read is load-bearing:
// clearing from the stale in-memory copy would revert the child's writes.
func (w *workOnOperation) clearSessionAndMetrics(
	ctx context.Context,
	vaultPath string,
	taskName string,
	sessionID string,
) error {
	refreshed, err := w.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "re-read task after spawn failure")
	}
	refreshed.ClearClaudeSessionID()
	var kept []domain.MetricsSession
	for _, m := range refreshed.MetricsSessions() {
		if m.SessionID != sessionID {
			kept = append(kept, m)
		}
	}
	refreshed.Set("metrics_sessions", kept)
	if err := w.taskStorage.WriteTask(ctx, refreshed); err != nil {
		return errors.Wrap(ctx, err, "clear session id after spawn failure")
	}
	return nil
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
			if IsOwnDailyNoteEntry(taskText, taskName) {
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
