// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"fmt"
	"regexp"
	"strings"
)

// PullRequestState is the lifecycle state a `# Pull Requests` entry declares for
// one pull request. The state lives in the entry text rather than in a
// frontmatter field so that checking it needs no network call — see
// PullRequestTaskBindingGuide.
type PullRequestState string

const (
	// PullRequestStateOpen means the pull request is still open. A task may not
	// reach a terminal status while one of its entries reads open.
	PullRequestStateOpen PullRequestState = "open"
	// PullRequestStateMerged means the pull request landed.
	PullRequestStateMerged PullRequestState = "merged"
	// PullRequestStateClosed means the pull request was closed unmerged.
	PullRequestStateClosed PullRequestState = "closed"
	// PullRequestStateUnknown means the entry could not be classified. It is
	// deliberately not an error: the vault carries thousands of free-form
	// entries written before this convention, and treating them as open would
	// fire the rule on every one of them.
	PullRequestStateUnknown PullRequestState = ""
)

// PullRequestEntry is one parsed line of a task's `# Pull Requests` section.
type PullRequestEntry struct {
	// Key is `owner/repo#number` — the machine-readable binding key.
	Key string
	// URL is the pull request URL as written in the entry.
	URL string
	// State is the declared lifecycle state.
	State PullRequestState
}

var (
	pullRequestsHeading = regexp.MustCompile(`^#\s+Pull Requests\s*$`)
	anyHeading          = regexp.MustCompile(`^#\s+\S`)
	pullRequestEntry    = regexp.MustCompile(
		`^\s*-\s*\[([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(\d+)\]\(([^)]+)\)\s*[—–-]\s*([A-Za-z]+)`,
	)
)

// ParsePullRequests returns every entry in a page's `# Pull Requests` section,
// in document order. Entries whose state token is not one of open, merged or
// closed are returned with PullRequestStateUnknown so a caller can report them
// without treating them as open.
func ParsePullRequests(content string) []PullRequestEntry {
	lines := strings.Split(content, "\n")
	entries := make([]PullRequestEntry, 0, 4)
	inSection := false
	for _, line := range lines {
		if pullRequestsHeading.MatchString(line) {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		// The section ends at the next heading of the same or higher level.
		if anyHeading.MatchString(line) {
			break
		}
		matches := pullRequestEntry.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		entries = append(entries, PullRequestEntry{
			Key:   matches[1] + "#" + matches[2],
			URL:   matches[3],
			State: parsePullRequestState(matches[4]),
		})
	}
	return entries
}

// parsePullRequestState maps an entry's leading state word onto a known state.
// An unrecognised word yields PullRequestStateUnknown rather than an error.
func parsePullRequestState(word string) PullRequestState {
	switch strings.ToLower(word) {
	case string(PullRequestStateOpen):
		return PullRequestStateOpen
	case string(PullRequestStateMerged):
		return PullRequestStateMerged
	case string(PullRequestStateClosed):
		return PullRequestStateClosed
	default:
		return PullRequestStateUnknown
	}
}

// OpenPullRequests returns the entries whose state is open. Entries the parser
// could not classify are excluded on purpose — see PullRequestStateUnknown.
func OpenPullRequests(content string) []PullRequestEntry {
	open := make([]PullRequestEntry, 0, 2)
	for _, entry := range ParsePullRequests(content) {
		if entry.State == PullRequestStateOpen {
			open = append(open, entry)
		}
	}
	return open
}

// isTerminalTaskStatus reports whether a task status ends the task's life, and
// so must not coexist with an open pull request.
func isTerminalTaskStatus(status string) bool {
	return status == "completed" || status == "aborted"
}

// openPullRequestRefusal returns the refusal result when a task still owns an
// open pull request, and reports whether it refused. The check runs before the
// status transition so a refusal leaves the file untouched. Entries the parser
// cannot classify are never treated as open, so a free-form section cannot block
// a completion.
func openPullRequestRefusal(content string, taskName string, vaultName string) (MutationResult, bool) {
	openPullRequests := OpenPullRequests(content)
	if len(openPullRequests) == 0 {
		return MutationResult{}, false
	}
	return MutationResult{
		Success: false,
		Name:    taskName,
		Vault:   vaultName,
		Error: fmt.Sprintf(
			"task still owns %d open pull request(s): %s — merge or close them first, or pass --force",
			len(openPullRequests),
			joinPullRequestKeys(openPullRequests),
		),
		Reason:  "open_pull_request",
		Pending: len(openPullRequests),
	}, true
}

// joinPullRequestKeys renders the entries' binding keys for a refusal message.
func joinPullRequestKeys(entries []PullRequestEntry) string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return strings.Join(keys, ", ")
}
