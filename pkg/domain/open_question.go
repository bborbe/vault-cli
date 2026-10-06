// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

// OpenQuestion is one item of a task's Open Questions section.
//
// Index is the item's 1-based position in the section and is the value a caller
// passes back to answer it. Text is the item's line with its list marker
// stripped and the remainder trimmed, so it never carries the `- `, `* `, or
// `N. ` that the file uses.
type OpenQuestion struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// OpenAnswer is one operator answer to an OpenQuestion.
//
// Index names the OpenQuestion it answers and Answer is the text recorded for
// it; the pair is written back into the task's Open Questions section.
type OpenAnswer struct {
	Index  int    `json:"index"`
	Answer string `json:"answer"`
}
