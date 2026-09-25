// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// The three explicit statements the rollup prints in place of a figure. A zero
// is a measurement; these are the absence of one, and the two must never be
// confused — a missing measurement is never rendered as a measurement of nothing.
const (
	rollupNoData           = "no data"
	rollupNoRecordedCounts = "no recorded counts"
	rollupUndefined        = "undefined"
)

// rollupDateLayout is the layout of the rendered week range.
const rollupDateLayout = "2006-01-02"

var (
	// rollupWeekTokenRegex matches a week token as YYYY-Wnn: four digits, a
	// literal dash, an uppercase W, exactly two digits.
	rollupWeekTokenRegex = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)

	// rollupDateStripRegex strips an ISO date from a filename stem.
	rollupDateStripRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	// rollupYearWeekStripRegex strips an ISO year-and-week from a filename stem.
	rollupYearWeekStripRegex = regexp.MustCompile(`\d{4}-w\d{1,2}`)
	// rollupBareWeekStripRegex strips a bare week number from a filename stem.
	rollupBareWeekStripRegex = regexp.MustCompile(`\bw\d{1,2}\b`)
	// rollupVersionStripRegex strips a version from a filename stem.
	rollupVersionStripRegex = regexp.MustCompile(`\bv?\d+(\.\d+)+\b`)
	// rollupMonthStripRegex strips a month name from a filename stem.
	rollupMonthStripRegex = regexp.MustCompile(
		`\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b`,
	)
	// rollupSeparatorRegex collapses every run of non-alphanumeric characters to
	// a single space, which also guarantees a family key contains no colon.
	rollupSeparatorRegex = regexp.MustCompile(`[^a-z0-9]+`)
)

//counterfeiter:generate -o ../../mocks/rollup-weekly-operation.go --fake-name RollupWeeklyOperation . RollupWeeklyOperation

// RollupWeeklyOperation computes the weekly unattended-delivery rollup for one vault.
type RollupWeeklyOperation interface {
	// Execute computes the rollup for one ISO week of one vault. week is the raw
	// --week token (YYYY-Wnn); an empty token means the last complete ISO week,
	// taken from the injected clock. vaultName is carried for error messages only.
	Execute(
		ctx context.Context,
		vaultPath string,
		vaultName string,
		week string,
	) (RollupWeeklyResult, error)
}

// NewRollupWeeklyOperation creates a new weekly rollup operation.
func NewRollupWeeklyOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
) RollupWeeklyOperation {
	return &rollupWeeklyOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
	}
}

type rollupWeeklyOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
}

// RollupWeeklyResult is the computed weekly rollup for one vault.
type RollupWeeklyResult struct {
	Year                 int            `json:"year"`
	Week                 int            `json:"week"`
	WeekStart            string         `json:"week_start"`
	WeekEnd              string         `json:"week_end"`
	HumanInteractions    string         `json:"human_interactions"`
	UnattendedDeliveries string         `json:"unattended_deliveries"`
	PerFamilyMedian      string         `json:"per_family_median"`
	Families             []RollupFamily `json:"families"`
}

// RollupFamily is one task family's median within the week's task set.
type RollupFamily struct {
	Name   string `json:"name"`
	Median string `json:"median"`
}

// Execute resolves the week, reads the vault's task files strictly, and derives
// the three figures plus the per-family breakdown behind them.
func (o *rollupWeeklyOperation) Execute(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	week string,
) (RollupWeeklyResult, error) {
	year, weekNumber, monday, sunday, err := o.resolveWeek(ctx, week)
	if err != nil {
		return RollupWeeklyResult{}, err
	}

	result := RollupWeeklyResult{
		Year:      year,
		Week:      weekNumber,
		WeekStart: monday.Format(rollupDateLayout),
		WeekEnd:   sunday.Format(rollupDateLayout),
		Families:  []RollupFamily{},
	}

	tasks, err := o.taskStorage.ListTasksStrict(ctx, vaultPath)
	if err != nil {
		return RollupWeeklyResult{}, errors.Wrapf(ctx, err, "list tasks of vault %s", vaultName)
	}

	weekTasks := selectRollupWeekTasks(tasks, year, weekNumber)
	if len(weekTasks) == 0 {
		result.HumanInteractions = rollupNoData
		result.UnattendedDeliveries = rollupNoData
		result.PerFamilyMedian = rollupNoData
		return result, nil
	}

	humanInteractions, unattended, recorded := rollupFigures(weekTasks)
	if recorded {
		result.HumanInteractions = strconv.Itoa(humanInteractions)
		result.UnattendedDeliveries = strconv.Itoa(unattended)
	} else {
		result.HumanInteractions = rollupNoRecordedCounts
		result.UnattendedDeliveries = rollupNoRecordedCounts
	}

	families, headline := rollupFamilyMedians(weekTasks)
	result.Families = families
	result.PerFamilyMedian = headline

	return result, nil
}

// resolveWeek resolves the raw --week token into the ISO year, week number and
// Monday/Sunday range. An empty token means the last complete ISO week, taken
// from the injected clock; a malformed or out-of-range token is a usage error
// naming the expected format.
func (o *rollupWeeklyOperation) resolveWeek(
	ctx context.Context,
	week string,
) (int, int, time.Time, time.Time, error) {
	if week == "" {
		prev := o.currentDateTime.Now().Time().AddDate(0, 0, -7)
		year, weekNumber := prev.ISOWeek()
		monday, sunday := isoWeekRange(year, weekNumber)
		return year, weekNumber, monday, sunday, nil
	}

	matches := rollupWeekTokenRegex.FindStringSubmatch(week)
	if len(matches) != 3 {
		return 0, 0, time.Time{}, time.Time{}, rollupInvalidWeekError(ctx, week)
	}
	year, yearErr := strconv.Atoi(matches[1])
	weekNumber, weekErr := strconv.Atoi(matches[2])
	if yearErr != nil || weekErr != nil {
		return 0, 0, time.Time{}, time.Time{}, rollupInvalidWeekError(ctx, week)
	}
	_, weeks := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	if weekNumber < 1 || weekNumber > weeks {
		return 0, 0, time.Time{}, time.Time{}, rollupInvalidWeekError(ctx, week)
	}

	monday, sunday := isoWeekRange(year, weekNumber)
	return year, weekNumber, monday, sunday, nil
}

// rollupInvalidWeekError is the single usage error for a rejected week token.
func rollupInvalidWeekError(ctx context.Context, week string) error {
	return errors.Errorf(ctx, "invalid --week %q: expected YYYY-Wnn, e.g. 2026-W37", week)
}

// isoWeekRange returns the Monday and Sunday of the given ISO week in UTC.
func isoWeekRange(year, week int) (time.Time, time.Time) {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	offset := (int(jan4.Weekday()) + 6) % 7
	monday := jan4.AddDate(0, 0, -offset+(week-1)*7)
	return monday, monday.AddDate(0, 0, 6)
}

// selectRollupWeekTasks returns the tasks whose metrics_completed_at falls in the
// given ISO week, judged in the timestamp's own offset — a task completing just
// after local midnight lands in its own local week, not the previous UTC one.
// A task with an absent or unparseable metrics_completed_at is in no week at all.
func selectRollupWeekTasks(tasks []*domain.Task, year, week int) []*domain.Task {
	selected := make([]*domain.Task, 0, len(tasks))
	for _, task := range tasks {
		completedAt := task.MetricsCompletedAt()
		if completedAt == nil {
			continue
		}
		taskYear, taskWeek := completedAt.Time().ISOWeek()
		if taskYear == year && taskWeek == week {
			selected = append(selected, task)
		}
	}
	return selected
}

// rollupFigures returns the week's human-interaction sum, its unattended-delivery
// count, and whether any member carries a recorded count. A recorded zero is a
// measurement: it is a delivery when the task is completed, and adds nothing to
// the sum. An absent or malformed count is neither.
func rollupFigures(tasks []*domain.Task) (int, int, bool) {
	var humanInteractions, unattended int
	recorded := false
	for _, task := range tasks {
		count := task.MetricsInteractionCount()
		if count == nil {
			continue
		}
		recorded = true
		humanInteractions += *count
		if *count == 0 && task.Status() == domain.TaskStatusCompleted {
			unattended++
		}
	}
	return humanInteractions, unattended, recorded
}

// rollupFamilyMedians groups the week's tasks by family and returns one row per
// family present, sorted ascending by name, plus the headline median taken over
// the families whose median is defined. A family with no recorded count among its
// members reports undefined and contributes no datum to the headline.
func rollupFamilyMedians(tasks []*domain.Task) ([]RollupFamily, string) {
	countsByFamily := make(map[string][]float64, len(tasks))
	names := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		name := rollupFamilyName(task.Name)
		names[name] = struct{}{}
		if count := task.MetricsInteractionCount(); count != nil {
			countsByFamily[name] = append(countsByFamily[name], float64(*count))
		}
	}

	families := make([]RollupFamily, 0, len(names))
	medians := make([]float64, 0, len(names))
	for name := range names {
		family := RollupFamily{Name: name, Median: rollupUndefined}
		if counts := countsByFamily[name]; len(counts) > 0 {
			median := rollupMedian(counts)
			family.Median = strconv.FormatFloat(median, 'f', -1, 64)
			medians = append(medians, median)
		}
		families = append(families, family)
	}
	sort.Slice(families, func(i, j int) bool { return families[i].Name < families[j].Name })

	headline := rollupUndefined
	if len(medians) > 0 {
		headline = strconv.FormatFloat(rollupMedian(medians), 'f', -1, 64)
	}
	return families, headline
}

// rollupFamilyName reduces a task filename stem to its family key: lowercase,
// strip dates, week numbers, versions and month names, collapse every separator
// run to one space, and trim. A stem left empty by the stripping (nothing but a
// date) falls back to the collapsed lowercase stem, so the key is never blank.
func rollupFamilyName(stem string) string {
	lowered := strings.ToLower(stem)
	stripped := rollupDateStripRegex.ReplaceAllString(lowered, " ")
	stripped = rollupYearWeekStripRegex.ReplaceAllString(stripped, " ")
	stripped = rollupBareWeekStripRegex.ReplaceAllString(stripped, " ")
	stripped = rollupVersionStripRegex.ReplaceAllString(stripped, " ")
	stripped = rollupMonthStripRegex.ReplaceAllString(stripped, " ")

	if key := rollupCollapse(stripped); key != "" {
		return key
	}
	return rollupCollapse(lowered)
}

// rollupCollapse collapses every run of non-alphanumeric characters to a single
// space and trims the result.
func rollupCollapse(value string) string {
	return strings.TrimSpace(rollupSeparatorRegex.ReplaceAllString(value, " "))
}

// rollupMedian returns the median of values. The caller guarantees a non-empty
// slice: the middle value for an odd count, the arithmetic mean of the two middle
// values for an even count. The caller's slice is not mutated.
func rollupMedian(values []float64) float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}
