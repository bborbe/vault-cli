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

var _ = Describe("ParseRecurringInterval", func() {
	var (
		result domain.RecurringInterval
		err    error
	)

	DescribeTable("named aliases",
		func(input string, expected domain.RecurringInterval) {
			result, err = domain.ParseRecurringInterval(input)
			Expect(err).To(BeNil())
			Expect(result).To(Equal(expected))
		},
		Entry("daily", "daily", domain.RecurringInterval{Days: 1}),
		Entry("weekly", "weekly", domain.RecurringInterval{Days: 7}),
		Entry("monthly", "monthly", domain.RecurringInterval{Months: 1}),
		Entry("quarterly", "quarterly", domain.RecurringInterval{Months: 3}),
		Entry("yearly", "yearly", domain.RecurringInterval{Years: 1}),
	)

	DescribeTable("numeric shorthand",
		func(input string, expected domain.RecurringInterval) {
			result, err = domain.ParseRecurringInterval(input)
			Expect(err).To(BeNil())
			Expect(result).To(Equal(expected))
		},
		Entry("1d", "1d", domain.RecurringInterval{Days: 1}),
		Entry("3d", "3d", domain.RecurringInterval{Days: 3}),
		Entry("2w", "2w", domain.RecurringInterval{Days: 14}),
		Entry("2m", "2m", domain.RecurringInterval{Months: 2}),
		Entry("1q", "1q", domain.RecurringInterval{Months: 3}),
		Entry("2q", "2q", domain.RecurringInterval{Months: 6}),
		Entry("1y", "1y", domain.RecurringInterval{Years: 1}),
		Entry("2y", "2y", domain.RecurringInterval{Years: 2}),
	)

	DescribeTable("invalid input returns error",
		func(input string) {
			result, err = domain.ParseRecurringInterval(input)
			Expect(err).NotTo(BeNil())
			Expect(result).To(Equal(domain.RecurringInterval{}))
		},
		Entry("empty string", ""),
		Entry("unknown alias", "foo"),
		Entry("zero days", "0d"),
		Entry("weekdays", "weekdays"),
	)
})

var _ = Describe("RecurringInterval AddTo", func() {
	DescribeTable("date arithmetic",
		func(interval domain.RecurringInterval, from time.Time, expected time.Time) {
			Expect(interval.AddTo(from)).To(Equal(expected))
		},
		Entry(
			"2m from Jan 31 → Mar 31",
			domain.RecurringInterval{Months: 2},
			time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		),
		Entry(
			"1m from Jan 31 → Mar 3 (Go AddDate overflow behavior)",
			domain.RecurringInterval{Months: 1},
			time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC),
		),
	)
})
