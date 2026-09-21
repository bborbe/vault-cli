// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:dupl // Structurally parallel to the sibling entity's defer; frozen field names prevent dedup
package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/topic-defer-operation.go --fake-name TopicDeferOperation . TopicDeferOperation
type TopicDeferOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		topicName string,
		dateStr string,
		vaultName string,
	) (MutationResult, error)
}

// NewTopicDeferOperation creates a new topic defer operation.
func NewTopicDeferOperation(
	topicStorage storage.TopicStorage,
	currentDateTime libtime.CurrentDateTime,
) TopicDeferOperation {
	return &topicDeferOperation{
		topicStorage:    topicStorage,
		currentDateTime: currentDateTime,
	}
}

type topicDeferOperation struct {
	topicStorage    storage.TopicStorage
	currentDateTime libtime.CurrentDateTime
}

// Execute sets defer_date on a topic without updating daily notes.
func (o *topicDeferOperation) Execute(
	ctx context.Context,
	vaultPath string,
	topicName string,
	dateStr string,
	vaultName string,
) (MutationResult, error) {
	now := o.currentDateTime.Now().Time()

	targetDate, err := parseDeferDate(ctx, dateStr, now)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "parse date")
	}

	if isDeferDateInPast(targetDate, now) {
		return MutationResult{
			Success: false,
			Error: fmt.Sprintf(
				"cannot defer to past date: %s",
				targetDate.Time().Format("2006-01-02"),
			),
		}, errors.Errorf(
			ctx,
			"cannot defer to past date: %s",
			targetDate.Time().Format("2006-01-02"),
		)
	}

	topic, err := o.topicStorage.FindTopicByName(ctx, vaultPath, topicName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find topic")
	}

	topic.SetDeferDate(targetDate.Ptr())

	if err := o.topicStorage.WriteTopic(ctx, topic); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write topic")
	}

	formattedDate := targetDate.Time().Format("2006-01-02")
	return MutationResult{
		Success: true,
		Name:    topic.Name,
		Vault:   vaultName,
		Message: formattedDate,
	}, nil
}
