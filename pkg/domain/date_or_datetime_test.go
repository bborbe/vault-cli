// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("DateOrDateTime", func() {
	Describe("MarshalText", func() {
		Context("date-only value (midnight UTC)", func() {
			It("serializes as YYYY-MM-DD", func() {
				t := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
				d := domain.DateOrDateTime(t)
				data, err := d.MarshalText()
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("2026-03-19"))
			})
		})

		Context("datetime with timezone", func() {
			It("serializes as RFC3339", func() {
				loc := time.FixedZone("CET", 3600)
				t := time.Date(2026, 3, 19, 16, 0, 0, 0, loc)
				d := domain.DateOrDateTime(t)
				data, err := d.MarshalText()
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("2026-03-19T16:00:00+01:00"))
			})
		})

		Context(
			"datetime with non-zero UTC hour (timezone offset makes UTC midnight differ)",
			func() {
				It("serializes as RFC3339 because UTC representation is non-zero", func() {
					// 2026-03-19T00:00:00+01:00 → UTC 2026-03-18T23:00:00Z (non-zero UTC hour)
					loc := time.FixedZone("CET", 3600)
					t := time.Date(2026, 3, 19, 0, 0, 0, 0, loc)
					d := domain.DateOrDateTime(t)
					data, err := d.MarshalText()
					Expect(err).To(BeNil())
					Expect(string(data)).To(Equal("2026-03-19T00:00:00+01:00"))
				})
			},
		)
	})

	Describe("UnmarshalText", func() {
		Context("YYYY-MM-DD input", func() {
			It("round-trips correctly back to YYYY-MM-DD", func() {
				var d domain.DateOrDateTime
				Expect(d.UnmarshalText([]byte("2026-03-19"))).To(Succeed())
				data, err := d.MarshalText()
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("2026-03-19"))
			})
		})

		Context("RFC3339 input", func() {
			It("round-trips correctly back to RFC3339", func() {
				var d domain.DateOrDateTime
				Expect(d.UnmarshalText([]byte("2026-03-19T16:00:00+01:00"))).To(Succeed())
				data, err := d.MarshalText()
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal("2026-03-19T16:00:00+01:00"))
			})
		})

		Context("empty string", func() {
			It("returns no error and leaves value zero", func() {
				var d domain.DateOrDateTime
				Expect(d.UnmarshalText([]byte(""))).To(Succeed())
				Expect(d.IsZero()).To(BeTrue())
			})
		})

		Context("invalid input", func() {
			It("returns an error", func() {
				var d domain.DateOrDateTime
				Expect(d.UnmarshalText([]byte("not-a-date"))).NotTo(Succeed())
			})
		})
	})

	Describe("Time", func() {
		It("returns the underlying time.Time", func() {
			t := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
			d := domain.DateOrDateTime(t)
			Expect(d.Time()).To(Equal(t))
		})
	})

	Describe("Ptr", func() {
		It("returns a non-nil pointer", func() {
			t := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
			d := domain.DateOrDateTime(t)
			Expect(d.Ptr()).NotTo(BeNil())
			Expect(d.Ptr().Time()).To(Equal(t))
		})
	})

	Describe("IsZero", func() {
		It("returns true for zero value", func() {
			var d domain.DateOrDateTime
			Expect(d.IsZero()).To(BeTrue())
		})

		It("returns false for non-zero value", func() {
			d := domain.DateOrDateTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			Expect(d.IsZero()).To(BeFalse())
		})
	})

	Describe("Before", func() {
		It("returns true when d is before other", func() {
			d := domain.DateOrDateTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			other := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
			Expect(d.Before(other)).To(BeTrue())
		})

		It("returns false when d is after other", func() {
			d := domain.DateOrDateTime(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
			other := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			Expect(d.Before(other)).To(BeFalse())
		})
	})
})
