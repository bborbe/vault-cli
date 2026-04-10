// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"fmt"

	"github.com/bborbe/collection"
	"github.com/bborbe/validation"
)

// Task represents a task in the Obsidian vault.
// Frontmatter is stored in TaskFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Task struct {
	TaskFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	// It is used by the storage layer to extract the markdown body on write.
	Content Content
}

// NewTask creates a Task from a parsed frontmatter map and metadata.
func NewTask(data map[string]any, meta FileMetadata, content Content) *Task {
	return &Task{
		TaskFrontmatter: NewTaskFrontmatter(data),
		FileMetadata:    meta,
		Content:         content,
	}
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	// TaskStatusTodo means the task is queued for action but not yet started.
	TaskStatusTodo TaskStatus = "todo"
	// TaskStatusInProgress means someone is actively working on the task.
	TaskStatusInProgress TaskStatus = "in_progress"
	// TaskStatusBacklog means the task is potential future work, not yet scheduled.
	TaskStatusBacklog TaskStatus = "backlog"
	// TaskStatusCompleted means the task is finished.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusHold means the task is blocked or paused.
	TaskStatusHold TaskStatus = "hold"
	// TaskStatusAborted means the task was cancelled and will not be completed.
	TaskStatusAborted TaskStatus = "aborted"
)

// AvailableTaskStatuses lists all valid canonical task status values.
var AvailableTaskStatuses = TaskStatuses{
	TaskStatusTodo,
	TaskStatusInProgress,
	TaskStatusBacklog,
	TaskStatusCompleted,
	TaskStatusHold,
	TaskStatusAborted,
}

// TaskStatuses is a collection of TaskStatus values.
type TaskStatuses []TaskStatus

// Contains returns true if the collection contains the given status.
func (t TaskStatuses) Contains(status TaskStatus) bool {
	return collection.Contains(t, status)
}

// String returns the string representation of the status.
func (s TaskStatus) String() string {
	return string(s)
}

// Validate returns an error if the status is not a valid canonical value.
func (s TaskStatus) Validate(ctx context.Context) error {
	if !AvailableTaskStatuses.Contains(s) {
		return fmt.Errorf("%w: unknown task status '%s'", validation.Error, s)
	}
	return nil
}

// Ptr returns a pointer to a copy of the status.
func (s TaskStatus) Ptr() *TaskStatus {
	return &s
}

// TaskID represents a task identifier (filename without .md extension).
type TaskID string

func (t TaskID) String() string {
	return string(t)
}

// IsValidTaskStatus returns true if the status is a valid canonical status value.
func IsValidTaskStatus(status TaskStatus) bool {
	return AvailableTaskStatuses.Contains(status)
}

// NormalizeTaskStatus converts alias status values to their canonical form.
// Returns the canonical status and true if valid, or empty and false if unknown.
func NormalizeTaskStatus(raw string) (TaskStatus, bool) {
	// Check if already valid canonical status
	status := TaskStatus(raw)
	if IsValidTaskStatus(status) {
		return status, true
	}

	// Migration map for legacy/alias status values
	migrationMap := map[string]TaskStatus{
		"next":     TaskStatusTodo,
		"current":  TaskStatusInProgress,
		"done":     TaskStatusCompleted,
		"deferred": TaskStatusHold,
	}

	if canonical, ok := migrationMap[raw]; ok {
		return canonical, true
	}

	return "", false
}
