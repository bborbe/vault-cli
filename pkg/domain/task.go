// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"fmt"

	libtime "github.com/bborbe/time"
)

// Task represents a task in the Obsidian vault with YAML frontmatter.
type Task struct {
	// Frontmatter fields
	Status          TaskStatus    `yaml:"status"`
	PageType        string        `yaml:"page_type"`
	Goals           []string      `yaml:"goals,omitempty"`
	Priority        Priority      `yaml:"priority,omitempty"`
	Assignee        string        `yaml:"assignee,omitempty"`
	DeferDate       *libtime.Date `yaml:"defer_date,omitempty"`
	Tags            []string      `yaml:"tags,omitempty"`
	Phase           string        `yaml:"phase,omitempty"`
	ClaudeSessionID string        `yaml:"claude_session_id,omitempty"`
	Recurring       string        `yaml:"recurring,omitempty"`
	LastCompleted   string        `yaml:"last_completed,omitempty"`
	PlannedDate     *libtime.Date `yaml:"planned_date,omitempty"`
	DueDate         *libtime.Date `yaml:"due_date,omitempty"`

	// Metadata
	Name     string `yaml:"-"` // Filename without extension
	Content  string `yaml:"-"` // Full markdown content including frontmatter
	FilePath string `yaml:"-"` // Absolute path to file
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

// TaskID represents a task identifier (filename without .md extension).
type TaskID string

func (t TaskID) String() string {
	return string(t)
}

// IsValidTaskStatus returns true if the status is a valid canonical status value.
func IsValidTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskStatusTodo,
		TaskStatusInProgress,
		TaskStatusBacklog,
		TaskStatusCompleted,
		TaskStatusHold,
		TaskStatusAborted:
		return true
	default:
		return false
	}
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
