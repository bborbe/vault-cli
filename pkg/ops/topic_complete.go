// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"

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
//
// goalStorage resolves the member goals named in the topic's Goals section:
// the closure gate needs each member's status, and topics carry no task
// relation of their own.
func NewTopicCompleteOperation(
	topicStorage storage.TopicStorage,
	goalStorage storage.GoalStorage,
) TopicCompleteOperation {
	return &topicCompleteOperation{
		topicStorage: topicStorage,
		goalStorage:  goalStorage,
	}
}

type topicCompleteOperation struct {
	topicStorage storage.TopicStorage
	goalStorage  storage.GoalStorage
}

// Execute marks a topic as completed.
//
// The closure gate reads the topic's own Success Criteria and Completion Gate
// boxes and the terminality of every member goal. A topic already at completed
// or aborted is a no-op. There is no open-task gate and no close-out machinery:
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

	if status := topic.GetField("status"); status == domain.TopicStatusCompleted || status == "aborted" {
		return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
	}

	members, err := o.goalMembers(ctx, vaultPath, string(topic.Content))
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}
	verdict, err := checkClosureGate(ctx, closureKindTopic, string(topic.Content), members)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}
	if verdict.Outcome == closureOutcomeTerminal {
		return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
	}
	if verdict.Outcome != closureOutcomeClose {
		msg := closureRefusalMessage("topic", verdict.OpenItems)
		return MutationResult{Success: false, Error: msg, OpenItems: verdict.OpenItems},
			errors.Errorf(ctx, "%s", msg)
	}

	if err := topic.SetField(ctx, "status", domain.TopicStatusCompleted); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
	}

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write topic")
	}

	return MutationResult{Success: true, Name: topic.Name, Vault: vaultName}, nil
}

// goalMembers resolves each goal named in the topic's Goals section to its
// status. A member whose goal file does not exist reads as open (Status ""):
// an unresolvable member is never treated as terminal.
func (o *topicCompleteOperation) goalMembers(
	ctx context.Context,
	vaultPath string,
	content string,
) ([]closureMember, error) {
	names := parseGoalSectionMembers(content)
	members := make([]closureMember, 0, len(names))
	for _, name := range names {
		goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, name)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				members = append(members, closureMember{Name: name})
				continue
			}
			return nil, errors.Wrapf(ctx, err, "find goal %s", name)
		}
		members = append(members, closureMember{Name: name, Status: string(goal.Status())})
	}
	return members, nil
}
