// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/topic-complete-operation.go --fake-name TopicCompleteOperation . TopicCompleteOperation
type TopicCompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		vaultName string,
	) (MutationResult, error)
}

// NewTopicCompleteOperation creates a new topic complete operation.
func NewTopicCompleteOperation(topicStorage storage.TopicStorage) TopicCompleteOperation {
	return &topicCompleteOperation{topicStorage: topicStorage}
}

type topicCompleteOperation struct {
	topicStorage storage.TopicStorage
}

// Execute marks a topic as completed.
//
// Unlike the goal variant there is no open-task gate and no close-out machinery:
// topics carry no task linkage and no close-out status, so the only observable
// transition is the status field.
func (o *topicCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	topicName string,
	vaultName string,
) (MutationResult, error) {
	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "find topic")
	}

	if topic.GetField("status") == domain.TopicStatusCompleted {
		msg := fmt.Sprintf("topic %q is already completed", topicName)
		return MutationResult{Success: false, Error: msg}, errors.Errorf(ctx, "%s", msg)
	}

	if err := topic.SetField(ctx, "status", domain.TopicStatusCompleted); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
	}

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write topic")
	}

	return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
}
