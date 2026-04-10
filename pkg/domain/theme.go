// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain //nolint:dupl

import (
	"context"
	"fmt"

	"github.com/bborbe/collection"
	"github.com/bborbe/validation"
)

// Theme represents a theme in the Obsidian vault.
// Frontmatter is stored in ThemeFrontmatter (a typed map wrapper that preserves
// unknown fields). Filesystem metadata is in the embedded FileMetadata.
type Theme struct {
	ThemeFrontmatter
	FileMetadata
	// Content is the full markdown content including the frontmatter block.
	Content Content
}

// NewTheme creates a Theme from a parsed frontmatter map and metadata.
func NewTheme(data map[string]any, meta FileMetadata, content Content) *Theme {
	return &Theme{
		ThemeFrontmatter: NewThemeFrontmatter(data),
		FileMetadata:     meta,
		Content:          content,
	}
}

// ThemeStatus represents the status of a theme.
type ThemeStatus string

const (
	ThemeStatusActive    ThemeStatus = "active"
	ThemeStatusCompleted ThemeStatus = "completed"
	ThemeStatusArchived  ThemeStatus = "archived"
)

// AvailableThemeStatuses lists all valid canonical theme status values.
var AvailableThemeStatuses = ThemeStatuses{
	ThemeStatusActive,
	ThemeStatusCompleted,
	ThemeStatusArchived,
}

// ThemeStatuses is a collection of ThemeStatus values.
type ThemeStatuses []ThemeStatus

// Contains returns true if the collection contains the given status.
func (t ThemeStatuses) Contains(status ThemeStatus) bool {
	return collection.Contains(t, status)
}

// Validate returns an error if the status is not a valid canonical value.
func (s ThemeStatus) Validate(ctx context.Context) error {
	if !AvailableThemeStatuses.Contains(s) {
		return fmt.Errorf("%w: unknown theme status '%s'", validation.Error, s)
	}
	return nil
}

// ThemeID represents a theme identifier (filename without .md extension).
type ThemeID string

func (t ThemeID) String() string {
	return string(t)
}
