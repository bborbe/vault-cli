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

//counterfeiter:generate -o ../../mocks/topic-workon-operation.go --fake-name TopicWorkOnOperation . TopicWorkOnOperation
type TopicWorkOnOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		assignee string,
		vaultName string,
		isInteractive bool,
		sessionDir string,
		vault *config.Vault,
	) (MutationResult, error)
}

// NewTopicWorkOnOperation creates a new topic work-on operation.
func NewTopicWorkOnOperation(
	topicStorage storage.TopicStorage,
	uuidGenerator func() string,
	starter ClaudeSessionStarter,
	resumer ClaudeResumer,
) TopicWorkOnOperation {
	return &topicWorkOnOperation{
		topicStorage:  topicStorage,
		uuidGenerator: uuidGenerator,
		starter:       starter,
		resumer:       resumer,
	}
}

type topicWorkOnOperation struct {
	topicStorage  storage.TopicStorage
	uuidGenerator func() string
	starter       ClaudeSessionStarter
	resumer       ClaudeResumer
}

// Execute marks a topic as in_progress, assigns it, and starts or resumes a Claude session.
// Unlike task work-on, topics have no daily-note update and no phase advancement.
func (o *topicWorkOnOperation) Execute(
	ctx context.Context,
	vaultPath string,
	topicName string,
	assignee string,
	vaultName string,
	isInteractive bool,
	sessionDir string,
	vault *config.Vault,
) (MutationResult, error) {
	var warnings []string

	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find topic")
	}

	// The status goes through SetField rather than a validating setter: prompt 1
	// defines only two plain string constants for topics, so there is no enum to
	// validate against and a page carrying a non-canonical prior status stays workable.
	if err := topic.SetField(ctx, "status", domain.TopicStatusInProgress); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "set status")
	}

	if w, err := applyTopicAssigneeMatrix(ctx, topic, assignee); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	} else if w != "" {
		warnings = append(warnings, w)
	}

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write topic")
	}

	sessionID, sessionErr := o.handleClaudeSession(ctx, topic, vaultPath, sessionDir, vault, isInteractive)
	if sessionErr != nil {
		if errors.Is(sessionErr, ErrStarterUnavailable) {
			// Soft failure — claude binary missing. Spec 014 Failure Modes table:
			// "Unchanged". Keep as warning, continue, CLI exits 0.
			warning := fmt.Sprintf("claude session: %v", sessionErr)
			warnings = append(warnings, warning)
			slog.Warn("workon warning", "warning", warning)
		} else {
			// Not logged here — see the note in workon.go sessionFailureResult: the
			// error is returned and printed, and a second stack-carrying copy on stderr
			// buries the child's reason in vault-ui's banner.
			return MutationResult{Success: false, Name: topic.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()},
				errors.Wrap(ctx, sessionErr, "start work-on session")
		}
	}

	if isInteractive && o.resumer != nil && sessionID != "" {
		return MutationResult{
			Success:   true,
			Name:      topic.Name,
			Vault:     vaultName,
			Warnings:  warnings,
			SessionID: sessionID,
			// Topic work-on carries the same resumed-turn defect as goal work-on, but
			// fixing it is a separate spec (029 § Constraints). Passing "" keeps argv
			// byte-identical to today's `claude --resume <id>`.
		}, o.resumer.ResumeSession(ctx, sessionID, sessionDir, "")
	}

	return MutationResult{
		Success:   true,
		Name:      topic.Name,
		Vault:     vaultName,
		Warnings:  warnings,
		SessionID: sessionID,
	}, nil
}

// applyTopicAssigneeMatrix updates the topic's assignee per the blank/equal/different
// rule so `topic work-on` never silently overrides a teammate's assignment.
//
// Returns a warning string when the topic already belongs to a different non-blank
// user (and the assignee is left unchanged); returns "" for the blank and
// already-self-assigned cases. Unlike the goal variant the write goes through
// SetField, which can fail, so the warning is paired with an error return.
func applyTopicAssigneeMatrix(
	ctx context.Context,
	topic *domain.Topic,
	assignee string,
) (string, error) {
	switch existing := topic.GetField("assignee"); existing {
	case "":
		if err := topic.SetField(ctx, "assignee", assignee); err != nil {
			return "", errors.Wrap(ctx, err, "set assignee")
		}
		return "", nil
	case assignee:
		return "", nil
	default:
		return fmt.Sprintf(
			"assignee not updated: topic owned by %s (current user: %s)",
			existing,
			assignee,
		), nil
	}
}

// persistTopicSessionID re-reads the topic from disk and writes back only the session id.
// The re-read is load-bearing on every branch: the headless turn mutates the same file
// and always finishes before this runs, so writing the stale in-memory copy would
// revert the session's own frontmatter changes.
//
// On failure it returns an empty id, never the one it was handed. The id is the Vault
// UI's signal that Resume will work; reporting an id whose write did not land would
// advertise a session that is not on disk.
func persistTopicSessionID(
	ctx context.Context,
	vaultPath string,
	topicName string,
	sessionID string,
	topicStorage storage.TopicStorage,
) (string, error) {
	refreshed, err := topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return "", errors.Wrap(ctx, err, "re-read topic after claude session")
	}
	if err := refreshed.SetField(ctx, "claude_session_id", sessionID); err != nil {
		return "", errors.Wrap(ctx, err, "save session id to topic")
	}
	if err := topicStorage.WriteTopic(ctx, refreshed); err != nil {
		return "", errors.Wrap(ctx, err, "save session id to topic")
	}
	return sessionID, nil
}

// handleClaudeSession starts or returns an existing Claude session for the topic.
// On both branches the session id is persisted only AFTER the headless turn has
// finished cleanly, so an id on disk means the session is resumable rather than merely
// that one was started. Nothing is written on any failure path, so there is no
// compensating clear: frontmatter the child wrote before failing stays untouched, and
// the Vault UI correctly keeps offering Start.
func (o *topicWorkOnOperation) handleClaudeSession(
	ctx context.Context,
	topic *domain.Topic,
	vaultPath string,
	sessionDir string,
	vault *config.Vault,
	isInteractive bool,
) (string, error) {
	if existing := topic.GetField("claude_session_id"); existing != "" {
		return existing, nil
	}
	if o.starter == nil {
		return "", ErrStarterUnavailable
	}
	// The bootstrap always runs headless `claude --print`, which cannot answer
	// AskUserQuestion; --non-interactive tells the work-on command to take safe
	// defaults instead of prompting (prevents the 5m headless hang).
	//
	// There is no topic-specific slash command: the spec's Non-goals keep the topic
	// slash-command ladder out of scope, so the goal work-on command is reused.
	prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnGoalCommand(), topic.FilePath)
	sessionID := o.uuidGenerator()
	slog.Info("starting claude session", "topic", topic.Name)
	if isInteractive {
		// TTY branch, unchanged: block through the headless turn, then re-read and
		// persist so frontmatter the session itself wrote survives.
		if err := o.starter.StartSession(ctx, sessionID, prompt, sessionDir, topic.Name, isInteractive); err != nil {
			return "", errors.Wrap(ctx, err, "start claude session")
		}
		sessionID, err := persistTopicSessionID(ctx, vaultPath, topic.Name, sessionID, o.topicStorage)
		return sessionID, err
	}
	// Non-interactive branch: persist the id only AFTER the turn has finished. An id on
	// disk is what makes the Vault UI offer Resume, and a resume against a still-running
	// turn hits a transcript another process is mid-write on. On any failure nothing is
	// persisted, so the button correctly stays on Start.
	if err := o.starter.StartSession(ctx, sessionID, prompt, sessionDir, topic.Name, isInteractive); err != nil {
		// No compensating clear needed: nothing was written for this id, so there is
		// nothing to undo. Frontmatter the child wrote before failing stays untouched.
		return "", errors.Wrap(ctx, err, "start claude session")
	}
	sessionID, err := persistTopicSessionID(ctx, vaultPath, topic.Name, sessionID, o.topicStorage)
	return sessionID, err
}
