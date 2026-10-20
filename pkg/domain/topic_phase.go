// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain //nolint:dupl // Goal and topic phases share the same enum shape but are distinct types

import (
	"context"
	"strings"

	"github.com/bborbe/collection"
	"github.com/bborbe/errors"
	"github.com/bborbe/validation"
)

// TopicPhase represents a phase in a topic's lifecycle.
type TopicPhase string

const (
	// TopicPhaseTodo means the topic is ready to start but needs planning.
	TopicPhaseTodo TopicPhase = "todo"
	// TopicPhasePlanning means the approach is being designed.
	TopicPhasePlanning TopicPhase = "planning"
	// TopicPhaseExecution means active work is underway.
	TopicPhaseExecution TopicPhase = "execution"
	// TopicPhaseAIReview means automated checks are running.
	TopicPhaseAIReview TopicPhase = "ai_review"
	// TopicPhaseHumanReview means manual review is required.
	TopicPhaseHumanReview TopicPhase = "human_review"
	// TopicPhaseDone means the topic is ready to close.
	TopicPhaseDone TopicPhase = "done"
)

// AvailableTopicPhases lists all valid canonical topic phase values.
var AvailableTopicPhases = TopicPhases{
	TopicPhaseTodo,
	TopicPhasePlanning,
	TopicPhaseExecution,
	TopicPhaseAIReview,
	TopicPhaseHumanReview,
	TopicPhaseDone,
}

// TopicPhases is a collection of TopicPhase values.
type TopicPhases []TopicPhase

// Contains returns true if the collection contains the given phase.
func (t TopicPhases) Contains(phase TopicPhase) bool {
	return collection.Contains(t, phase)
}

// String returns the string representation of the phase.
func (t TopicPhase) String() string {
	return string(t)
}

// String returns the comma-separated list of phases in declaration order.
func (t TopicPhases) String() string {
	parts := make([]string, 0, len(t))
	for _, p := range t {
		parts = append(parts, string(p))
	}
	return strings.Join(parts, ", ")
}

// Validate returns an error if the phase is not a valid canonical value.
func (t TopicPhase) Validate(ctx context.Context) error {
	if !AvailableTopicPhases.Contains(t) {
		return errors.Wrapf(ctx, validation.Error,
			"unknown topic phase '%s' (valid: %s)", t, AvailableTopicPhases)
	}
	return nil
}

// Ptr returns a pointer to a copy of the phase.
func (t TopicPhase) Ptr() *TopicPhase {
	return &t
}
