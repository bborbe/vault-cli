// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"github.com/bborbe/errors"
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
func ParseRecurringInterval(ctx context.Context, s string) (RecurringInterval, error) {
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
		return RecurringInterval{}, errors.Errorf(ctx, "unknown recurring interval: %q", s)
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return RecurringInterval{}, errors.Wrapf(
			ctx,
			err,
			"invalid recurring interval number in %q",
			s,
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
		return RecurringInterval{}, errors.Errorf(
			ctx,
			"unknown unit %q in recurring interval %q",
			matches[2],
			s,
		)
	}
}

// ParseRecurringIntervalDefault parses s and returns def when s cannot be parsed.
// The second return value reports whether s parsed successfully, so callers can
// report the fallback without parsing a second time.
func ParseRecurringIntervalDefault(
	ctx context.Context,
	s string,
	def RecurringInterval,
) (RecurringInterval, bool) {
	interval, err := ParseRecurringInterval(ctx, s)
	if err != nil {
		return def, false
	}
	return interval, true
}
