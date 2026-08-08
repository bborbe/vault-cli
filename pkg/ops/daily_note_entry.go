// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"strings"
	"unicode"
)

// trimLeadingDecoration returns s with its leading run of decoration runes
// removed. A decoration rune is any rune that is neither a letter nor a digit
// and is not the wikilink opener. Iteration is over runes, not bytes, so a
// multi-rune emoji with a variation selector is consumed whole; and because
// the test is Unicode-aware, a leading letter in any script — Latin, CJK,
// Cyrillic — ends the run and leaves the text classified as prose.
func trimLeadingDecoration(s string) string {
	for i, r := range s {
		if r == '[' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return s[i:]
		}
	}
	return ""
}

// IsOwnDailyNoteEntry reports whether checkboxText is the daily-note entry
// that belongs to taskName.
//
// checkboxText is capture group 3 of storage.CheckboxRegex — everything after
// the `- [x] ` marker on a daily-note checkbox line.
//
// A checkbox line is a task's own entry iff its text, after stripping any
// leading decoration, begins with a wikilink resolving to that task. Decoration
// is any leading run of runes that are not letters, digits, or the wikilink
// opener `[`. Category emoji, symbols such as section-sign or double-dagger,
// and markdown emphasis markers such as `**` are all decoration by character
// class. Because the test is Unicode-aware, a leading letter in any script —
// Latin, CJK, Cyrillic — ends the decoration run and makes the line a mention,
// so `作業 [[Some Task]]` and `Задача по [[Some Task]]` are mentions. A digit
// also ends the run, so a keycap prefix such as `1️⃣ [[Some Task]]` is not
// recognised as an own entry. Trailing prose after the wikilink does not
// disqualify it, so `[[Some Task]] — due today` is Some Task's own entry. A
// wikilink to the task appearing anywhere else in the text is a mention, never
// an own entry, so `Chain — [[Other]] → [[Some Task]].` is not Some Task's
// own entry.
//
// Alias (`[[Task|label]]`) and heading (`[[Task#Section]]`) link forms
// resolve to the same task. Comparison is case-insensitive and literal —
// the task name is never interpreted as a pattern.
func IsOwnDailyNoteEntry(checkboxText string, taskName string) bool {
	// Step 1: strip leading decoration
	trimmed := trimLeadingDecoration(checkboxText)

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
