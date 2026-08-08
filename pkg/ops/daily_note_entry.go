// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import "strings"

// IsOwnDailyNoteEntry reports whether checkboxText is the daily-note entry
// that belongs to taskName.
//
// checkboxText is capture group 3 of storage.CheckboxRegex — everything after
// the `- [x] ` marker on a daily-note checkbox line.
//
// A checkbox line is a task's own entry iff its text, after trimming leading
// whitespace, begins with a wikilink resolving to that task. Trailing prose
// after the wikilink does not disqualify it, so
// `[[Some Task]] — due today` is Some Task's own entry. A wikilink to the
// task appearing anywhere else in the text is a mention, never an own entry,
// so `Chain — [[Other]] → [[Some Task]].` is not Some Task's own entry.
//
// Alias (`[[Task|label]]`) and heading (`[[Task#Section]]`) link forms
// resolve to the same task. Comparison is case-insensitive and literal —
// the task name is never interpreted as a pattern.
func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool {
	// Step 1: trim leading whitespace
	trimmed := strings.TrimLeft(checkboxText, " \t")

	// Step 2: must start with [[
	if !strings.HasPrefix(trimmed, "[[") {
		return false
	}

	// Step 3: find the closing ]]
	afterBrackets := trimmed[2:]
	closingIdx := strings.Index(afterBrackets, "]]")
	if closingIdx == -1 {
		return false
	}

	// Step 4: link target is everything before the closing ]]
	linkTarget := afterBrackets[:closingIdx]

	// Step 5: strip alias suffix (|label)
	if aliasIdx := strings.Index(linkTarget, "|"); aliasIdx != -1 {
		linkTarget = linkTarget[:aliasIdx]
	}

	// Step 6: strip heading suffix (#section)
	if headingIdx := strings.Index(linkTarget, "#"); headingIdx != -1 {
		linkTarget = linkTarget[:headingIdx]
	}

	// Step 7: case-insensitive full-string comparison
	return strings.EqualFold(strings.TrimSpace(linkTarget), strings.TrimSpace(taskName))
}
