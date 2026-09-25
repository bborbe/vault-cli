// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

// Topic represents a topic page in the Obsidian vault.
// Frontmatter is stored in TopicFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Topic struct {
	TopicFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content
}

// NewTopic creates a Topic from a parsed frontmatter map and metadata.
func NewTopic(data map[string]any, meta FileMetadata, content Content) *Topic {
	return &Topic{
		TopicFrontmatter: NewTopicFrontmatter(data),
		FileMetadata:     meta,
		Content:          content,
	}
}

// TopicStatusInProgress is the status value `vault-cli topic work-on` writes.
// It is the same literal the goal family uses for its in-progress status.
const TopicStatusInProgress = "in_progress"

// TopicStatusCompleted is the status value `vault-cli topic complete` writes.
// It is the same literal the goal family uses for its completed status.
const TopicStatusCompleted = "completed"
