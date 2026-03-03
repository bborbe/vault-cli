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
