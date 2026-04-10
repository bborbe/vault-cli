// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

// Content is the full markdown file content including the frontmatter block.
// It is embedded in entity structs (Task, Goal, Theme, Objective, Vision)
// alongside FileMetadata and an entity-specific XxxFrontmatter type.
// The storage layer extracts the markdown body from Content on write.
type Content string

// String returns the underlying string value.
func (c Content) String() string {
	return string(c)
}
