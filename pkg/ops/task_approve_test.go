// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	liberrors "github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
	"github.com/bborbe/vault-cli/pkg/storage"
)

var _ = Describe("TaskApproveOperation", func() {
	var (
		ctx         context.Context
		mockStorage *mocks.Storage
		pinned      libtime.CurrentDateTime
		approveOp   ops.TaskApproveOperation
	)

	// seedTask replaces the task the storage returns, so a spec can control the
	// phase and the pre-existing frontmatter the approve guards inspect.
	seedTask := func(fields map[string]any) {
		mockStorage.FindTaskByNameReturns(
			domain.NewTask(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content("")),
			nil,
		)
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		mockStorage.WriteTaskReturns(nil)

		pinned = libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		approveOp = ops.NewTaskApproveOperation(mockStorage, pinned, 0)

		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone"})
	})

	It("writes all three approval keys in exactly one write", func() {
		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())
		Expect(result.Name).To(Equal("Alpha"))
		Expect(result.Vault).To(Equal("vault-a"))

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Status()).To(Equal(domain.TaskStatusNext))
		Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
		Expect(written.GetString("approved_by")).To(Equal("operator"))
		Expect(written.Get("approved_at")).NotTo(BeNil())
	})

	It("preserves an in_progress status across the approval", func() {
		seedTask(map[string]any{"status": "in_progress", "phase": "todo", "assignee": "someone"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Status()).To(Equal(domain.TaskStatusInProgress))
		Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
	})

	It("records the injected instant as a bare time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		raw, ok := written.Get("approved_at").(time.Time)
		Expect(ok).To(BeTrue())
		Expect(raw.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("serializes approved_at unquoted and round-trips it as a time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		out, marshalErr := yaml.Marshal(written.RawMap())
		Expect(marshalErr).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring("approved_at: 2026-03-03T12:00:00Z"))
		Expect(string(out)).NotTo(ContainSubstring(`approved_at: "2026-03-03T12:00:00Z"`))

		var roundTripped map[string]any
		Expect(yaml.Unmarshal(out, &roundTripped)).To(Succeed())
		rt, ok := roundTripped["approved_at"].(time.Time)
		Expect(ok).To(BeTrue())
		Expect(rt.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("writes the approver named on the call", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "Manager Layer", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.GetString("approved_by")).To(Equal("Manager Layer"))
	})

	It("preserves every other frontmatter key", func() {
		seedTask(map[string]any{
			"status":          "next",
			"phase":           "todo",
			"assignee":        "someone",
			"priority":        1,
			"page_type":       "task",
			"task_identifier": "22222222-2222-2222-2222-222222222222",
			"custom_key":      "keep",
		})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("someone"))
		Expect(written.Priority()).To(Equal(domain.Priority(1)))
		Expect(written.PageType()).To(Equal("task"))
		Expect(written.TaskIdentifier()).To(Equal("22222222-2222-2222-2222-222222222222"))
		Expect(written.Get("custom_key")).To(Equal("keep"))
	})

	// The ownership rule: approval is the one moment an owner is fixed. The
	// precedence is flag, then the task's own assignee, then the configured
	// current user — and a task that resolves to no owner is refused unwritten.

	It("AC1: fills an empty assignee from the configured current user", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("bborbe"))
	})

	It("AC2: keeps an existing assignee over the configured current user", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone-else"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("someone-else"))
	})

	It("AC3: the flag overrides an empty assignee", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "X", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("X"))
	})

	It("AC3: the flag overrides an existing assignee", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "assignee": "someone-else"})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "X", "bborbe")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Assignee()).To(Equal("X"))
	})

	It("AC4: refuses to approve an unowned task with zero writes", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo"})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("assignee"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(1))
	})

	It("refuses an empty approver before reading the task", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_by"))
		Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(0))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("refuses a zero clock with zero writes", func() {
		zero := libtime.NewCurrentDateTime()
		zero.SetNow(libtime.DateTime(time.Time{}))
		zeroOp := ops.NewTaskApproveOperation(mockStorage, zero, 0)

		_, execErr := zeroOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_at"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	DescribeTable("refuses a task that is not waiting in the inbox",
		func(fields map[string]any, expectedPhase string) {
			seedTask(fields)

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(expectedPhase))
			Expect(execErr.Error()).To(ContainSubstring("todo"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("planning", map[string]any{"status": "next", "phase": "planning"}, "planning"),
		Entry("execution", map[string]any{"status": "next", "phase": "execution"}, "execution"),
		Entry("ai_review", map[string]any{"status": "next", "phase": "ai_review"}, "ai_review"),
		Entry("human_review", map[string]any{"status": "next", "phase": "human_review"}, "human_review"),
		Entry("done", map[string]any{"status": "next", "phase": "done"}, "done"),
		Entry("no phase key at all", map[string]any{"status": "next"}, "(none)"),
	)

	DescribeTable("advances a todo task that already carries an approval record",
		func(fields map[string]any) {
			seedTask(fields)
			// Snapshot the seeded record BEFORE Execute. domain.NewTask stores the
			// caller's map by reference, so reading fields[...] after the call would
			// compare the written map against itself and the assertion could never
			// fail — a tautology, not a preservation check.
			seededBy := fields["approved_by"]
			seededAt := fields["approved_at"]

			result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "someone", "current")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(result.Name).To(Equal("Alpha"))
			Expect(result.Vault).To(Equal("vault-a"))

			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockStorage.WriteTaskArgsForCall(0)
			Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
			// Byte-identical preservation: the written value is the seeded value,
			// including nil for a key that was never there.
			// Gomega's Equal refuses a nil-to-nil comparison, so the absent-key
			// case is asserted explicitly with BeNil.
			expectSameValue := func(actual, expected any) {
				if expected == nil {
					Expect(actual).To(BeNil())
					return
				}
				Expect(actual).To(Equal(expected))
			}
			expectSameValue(written.Get("approved_by"), seededBy)
			expectSameValue(written.Get("approved_at"), seededAt)
			// The repair is not an approval: no owner is resolved or written even
			// though --assignee and current_user are both non-empty here.
			Expect(written.Get("assignee")).To(BeNil())
		},
		Entry("both keys", map[string]any{
			"status": "next", "phase": "todo",
			"approved_by": "someone", "approved_at": "2026-01-01T00:00:00Z",
		}),
		Entry("only approved_by", map[string]any{
			"status": "next", "phase": "todo", "approved_by": "someone",
		}),
		Entry("only approved_at", map[string]any{
			"status": "next", "phase": "todo", "approved_at": "2026-01-01T00:00:00Z",
		}),
		Entry("an empty approved_by value counts as present", map[string]any{
			"status": "next", "phase": "todo", "approved_by": "",
		}),
	)

	It("still refuses an approved row that is past the inbox", func() {
		seedTask(map[string]any{
			"status": "in_progress", "phase": "execution",
			"approved_by": "someone", "approved_at": "2026-01-01T00:00:00Z",
		})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring(`not "todo"`))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("refuses an empty approver even on an already-approved row", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "approved_by": "someone"})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_by"))
		Expect(mockStorage.FindTaskByNameCallCount()).To(Equal(0))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("wraps a write failure on the repair path", func() {
		seedTask(map[string]any{"status": "next", "phase": "todo", "approved_by": "someone"})
		mockStorage.WriteTaskReturns(liberrors.New(ctx, "boom"))

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("write task"))
	})

	DescribeTable("refuses a terminal row that is still at phase todo, with zero writes",
		func(status string) {
			seedTask(map[string]any{"status": status, "phase": "todo", "approved_by": "someone"})

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			// The refusal names the terminal status and says the row is not
			// re-opened by an approval, so the operator knows this is a deliberate
			// decision rather than a missing verb.
			Expect(execErr.Error()).To(ContainSubstring(status))
			Expect(execErr.Error()).To(ContainSubstring("not re-opened by an approval"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		},
		Entry("completed", "completed"),
		Entry("aborted", "aborted"),
	)

	It("treats a present-but-null approval key as a record", func() {
		// YAML `approved_by:` with nothing after the colon parses to nil, so
		// task.Get returns nil exactly as for an absent key. The lint rule's
		// line-anchored matcher counts that shape as a record, so approve must
		// too or the two verbs disagree and the deadlock survives for it.
		seedTask(map[string]any{"status": "in_progress", "phase": "todo", "approved_by": nil})

		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())

		Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTaskArgsForCall(0)
		Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
		// No approval is fabricated onto the null key.
		Expect(written.Get("approved_by")).To(BeNil())
		Expect(written.Get("approved_at")).To(BeNil())
	})

	It("wraps a find failure", func() {
		mockStorage.FindTaskByNameReturns(nil, liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("find task"))
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	It("wraps a write failure", func() {
		mockStorage.WriteTaskReturns(liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("write task"))
	})

	It("preserves the not-found class", func() {
		mockStorage.FindTaskByNameReturns(nil, storage.ErrNotFound)
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
		Expect(execErr).To(HaveOccurred())
		Expect(stderrors.Is(execErr, storage.ErrNotFound)).To(BeTrue())
		Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
	})

	Describe("admission cap", func() {
		// A small injected cap keeps the boundary under test at 4/5 rather than
		// needing 109/110-row fixtures wherever the cap value is the subject.
		newCapOp := func(limit int) ops.TaskApproveOperation {
			return ops.NewTaskApproveOperation(mockStorage, pinned, limit)
		}

		approvedRow := func(name string) *domain.Task {
			return domain.NewTask(
				map[string]any{
					"status":      "in_progress",
					"phase":       "planning",
					"approved_at": "2026-01-01T00:00:00Z",
				},
				domain.FileMetadata{Name: name},
				domain.Content(""),
			)
		}

		approvedRows := func(n int) []*domain.Task {
			rows := make([]*domain.Task, 0, n)
			for i := 0; i < n; i++ {
				rows = append(rows, approvedRow(fmt.Sprintf("Approved Row %d", i)))
			}
			return rows
		}

		It("approves when the count is one below the cap", func() {
			capOp := newCapOp(5)
			mockStorage.ListTasksStrictReturns(approvedRows(4), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockStorage.WriteTaskArgsForCall(0)
			Expect(written.Get("approved_at")).NotTo(BeNil())
		})

		It("approves an operator-flagged row at the cap", func() {
			capOp := newCapOp(5)
			seedTask(map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": true, "flag_set_by": "operator",
			})
			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockStorage.WriteTaskArgsForCall(0)
			Expect(written.Get("approved_at")).NotTo(BeNil())
		})

		It("approves an operator-flagged row above the cap", func() {
			capOp := newCapOp(5)
			seedTask(map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": "true", "flag_set_by": "operator",
			})
			mockStorage.ListTasksStrictReturns(approvedRows(12), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		})

		It("approves an operator-flagged row whose flag is the hand-quoted string \"yes\"", func() {
			capOp := newCapOp(5)
			seedTask(map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": "yes", "flag_set_by": "operator",
			})
			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		})

		It("still refuses an unflagged row at the cap", func() {
			capOp := newCapOp(5)
			seedTask(map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone", "flag": false,
			})
			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})

		DescribeTable("a flag without operator provenance is not an exemption",
			func(fields map[string]any) {
				capOp := newCapOp(5)
				seedTask(fields)
				mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

				_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
				Expect(execErr).To(HaveOccurred())
				Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
			},
			Entry("agent actor", map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": true, "flag_set_by": "agent-x",
			}),
			Entry("unattributed (task set without --by)", map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": true, "flag_set_by": "unknown",
			}),
			Entry("pre-field, no provenance", map[string]any{
				"status": "next", "phase": "todo", "assignee": "someone",
				"flag": true,
			}),
		)

		It("counts an approved operator-flagged row toward the cap", func() {
			capOp := newCapOp(5)
			rows := append(approvedRows(4), domain.NewTask(
				map[string]any{
					"status": "in_progress", "phase": "planning",
					"approved_at": "2026-01-01T00:00:00Z",
					"flag":        true, "flag_set_by": "operator",
				},
				domain.FileMetadata{Name: "Flagged Approved"}, domain.Content(""),
			))
			mockStorage.ListTasksStrictReturns(rows, nil)

			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
		})

		It("refuses at the cap, naming the count, the cap and the vault, with no write", func() {
			capOp := newCapOp(5)
			mockStorage.ListTasksStrictReturns(approvedRows(5), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(result.Success).To(BeFalse())
			Expect(execErr.Error()).To(ContainSubstring("is 5"))
			Expect(execErr.Error()).To(ContainSubstring("admission cap of 5"))
			Expect(execErr.Error()).To(ContainSubstring("vault-a"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("excludes completed and aborted rows from the count", func() {
			capOp := newCapOp(5)
			rows := approvedRows(4)
			rows = append(
				rows,
				domain.NewTask(
					map[string]any{"status": "completed", "approved_at": "2026-01-01T00:00:00Z"},
					domain.FileMetadata{Name: "Done"},
					domain.Content(""),
				),
				domain.NewTask(
					map[string]any{"status": "aborted", "approved_at": "2026-01-01T00:00:00Z"},
					domain.FileMetadata{Name: "Gone"},
					domain.Content(""),
				),
			)
			mockStorage.ListTasksStrictReturns(rows, nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
		})

		It("fails closed when the count cannot be read", func() {
			capOp := newCapOp(5)
			mockStorage.ListTasksStrictReturns(nil, liberrors.New(ctx, "boom"))

			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring("could not be read"))
			Expect(execErr.Error()).To(ContainSubstring("boom"))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("uses the default cap when constructed with 0", func() {
			// The field is unexported and this suite is in ops_test, so the
			// substitution is asserted behaviourally: seeding exactly the default
			// number of counted rows must refuse.
			capOp := newCapOp(0)
			mockStorage.ListTasksStrictReturns(
				approvedRows(ops.DefaultMaxApprovedNotStarted),
				nil,
			)

			_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(
				fmt.Sprintf("admission cap of %d", ops.DefaultMaxApprovedNotStarted),
			))
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("advances an already-approved row even at the cap", func() {
			capOp := newCapOp(1)
			seedTask(map[string]any{"status": "next", "phase": "todo", "approved_by": "someone"})
			mockStorage.ListTasksStrictReturns(approvedRows(1), nil)

			result, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
			Expect(execErr).To(BeNil())
			Expect(result.Success).To(BeTrue())
			Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			_, written := mockStorage.WriteTaskArgsForCall(0)
			Expect(written.Phase()).To(Equal(domain.TaskPhasePlanning.Ptr()))
		})

		DescribeTable("counts approved_at only when it is present and non-empty",
			func(fields map[string]any, expectRefusal bool) {
				capOp := newCapOp(1)
				mockStorage.ListTasksStrictReturns(
					[]*domain.Task{
						domain.NewTask(fields, domain.FileMetadata{Name: "Row"}, domain.Content("")),
					},
					nil,
				)

				_, execErr := capOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator", "", "")
				if expectRefusal {
					Expect(execErr).To(HaveOccurred())
					Expect(execErr.Error()).To(ContainSubstring("admission cap of 1"))
					Expect(mockStorage.WriteTaskCallCount()).To(Equal(0))
					return
				}
				Expect(execErr).To(BeNil())
				Expect(mockStorage.WriteTaskCallCount()).To(Equal(1))
			},
			Entry("absent", map[string]any{"status": "in_progress"}, false),
			Entry("nil value", map[string]any{"status": "in_progress", "approved_at": nil}, false),
			Entry("empty string", map[string]any{"status": "in_progress", "approved_at": ""}, false),
			// The quoted-string shape a hand-authored task file decodes to; a
			// Get(...) type assertion would silently miss it.
			Entry(
				"quoted string",
				map[string]any{"status": "in_progress", "approved_at": "2026-01-01T00:00:00Z"},
				true,
			),
		)
	})
})
