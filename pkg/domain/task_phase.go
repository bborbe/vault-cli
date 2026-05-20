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

// TaskPhase represents a phase in a task's lifecycle.
type TaskPhase string

const (
	// TaskPhaseTodo means the task is ready to start but needs planning.
	TaskPhaseTodo TaskPhase = "todo"
	// TaskPhasePlanning means the approach is being designed.
	TaskPhasePlanning TaskPhase = "planning"
	// TaskPhaseExecution means active implementation is underway.
	// This is the canonical value; "in_progress" is accepted as an alias via NormalizeTaskPhase.
	TaskPhaseExecution TaskPhase = "execution"
	// TaskPhaseInProgress is an alias for TaskPhaseExecution kept for backward compatibility.
	// Existing vault files with phase: "in_progress" continue to read and validate via NormalizeTaskPhase.
	// Do not use TaskPhaseInProgress for new writes — use TaskPhaseExecution.
	TaskPhaseInProgress TaskPhase = "in_progress"
	// TaskPhaseAIReview means automated checks are running.
	TaskPhaseAIReview TaskPhase = "ai_review"
	// TaskPhaseHumanReview means manual review is required.
	TaskPhaseHumanReview TaskPhase = "human_review"
	// TaskPhaseDone means the task is ready to close.
	TaskPhaseDone TaskPhase = "done"
)

// AvailableTaskPhases lists all valid canonical task phase values.
var AvailableTaskPhases = TaskPhases{
	TaskPhaseTodo,
	TaskPhasePlanning,
	TaskPhaseExecution,
	TaskPhaseAIReview,
	TaskPhaseHumanReview,
	TaskPhaseDone,
}

// TaskPhases is a collection of TaskPhase values.
type TaskPhases []TaskPhase

// Contains returns true if the collection contains the given phase.
func (t TaskPhases) Contains(phase TaskPhase) bool {
	return collection.Contains(t, phase)
}

// String returns the string representation of the phase.
func (t TaskPhase) String() string {
	return string(t)
}

// Validate returns an error if the phase is not a valid canonical value.
func (t TaskPhase) Validate(ctx context.Context) error {
	if !AvailableTaskPhases.Contains(t) {
		return errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", t)
	}
	return nil
}

// Ptr returns a pointer to a copy of the phase.
func (t TaskPhase) Ptr() *TaskPhase {
	return &t
}

// IsValidTaskPhase returns true if the phase is a valid canonical phase value.
func IsValidTaskPhase(phase TaskPhase) bool {
	return AvailableTaskPhases.Contains(phase)
}

// NormalizeTaskPhase converts alias phase values to their canonical form.
// Returns the canonical phase and true if valid, or empty and false if unknown.
func NormalizeTaskPhase(raw string) (TaskPhase, bool) {
	// Check if already valid canonical phase
	phase := TaskPhase(raw)
	if IsValidTaskPhase(phase) {
		return phase, true
	}

	// Migration map for legacy/alias phase values
	migrationMap := map[string]TaskPhase{
		"in_progress": TaskPhaseExecution,
	}

	if canonical, ok := migrationMap[raw]; ok {
		return canonical, true
	}

	return "", false
}
