// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain

import (
	"context"
	"time"

	libtime "github.com/bborbe/time"
)

// DateOrDateTime wraps time.Time and serializes as YYYY-MM-DD for pure date values
// (midnight UTC) and RFC3339 for datetime values with a time component.
type DateOrDateTime time.Time

// MarshalText implements encoding.TextMarshaler.
// Values with zero hour/minute/second/nanosecond in UTC serialize as YYYY-MM-DD;
// all others serialize as RFC3339.
func (d DateOrDateTime) MarshalText() ([]byte, error) {
	t := time.Time(d).UTC()
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return []byte(time.Time(d).Format(time.DateOnly)), nil
	}
	return []byte(time.Time(d).Format(time.RFC3339)), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
// Accepts YYYY-MM-DD and RFC3339 formats (delegated to libtime.ParseTime).
func (d *DateOrDateTime) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	ctx := context.Background()
	t, err := libtime.ParseTime(ctx, string(data))
	if err != nil {
		return err
	}
	*d = DateOrDateTime(*t)
	return nil
}

// Time returns the underlying time.Time value.
func (d DateOrDateTime) Time() time.Time {
	return time.Time(d)
}

// Ptr returns a pointer to a copy of d.
func (d DateOrDateTime) Ptr() *DateOrDateTime {
	return &d
}

// IsZero reports whether d represents the zero time instant.
func (d DateOrDateTime) IsZero() bool {
	return time.Time(d).IsZero()
}

// Before reports whether d is before other.
func (d DateOrDateTime) Before(other time.Time) bool {
	return time.Time(d).Before(other)
}

// Format returns a textual representation of d using the given layout.
func (d DateOrDateTime) Format(layout string) string {
	return time.Time(d).Format(layout)
}
