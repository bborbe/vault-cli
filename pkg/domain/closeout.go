// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import "strings"

// closeOutFields are the frontmatter keys that must both be present and
// non-empty before a task or goal may transition to status aborted (a
// "close-out"). aborted_reason is the free-text why; gate_successor names
// where any risk gate the entity owned moves, or the literal "none".
// Both names are frozen by spec 037; the guard is consulted for aborted
// only — completed never reads these fields (spec 039).
var closeOutFields = []string{"aborted_reason", "gate_successor"}

// missingCloseOutFields returns the subset of close-out fields that are absent
// or whitespace-only on the frontmatter. An empty slice means a close-out
// transition is allowed.
func missingCloseOutFields(f FrontmatterMap) []string {
	var missing []string
	for _, field := range closeOutFields {
		if strings.TrimSpace(f.GetString(field)) == "" {
			missing = append(missing, field)
		}
	}
	return missing
}
