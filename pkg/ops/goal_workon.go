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
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) GoalWorkOnOperation {
	return &goalWorkOnOperation{
		goalStorage: goalStorage,
		starter:     starter,
		resumer:     resumer,
	}
}

type goalWorkOnOperation struct {
	goalStorage storage.GoalStorage
	starter     ClaudeSessionStarter
	resumer     ClaudeResumer
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

	sessionID, sessionErr := g.handleClaudeSession(ctx, goal, vaultPath, sessionDir, vault)
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
// Used after StartSession blocks so that frontmatter the headless turn wrote is not
// reverted by writing the stale in-memory copy.
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
// On a fresh start it re-reads the goal from disk after the session returns, so
// frontmatter the session itself wrote during the blocking turn survives.
func (g *goalWorkOnOperation) handleClaudeSession(
	ctx context.Context,
	goal *domain.Goal,
	vaultPath string,
	sessionDir string,
	vault *config.Vault,
) (string, error) {
	if existing := goal.ClaudeSessionID(); existing != "" {
		return existing, nil
	}
	if g.starter == nil {
		return "", ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnGoalCommand(), goal.FilePath)
	slog.Info("starting claude session", "goal", goal.Name)
	sessionID, err := g.starter.StartSession(ctx, prompt, sessionDir, goal.Name)
	if err != nil {
		return "", errors.Wrap(ctx, err, "start claude session")
	}
	return persistGoalSessionID(ctx, vaultPath, goal.Name, sessionID, g.goalStorage)
}
