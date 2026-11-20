// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// closureKind selects the section set a closure gate reads.
type closureKind string

const (
	// closureKindGoal gates a goal on Success Criteria + Definition of Done.
	closureKindGoal closureKind = "goal"
	// closureKindTopic gates a topic on Success Criteria + Completion Gate.
	closureKindTopic closureKind = "topic"
)

// closureOutcome is the verdict of a closure gate.
type closureOutcome string

const (
	// closureOutcomeOpen means the page is measurable and at least one box or member is open.
	closureOutcomeOpen closureOutcome = "OPEN"
	// closureOutcomeUnmeasurable means the page's evidence cannot be read at all.
	closureOutcomeUnmeasurable closureOutcome = "UNMEASURABLE"
	// closureOutcomeClose means every box is ticked and every member is terminal.
	closureOutcomeClose closureOutcome = "CLOSE"
	// closureOutcomeTerminal means the page is already completed or aborted.
	closureOutcomeTerminal closureOutcome = "TERMINAL"
)

// closureMember is a linked task (goal kind) or a member goal (topic kind).
// Fields are exported so the external ops_test package can build values.
type closureMember struct {
	Name   string
	Status string
}

// closureVerdict is the result of checkClosureGate.
type closureVerdict struct {
	Outcome   closureOutcome
	OpenItems []string
}

// dodRequiredAsOf is the grandfathering cutoff. It is the fourth copy of the
// value: claude-supervisor/scripts/goal-closure-read.py, agents/goal-auditor.md
// and agents/task-auditor.md carry the other three. They move together.
const dodRequiredAsOf = "2026-06-26"

var (
	// closureCheckboxRegex matches a checkbox item with any bullet marker and any
	// single-character marker. Capture 1 is the marker, 2 the item text.
	closureCheckboxRegex = regexp.MustCompile(`^[ \t]*[-*+][ \t]+\[(.)\][ \t]*(.*)$`)

	// closureListItemRegex matches any markdown list item, checkbox or not.
	closureListItemRegex = regexp.MustCompile(`^[ \t]*([-*+]|\d+\.)[ \t]+`)

	// closureHeadingRegex matches an ATX heading line. Capture 1 is the `#` run.
	closureHeadingRegex = regexp.MustCompile(`^(#{1,6})[ \t]+`)

	// closureISODateRegex matches a zero-padded ISO date.
	closureISODateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	// closureGoalEntryRegex matches a Goals-section entry declaration.
	closureGoalEntryRegex = regexp.MustCompile(`^- \[\[([^\]]+)\]\]`)
)

// checkClosureGate reads the page's own closure evidence and returns a verdict.
// It is the single implementation behind both `goal complete` and
// `topic complete`; neither verb may carry a second copy of the rule.
//
// The section set is fixed by kind, never guessed from the page:
// goal -> "# Success Criteria" + "# Definition of Done";
// topic -> "# Success Criteria" + "# Completion Gate".
//
// The check is read-only. A page already at completed/aborted returns TERMINAL
// with no items. On any refusal OpenItems holds one human-readable string per
// blocking item, in document order (sections first, then members).
func checkClosureGate(
	ctx context.Context,
	kind closureKind,
	content string,
	members []closureMember,
) (closureVerdict, error) {
	doc := normaliseDocument(content)
	fields, err := storage.ParseFrontmatterMap(ctx, []byte(doc))
	if err != nil {
		return closureVerdict{}, errors.Wrap(ctx, err, "parse frontmatter")
	}

	if status := stringField(fields, "status"); isTerminalStatus(status) {
		return closureVerdict{Outcome: closureOutcomeTerminal}, nil
	}

	body, err := stripFrontmatter(ctx, doc)
	if err != nil {
		return closureVerdict{}, err
	}
	body, fenceOK := stripFencedBlocks(body)
	if !fenceOK {
		return closureVerdict{
			Outcome:   closureOutcomeUnmeasurable,
			OpenItems: []string{"unterminated code fence"},
		}, nil
	}

	items, unmeasurable := closureSectionItems(kind, body, createdField(fields))
	items = append(items, closureMemberItems(kind, members)...)

	outcome := closureOutcomeClose
	switch {
	case unmeasurable:
		outcome = closureOutcomeUnmeasurable
	case len(items) > 0:
		outcome = closureOutcomeOpen
	}
	return closureVerdict{Outcome: outcome, OpenItems: items}, nil
}

// closureSectionItems walks the kind's fixed section set and returns the open
// section items plus whether any of them is unmeasurable.
func closureSectionItems(
	kind closureKind,
	body string,
	created string,
) (items []string, unmeasurable bool) {
	for _, heading := range closureHeadings(kind) {
		lines, found := findSection(body, heading)
		var sectionItems []string
		var sectionUnmeasurable bool
		if found {
			sectionItems, sectionUnmeasurable = presentSectionItems(kind, heading, lines)
		} else {
			sectionItems, sectionUnmeasurable = absentSectionItems(kind, heading, created, body)
		}
		items = append(items, sectionItems...)
		unmeasurable = unmeasurable || sectionUnmeasurable
	}
	return items, unmeasurable
}

// absentSectionItems handles a section that the page does not carry.
func absentSectionItems(
	kind closureKind,
	heading string,
	created string,
	body string,
) ([]string, bool) {
	if kind == closureKindGoal && heading == "# Definition of Done" {
		exempt, message := grandfatherDefinitionOfDone(created, body)
		if exempt {
			return nil, false
		}
		return []string{message}, true
	}
	return []string{heading + ": section absent"}, true
}

// presentSectionItems handles a section the page does carry.
func presentSectionItems(
	kind closureKind,
	heading string,
	lines []string,
) ([]string, bool) {
	open, total := sectionCheckboxItems(lines)
	if total == 0 {
		return []string{heading + ": section contains no checkbox"}, true
	}
	items := open
	if kind != closureKindTopic || heading != "# Completion Gate" {
		return items, false
	}
	violations := nonCheckboxListItems(lines)
	for _, violation := range violations {
		items = append(items, "# Completion Gate: non-checkbox list item: "+violation)
	}
	return items, len(violations) > 0
}

// closureHeadings returns the fixed section set for a kind, in document order.
func closureHeadings(kind closureKind) []string {
	if kind == closureKindTopic {
		return []string{"# Success Criteria", "# Completion Gate"}
	}
	return []string{"# Success Criteria", "# Definition of Done"}
}

// grandfatherDefinitionOfDone reports whether an absent Definition of Done
// section is exempt for a goal, and otherwise the refusal item to record.
func grandfatherDefinitionOfDone(created string, body string) (bool, string) {
	if hasDoDShapedHeading(body) {
		return false, "# Definition of Done: section absent"
	}
	if created == "" {
		return true, ""
	}
	if !closureISODateRegex.MatchString(created) {
		return false, fmt.Sprintf(
			"# Definition of Done: section absent (created %q is not an ISO date)",
			created,
		)
	}
	if created < dodRequiredAsOf {
		return true, ""
	}
	return false, fmt.Sprintf(
		"# Definition of Done: section absent (created %s is not before %s)",
		created,
		dodRequiredAsOf,
	)
}

// closureMemberItems returns the refusal items for non-terminal members.
func closureMemberItems(kind closureKind, members []closureMember) []string {
	var items []string
	for _, member := range members {
		if !closureMemberOpen(kind, member.Status) {
			continue
		}
		if member.Status == "" {
			items = append(items, member.Name)
			continue
		}
		items = append(items, fmt.Sprintf("%s (%s)", member.Name, member.Status))
	}
	return items
}

// closureMemberOpen reports whether a member status blocks a close for the kind.
func closureMemberOpen(kind closureKind, status string) bool {
	if status == "" {
		return true
	}
	if kind == closureKindGoal {
		switch status {
		case string(domain.TaskStatusNext),
			string(domain.TaskStatusTodo),
			string(domain.TaskStatusInProgress):
			return true
		default:
			return false
		}
	}
	return status != domain.TopicStatusCompleted && status != "aborted"
}

// isTerminalStatus reports whether a page status is completed or aborted.
func isTerminalStatus(status string) bool {
	return status == string(domain.GoalStatusCompleted) ||
		status == string(domain.GoalStatusAborted)
}

// stringField reads a string field, treating a non-string value as empty.
func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

// closureDateLayout is the date-only shape the grandfathering cutoff compares.
const closureDateLayout = "2006-01-02"

// createdField reads the `created` frontmatter value as a bare ISO date.
//
// ⚠️ The value must be COERCED, never type-asserted to string. YAML resolves an
// unquoted `created: 2026-07-01` — and an unquoted RFC3339 timestamp — to
// time.Time, so a string-only accessor yields "", which is indistinguishable
// from an absent field and therefore GRANDFATHERS the Definition of Done. That
// fails open: a post-cutoff goal with no DoD closes unexamined, which is the one
// case the exemption must refuse. Measured 2026-11-19: 70 live goal pages in the
// Personal vault carry the unquoted form.
//
// An absent field returns "" (genuinely grandfathered). A present value of an
// unreadable type returns its raw spelling, so the caller refuses it as
// "not an ISO date" rather than exempting the page.
func createdField(fields map[string]any) string {
	value, ok := fields["created"]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case time.Time:
		return typed.Format(closureDateLayout)
	case string:
		return trimToDate(typed)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// trimToDate returns the date part of a timestamp, or the input unchanged when
// it carries no time component.
func trimToDate(value string) string {
	if index := strings.IndexAny(value, "T "); index > 0 {
		return value[:index]
	}
	return value
}

// normaliseDocument strips one leading BOM and normalises CRLF and CR to LF.
func normaliseDocument(content string) string {
	doc := strings.TrimPrefix(content, "\ufeff")
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	return strings.ReplaceAll(doc, "\r", "\n")
}

// frontmatterBody returns the body after the frontmatter block.
func frontmatterBody(doc string) (string, bool) {
	const open = "---\n"
	const delimiter = "\n---\n"
	if !strings.HasPrefix(doc, open) {
		return "", false
	}
	rest := doc[len(open):]
	index := strings.Index(rest, delimiter)
	if index < 0 {
		return "", false
	}
	return rest[index+len(delimiter):], true
}

// stripFrontmatter returns the body after the frontmatter block.
func stripFrontmatter(ctx context.Context, doc string) (string, error) {
	body, ok := frontmatterBody(doc)
	if !ok {
		return "", errors.Errorf(ctx, "frontmatter block not found")
	}
	return body, nil
}

// stripFencedBlocks drops fenced code blocks and reports whether every fence
// was terminated.
func stripFencedBlocks(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	var fence string
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if fence == "" {
			if strings.HasPrefix(trimmed, "```") {
				fence = "`"
				continue
			}
			if strings.HasPrefix(trimmed, "~~~") {
				fence = "~"
				continue
			}
			kept = append(kept, line)
			continue
		}
		if strings.HasPrefix(trimmed, strings.Repeat(fence, 3)) {
			fence = ""
		}
	}
	return strings.Join(kept, "\n"), fence == ""
}

// headingLevel returns the ATX heading level of a line, or 0 for a non-heading.
func headingLevel(line string) int {
	matches := closureHeadingRegex.FindStringSubmatch(line)
	if matches == nil {
		return 0
	}
	return len(matches[1])
}

// headingText returns the trimmed text of an ATX heading line.
func headingText(line string) string {
	matches := closureHeadingRegex.FindStringSubmatch(line)
	if matches == nil {
		return ""
	}
	return strings.TrimSpace(line[len(matches[1]):])
}

// findSection returns the body lines of the first level-1 heading with the
// given exact text, up to the next level-1 heading or end of document.
func findSection(body string, heading string) ([]string, bool) {
	target := strings.TrimSpace(strings.TrimPrefix(heading, "#"))
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		level := headingLevel(line)
		if start < 0 {
			if level == 1 && headingText(line) == target {
				start = i
			}
			continue
		}
		if level == 1 {
			return lines[start+1 : i], true
		}
	}
	if start < 0 {
		return nil, false
	}
	return lines[start+1:], true
}

// hasDoDShapedHeading reports whether any line spells a Definition of Done
// heading in any form: ATX, setext text line, or a list item.
func hasDoDShapedHeading(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		text := strings.TrimSpace(line)
		text = strings.TrimSpace(strings.TrimLeft(text, "#"))
		if len(text) > 0 && (text[0] == '-' || text[0] == '*' || text[0] == '+') {
			rest := text[1:]
			if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
				text = strings.TrimSpace(rest)
			}
		}
		if text == "Definition of Done" {
			return true
		}
	}
	return false
}

// sectionCheckboxItems returns the open checkbox items and the total checkbox
// count for a section. Only `x` and `X` count as done.
func sectionCheckboxItems(lines []string) ([]string, int) {
	var open []string
	total := 0
	for _, line := range lines {
		matches := closureCheckboxRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		total++
		if matches[1] == "x" || matches[1] == "X" {
			continue
		}
		text := strings.TrimSpace(matches[2])
		if text == "" {
			text = strings.TrimSpace(line)
		}
		open = append(open, text)
	}
	return open, total
}

// nonCheckboxListItems returns every list item in lines that is not a checkbox.
func nonCheckboxListItems(lines []string) []string {
	var violations []string
	for _, line := range lines {
		if !closureListItemRegex.MatchString(line) {
			continue
		}
		if closureCheckboxRegex.MatchString(line) {
			continue
		}
		violations = append(violations, strings.TrimSpace(line))
	}
	return violations
}

// parseGoalSectionMembers returns the goal names declared in a topic page's
// Goals section, bounded level-relative.
func parseGoalSectionMembers(content string) []string {
	doc := normaliseDocument(content)
	body, ok := frontmatterBody(doc)
	if !ok {
		body = doc
	}
	body, fenceOK := stripFencedBlocks(body)
	if !fenceOK {
		return nil
	}

	lines := strings.Split(body, "\n")
	start := -1
	level := 0
	for i, line := range lines {
		current := headingLevel(line)
		if current < 1 || current > 2 {
			continue
		}
		if headingText(line) == "Goals" {
			start = i
			level = current
			break
		}
	}
	if start < 0 {
		return nil
	}

	var names []string
	seen := make(map[string]bool)
	for _, line := range lines[start+1:] {
		current := headingLevel(line)
		if current > 0 && current <= level {
			break
		}
		matches := closureGoalEntryRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		name := normaliseGoalLink(matches[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// normaliseGoalLink strips the wikilink wrapper, any alias/heading suffix and
// any leading directory path so '[[24 Goals/Some Goal]]', '[[Some Goal]]',
// '[[Some Goal|label]]' and 'Some Goal' compare equal. A goal or topic name is
// a filename, so a '/' can only be a path separator and never part of the name.
func normaliseGoalLink(link string) string {
	name := strings.TrimSpace(link)
	name = strings.TrimPrefix(name, "[[")
	name = strings.TrimSuffix(name, "]]")
	if index := strings.Index(name, "|"); index >= 0 {
		name = name[:index]
	}
	if index := strings.Index(name, "#"); index >= 0 {
		name = name[:index]
	}
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	return strings.TrimSpace(name)
}

// closureRefusalMessage builds the single-line error text for a refusal.
func closureRefusalMessage(subject string, items []string) string {
	return fmt.Sprintf(
		"cannot complete %s: %d closure item(s) open: %s",
		subject,
		len(items),
		strings.Join(items, "; "),
	)
}
