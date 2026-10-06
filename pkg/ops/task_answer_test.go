// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("TaskAnswerOperation", func() {
	var (
		ctx       context.Context
		vaultPath string
		answerOp  ops.TaskAnswerOperation
	)

	// The operation is driven against a real task storage on a temp vault so the
	// refusal specs can compare the file's bytes before and after, and the
	// success specs can assert that only the answered line changed.
	const taskWithQuestions = `---
page_type: task
status: next
task_identifier: 11111111-1111-4111-8111-111111111111
---
# Summary

Body.

# Open Questions

- Which database?
- How long is the window?
- Who owns the rollout?

# Progress

- 2026-01-01 started
`

	const taskWithoutQuestions = `---
page_type: task
status: next
task_identifier: 22222222-2222-4222-8222-222222222222
---
# Summary

Body.
`

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		vaultPath, err = os.MkdirTemp("", "vault-answer-test-*")
		Expect(err).NotTo(HaveOccurred())
		answerOp = ops.NewTaskAnswerOperation(
			storage.NewTaskStorage(storage.DefaultConfig()),
		)
	})

	AfterEach(func() {
		_ = os.RemoveAll(vaultPath)
	})

	taskPath := func() string {
		return filepath.Join(vaultPath, "Tasks", "Alpha.md")
	}

	writeTask := func(content string) {
		Expect(os.MkdirAll(filepath.Join(vaultPath, "Tasks"), 0750)).To(Succeed())
		Expect(os.WriteFile(taskPath(), []byte(content), 0600)).To(Succeed())
	}

	readTask := func() string {
		data, err := os.ReadFile(taskPath()) //#nosec G304 -- test file
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}

	// changedLineIndices returns the positions at which before and after differ.
	// It fails when the two have a different number of lines, so a rewrite that
	// added or removed a line is caught rather than silently compared.
	changedLineIndices := func(before, after string) []int {
		beforeLines := strings.Split(before, "\n")
		afterLines := strings.Split(after, "\n")
		Expect(afterLines).To(HaveLen(len(beforeLines)))
		var changed []int
		for i := range beforeLines {
			if beforeLines[i] != afterLines[i] {
				changed = append(changed, i)
			}
		}
		return changed
	}

	It("records the answer and changes only the answered line", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		result, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "One minute"}},
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.Name).To(Equal("Alpha"))
		Expect(result.Vault).To(Equal("test"))

		after := readTask()
		Expect(after).To(ContainSubstring("- How long is the window? → **One minute**"))
		changed := changedLineIndices(before, after)
		Expect(changed).To(HaveLen(1))
		Expect(strings.Split(after, "\n")[changed[0]]).
			To(Equal("- How long is the window? → **One minute**"))
	})

	It("answers several questions and leaves the unanswered ones untouched", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{
				{Index: 1, Answer: "Postgres"},
				{Index: 3, Answer: "The platform team"},
			},
		)
		Expect(err).NotTo(HaveOccurred())

		after := readTask()
		Expect(after).To(ContainSubstring("- Which database? → **Postgres**"))
		Expect(after).To(ContainSubstring("- Who owns the rollout? → **The platform team**"))
		Expect(after).To(ContainSubstring("- How long is the window?\n"))
		Expect(changedLineIndices(before, after)).To(HaveLen(2))
	})

	It("preserves a numbered list marker", func() {
		writeTask(`---
page_type: task
status: next
---
# Open Questions

1. Alpha?
2. Beta?
`)

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "Yes"}},
		)
		Expect(err).NotTo(HaveOccurred())

		after := readTask()
		Expect(after).To(ContainSubstring("2. Beta? → **Yes**"))
		Expect(after).NotTo(ContainSubstring("- Beta?"))
	})

	It("replaces the previous answer instead of appending a second one", func() {
		writeTask(taskWithQuestions)

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "First"}},
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "Second"}},
		)
		Expect(err).NotTo(HaveOccurred())

		after := readTask()
		Expect(strings.Count(after, " → **")).To(Equal(1))
		Expect(after).To(ContainSubstring("- How long is the window? → **Second**"))
		Expect(after).NotTo(ContainSubstring("First"))
	})

	It("refuses an index that names no item and writes nothing", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 9, Answer: "nowhere"}},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("9"))
		Expect(err.Error()).To(ContainSubstring("Alpha"))
		Expect(readTask()).To(Equal(before))
	})

	It("refuses answers for a task with no Open Questions section and writes nothing", func() {
		writeTask(taskWithoutQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 1, Answer: "nowhere"}},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("1"))
		Expect(err.Error()).To(ContainSubstring("Alpha"))
		Expect(readTask()).To(Equal(before))
	})

	It("refuses a whole batch when one index is invalid, writing nothing", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{
				{Index: 1, Answer: "valid"},
				{Index: 9, Answer: "invalid"},
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(readTask()).To(Equal(before))
	})

	It("wraps a find failure", func() {
		_, err := answerOp.Execute(
			ctx, vaultPath, "Missing", "test",
			[]domain.OpenAnswer{{Index: 1, Answer: "x"}},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("find task"))
	})

	// The answer is written into the task file as a single line. A break would
	// split it across two, and a fragment shaped like an ATX heading would inject
	// a real heading — changing how every later parse of the task behaves,
	// including this command's own index mapping.
	It("refuses an answer containing a line break and writes nothing", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 1, Answer: "line one\n# Progress\nline two"}},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("line break"))
		Expect(readTask()).To(Equal(before))
	})

	It("refuses a whole batch when one answer contains a line break", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{
				{Index: 1, Answer: "fine"},
				{Index: 2, Answer: "has a\rbreak"},
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(readTask()).To(Equal(before))
	})

	// The delimiter is where the question ends, so an answer carrying it would
	// make the written line ambiguous and a later re-answer would split at the
	// wrong place — destroying the previous answer and grafting part of it into
	// the question.
	It("refuses an answer containing the question/answer delimiter", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 1, Answer: "Use Redis → **HA**"}},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("separates a question from its answer"))
		Expect(readTask()).To(Equal(before))
	})

	// Two answers for one index would silently keep the last, leaving the caller
	// unable to tell which of its flags took effect.
	It("refuses a duplicate index and writes nothing", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{
				{Index: 1, Answer: "first"},
				{Index: 1, Answer: "second"},
			},
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("answered more than once"))
		Expect(readTask()).To(Equal(before))
	})

	// The reader splits a recorded answer off its question, so re-answering
	// replaces the answer rather than appending to it. This is the round-trip
	// that a re-derived split got wrong.
	It("replaces a recorded answer without disturbing the question", func() {
		writeTask(taskWithQuestions)
		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "one week"}},
		)
		Expect(err).NotTo(HaveOccurred())

		_, err = answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 2, Answer: "two weeks"}},
		)
		Expect(err).NotTo(HaveOccurred())

		after := readTask()
		Expect(after).To(ContainSubstring("- How long is the window? → **two weeks**"))
		Expect(after).NotTo(ContainSubstring("one week"))
		Expect(strings.Count(after, "→ **")).To(Equal(1))
	})

	// A CRLF body leaves a trailing \r on every line. The rewritten line must
	// keep it, or it becomes the only LF line in an otherwise-CRLF file.
	//
	// The frontmatter stays LF deliberately: the storage layer's frontmatter
	// parser rejects a fully-CRLF file outright ("no frontmatter found"), so a
	// CRLF body over LF frontmatter is the only shape that reaches this code.
	It("preserves CRLF line endings when it answers", func() {
		// Split after the closing frontmatter fence, keeping it LF.
		splitAt := 4 + strings.Index(taskWithQuestions[4:], "---\n") + 4
		lfFrontmatter, body := taskWithQuestions[:splitAt], taskWithQuestions[splitAt:]
		writeTask(lfFrontmatter + strings.ReplaceAll(body, "\n", "\r\n"))
		before := readTask()

		_, err := answerOp.Execute(
			ctx, vaultPath, "Alpha", "test",
			[]domain.OpenAnswer{{Index: 1, Answer: "Postgres"}},
		)
		Expect(err).NotTo(HaveOccurred())

		after := readTask()
		Expect(after).To(ContainSubstring("- Which database? → **Postgres**\r\n"))
		Expect(changedLineIndices(before, after)).To(HaveLen(1))
		// Every line of the body still ends CRLF — the rewrite did not convert the
		// answered line to LF. The count is scoped to the body because the
		// pre-existing WriteTask re-serializes the frontmatter as LF.
		writtenBody := after[strings.Index(after, "# Summary"):]
		Expect(strings.Count(writtenBody, "\r\n")).To(Equal(strings.Count(writtenBody, "\n")))
	})

	It("succeeds without writing when no answers are supplied", func() {
		writeTask(taskWithQuestions)
		before := readTask()

		result, err := answerOp.Execute(ctx, vaultPath, "Alpha", "test", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(readTask()).To(Equal(before))
	})
})
