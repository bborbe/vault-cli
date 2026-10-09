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
//
// starter/resumer are the vault-default pair (the vault's configured
// claude_script). launcherFactory builds the equivalent pair for a task that
// resolves to a different launcher; it is only consulted when the resolved
// launcher differs from the vault default.
func NewWorkOnOperation(
	taskStorage storage.TaskStorage,
	dailyNoteStorage storage.DailyNoteStorage,
	goalStorage storage.GoalStorage,
	currentDateTime libtime.CurrentDateTime,
	uuidGenerator func() string,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
	launcherFactory LauncherFactory,
) WorkOnOperation {
	return &workOnOperation{
		taskStorage:      taskStorage,
		dailyNoteStorage: dailyNoteStorage,
		goalStorage:      goalStorage,
		currentDateTime:  currentDateTime,
		uuidGenerator:    uuidGenerator,
		starter:          starter,
		resumer:          resumer,
		launcherFactory:  launcherFactory,
	}
}

type workOnOperation struct {
	taskStorage      storage.TaskStorage
	dailyNoteStorage storage.DailyNoteStorage
	goalStorage      storage.GoalStorage
	currentDateTime  libtime.CurrentDateTime
	uuidGenerator    func() string
	starter          ClaudeSessionStarter
	resumer          ClaudeResumer
	launcherFactory  LauncherFactory
}

// Execute marks a task as in_progress, assigns it, and starts or resumes a Claude
// session. A task that enters the workflow (no phase at all) is advanced to planning;
// a task still waiting in the approval inbox (phase "todo") is refused and nothing is
// written — leaving the inbox is the operator's approval and is performed by
// `vault-cli task approve`. A mid-flight phase (planning, execution, in_progress,
// ai_review, human_review, done, ...) is preserved.
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

	starter, resumer, launcherWarnings, err := w.resolveSessionTargets(
		ctx, vaultPath, vault, task, isInteractive,
	)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}
	warnings = append(warnings, launcherWarnings...)

	if err := advancePhaseIfEntering(ctx, task, taskName); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	_ = task.SetStatus(domain.TaskStatusInProgress)

	if w := applyAssigneeMatrix(task, assignee); w != "" {
		warnings = append(warnings, w)
	}

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

	sessionID, sessionErr := w.handleClaudeSession(
		ctx, task, vaultPath, sessionDir, vault, isInteractive, starter,
	)
	if sessionErr != nil {
		if errors.Is(sessionErr, ErrStarterUnavailable) {
			warnings = appendSessionWarning(warnings, sessionErr)
		} else {
			return sessionFailureResult(task, vaultName, warnings, sessionID, sessionErr),
				errors.Wrap(ctx, sessionErr, "start work-on session")
		}
	}

	if isInteractive && resumer != nil && sessionID != "" {
		return w.resumeInteractive(ctx, vault, task, vaultName, sessionDir, sessionID, warnings, resumer)
	}

	return successResult(task, vaultName, warnings, sessionID), nil
}

// resolveSessionTargets resolves the task's launcher and returns the
// starter/resumer pair to use, plus any non-fatal resolution warnings.
//
// It runs before any write: a goal conflict or a rejected value must leave the
// task file and the daily note untouched. An explicitly chosen launcher that
// cannot be found is likewise a hard error raised here, never a silent fall back
// to the vault default.
func (w *workOnOperation) resolveSessionTargets(
	ctx context.Context,
	vaultPath string,
	vault *config.Vault,
	task *domain.Task,
	isInteractive bool,
) (ClaudeSessionStarter, ClaudeResumer, []string, error) {
	vaultScript := vault.GetClaudeScript()
	resolvedLauncher, warnings, err := ResolveTaskLauncher(
		ctx, w.goalStorage, vaultPath, vaultScript, task,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	starter, resumer, err := w.sessionPair(ctx, resolvedLauncher, vaultScript, task, isInteractive)
	if err != nil {
		return nil, nil, warnings, err
	}
	return starter, resumer, warnings, nil
}

// resumeInteractive runs the interactive turn-2 resume and returns its result.
//
// Turn 1 ran headless with --non-interactive. Since v0.109.0 that turn
// auto-chains plan-task -> execute-task under a NO-ASK contract, so it can end
// anywhere from phase: planning (a gate needed an answer it could not ask for) to
// phase: execution. Turn 2 is interactive, so re-invoke the same command WITHOUT
// the flag: it resumes the chain from whatever phase turn 1 left on disk and can
// ask the questions turn 1 had to skip.
func (w *workOnOperation) resumeInteractive(
	ctx context.Context,
	vault *config.Vault,
	task *domain.Task,
	vaultName string,
	sessionDir string,
	sessionID string,
	warnings []string,
	resumer ClaudeResumer,
) (MutationResult, error) {
	continuation := fmt.Sprintf(`%s "%s"`, vault.GetWorkOnCommand(), task.FilePath)
	return MutationResult{
		Success:   true,
		Name:      task.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}, resumer.ResumeSession(ctx, sessionID, sessionDir, continuation)
}

// successResult builds the success MutationResult for a completed work-on.
func successResult(
	task *domain.Task,
	vaultName string,
	warnings []string,
	sessionID string,
) MutationResult {
	return MutationResult{
		Success:   true,
		Name:      task.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}
}

// sessionPair returns the starter/resumer pair to use for the resolved launcher.
// The vault-default pair is used unchanged when the resolved launcher is the
// vault's own claude_script. For an explicitly chosen launcher the pair is built
// through the factory, and a missing member is a hard error naming the launcher —
// an operator who named a launcher must not silently get the vault default.
//
// A nil starter is only fatal on the fresh-start path (the cached-session path
// spawns nothing); a nil resumer is only fatal when the session will be resumed
// interactively. Both checks run before any write, so a rejected launcher leaves
// the task file and the daily note untouched.
func (w *workOnOperation) sessionPair(
	ctx context.Context,
	resolvedLauncher string,
	vaultScript string,
	task *domain.Task,
	isInteractive bool,
) (ClaudeSessionStarter, ClaudeResumer, error) {
	if resolvedLauncher == vaultScript {
		return w.starter, w.resumer, nil
	}
	var starter ClaudeSessionStarter
	var resumer ClaudeResumer
	if w.launcherFactory != nil {
		starter, resumer = w.launcherFactory(resolvedLauncher)
	}
	if task.ClaudeSessionID() == "" && starter == nil {
		return nil, nil, errors.Errorf(ctx, "launcher %q not found", resolvedLauncher)
	}
	if isInteractive && resumer == nil {
		return nil, nil, errors.Errorf(ctx, "launcher %q not found", resolvedLauncher)
	}
	return starter, resumer, nil
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
	// Deliberately not logged here: sessionErr is returned to the caller and printed
	// by the CLI. Logging it again sent a second copy — with a full bborbe/errors stack
	// — to stderr, and vault-ui concatenates stderr into its banner, burying the child's
	// own reason under ~40 lines of trace. See spec 045 SC5.
	return MutationResult{Success: false, Name: task.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()}
}

// advancePhaseIfEntering moves a task into the planning phase when it enters the
// workflow, and refuses a task that is still waiting in the approval inbox.
//
// Entering means the task carries no phase at all (a missing "phase" key or an
// empty one): such a row predates the phase lifecycle or was filed before the
// field was written, and it is advanced to planning exactly as it always has been.
//
// A row at "todo" is the operator's approval inbox. Leaving it is the operator's
// approval and is performed by `vault-cli task approve`, which records approved_by
// and approved_at in the same write. Advancing it here would produce a planning row
// with no approval record — the state the approval gate exists to prevent — so this
// refuses and the caller writes nothing.
//
// Resuming a mid-flight task (planning, execution, in_progress, ai_review,
// human_review, done, ...) must not reset progress backward.
func advancePhaseIfEntering(ctx context.Context, task *domain.Task, taskName string) error {
	currentPhase := task.Phase()
	if currentPhase != nil && *currentPhase == domain.TaskPhaseTodo {
		return errors.Errorf(
			ctx,
			"refusing to work on %q: task is at phase %q, not yet approved; run `vault-cli task approve %q` first",
			taskName,
			string(domain.TaskPhaseTodo),
			taskName,
		)
	}
	if currentPhase == nil {
		task.SetPhase(domain.TaskPhasePlanning.Ptr())
	}
	return nil
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
// and one metrics_sessions entry in a single write. The re-read is load-bearing on
// every branch because the task file is a shared, concurrently-written vault file (the
// headless turn mutates it too): writing the stale in-memory copy would revert those
// changes. Used pre-spawn on the fresh-start path (the session id is new and must be
// on disk before the child exists) and on the cached-session path (the id already
// exists and is preserved).
//
// On failure it returns an empty id, never the one it was handed. The id is the Vault
// UI's signal that Resume will work; reporting an id whose write did not land would
// advertise a session that is not on disk.
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
		return "", errors.Wrap(ctx, err, "re-read task after claude session")
	}
	if refreshed.ClaudeSessionID() == "" {
		refreshed.SetClaudeSessionID(sessionID)
	}
	refreshed.AppendMetricsSession(domain.MetricsSession{
		SessionID: sessionID,
		StartedAt: startedAt,
	})
	if err := taskStorage.WriteTask(ctx, refreshed); err != nil {
		return "", errors.Wrap(ctx, err, "save session id to task")
	}
	return sessionID, nil
}

// clearSessionAndMetrics re-reads the task after a spawn failure and clears only the
// claude_session_id and the metrics_sessions entry for this run, preserving any
// frontmatter the child wrote before failing. The re-read is load-bearing: clearing
// from the stale in-memory copy would revert the child's writes.
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

// handleClaudeSession starts or returns an existing Claude session for the task.
// On the fresh-start path the session id and its metrics entry are persisted BEFORE
// the child is spawned, so the child's own session-connect reads the field already set
// and keeps the fresh session; a spawn failure triggers a re-read-based compensating
// clear that removes the id and this run's metrics entry, preserving the invariant that
// an id on disk means a resumable session. The cached-session path is unchanged: the
// existing id is re-read and re-persisted with a metrics entry.
func (w *workOnOperation) handleClaudeSession(
	ctx context.Context,
	task *domain.Task,
	vaultPath string,
	sessionDir string,
	vault *config.Vault,
	isInteractive bool,
	starter ClaudeSessionStarter,
) (string, error) {
	if existing := task.ClaudeSessionID(); existing != "" {
		startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
		sessionID, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, existing, startedAt, w.taskStorage)
		return sessionID, err
	}
	if starter == nil {
		return "", ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnCommand(), task.FilePath)
	sessionID := w.uuidGenerator()
	slog.Info("starting claude session", "task", task.Name)
	// Persist id + metrics BEFORE the child exists, on both branches, so the child's own
	// /vault-cli:work-on-task session-connect reads claude_session_id already set and its
	// "already connected, do NOT overwrite" step fires instead of the transcript mtime
	// scan. persistSessionAndMetrics re-reads before writing, so this never reverts
	// concurrent edits to the same vault file.
	startedAt := libtime.DateOrDateTime(w.currentDateTime.Now().Time())
	if _, err := persistSessionAndMetrics(ctx, vaultPath, task.Name, sessionID, startedAt, w.taskStorage); err != nil {
		return "", errors.Wrap(ctx, err, "persist claude session before spawn")
	}
	if err := starter.StartSession(ctx, sessionID, prompt, sessionDir, task.Name, isInteractive); err != nil {
		// Compensating clear: a failed turn must not leave a resumable-looking id on
		// disk. Re-read and clear only the id and this run's metrics entry, preserving
		// any frontmatter the child wrote before failing.
		if clearErr := w.clearSessionAndMetrics(ctx, vaultPath, task.Name, sessionID); clearErr != nil {
			slog.Warn("workon warning", "warning", fmt.Sprintf("failed to clear claude session id after spawn failure: %v", clearErr))
		}
		return "", errors.Wrap(ctx, err, "start claude session")
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
