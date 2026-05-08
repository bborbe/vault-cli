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

var _ = Describe("ThemeFrontmatter", func() {
	var (
		ctx context.Context
		fm  domain.ThemeFrontmatter
	)

	BeforeEach(func() {
		ctx = context.Background()
		fm = domain.NewThemeFrontmatter(nil)
	})

	Describe("StartDate", func() {
		It("returns nil for missing start_date", func() {
			Expect(fm.StartDate()).To(BeNil())
		})

		It("parses a YAML date literal (time.Time path)", func() {
			fm = domain.NewThemeFrontmatter(
				map[string]any{"start_date": time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-01-15"))
		})

		It("parses a string value (hand-authored date)", func() {
			fm = domain.NewThemeFrontmatter(map[string]any{"start_date": "2025-01-15"})
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-01-15"))
		})

		It("parses an RFC3339 string value", func() {
			fm = domain.NewThemeFrontmatter(
				map[string]any{"start_date": "2025-01-15T14:30:00+01:00"},
			)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2025-01-15T14:30:00+01:00"))
		})
	})

	Describe("SetStartDate", func() {
		It("returns nil after SetStartDate(nil)", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
			fm.SetStartDate(&d)
			fm.SetStartDate(nil)
			Expect(fm.StartDate()).To(BeNil())
		})

		It("round-trips a date-only value as YYYY-MM-DD", func() {
			d := libtime.DateOrDateTime(time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC))
			fm.SetStartDate(&d)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2026-03-17"))
			Expect(fm.GetField("start_date")).To(Equal("2026-03-17"))
		})

		It("round-trips an RFC3339 value preserving timezone", func() {
			loc, err := time.LoadLocation("Europe/Berlin")
			Expect(err).To(BeNil())
			d := libtime.DateOrDateTime(time.Date(2026, 3, 17, 9, 0, 0, 0, loc))
			fm.SetStartDate(&d)
			result := fm.StartDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2026-03-17T09:00:00+01:00"))
		})
	})

	Describe("SetField / GetField - start_date", func() {
		It("round-trips a date-only value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "start_date", "2025-01-15")).To(Succeed())
			Expect(fm.GetField("start_date")).To(Equal("2025-01-15"))
		})

		It("round-trips an RFC3339 value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "start_date", "2025-03-01T09:00:00Z")).To(Succeed())
			Expect(fm.GetField("start_date")).To(Equal("2025-03-01T09:00:00Z"))
		})

		It("returns error for invalid date format", func() {
			Expect(fm.SetField(ctx, "start_date", "not-a-date")).NotTo(Succeed())
		})

		It("clears start_date on empty value", func() {
			Expect(fm.SetField(ctx, "start_date", "2025-01-15")).To(Succeed())
			Expect(fm.SetField(ctx, "start_date", "")).To(Succeed())
			Expect(fm.GetField("start_date")).To(Equal(""))
		})
	})

	Describe("TargetDate", func() {
		It("returns nil for missing target_date", func() {
			Expect(fm.TargetDate()).To(BeNil())
		})

		It("parses a YAML date literal (time.Time path)", func() {
			fm = domain.NewThemeFrontmatter(
				map[string]any{"target_date": time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)},
			)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
		})

		It("parses a string value (hand-authored date)", func() {
			fm = domain.NewThemeFrontmatter(map[string]any{"target_date": "2025-12-31"})
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
		})

		It("parses an RFC3339 string value", func() {
			fm = domain.NewThemeFrontmatter(
				map[string]any{"target_date": "2025-12-31T23:59:59+05:30"},
			)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2025-12-31T23:59:59+05:30"))
		})
	})

	Describe("SetTargetDate", func() {
		It("returns nil after SetTargetDate(nil)", func() {
			d := libtime.DateOrDateTime(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
			fm.SetTargetDate(&d)
			fm.SetTargetDate(nil)
			Expect(fm.TargetDate()).To(BeNil())
		})

		It("round-trips a date-only value as YYYY-MM-DD", func() {
			d := libtime.DateOrDateTime(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
			fm.SetTargetDate(&d)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Time().UTC().Format("2006-01-02")).To(Equal("2025-12-31"))
			Expect(fm.GetField("target_date")).To(Equal("2025-12-31"))
		})

		It("round-trips an RFC3339 value preserving timezone", func() {
			loc, err := time.LoadLocation("Europe/Berlin")
			Expect(err).To(BeNil())
			d := libtime.DateOrDateTime(time.Date(2025, 12, 31, 23, 0, 0, 0, loc))
			fm.SetTargetDate(&d)
			result := fm.TargetDate()
			Expect(result).NotTo(BeNil())
			Expect(result.Format(time.RFC3339)).To(Equal("2025-12-31T23:00:00+01:00"))
		})
	})

	Describe("SetField / GetField - target_date", func() {
		It("round-trips a date-only value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "target_date", "2025-12-31")).To(Succeed())
			Expect(fm.GetField("target_date")).To(Equal("2025-12-31"))
		})

		It("round-trips an RFC3339 value via SetField/GetField", func() {
			Expect(fm.SetField(ctx, "target_date", "2025-12-31T23:59:59Z")).To(Succeed())
			Expect(fm.GetField("target_date")).To(Equal("2025-12-31T23:59:59Z"))
		})

		It("returns error for invalid date format", func() {
			Expect(fm.SetField(ctx, "target_date", "not-a-date")).NotTo(Succeed())
		})

		It("clears target_date on empty value", func() {
			Expect(fm.SetField(ctx, "target_date", "2025-12-31")).To(Succeed())
			Expect(fm.SetField(ctx, "target_date", "")).To(Succeed())
			Expect(fm.GetField("target_date")).To(Equal(""))
		})
	})
})
