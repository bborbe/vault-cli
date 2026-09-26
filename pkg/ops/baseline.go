// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/storage"
)

// The baseline file's frontmatter keys. They are frozen: the file is authored by
// hand outside the code and read here, so renaming one is a behaviour change, not
// a style choice. docs/baseline-file.md is the authority for the key set.
const (
	rollupBaselineKeyCaptured      = "baseline_captured"
	rollupBaselineKeyHumanTotal    = "baseline_human_total"
	rollupBaselineKeyMedian        = "baseline_median"
	rollupBaselineKeyWeeks         = "baseline_weeks"
	rollupBaselineKeyAgentCoverage = "baseline_agent_coverage"
)

// rollupBaselineDateLayout is the layout the capture date is rendered in, both
// when it arrives as a YAML timestamp and when it is echoed from a string.
const rollupBaselineDateLayout = "2006-01-02"

// RollupBaseline is a vault's stored baseline: the five figures the file records,
// echoed verbatim, plus the delta from each computed figure to its analogue here.
// It is nil on a RollupWeeklyResult whose vault has no baseline configured.
type RollupBaseline struct {
	Captured      string               `json:"captured"`
	HumanTotal    int                  `json:"human_total"`
	Median        int                  `json:"median"`
	Weeks         map[string]int       `json:"weeks"`
	AgentCoverage string               `json:"agent_coverage"`
	Deltas        RollupBaselineDeltas `json:"deltas"`
}

// RollupBaselineDeltas holds one row per computed figure that has a baseline
// analogue. A figure with no analogue — the unattended-delivery count, the stored
// total, the stored agent coverage — has no field here, and a figure whose row is
// omitted (the requested week is absent from Weeks, or the computed figure is not
// a number) leaves its pointer nil.
type RollupBaselineDeltas struct {
	HumanInteractions *RollupBaselineDelta `json:"human_interactions,omitempty"`
	PerFamilyMedian   *RollupBaselineDelta `json:"per_family_median,omitempty"`
}

// RollupBaselineDelta is one computed-versus-stored movement. Computed is the
// rollup's own figure parsed from its rendered string, Baseline the stored
// analogue, Delta their difference, and Mismatch marks a row whose two figures
// measure different things — the JSON form of the plain report's frozen
// `[definitional mismatch]` marker.
type RollupBaselineDelta struct {
	Computed float64 `json:"computed"`
	Baseline float64 `json:"baseline"`
	Delta    float64 `json:"delta"`
	Mismatch bool    `json:"mismatch,omitempty"`
}

// readRollupBaseline reads and validates the baseline file at resolvedPath for
// vaultName. Every figure is echoed as the file stores it: none is recomputed,
// rounded, reformatted or normalised. A missing or unreadable file, a frontmatter
// block that will not parse, a required key that is absent, and a value of the
// wrong shape are each an error naming the file and the key.
func readRollupBaseline(
	ctx context.Context,
	resolvedPath string,
	vaultName string,
) (*RollupBaseline, error) {
	content, err := os.ReadFile(resolvedPath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrapf(
			ctx, err, "read baseline file %s of vault %s", resolvedPath, vaultName,
		)
	}

	m, err := storage.ParseFrontmatterMap(ctx, content)
	if err != nil {
		return nil, errors.Wrapf(
			ctx, err, "parse baseline file %s of vault %s", resolvedPath, vaultName,
		)
	}

	where := fmt.Sprintf("%s of vault %s", resolvedPath, vaultName)

	captured, err := rollupBaselineCaptured(ctx, m, where)
	if err != nil {
		return nil, err
	}
	humanTotal, err := rollupBaselineInt(ctx, m, rollupBaselineKeyHumanTotal, where)
	if err != nil {
		return nil, err
	}
	median, err := rollupBaselineInt(ctx, m, rollupBaselineKeyMedian, where)
	if err != nil {
		return nil, err
	}
	weeks, err := rollupBaselineWeeks(ctx, m, where)
	if err != nil {
		return nil, err
	}
	agentCoverage, err := rollupBaselineString(ctx, m, rollupBaselineKeyAgentCoverage, where)
	if err != nil {
		return nil, err
	}

	return &RollupBaseline{
		Captured:      captured,
		HumanTotal:    humanTotal,
		Median:        median,
		Weeks:         weeks,
		AgentCoverage: agentCoverage,
	}, nil
}

// rollupBaselineCaptured extracts the capture date. The contract's example writes
// the date unquoted, so YAML resolves it as a timestamp; the value is rendered
// back to its date form rather than to a Go time string.
func rollupBaselineCaptured(ctx context.Context, m map[string]any, where string) (string, error) {
	value, ok := m[rollupBaselineKeyCaptured]
	if !ok {
		return "", errors.Errorf(
			ctx, "baseline file %s is missing required key %s", where, rollupBaselineKeyCaptured,
		)
	}
	switch typed := value.(type) {
	case time.Time:
		return typed.Format(rollupBaselineDateLayout), nil
	case string:
		if typed == "" {
			return "", errors.Errorf(
				ctx, "baseline file %s has an empty %s", where, rollupBaselineKeyCaptured,
			)
		}
		return typed, nil
	default:
		return "", errors.Errorf(
			ctx, "baseline file %s has a non-date %s", where, rollupBaselineKeyCaptured,
		)
	}
}

// rollupBaselineInt extracts an integer figure. Nothing is coerced: a string or a
// float64 is an error naming the key, never a parsed number.
func rollupBaselineInt(
	ctx context.Context,
	m map[string]any,
	key string,
	where string,
) (int, error) {
	value, ok := m[key]
	if !ok {
		return 0, errors.Errorf(ctx, "baseline file %s is missing required key %s", where, key)
	}
	typed, ok := value.(int)
	if !ok {
		return 0, errors.Errorf(ctx, "baseline file %s has a non-integer %s", where, key)
	}
	return typed, nil
}

// rollupBaselineWeeks extracts the per-week figure map. Every entry must be an
// integer; a malformed entry names both the key and the offending week token.
func rollupBaselineWeeks(
	ctx context.Context,
	m map[string]any,
	where string,
) (map[string]int, error) {
	value, ok := m[rollupBaselineKeyWeeks]
	if !ok {
		return nil, errors.Errorf(
			ctx, "baseline file %s is missing required key %s", where, rollupBaselineKeyWeeks,
		)
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, errors.Errorf(
			ctx, "baseline file %s has a non-map %s", where, rollupBaselineKeyWeeks,
		)
	}
	weeks := make(map[string]int, len(raw))
	for week, entry := range raw {
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx, ctx.Err(), "context cancelled")
		default:
		}
		figure, ok := entry.(int)
		if !ok {
			return nil, errors.Errorf(
				ctx,
				"baseline file %s has a non-integer %s for week %s",
				where,
				rollupBaselineKeyWeeks,
				week,
			)
		}
		weeks[week] = figure
	}
	return weeks, nil
}

// rollupBaselineString extracts a string figure. The stored text is echoed, never
// parsed into numbers.
func rollupBaselineString(
	ctx context.Context,
	m map[string]any,
	key string,
	where string,
) (string, error) {
	value, ok := m[key]
	if !ok {
		return "", errors.Errorf(ctx, "baseline file %s is missing required key %s", where, key)
	}
	typed, ok := value.(string)
	if !ok {
		return "", errors.Errorf(ctx, "baseline file %s has a non-string %s", where, key)
	}
	return typed, nil
}

// rollupBaselineDeltas maps the rollup's computed figures to their baseline
// analogues. The mapping is fixed: HumanInteractions maps to the baseline's figure
// for the requested week when that week is a key of Weeks; PerFamilyMedian maps to
// Median and its row is marked a definitional mismatch, because the stored figure
// is a per-task median while the computed one is a median of per-family medians.
// A computed figure that is not a number — `no data`, `no recorded counts`,
// `undefined` — produces no row at all, and never a row reading zero.
func rollupBaselineDeltas(
	baseline *RollupBaseline,
	weekToken string,
	computedHumanInteractions string,
	computedPerFamilyMedian string,
) RollupBaselineDeltas {
	deltas := RollupBaselineDeltas{}
	if computed, err := strconv.ParseFloat(computedHumanInteractions, 64); err == nil {
		if stored, ok := baseline.Weeks[weekToken]; ok {
			deltas.HumanInteractions = &RollupBaselineDelta{
				Computed: computed,
				Baseline: float64(stored),
				Delta:    computed - float64(stored),
			}
		}
	}
	if computed, err := strconv.ParseFloat(computedPerFamilyMedian, 64); err == nil {
		deltas.PerFamilyMedian = &RollupBaselineDelta{
			Computed: computed,
			Baseline: float64(baseline.Median),
			Delta:    computed - float64(baseline.Median),
			Mismatch: true,
		}
	}
	return deltas
}

// rollupBaselineWeekToken renders a resolved ISO year and week as the YYYY-Wnn
// token used as a key of the baseline's week map. The week is zero-padded to two
// digits, matching the contract's keys.
func rollupBaselineWeekToken(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}
