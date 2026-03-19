// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/bborbe/collection"
	"github.com/bborbe/validation"
)

// Task represents a task in the Obsidian vault with YAML frontmatter.
type Task struct {
	// Frontmatter fields
	Status          TaskStatus      `yaml:"status"`
	PageType        string          `yaml:"page_type"`
	Goals           []string        `yaml:"goals,omitempty"`
	Priority        Priority        `yaml:"priority,omitempty"`
	Assignee        string          `yaml:"assignee,omitempty"`
	DeferDate       *DateOrDateTime `yaml:"defer_date,omitempty"`
	Tags            []string        `yaml:"tags,omitempty"`
	Phase           string          `yaml:"phase,omitempty"`
	ClaudeSessionID string          `yaml:"claude_session_id,omitempty"`
	Recurring       string          `yaml:"recurring,omitempty"`
	LastCompleted   string          `yaml:"last_completed,omitempty"`
	CompletedDate   string          `yaml:"completed_date,omitempty"`
	PlannedDate     *DateOrDateTime `yaml:"planned_date,omitempty"`
	DueDate         *DateOrDateTime `yaml:"due_date,omitempty"`

	// Metadata
	Name         string     `yaml:"-"` // Filename without extension
	Content      string     `yaml:"-"` // Full markdown content including frontmatter
	FilePath     string     `yaml:"-"` // Absolute path to file
	ModifiedDate *time.Time `yaml:"-"` // File modification time, populated by storage layer
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusBacklog    TaskStatus = "backlog"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusHold       TaskStatus = "hold"
	TaskStatusAborted    TaskStatus = "aborted"
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

// UnmarshalYAML implements custom YAML unmarshaling that normalizes status values.
func (s *TaskStatus) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	normalized, ok := NormalizeTaskStatus(raw)
	if !ok {
		return fmt.Errorf("invalid task status: %q", raw)
	}
	*s = normalized
	return nil
}
