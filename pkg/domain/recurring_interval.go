// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// RecurringInterval represents a time interval for recurring tasks.
type RecurringInterval struct {
	Years  int
	Months int
	Days   int
}

// AddTo adds the interval to the given time and returns the result.
func (r RecurringInterval) AddTo(t time.Time) time.Time {
	return t.AddDate(r.Years, r.Months, r.Days)
}

var recurringShorthandRegex = regexp.MustCompile(`^([1-9]\d*)([dwmqy])$`)

// ParseRecurringInterval parses a recurring interval string into a RecurringInterval.
// Named aliases: daily, weekly, monthly, quarterly, yearly.
// Numeric shorthand: <N><unit> where unit is d, w, m, q, or y.
// Note: "weekdays" is NOT handled here — check for it before calling this function.
func ParseRecurringInterval(s string) (RecurringInterval, error) {
	switch s {
	case "daily":
		return RecurringInterval{Days: 1}, nil
	case "weekly":
		return RecurringInterval{Days: 7}, nil
	case "monthly":
		return RecurringInterval{Months: 1}, nil
	case "quarterly":
		return RecurringInterval{Months: 3}, nil
	case "yearly":
		return RecurringInterval{Years: 1}, nil
	}

	matches := recurringShorthandRegex.FindStringSubmatch(s)
	if matches == nil {
		return RecurringInterval{}, fmt.Errorf("unknown recurring interval: %q", s)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return RecurringInterval{}, fmt.Errorf(
			"invalid recurring interval number in %q: %w",
			s,
			err,
		)
	}

	switch matches[2] {
	case "d":
		return RecurringInterval{Days: n}, nil
	case "w":
		return RecurringInterval{Days: n * 7}, nil
	case "m":
		return RecurringInterval{Months: n}, nil
	case "q":
		return RecurringInterval{Months: n * 3}, nil
	case "y":
		return RecurringInterval{Years: n}, nil
	default:
		return RecurringInterval{}, fmt.Errorf(
			"unknown unit %q in recurring interval %q",
			matches[2],
			s,
		)
	}
}
