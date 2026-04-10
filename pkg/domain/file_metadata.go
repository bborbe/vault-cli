// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import "time"

// FileMetadata holds the filesystem metadata for an entity file.
// It is embedded in all entity structs (Task, Goal, Theme, Objective, Vision)
// and is never stored in YAML frontmatter.
type FileMetadata struct {
	// Name is the filename without the .md extension.
	Name string
	// FilePath is the absolute path to the markdown file.
	FilePath string
	// ModifiedDate is the file's last-modified time (UTC), populated by the storage layer.
	ModifiedDate *time.Time
}
