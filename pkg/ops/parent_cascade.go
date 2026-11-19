// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
)

// ParentResult is the outcome of one cascade attempt on a parent page.
type ParentResult string

const (
	// ParentResultCompleted means the parent is terminal after the attempt.
	ParentResultCompleted ParentResult = "completed"
	// ParentResultRefused means the parent refused to close and stays open.
	ParentResultRefused ParentResult = "refused"
)

// ParentAttempt is one entry of a completion cascade report: the parent's name,
// the outcome, and (on a refusal) the items that blocked the close. The JSON
// tags are the frozen wire shape: a completed entry omits open_items.
type ParentAttempt struct {
	Name      string       `json:"name"`
	Result    ParentResult `json:"result"`
	OpenItems []string     `json:"open_items,omitempty"`
}

// parentAttemptFunc completes one parent and returns the operation's result.
type parentAttemptFunc func(ctx context.Context, name string) (MutationResult, error)

// cascadeParents attempts every parent in document order and returns one report
// entry per attempt. It never returns an error: a refusal is recorded, never
// propagated, so the child's outcome and the command's exit code are unaffected.
// Each raw entry is wikilink-normalised before the attempt; an entry that
// normalises to the empty string is skipped rather than attempted.
func cascadeParents(
	ctx context.Context,
	names []string,
	attempt parentAttemptFunc,
) []ParentAttempt {
	attempts := make([]ParentAttempt, 0, len(names))
	for _, raw := range names {
		name := normaliseGoalLink(raw)
		if name == "" {
			continue
		}
		result, err := attempt(ctx, name)
		if result.Success {
			attempts = append(attempts, ParentAttempt{Name: name, Result: ParentResultCompleted})
			attempts = append(attempts, result.Parents...)
			continue
		}
		attempts = append(attempts, ParentAttempt{
			Name:      name,
			Result:    ParentResultRefused,
			OpenItems: refusalItems(result, err),
		})
		attempts = append(attempts, result.Parents...)
	}
	return attempts
}

// refusalItems returns the items a refused attempt reports: the operation's own
// OpenItems, else its error text, else its Error field, else a fixed fallback so
// a refused entry never carries an empty list.
func refusalItems(result MutationResult, err error) []string {
	if len(result.OpenItems) > 0 {
		return result.OpenItems
	}
	if err != nil {
		return []string{err.Error()}
	}
	if result.Error != "" {
		return []string{result.Error}
	}
	return []string{"refused"}
}
