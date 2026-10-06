// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
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
	items, err := storage.ParseOpenQuestions(ctx, string(task.Content))
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	answerByIndex, err := answerIndex(ctx, taskName, answers, len(items))
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
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

// answerIndex validates the answers and collapses them into a map keyed by the
// question they address. It refuses an index that names no item, an answer the
// one-line format cannot carry, and a duplicate index — so the whole batch is
// rejected before the task file is touched, and a caller cannot be left unable
// to tell which of its flags took effect.
func answerIndex(
	ctx context.Context,
	taskName string,
	answers []domain.OpenAnswer,
	itemCount int,
) (map[int]string, error) {
	answerByIndex := make(map[int]string, len(answers))
	for _, answer := range answers {
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx, ctx.Err(), "context cancelled")
		default:
		}
		if answer.Index < 1 || answer.Index > itemCount {
			return nil, errors.Errorf(
				ctx,
				"refusing to answer %q: no open question %d (the Open Questions section has %d item(s))",
				taskName, answer.Index, itemCount,
			)
		}
		// An answer must survive being written and read back. Each of these breaks
		// that: a line break splits the answer across two lines (and a fragment
		// shaped like an ATX heading would then inject a real one, changing how
		// every later parse of this task behaves, including this command's own
		// index mapping); the delimiter is where the question ends; and `**` closes
		// the emphasis the answer is written inside. Refuse the whole batch rather
		// than write a file that no longer round-trips.
		for _, bad := range []struct{ seq, why string }{
			{"\n", "a line break would split it across two lines"},
			{"\r", "a line break would split it across two lines"},
			{storage.AnswerDelimiter, "it is what separates a question from its answer"},
			{"**", "it would close the emphasis the answer is written in"},
		} {
			if !strings.Contains(answer.Answer, bad.seq) {
				continue
			}
			return nil, errors.Errorf(
				ctx,
				"refusing to answer %q: the answer to question %d contains %q — %s",
				taskName, answer.Index, bad.seq, bad.why,
			)
		}
		if _, duplicate := answerByIndex[answer.Index]; duplicate {
			return nil, errors.Errorf(
				ctx,
				"refusing to answer %q: question %d is answered more than once",
				taskName, answer.Index,
			)
		}
		answerByIndex[answer.Index] = answer.Answer
	}
	return answerByIndex, nil
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
		// item.Question is the shared parse's own split, so this replaces exactly
		// the text the reader called the question — on a re-answer it discards the
		// answer the reader already separated out, rather than re-deriving where
		// the previous answer began.
		lines[item.Line] = storage.FormatOpenQuestionItem(item, answer, ending)
	}
	return lines, nil
}
