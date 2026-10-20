// Copyright (c) 2025 Benjamin Borbe All rights reserved.
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

// GoalPhase represents a phase in a goal's lifecycle.
type GoalPhase string

const (
	// GoalPhaseTodo means the goal is ready to start but needs planning.
	GoalPhaseTodo GoalPhase = "todo"
	// GoalPhasePlanning means the approach is being designed.
	GoalPhasePlanning GoalPhase = "planning"
	// GoalPhaseExecution means active work is underway.
	GoalPhaseExecution GoalPhase = "execution"
	// GoalPhaseAIReview means automated checks are running.
	GoalPhaseAIReview GoalPhase = "ai_review"
	// GoalPhaseHumanReview means manual review is required.
	GoalPhaseHumanReview GoalPhase = "human_review"
	// GoalPhaseDone means the goal is ready to close.
	GoalPhaseDone GoalPhase = "done"
)

// AvailableGoalPhases lists all valid canonical goal phase values.
var AvailableGoalPhases = GoalPhases{
	GoalPhaseTodo,
	GoalPhasePlanning,
	GoalPhaseExecution,
	GoalPhaseAIReview,
	GoalPhaseHumanReview,
	GoalPhaseDone,
}

// GoalPhases is a collection of GoalPhase values.
type GoalPhases []GoalPhase

// Contains returns true if the collection contains the given phase.
func (g GoalPhases) Contains(phase GoalPhase) bool {
	return collection.Contains(g, phase)
}

// String returns the string representation of the phase.
func (g GoalPhase) String() string {
	return string(g)
}

// String returns the comma-separated list of phases in declaration order.
func (g GoalPhases) String() string {
	parts := make([]string, 0, len(g))
	for _, p := range g {
		parts = append(parts, string(p))
	}
	return strings.Join(parts, ", ")
}

// Validate returns an error if the phase is not a valid canonical value.
func (g GoalPhase) Validate(ctx context.Context) error {
	if !AvailableGoalPhases.Contains(g) {
		return errors.Wrapf(ctx, validation.Error,
			"unknown goal phase '%s' (valid: %s)", g, AvailableGoalPhases)
	}
	return nil
}

// Ptr returns a pointer to a copy of the phase.
func (g GoalPhase) Ptr() *GoalPhase {
	return &g
}
