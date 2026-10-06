// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"regexp"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/task-answer-operation.go --fake-name TaskAnswerOperation . TaskAnswerOperation
type TaskAnswerOperation interface {
	// Execute records the operator's answers into the named task's `Open
	// Questions` section, one answer per named item, and writes the task back in
	// a single write.
	//
	// Each answer names an item by its 1-based index and is rendered as
	// `<original marker and indentation><question text> → **<answer>**`, so the
	// file's own list style is preserved and each question keeps its answer on
	// the same line. Answering the same item again replaces the previous answer
	// rather than adding a second one, so the command is safe to re-run.
	//
	// Every other line — the frontmatter, the heading, the other sections, and
	// the items the caller did not answer — is left byte-identical. An index that
	// names no item in the section, and any answer for a task that has no such
	// section, is refused with nothing written; the error names the offending
	// index and the task.
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		answers []domain.OpenAnswer,
	) (MutationResult, error)
}

// NewTaskAnswerOperation creates a new task answer operation.
func NewTaskAnswerOperation(taskStorage storage.TaskStorage) TaskAnswerOperation {
	return &taskAnswerOperation{taskStorage: taskStorage}
}

type taskAnswerOperation struct {
	taskStorage storage.TaskStorage
}

// These mirror the storage reader's section parsing. The rewrite needs the
// marker and the physical line, which the reader's []domain.OpenQuestion does
// not carry, so the op locates the section itself; the question text and the
// set of valid indices come from the reader, never from this scan.
var (
	answerItemRegex    = regexp.MustCompile(`^([ \t]*)([-*]|\d+\.)([ \t]+)(.*)$`)
	answerHeadingRegex = regexp.MustCompile(`^(#{1,6})[ \t]+(.*?)[ \t]*$`)
	answerSuffixRegex  = regexp.MustCompile(`^(.*) → \*\*.*\*\*$`)
)

// Execute records the operator's answers into the task's Open Questions section.
func (o *taskAnswerOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	answers []domain.OpenAnswer,
) (MutationResult, error) {
	// The reader is the authority on what the section holds: its ordered texts
	// and its item count are what every index below is validated against.
	questions, err := o.taskStorage.ReadOpenQuestions(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(
			ctx,
			err,
			"read open questions",
		)
	}

	answerByIndex := make(map[int]string, len(answers))
	for _, answer := range answers {
		if answer.Index < 1 || answer.Index > len(questions) {
			err := errors.Errorf(
				ctx,
				"refusing to answer %q: no open question %d (the Open Questions section has %d item(s))",
				taskName, answer.Index, len(questions),
			)
			return MutationResult{Success: false, Error: err.Error()}, err
		}
		answerByIndex[answer.Index] = answer.Answer
	}
	if len(answerByIndex) == 0 {
		return MutationResult{Success: true, Name: taskName, Vault: vaultName}, nil
	}

	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "find task")
	}

	lines := strings.Split(string(task.Content), "\n")
	start, end := answerSectionBounds(lines)
	if start < 0 {
		err := errors.Errorf(
			ctx,
			"refusing to answer %q: no Open Questions section found",
			taskName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// Walk the section's top-level items in order, counting exactly the items the
	// reader counted (blank items skipped), so the running index addresses the
	// same question the caller named.
	index := 0
	for i := start + 1; i < end; i++ {
		marker, text, ok := answerItem(lines[i])
		if !ok || text == "" {
			continue
		}
		index++
		answer, answered := answerByIndex[index]
		if !answered {
			continue
		}
		lines[i] = marker + stripAnswerSuffix(text) + " → **" + answer + "**"
	}

	task.Content = domain.Content(strings.Join(lines, "\n"))
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write task")
	}

	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
}

// answerSectionBounds returns the half-open line range [start+1, end) of the
// first `Open Questions` section, and (-1, -1) when there is none. It mirrors
// the storage reader's section bounds so both agree on where the section is.
func answerSectionBounds(lines []string) (int, int) {
	start, level := -1, 0
	for i, line := range lines {
		lineLevel, text, ok := answerHeading(line)
		if !ok {
			continue
		}
		if start < 0 {
			if text == "Open Questions" {
				start, level = i, lineLevel
			}
			continue
		}
		if lineLevel <= level {
			return start, i
		}
	}
	if start < 0 {
		return -1, -1
	}
	return start, len(lines)
}

// answerHeading returns the level and text of an ATX markdown heading line.
func answerHeading(line string) (int, string, bool) {
	matches := answerHeadingRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		return 0, "", false
	}
	return len(matches[1]), matches[2], true
}

// answerItem returns the marker (list marker plus the whitespace after it) and
// the trimmed text of a top-level list item line. An indented item is not a
// top-level item and is rejected.
func answerItem(line string) (string, string, bool) {
	matches := answerItemRegex.FindStringSubmatch(line)
	if len(matches) != 5 || matches[1] != "" {
		return "", "", false
	}
	return matches[2] + matches[3], strings.TrimSpace(matches[4]), true
}

// stripAnswerSuffix removes a trailing ` → **<answer>**` from text, so an item
// that was already answered is rewritten rather than accumulating a second
// answer. Text without such a suffix is returned unchanged.
func stripAnswerSuffix(text string) string {
	if matches := answerSuffixRegex.FindStringSubmatch(text); len(matches) == 2 {
		return matches[1]
	}
	return text
}
