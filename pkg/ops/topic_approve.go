// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // Structurally parallel to the sibling goal's approve; frozen field names prevent dedup
package ops

import (
	"context"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/topic-approve-operation.go --fake-name TopicApproveOperation . TopicApproveOperation
type TopicApproveOperation interface {
	// Execute approves the named topic: it moves a topic waiting in the approval
	// inbox (phase todo) to phase planning and records, in the same write, who
	// approved it and the instant of the transition. A topic that is not waiting
	// in the inbox, or that already carries an approval record, is refused with
	// nothing written.
	//
	// Topics are not required to have an owner, and this verb does not assign
	// one.
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		vaultName string,
		approvedBy string,
	) (MutationResult, error)
}

// NewTopicApproveOperation creates a new topic approve operation.
func NewTopicApproveOperation(
	topicStorage storage.TopicStorage,
	currentDateTime libtime.CurrentDateTime,
) TopicApproveOperation {
	return &topicApproveOperation{
		topicStorage:    topicStorage,
		currentDateTime: currentDateTime,
	}
}

type topicApproveOperation struct {
	topicStorage    storage.TopicStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute approves a topic waiting in the approval inbox.
func (o *topicApproveOperation) Execute(
	ctx context.Context,
	vaultPath string,
	topicName string,
	vaultName string,
	approvedBy string,
) (MutationResult, error) {
	// Refuse an empty approver before reading the topic: a present-but-blank
	// approved_by is worse than no record at all, and nothing is written here.
	if approvedBy == "" {
		err := errors.Errorf(
			ctx,
			"approved_by must not be empty: refusing to record a blank approver on topic %q",
			topicName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find topic")
	}

	// Read and validate the clock before any guard: a zero instant would be
	// recorded as a placeholder rather than a measurement, so it is refused.
	now := o.currentDateTime.Now()
	if now.Time().IsZero() {
		err := errors.Errorf(
			ctx,
			"approved_at is not a valid instant: refusing to record a zero timestamp on topic %q",
			topicName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The inbox guard: only a topic at phase todo is waiting for approval. The
	// message names both the current phase and the expected one so the operator
	// can see what was found and what was required.
	if phase := topic.Phase(); phase == nil || *phase != domain.TopicPhaseTodo {
		current := "(none)"
		if phase != nil {
			current = string(*phase)
		}
		err := errors.Errorf(
			ctx,
			"refusing to approve %q: topic is at phase %q, not %q; only a topic in the approval inbox (phase %q) can be approved",
			topicName, current, string(domain.TopicPhaseTodo), string(domain.TopicPhaseTodo),
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The existing-record guard: a todo row carrying an approval record is not
	// awaiting approval, and re-approving must not overwrite who approved it first.
	if err := refuseExistingEntityApprovalRecord(ctx, "topic", topicName, topic.Get); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All three keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	topic.SetPhase(domain.TopicPhasePlanning.Ptr())
	topic.Set("approved_by", approvedBy)
	topic.Set("approved_at", now.Time())

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write topic")
	}

	return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
}
