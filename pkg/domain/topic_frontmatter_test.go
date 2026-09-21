// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"
	"time"

	libtime "github.com/bborbe/time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("TopicFrontmatter", func() {
	var (
		ctx context.Context
		fm  domain.TopicFrontmatter
	)

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewTopicFrontmatter(nil)
	})

	Describe("GetField phase", func() {
		It("returns the raw on-disk phase string verbatim", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "planning"})
			Expect(fm.GetField("phase")).To(Equal("planning"))
		})

		It("returns a non-canonical phase value without rejecting the page", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "whatever-the-vault-holds"})
			Expect(fm.GetField("phase")).To(Equal("whatever-the-vault-holds"))
		})

		It("returns empty and leaves the key absent when the page has no phase line", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"status": "in_progress"})
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})

		It("returns empty for an empty frontmatter map", func() {
			Expect(domain.NewTopicFrontmatter(nil).GetField("phase")).To(Equal(""))
		})
	})

	Describe("phase is never written by the topic wrapper", func() {
		It("does not inject phase on an unrelated mutation", func() {
			fm = domain.NewTopicFrontmatter(nil)
			Expect(fm.SetField(ctx, "status", "in_progress")).To(Succeed())
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})

		It("does not default phase when another key is cleared", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"assignee": "alice"})
			fm.ClearField("assignee")
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})
	})

	Describe("Phase", func() {
		It("returns nil for a topic with no phase line", func() {
			Expect(domain.NewTopicFrontmatter(nil).Phase()).To(BeNil())
			Expect(
				domain.NewTopicFrontmatter(map[string]any{"status": "in_progress"}).Phase(),
			).To(BeNil())
		})

		It("returns a pointer for a canonical topic phase value", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "execution"})
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TopicPhaseExecution))
		})

		It("returns a pointer for a non-canonical on-disk value without rejecting the page", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "in_progress"})
			Expect(fm.Phase()).NotTo(BeNil())
			Expect(*fm.Phase()).To(Equal(domain.TopicPhase("in_progress")))
		})
	})

	Describe("SetPhase", func() {
		It("stores the topic phase string via pointer", func() {
			fm.SetPhase(domain.TopicPhaseDone.Ptr())
			Expect(fm.GetField("phase")).To(Equal("done"))
		})

		It("a nil topic phase pointer deletes the key", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "todo"})
			fm.SetPhase(nil)
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})
	})

	Describe("SetField phase", func() {
		DescribeTable("accepts each canonical topic phase value",
			func(value string) {
				fm = domain.NewTopicFrontmatter(nil)
				Expect(fm.SetField(ctx, "phase", value)).To(Succeed())
				Expect(fm.GetField("phase")).To(Equal(value))
			},
			Entry("todo", "todo"),
			Entry("planning", "planning"),
			Entry("execution", "execution"),
			Entry("done", "done"),
		)

		It("rejects a non-canonical value with the validator's wording", func() {
			err := fm.SetField(ctx, "phase", "bogus")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown topic phase 'bogus'"))
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})

		It("rejects the task-only phase in_progress", func() {
			err := fm.SetField(ctx, "phase", "in_progress")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown topic phase"))
		})

		It("clears the phase key on an empty value", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"phase": "execution"})
			Expect(fm.SetField(ctx, "phase", "")).To(Succeed())
			Expect(fm.GetField("phase")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("phase"))
		})
	})

	Describe("SetField / GetField - unknown field round-trip", func() {
		It("round-trips an unknown key", func() {
			Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
			Expect(fm.GetField("custom_note")).To(Equal("hello"))
		})

		It("returns empty for an absent key", func() {
			Expect(fm.GetField("unknown_key")).To(Equal(""))
		})

		It("clears a key with ClearField", func() {
			Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
			fm.ClearField("custom_note")
			Expect(fm.GetField("custom_note")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("custom_note"))
		})
	})

	Describe("SetField / GetField - status and assignee", func() {
		It("stores and reads status as a plain string with no validation", func() {
			Expect(fm.SetField(ctx, "status", "completed")).To(Succeed())
			Expect(fm.GetField("status")).To(Equal("completed"))
		})

		It("accepts a status value no enum would recognise", func() {
			Expect(fm.SetField(ctx, "status", "some-vault-local-status")).To(Succeed())
			Expect(fm.GetField("status")).To(Equal("some-vault-local-status"))
		})

		It("round-trips assignee", func() {
			Expect(fm.SetField(ctx, "assignee", "alice")).To(Succeed())
			Expect(fm.GetField("assignee")).To(Equal("alice"))
		})
	})

	Describe("Tags", func() {
		It("returns nil for a missing key", func() {
			Expect(fm.Tags()).To(BeNil())
		})

		It("returns the list for a []any value", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"tags": []any{"urgent", "q1"}})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})

		It("returns the list for a []string value", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"tags": []string{"urgent", "q1"}})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})
	})

	Describe("SetTags", func() {
		It("stores a list", func() {
			fm.SetTags([]string{"urgent", "q1"})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})

		It("deletes the key when set to nil", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"tags": []any{"urgent"}})
			fm.SetTags(nil)
			Expect(fm.Get("tags")).To(BeNil())
			Expect(fm.Keys()).NotTo(ContainElement("tags"))
		})

		It("deletes the key when set to an empty slice", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"tags": []any{"urgent"}})
			fm.SetTags([]string{})
			Expect(fm.Get("tags")).To(BeNil())
			Expect(fm.Keys()).NotTo(ContainElement("tags"))
		})
	})

	Describe("SetField / GetField - tags", func() {
		It("joins tags with a comma on GetField and reads them back split", func() {
			Expect(fm.SetField(ctx, "tags", "urgent,q1")).To(Succeed())
			Expect(fm.GetField("tags")).To(Equal("urgent,q1"))
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})
	})

	Describe("DeferDate", func() {
		It("returns nil for a missing key", func() {
			Expect(fm.DeferDate()).To(BeNil())
		})

		It("reads a date-only value", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"defer_date": "2027-03-19"})
			d := fm.DeferDate()
			Expect(d).NotTo(BeNil())
			Expect(d.Time().UTC().Format("2006-01-02")).To(Equal("2027-03-19"))
		})
	})

	Describe("SetDeferDate", func() {
		It("stores a date and reads it back at day granularity", func() {
			d := libtime.DateOrDateTime(time.Date(2027, 3, 19, 0, 0, 0, 0, time.UTC))
			fm.SetDeferDate(&d)
			Expect(fm.DeferDate().Time().Format("2006-01-02")).To(Equal("2027-03-19"))
		})

		It("deletes the key when set to nil", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{"defer_date": "2027-03-19"})
			fm.SetDeferDate(nil)
			Expect(fm.DeferDate()).To(BeNil())
			Expect(fm.Keys()).NotTo(ContainElement("defer_date"))
		})
	})

	Describe("SetField / GetField - defer_date", func() {
		It("round-trips a date-only value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "defer_date", "2027-03-19")).To(Succeed())
			Expect(fm.GetField("defer_date")).To(Equal("2027-03-19"))
		})

		It("returns an error for an unparseable value", func() {
			Expect(fm.SetField(ctx, "defer_date", "not-a-date")).NotTo(Succeed())
		})

		It("clears the key on an empty value", func() {
			Expect(fm.SetField(ctx, "defer_date", "2027-03-19")).To(Succeed())
			Expect(fm.SetField(ctx, "defer_date", "")).To(Succeed())
			Expect(fm.GetField("defer_date")).To(Equal(""))
			Expect(fm.Keys()).NotTo(ContainElement("defer_date"))
		})
	})

	Describe("round-trip", func() {
		It("preserves every key including unknown ones through GetField and SetField", func() {
			fm = domain.NewTopicFrontmatter(map[string]any{
				"status":      "in_progress",
				"phase":       "execution",
				"tags":        []any{"a", "b"},
				"unknown_one": "x",
				"unknown_two": 7,
			})
			Expect(fm.SetField(ctx, "status", "completed")).To(Succeed())
			Expect(fm.Keys()).To(
				ContainElements("status", "phase", "tags", "unknown_one", "unknown_two"),
			)
			Expect(fm.GetField("phase")).To(Equal("execution"))
			Expect(fm.GetField("unknown_one")).To(Equal("x"))
		})
	})
})
