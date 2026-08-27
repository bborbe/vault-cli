// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-workon-operation.go --fake-name GoalWorkOnOperation . GoalWorkOnOperation
type GoalWorkOnOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		goalName string,
		assignee string,
		vaultName string,
		isInteractive bool,
		sessionDir string,
		vault *config.Vault,
	) (MutationResult, error)
}

// NewGoalWorkOnOperation creates a new goal work-on operation.
func NewGoalWorkOnOperation(
	goalStorage storage.GoalStorage,
	uuidGenerator func() string,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) GoalWorkOnOperation {
	return &goalWorkOnOperation{
		goalStorage:   goalStorage,
		uuidGenerator: uuidGenerator,
		starter:       starter,
		resumer:       resumer,
	}
}

type goalWorkOnOperation struct {
	goalStorage   storage.GoalStorage
	uuidGenerator func() string
	starter       ClaudeSessionStarter
	resumer       ClaudeResumer
}

// Execute marks a goal as in_progress, assigns it, and starts or resumes a Claude session.
// Unlike task work-on, goals have no daily-note update and no phase advancement.
func (g *goalWorkOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	assignee string,
	vaultName string,
	isInteractive bool,
	sessionDir string,
	vault *config.Vault,
) (MutationResult, error) {
	var warnings []string

	goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"find goal",
		)
	}

	if err := goal.SetStatus(domain.GoalStatusInProgress); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"set goal status",
		)
	}

	if w := applyGoalAssigneeMatrix(goal, assignee); w != "" {
		warnings = append(warnings, w)
	}

	if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"write goal",
		)
	}

	sessionID, sessionWarnings, sessionErr := g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault, isInteractive)
	warnings = append(warnings, sessionWarnings...)
	if sessionErr != nil {
		if errors.Is(sessionErr, ErrStarterUnavailable) {
			// Soft failure — claude binary missing. Spec 014 Failure Modes table:
			// "Unchanged". Keep as warning, continue, CLI exits 0.
			warning := fmt.Sprintf("claude session: %v", sessionErr)
			warnings = append(warnings, warning)
			slog.Warn("workon warning", "warning", warning)
		} else {
			slog.Warn("workon session error", "error", sessionErr)
			return MutationResult{Success: false, Name: goal.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()},
				errors.Wrap(ctx, sessionErr, "start work-on session")
		}
	}

	if isInteractive && g.resumer != nil && sessionID != "" {
		return MutationResult{
			Success:   true,
			Name:      goal.Name,
			Vault:     vaultName,
			Warnings:  warnings,
			SessionID: sessionID,
			// Goal work-on carries the same resumed-turn defect as task work-on, but
			// fixing it is a separate spec (029 § Constraints). Passing "" keeps argv
			// byte-identical to today's `claude --resume <id>`.
		}, g.resumer.ResumeSession(ctx, sessionID, sessionDir, "")
	}

	return MutationResult{
		Success:   true,
		Name:      goal.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}, nil
}

// applyGoalAssigneeMatrix updates the goal's assignee per the blank/equal/different rule
// so `goal work-on` never silently overrides a teammate's assignment.
//
// Returns a warning string when the goal already belongs to a different non-blank
// user (and the assignee is left unchanged); returns "" for the blank and
// already-self-assigned cases.
func applyGoalAssigneeMatrix(goal *domain.Goal, assignee string) string {
	switch existing := goal.Assignee(); existing {
	case "":
		goal.SetAssignee(assignee)
		return ""
	case assignee:
		return ""
	default:
		return fmt.Sprintf(
			"assignee not updated: goal owned by %s (current user: %s)",
			existing,
			assignee,
		)
	}
}

// persistGoalSessionID re-reads the goal from disk and writes back only the session id.
// The re-read is load-bearing: on the interactive branch the StartSession call blocks
// for the entire headless turn and that turn writes to this very goal file, so writing
// the stale in-memory copy would revert the session's own frontmatter changes; on the
// non-interactive branch the persist runs before the child is spawned.
func persistGoalSessionID(
	ctx context.Context,
	vaultPath string,
	goalName string,
	sessionID string,
	goalStorage storage.GoalStorage,
) (string, error) {
	refreshed, err := goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return sessionID, errors.Wrap(ctx, err, "re-read goal after claude session")
	}
	refreshed.SetClaudeSessionID(sessionID)
	if err := goalStorage.WriteGoal(ctx, refreshed); err != nil {
		return sessionID, errors.Wrap(ctx, err, "save session id to goal")
	}
	return sessionID, nil
}

// handleClaudeSession starts or returns an existing Claude session for the goal.
// On the non-interactive branch the session id is persisted BEFORE the child is
// spawned, so the session's own read-modify-write always reads a file that already
// contains the id. A spawn failure inside the liveness window triggers a
// compensating re-read-based clear that removes the id while preserving any
// frontmatter the child wrote before dying; a failed clear is surfaced as a warning
// rather than masking the spawn error. On the interactive branch the id is persisted
// after the blocking turn so frontmatter the session itself wrote survives.
func (g *goalWorkOnOperation) handleClaudeSession(
	ctx context.Context,
	goal *domain.Goal,
	vaultPath string,
	sessionDir string,
	vault *config.Vault,
	isInteractive bool,
) (string, []string, error) {
	if existing := goal.ClaudeSessionID(); existing != "" {
		return existing, nil, nil
	}
	if g.starter == nil {
		return "", nil, ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnGoalCommand(), goal.FilePath)
	sessionID := g.uuidGenerator()
	slog.Info("starting claude session", "goal", goal.Name)
	if isInteractive {
		// TTY branch, unchanged: block through the headless turn, then re-read and
		// persist so frontmatter the session itself wrote survives.
		if err := g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive); err != nil {
			return "", nil, errors.Wrap(ctx, err, "start claude session")
		}
		sessionID, err := persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)
		return sessionID, nil, err
	}
	// Non-interactive branch: persist the id BEFORE the child exists, so the session's
	// own read-modify-write always reads a file that already contains it.
	if _, err := persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage); err != nil {
		return "", nil, errors.Wrap(ctx, err, "persist claude session id before spawn")
	}
	if err := g.starter.StartSession(ctx, sessionID, prompt, sessionDir, goal.Name, isInteractive); err != nil {
		// Compensating clear: the child may have written frontmatter before dying
		// inside the window (e.g. phase: execution). Re-read and clear only the id,
		// preserving every other field on disk.
		if clearErr := g.clearGoalSession(ctx, vaultPath, goal.Name); clearErr != nil {
			return "", []string{fmt.Sprintf("failed to clear claude session id after spawn failure: %v", clearErr)},
				errors.Wrap(ctx, err, "start claude session")
		}
		return "", nil, errors.Wrap(ctx, err, "start claude session")
	}
	return sessionID, nil, nil
}

// clearGoalSession re-reads the goal after a spawn failure and clears only the
// claude_session_id, preserving any frontmatter the child wrote before dying.
// The re-read is load-bearing: clearing from the stale in-memory copy would
// revert the child's writes.
func (g *goalWorkOnOperation) clearGoalSession(
	ctx context.Context,
	vaultPath string,
	goalName string,
) error {
	refreshed, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return errors.Wrap(ctx, err, "re-read goal after spawn failure")
	}
	refreshed.ClearClaudeSessionID()
	if err := g.goalStorage.WriteGoal(ctx, refreshed); err != nil {
		return errors.Wrap(ctx, err, "clear goal session id after spawn failure")
	}
	return nil
}
