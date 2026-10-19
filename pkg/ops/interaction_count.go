// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
)

//counterfeiter:generate -o ../../mocks/interaction-counter.go --fake-name InteractionCounter . InteractionCounter

// InteractionCounter counts user interactions from a task's recorded Claude session logs.
type InteractionCounter interface {
	// Count returns the total number of type:"user" entries across the session JSONL
	// logs for the given session ids. Missing, unreadable, malformed, or unsafe session
	// ids contribute 0. It never returns an error and never blocks completion.
	Count(ctx context.Context, sessionIDs []string) int

	// CountOperatorPrompts returns the number of entries across the session JSONL
	// logs for the given session ids that the operator typed: user turns that are
	// not tool results, not isMeta injections, not non-human turn origins, not
	// system prompt sources, and not one of the known legacy noise prefixes.
	// Missing, unreadable, malformed, or unsafe session ids contribute 0. It never
	// returns an error and never blocks completion.
	CountOperatorPrompts(ctx context.Context, sessionIDs []string) int
}

// NewInteractionCounter creates an InteractionCounter that reads session logs under
// projectsDir/<encoded sessionDir>/. projectsDir is the Claude Code projects base
// (typically <home>/.claude/projects); sessionDir is the cwd the sessions were started
// in (the vault path or its session_project_dir override).
func NewInteractionCounter(projectsDir string, sessionDir string) InteractionCounter {
	return &interactionCounter{projectsDir: projectsDir, sessionDir: sessionDir}
}

type interactionCounter struct {
	projectsDir string
	sessionDir  string
}

// Count returns the total number of user turns across the session logs for the
// given session ids, counting each distinct id once. An unsafe id is skipped
// before any path is built.
func (c *interactionCounter) Count(ctx context.Context, sessionIDs []string) int {
	return c.sum(ctx, sessionIDs, countUserTurnsInSessionLog)
}

// CountOperatorPrompts returns the number of operator-typed prompts across the
// session logs for the given session ids, counting each distinct id once.
func (c *interactionCounter) CountOperatorPrompts(
	ctx context.Context,
	sessionIDs []string,
) int {
	return c.sum(ctx, sessionIDs, countOperatorPromptsInSessionLog)
}

// sum walks the distinct, safe session ids in order and adds one per-file count
// for each. countFile is either the all-user counter or the operator-prompt
// counter, so both measures share the same traversal rules.
func (c *interactionCounter) sum(
	ctx context.Context,
	sessionIDs []string,
	countFile func(context.Context, string) int,
) int {
	total := 0
	seen := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		select {
		case <-ctx.Done():
			return total
		default:
		}
		if !isSafeSessionID(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		path := filepath.Join(c.projectsDir, encodeProjectDir(c.sessionDir), id+".jsonl")
		total += countFile(ctx, path)
	}
	return total
}

// isSafeSessionID reports whether id can be used as a file basename inside the
// encoded project directory. This is the spec's path-traversal guard: a session id
// from the task must never escape the encoded project directory.
func isSafeSessionID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) {
		return false
	}
	if strings.Contains(id, "..") {
		return false
	}
	return true
}

// encodeProjectDir encodes a session cwd the way Claude Code encodes it for the
// projects directory: every "/" (including a leading one) becomes "-" and every
// other character outside [A-Za-z0-9-] becomes "-". E.g.
// /Users/bborbe/Documents/vault -> -Users-bborbe-Documents-vault.
func encodeProjectDir(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for _, r := range path {
		switch {
		case r == '/':
			b.WriteByte('-')
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// countUserTurnsInSessionLog counts the type:"user" lines in one session JSONL log.
// A missing, unreadable, or malformed log contributes 0 and never fails; the log is
// streamed line by line so a very large session never loads fully into memory.
func countUserTurnsInSessionLog(ctx context.Context, path string) int {
	f, err := os.Open(path) //#nosec G304 -- session ids are validated by isSafeSessionID; read-only best-effort
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return count
		default:
		}
		var entry struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type == "user" {
			count++
		}
	}
	return count
}

// operatorPromptSources are the promptSource values that identify an
// operator-typed prompt. Every other non-empty value (notably "system") is
// excluded.
var operatorPromptSources = map[string]struct{}{
	"typed":               {},
	"queued":              {},
	"suggestion_accepted": {},
}

// operatorPromptNoisePrefixes are the legacy transcript rows that carry none of
// the signal fields and are not operator prompts. A legacy row whose text starts
// with one of these is excluded. Slash-command markers are deliberately absent:
// <command-name> and <command-message> rows are operator-typed and count.
var operatorPromptNoisePrefixes = []string{
	"<local-command-stdout>",
	"<local-command-caveat>",
	"Caveat: The messages below",
	"This session is being continued",
	"[Request interrupted by user]",
	"<task-notification>",
	"Another Claude session sent a message:",
}

// operatorPromptEntry is the subset of a session JSONL line the operator-prompt
// predicate reads. A field left absent decodes to its zero value; ToolUseResult
// is a RawMessage so presence (including a literal null) is distinguishable.
type operatorPromptEntry struct {
	Type          string          `json:"type"`
	IsMeta        bool            `json:"isMeta"`
	TurnOrigin    string          `json:"turnOrigin"`
	PromptSource  string          `json:"promptSource"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
	Message       struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// countOperatorPromptsInSessionLog counts the operator-typed prompts in one
// session JSONL log. A missing, unreadable, or malformed log contributes 0 and
// never fails; the log is streamed line by line so a very large session never
// loads fully into memory.
func countOperatorPromptsInSessionLog(ctx context.Context, path string) int {
	f, err := os.Open(path) //#nosec G304 -- session ids are validated by isSafeSessionID; read-only best-effort
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return count
		default:
		}
		var entry operatorPromptEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type != "user" {
			continue
		}
		if isOperatorPrompt(entry) {
			count++
		}
	}
	return count
}

// isOperatorPrompt reports whether one type:"user" entry is a prompt the
// operator typed, applying the measured predicate.
func isOperatorPrompt(entry operatorPromptEntry) bool {
	if len(entry.ToolUseResult) > 0 {
		return false
	}
	text, hasToolResult := operatorPromptText(entry.Message.Content)
	if hasToolResult {
		return false
	}
	if entry.IsMeta {
		return false
	}
	if entry.TurnOrigin != "" && entry.TurnOrigin != "human" {
		return false
	}
	if entry.PromptSource != "" {
		if _, ok := operatorPromptSources[entry.PromptSource]; !ok {
			return false
		}
	}
	legacy := entry.TurnOrigin == "" && entry.PromptSource == ""
	if legacy && hasOperatorPromptNoisePrefix(text) {
		return false
	}
	return true
}

// operatorPromptText returns the entry's text and whether its content carries a
// tool_result block. Content may be a plain string or an array of blocks; only
// text blocks contribute to the returned text.
func operatorPromptText(content json.RawMessage) (string, bool) {
	if len(content) == 0 {
		return "", false
	}
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text, false
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return "", false
	}
	var builder strings.Builder
	for _, block := range blocks {
		if block.Type == "tool_result" {
			return "", true
		}
		if block.Type == "text" {
			builder.WriteString(block.Text)
		}
	}
	return builder.String(), false
}

// hasOperatorPromptNoisePrefix reports whether a legacy row's text starts with
// one of the known non-operator prefixes.
func hasOperatorPromptNoisePrefix(text string) bool {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range operatorPromptNoisePrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// metricsSessionIDs extracts the session ids from metrics_sessions entries.
func metricsSessionIDs(sessions []domain.MetricsSession) []string {
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.SessionID)
	}
	return ids
}

// earliestStartedAt returns the minimum StartedAt across the sessions. Callers
// guarantee a non-empty slice; an empty slice returns the zero value rather than
// panicking.
func earliestStartedAt(sessions []domain.MetricsSession) libtime.DateOrDateTime {
	if len(sessions) == 0 {
		return libtime.DateOrDateTime{}
	}
	earliest := sessions[0].StartedAt
	for _, s := range sessions[1:] {
		if s.StartedAt.Before(earliest) {
			earliest = s.StartedAt
		}
	}
	return earliest
}
