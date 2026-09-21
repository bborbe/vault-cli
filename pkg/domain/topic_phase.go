// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"

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
	// TopicPhaseDone means the topic is ready to close.
	TopicPhaseDone TopicPhase = "done"
)

// AvailableTopicPhases lists all valid canonical topic phase values.
var AvailableTopicPhases = TopicPhases{
	TopicPhaseTodo,
	TopicPhasePlanning,
	TopicPhaseExecution,
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

// Validate returns an error if the phase is not a valid canonical value.
func (t TopicPhase) Validate(ctx context.Context) error {
	if !AvailableTopicPhases.Contains(t) {
		return errors.Wrapf(ctx, validation.Error, "unknown topic phase '%s'", t)
	}
	return nil
}

// Ptr returns a pointer to a copy of the phase.
func (t TopicPhase) Ptr() *TopicPhase {
	return &t
}
