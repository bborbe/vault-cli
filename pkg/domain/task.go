// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

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

// TaskID represents a task identifier (filename without .md extension).
type TaskID string

func (t TaskID) String() string {
	return string(t)
}
