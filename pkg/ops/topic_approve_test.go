// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
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
)

var _ = Describe("TopicApproveOperation", func() {
	var (
		ctx         context.Context
		mockStorage *mocks.Storage
		pinned      libtime.CurrentDateTime
		approveOp   ops.TopicApproveOperation
	)

	// seedTopic replaces the topic the storage returns, so a spec can control the
	// phase and the pre-existing frontmatter the approve guards inspect.
	seedTopic := func(fields map[string]any) {
		mockStorage.FindTopicByNameReturns(
			domain.NewTopic(fields, domain.FileMetadata{Name: "Alpha"}, domain.Content("")),
			nil,
		)
	}

	BeforeEach(func() {
		ctx = context.Background()
		mockStorage = &mocks.Storage{}
		mockStorage.WriteTopicReturns(nil)

		pinned = libtime.NewCurrentDateTime()
		pinned.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))
		approveOp = ops.NewTopicApproveOperation(mockStorage, pinned)

		seedTopic(map[string]any{"status": "next", "phase": "todo"})
	})

	It("writes all three approval keys in exactly one write", func() {
		result, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(BeNil())
		Expect(result.Success).To(BeTrue())
		Expect(result.Name).To(Equal("Alpha"))
		Expect(result.Vault).To(Equal("vault-a"))

		Expect(mockStorage.WriteTopicCallCount()).To(Equal(1))
		_, written := mockStorage.WriteTopicArgsForCall(0)
		Expect(written.Phase()).To(Equal(domain.TopicPhasePlanning.Ptr()))
		Expect(written.GetString("approved_by")).To(Equal("operator"))
		Expect(written.Get("approved_at")).NotTo(BeNil())
	})

	It("records the injected instant as a bare time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTopicArgsForCall(0)
		raw, ok := written.Get("approved_at").(time.Time)
		Expect(ok).To(BeTrue())
		Expect(raw.Equal(pinned.Now().Time())).To(BeTrue())
	})

	It("serializes approved_at unquoted and round-trips it as a time.Time", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTopicArgsForCall(0)
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
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "Manager Layer")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTopicArgsForCall(0)
		Expect(written.GetString("approved_by")).To(Equal("Manager Layer"))
	})

	It("preserves every other frontmatter key", func() {
		seedTopic(map[string]any{
			"status":     "next",
			"phase":      "todo",
			"page_type":  "topic",
			"tags":       []string{"work", "important"},
			"defer_date": "2026-04-01",
			"custom_key": "keep",
		})

		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(BeNil())

		_, written := mockStorage.WriteTopicArgsForCall(0)
		Expect(written.GetField("status")).To(Equal("next"))
		Expect(written.GetField("page_type")).To(Equal("topic"))
		Expect(written.Tags()).To(Equal([]string{"work", "important"}))
		Expect(written.GetField("defer_date")).To(Equal("2026-04-01"))
		Expect(written.Get("custom_key")).To(Equal("keep"))
	})

	It("refuses an empty approver before reading the topic", func() {
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_by"))
		Expect(mockStorage.FindTopicByNameCallCount()).To(Equal(0))
		Expect(mockStorage.WriteTopicCallCount()).To(Equal(0))
	})

	It("refuses a zero clock with zero writes", func() {
		zero := libtime.NewCurrentDateTime()
		zero.SetNow(libtime.DateTime(time.Time{}))
		zeroOp := ops.NewTopicApproveOperation(mockStorage, zero)

		_, execErr := zeroOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("approved_at"))
		Expect(mockStorage.WriteTopicCallCount()).To(Equal(0))
	})

	DescribeTable("refuses a topic that is not waiting in the inbox",
		func(fields map[string]any, expectedPhase string) {
			seedTopic(fields)

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(expectedPhase))
			Expect(execErr.Error()).To(ContainSubstring("todo"))
			Expect(mockStorage.WriteTopicCallCount()).To(Equal(0))
		},
		Entry("planning", map[string]any{"status": "next", "phase": "planning"}, "planning"),
		Entry("execution", map[string]any{"status": "next", "phase": "execution"}, "execution"),
		Entry("done", map[string]any{"status": "next", "phase": "done"}, "done"),
		Entry("no phase key at all", map[string]any{"status": "next"}, "(none)"),
	)

	DescribeTable("refuses a todo topic that already carries an approval record",
		func(fields map[string]any, expectedKeys string) {
			seedTopic(fields)

			_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
			Expect(execErr).To(HaveOccurred())
			Expect(execErr.Error()).To(ContainSubstring(expectedKeys))
			Expect(execErr.Error()).To(ContainSubstring("re-approving"))
			Expect(mockStorage.WriteTopicCallCount()).To(Equal(0))
		},
		Entry("only approved_by",
			map[string]any{"status": "next", "phase": "todo", "approved_by": "someone"},
			"approved_by",
		),
		Entry("only approved_at",
			map[string]any{
				"status":      "next",
				"phase":       "todo",
				"approved_at": "2026-01-01T00:00:00Z",
			},
			"approved_at",
		),
		Entry("both keys",
			map[string]any{
				"status":      "next",
				"phase":       "todo",
				"approved_by": "someone",
				"approved_at": "2026-01-01T00:00:00Z",
			},
			"approved_by, approved_at",
		),
	)

	It("wraps a find failure", func() {
		mockStorage.FindTopicByNameReturns(nil, liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("find topic"))
		Expect(mockStorage.WriteTopicCallCount()).To(Equal(0))
	})

	It("wraps a write failure", func() {
		mockStorage.WriteTopicReturns(liberrors.New(ctx, "boom"))
		_, execErr := approveOp.Execute(ctx, "/vault", "Alpha", "vault-a", "operator")
		Expect(execErr).To(HaveOccurred())
		Expect(execErr.Error()).To(ContainSubstring("write topic"))
	})
})
