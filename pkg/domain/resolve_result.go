// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

// ResolveResult is the outcome of resolving a name to a task, a goal, or neither.
// It is the JSON contract consumed by slash commands to auto-detect entity type.
type ResolveResult struct {
	// Type is "task", "goal", or "" (empty string when not found).
	Type string `json:"type"`
	// Name echoes the input name that was resolved.
	Name string `json:"name"`
	// Found reports whether the name matched a task or a goal.
	Found bool `json:"found"`
}
