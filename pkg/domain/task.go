// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"time"
)

// Task represents a task in the Obsidian vault with YAML frontmatter.
type Task struct {
	// Frontmatter fields
	Status          TaskStatus `yaml:"status"`
	PageType        string     `yaml:"page_type"`
	Goals           []string   `yaml:"goals,omitempty"`
	Priority        Priority   `yaml:"priority,omitempty"`
	Assignee        string     `yaml:"assignee,omitempty"`
	DeferDate       *time.Time `yaml:"defer_date,omitempty"`
	Tags            []string   `yaml:"tags,omitempty"`
	Phase           string     `yaml:"phase,omitempty"`
	ClaudeSessionID string     `yaml:"claude_session_id,omitempty"`
	Recurring       string     `yaml:"recurring,omitempty"`
	LastCompleted   string     `yaml:"last_completed,omitempty"`
	PlannedDate     *time.Time `yaml:"planned_date,omitempty"`

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

// NormalizeTaskStatus returns the canonical status value for a given status string.
// If the status is already canonical, it returns it unchanged.
// If the status is a known legacy/alternative value, it returns the canonical equivalent.
// Otherwise, it returns the input unchanged.
func NormalizeTaskStatus(status string) string {
	// Check if already valid
	if IsValidTaskStatus(TaskStatus(status)) {
		return status
	}

	// Migration map for legacy status values
	migrationMap := map[string]string{
		"next":    string(TaskStatusTodo),
		"current": string(TaskStatusInProgress),
		"done":    string(TaskStatusCompleted),
	}

	if canonical, ok := migrationMap[status]; ok {
		return canonical
	}

	return status
}
