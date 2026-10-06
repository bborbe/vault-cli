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

// answerSuffixRegex matches an item that already carries an answer, so
// re-answering replaces that suffix instead of appending a second one.
var answerSuffixRegex = regexp.MustCompile(`^(.*) → \*\*.*\*\*$`)

// Execute records the operator's answers into the task's Open Questions section.
func (o *taskAnswerOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	answers []domain.OpenAnswer,
) (MutationResult, error) {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "find task")
	}

	// ParseOpenQuestions is the authority on what the section holds: its item
	// count and each item's physical line are what every index below is validated
	// against and rewritten through. Deriving both from the content already in
	// hand keeps the reader and this rewriter on ONE parse, so the index a caller
	// names always addresses the line this op edits — and avoids a second vault
	// walk on a write path that already holds the task.
	items := storage.ParseOpenQuestions(ctx, string(task.Content))

	answerByIndex := make(map[int]string, len(answers))
	for _, answer := range answers {
		select {
		case <-ctx.Done():
			return MutationResult{Success: false, Error: ctx.Err().Error()}, errors.Wrap(
				ctx,
				ctx.Err(),
				"context cancelled",
			)
		default:
		}
		if answer.Index < 1 || answer.Index > len(items) {
			err := errors.Errorf(
				ctx,
				"refusing to answer %q: no open question %d (the Open Questions section has %d item(s))",
				taskName, answer.Index, len(items),
			)
			return MutationResult{Success: false, Error: err.Error()}, err
		}
		// An answer is written into the task file as a single line. A line break
		// would split it across two, and a fragment shaped like an ATX heading
		// would inject a real heading — changing how every later parse of this
		// task behaves, including this command's own index mapping. Refuse the
		// whole batch rather than write a file that no longer round-trips.
		if strings.ContainsAny(answer.Answer, "\r\n") {
			err := errors.Errorf(
				ctx,
				"refusing to answer %q: the answer to question %d contains a line break, "+
					"and an answer must stay on one line",
				taskName, answer.Index,
			)
			return MutationResult{Success: false, Error: err.Error()}, err
		}
		answerByIndex[answer.Index] = answer.Answer
	}
	if len(answerByIndex) == 0 {
		return MutationResult{Success: true, Name: taskName, Vault: vaultName}, nil
	}

	lines, err := applyAnswers(
		ctx,
		strings.Split(string(task.Content), "\n"),
		items,
		answerByIndex,
	)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	task.Content = domain.Content(strings.Join(lines, "\n"))
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "write task")
	}

	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
}

// applyAnswers rewrites the answered items in place, leaving every other line
// untouched. Each item is addressed by the line the shared parse reported, so
// the index the caller named and the line this op edits cannot drift apart. The
// item's own marker, indentation and line ending are preserved, and an existing
// ` → **…**` suffix is replaced rather than appended to, so answering twice is
// idempotent.
func applyAnswers(
	ctx context.Context,
	lines []string,
	items []storage.OpenQuestionItem,
	answerByIndex map[int]string,
) ([]string, error) {
	for _, item := range items {
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx, ctx.Err(), "context cancelled")
		default:
		}
		answer, answered := answerByIndex[item.Index]
		if !answered {
			continue
		}
		// A CRLF file leaves a trailing \r on the line; keep it so the rewritten
		// line does not become the only LF line in an otherwise-CRLF file.
		ending := ""
		if strings.HasSuffix(lines[item.Line], "\r") {
			ending = "\r"
		}
		lines[item.Line] = item.Marker +
			stripAnswerSuffix(item.Text) + " → **" + answer + "**" + ending
	}
	return lines, nil
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
