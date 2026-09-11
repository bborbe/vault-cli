// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"strings"

	"github.com/bborbe/vault-cli/pkg/domain"
)

// IsBlocked reports whether the entity that declares blockedBy is blocked.
//
// blockedBy is the raw blocked_by frontmatter list: plain names or [[wikilinks]].
// entities is every page of the same kind — the same directory the listing came
// from, BEFORE any status filter is applied. An empty blockedBy is unblocked.
// Otherwise the entity is blocked iff at least one named blocker cannot be
// confirmed completed: a blocker whose page is absent, unreadable, or carries no
// parseable status counts as not completed, so "cannot verify it is done" reads
// as blocked rather than as permission to start. Resolution is a single status
// read per blocker — a blocker's own blockedBy is never followed, so a dependency
// cycle terminates immediately.
func IsBlocked(blockedBy []string, entities []*domain.Page) bool {
	if len(blockedBy) == 0 {
		return false
	}
	for _, name := range blockedBy {
		stripped := strings.TrimPrefix(name, "[[")
		stripped = strings.TrimSuffix(stripped, "]]")
		found := false
		for _, entity := range entities {
			if !strings.EqualFold(entity.Name, stripped) {
				continue
			}
			found = true
			if entity.Status() != domain.TaskStatusCompleted {
				return true
			}
			break
		}
		if !found {
			return true
		}
	}
	return false
}
