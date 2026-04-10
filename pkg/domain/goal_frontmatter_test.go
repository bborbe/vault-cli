// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("GoalFrontmatter", func() {
	var (
		ctx context.Context
		fm  domain.GoalFrontmatter
	)

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewGoalFrontmatter(nil)
	})

	Describe("Status", func() {
		It("returns empty for missing key", func() {
			Expect(fm.Status()).To(Equal(domain.GoalStatus("")))
		})

		It("returns canonical status for known value", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"status": "active"})
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})
	})

	Describe("SetStatus", func() {
		It("stores valid status", func() {
			Expect(fm.SetStatus(domain.GoalStatusActive)).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})

		It("returns error for invalid status", func() {
			Expect(fm.SetStatus(domain.GoalStatus("garbage"))).NotTo(BeNil())
		})
	})

	Describe("SetField / GetField - unknown field round-trip", func() {
		It("stores and retrieves unknown fields without error", func() {
			Expect(fm.SetField(ctx, "custom_note", "hello")).To(Succeed())
			Expect(fm.GetField("custom_note")).To(Equal("hello"))
		})

		It("does not error on set of unknown field", func() {
			Expect(fm.SetField(ctx, "unknown_key", "value")).To(Succeed())
		})

		It("returns empty string for unset unknown field", func() {
			Expect(fm.GetField("unknown_key")).To(Equal(""))
		})
	})

	Describe("SetField status", func() {
		It("sets status to active via SetField", func() {
			Expect(fm.SetField(ctx, "status", "active")).To(Succeed())
			Expect(fm.Status()).To(Equal(domain.GoalStatusActive))
		})

		It("returns error for invalid status via SetField", func() {
			Expect(fm.SetField(ctx, "status", "invalid")).NotTo(BeNil())
		})
	})

	Describe("Tags", func() {
		It("returns nil for missing tags", func() {
			Expect(fm.Tags()).To(BeNil())
		})

		It("returns tags as string slice", func() {
			fm = domain.NewGoalFrontmatter(map[string]any{"tags": []any{"urgent", "q1"}})
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})

		It("SetField joins tags with comma from SetField and splits on GetField", func() {
			Expect(fm.SetField(ctx, "tags", "urgent,q1")).To(Succeed())
			Expect(fm.GetField("tags")).To(Equal("urgent,q1"))
			Expect(fm.Tags()).To(Equal([]string{"urgent", "q1"}))
		})
	})

	Describe("StartDate round-trip", func() {
		It("returns nil for missing start_date", func() {
			Expect(fm.StartDate()).To(BeNil())
		})

		It("round-trips a date set via SetStartDate", func() {
			t := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
			fm.SetStartDate(&t)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(*result).To(Equal(t))
		})

		It("returns nil after SetStartDate(nil)", func() {
			t := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
			fm.SetStartDate(&t)
			fm.SetStartDate(nil)
			Expect(fm.StartDate()).To(BeNil())
		})
	})

	Describe("Priority validation", func() {
		It("rejects negative priority via SetPriority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(-1))).NotTo(BeNil())
		})

		It("accepts zero priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(0))).To(Succeed())
		})

		It("accepts positive priority", func() {
			Expect(fm.SetPriority(ctx, domain.Priority(3))).To(Succeed())
			Expect(fm.Priority()).To(Equal(domain.Priority(3)))
		})
	})

	Describe("ClearField", func() {
		It("clears an existing field", func() {
			Expect(fm.SetField(ctx, "status", "active")).To(Succeed())
			fm.ClearField("status")
			Expect(fm.GetField("status")).To(Equal(""))
		})

		It("does not error on clearing unknown field", func() {
			Expect(func() { fm.ClearField("nonexistent") }).NotTo(Panic())
			Expect(fm.GetField("nonexistent")).To(Equal(""))
		})
	})
})
