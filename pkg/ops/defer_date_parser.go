// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
)

// parseDeferDate parses a date string using the same rules as task defer:
// +Nd (relative days), weekday names, YYYY-MM-DD (ISO date), RFC3339 datetime.
func parseDeferDate(
	ctx context.Context,
	dateStr string,
	now time.Time,
) (libtime.DateOrDateTime, error) {
	// Handle relative dates: +1d, +7d, etc.
	if matched, _ := regexp.MatchString(`^\+\d+d$`, dateStr); matched {
		var days int
		if _, err := fmt.Sscanf(dateStr, "+%dd", &days); err != nil {
			return libtime.DateOrDateTime{}, errors.Wrapf(
				ctx,
				err,
				"parse relative date %s",
				dateStr,
			)
		}
		t := libtime.ToDate(now.AddDate(0, 0, days)).Time()
		return libtime.DateOrDateTime(t), nil
	}

	// Handle weekday names
	weekdayMap := map[string]time.Weekday{
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
		"sunday":    time.Sunday,
	}
	if weekday, ok := weekdayMap[strings.ToLower(dateStr)]; ok {
		t := libtime.ToDate(nextWeekday(now, weekday)).Time()
		return libtime.DateOrDateTime(t), nil
	}

	// Handle ISO date: 2024-12-31
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return libtime.DateOrDateTime(t), nil
	}

	// Handle RFC3339 datetime: 2026-03-19T16:00:00+01:00
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return libtime.DateOrDateTime(t), nil
	}

	return libtime.DateOrDateTime{}, errors.Errorf(ctx,
		"invalid date format: %s (use +Nd, weekday, YYYY-MM-DD, or RFC3339)",
		dateStr,
	)
}

// isDeferDateInPast reports whether targetDate is in the past relative to now.
// Date-only values (midnight UTC) are compared at day granularity so "today" is never past.
func isDeferDateInPast(targetDate libtime.DateOrDateTime, now time.Time) bool {
	targetT := targetDate.Time()
	targetUTC := targetT.UTC()
	if targetUTC.Hour() == 0 && targetUTC.Minute() == 0 && targetUTC.Second() == 0 &&
		targetUTC.Nanosecond() == 0 {
		todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return targetT.Before(todayMidnight)
	}
	return targetT.Before(now)
}

// nextWeekday returns the next occurrence of the specified weekday after from.
func nextWeekday(from time.Time, targetWeekday time.Weekday) time.Time {
	daysUntil := (int(targetWeekday) - int(from.Weekday()) + 7) % 7
	if daysUntil == 0 {
		daysUntil = 7 // Next week's occurrence
	}
	return from.AddDate(0, 0, daysUntil)
}
