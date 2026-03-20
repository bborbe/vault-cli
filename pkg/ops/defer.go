// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/defer-operation.go --fake-name DeferOperation . DeferOperation
type DeferOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		dateStr string,
		vaultName string,
	) (MutationResult, error)
}

// NewDeferOperation creates a new defer operation.
func NewDeferOperation(
	taskStorage storage.TaskStorage,
	dailyNoteStorage storage.DailyNoteStorage,
	currentDateTime libtime.CurrentDateTime,
) DeferOperation {
	return &deferOperation{
		taskStorage:      taskStorage,
		dailyNoteStorage: dailyNoteStorage,
		currentDateTime:  currentDateTime,
	}
}

type deferOperation struct {
	taskStorage      storage.TaskStorage
	dailyNoteStorage storage.DailyNoteStorage
	currentDateTime  libtime.CurrentDateTime
}

// Execute defers a task to a specific date or datetime.
func (d *deferOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	dateStr string,
	vaultName string,
) (MutationResult, error) {
	// Parse initial target date
	targetDate, err := d.parseDate(dateStr)
	if err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"parse date",
			)
	}

	// Find task early so we can adjust +Nd for time preservation
	task, err := d.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(
				ctx,
				err,
				"find task",
			)
	}

	// For +Nd: if existing DeferDate has a time component, preserve it
	if matched, _ := regexp.MatchString(`^\+\d+d$`, dateStr); matched && task.DeferDate != nil {
		existingT := task.DeferDate.Time()
		existingUTC := existingT.UTC()
		if existingUTC.Hour() != 0 || existingUTC.Minute() != 0 || existingUTC.Second() != 0 ||
			existingUTC.Nanosecond() != 0 {
			var days int
			if _, err := fmt.Sscanf(dateStr, "+%dd", &days); err == nil {
				targetDate = domain.DateOrDateTime(existingT.AddDate(0, 0, days))
			}
		}
	}

	// Validate target is not in the past (date-only: compare at day level; datetime: compare at full precision)
	now := d.currentDateTime.Now().Time()
	if d.isInPast(targetDate, now) {
		baseErr := fmt.Errorf(
			"cannot defer to past date: %s",
			targetDate.Time().Format("2006-01-02"),
		) //nolint:goerr113
		return MutationResult{
				Success: false,
				Error:   baseErr.Error(),
			}, errors.Wrap(
				ctx,
				baseErr,
				"validate date",
			)
	}

	// Find and update task
	task, err = d.findAndDeferTask(ctx, task, targetDate)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// Update daily notes
	warnings := d.updateDailyNotes(ctx, vaultPath, task.Name, targetDate)

	// Return result with formatted date in Message field
	formattedDate := targetDate.Time().Format("2006-01-02")
	return MutationResult{
		Success:  true,
		Name:     task.Name,
		Vault:    vaultName,
		Warnings: warnings,
		Message:  formattedDate,
	}, nil
}

// isInPast reports whether targetDate is in the past relative to now.
// For date-only values (midnight UTC), comparison is at day granularity.
// For datetime values, comparison is at full precision.
func (d *deferOperation) isInPast(targetDate domain.DateOrDateTime, now time.Time) bool {
	targetT := targetDate.Time()
	targetUTC := targetT.UTC()
	if targetUTC.Hour() == 0 && targetUTC.Minute() == 0 && targetUTC.Second() == 0 &&
		targetUTC.Nanosecond() == 0 {
		// Date-only: compare at date level so "today" is never in the past
		todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return targetT.Before(todayMidnight)
	}
	return targetT.Before(now)
}

// findAndDeferTask updates task defer status and writes it.
func (d *deferOperation) findAndDeferTask(
	ctx context.Context,
	task *domain.Task,
	targetDate domain.DateOrDateTime,
) (*domain.Task, error) {
	task.DeferDate = targetDate.Ptr()

	// Clear planned_date if it's before the defer target date
	if task.PlannedDate != nil && task.PlannedDate.Before(targetDate.Time()) {
		task.PlannedDate = nil
	}

	if err := d.taskStorage.WriteTask(ctx, task); err != nil {
		return nil, errors.Wrap(ctx, err, "write task")
	}
	return task, nil
}

// updateDailyNotes updates daily notes and returns warnings.
func (d *deferOperation) updateDailyNotes(
	ctx context.Context,
	vaultPath string,
	taskName string,
	targetDate domain.DateOrDateTime,
) []string {
	var warnings []string
	today := d.currentDateTime.Now().Format("2006-01-02")
	if err := d.removeFromDailyNote(ctx, vaultPath, today, taskName); err != nil {
		w := fmt.Sprintf("failed to update today's daily note: %v", err)
		warnings = append(warnings, w)
		slog.Warn("defer warning", "warning", w)
	}
	targetDateStr := targetDate.Time().Format("2006-01-02")
	if err := d.addToDailyNote(ctx, vaultPath, targetDateStr, taskName); err != nil {
		w := fmt.Sprintf("failed to update target daily note: %v", err)
		warnings = append(warnings, w)
		slog.Warn("defer warning", "warning", w)
	}
	return warnings
}

// parseDate parses various date formats: +Nd, weekday names, ISO dates, RFC3339 datetimes.
func (d *deferOperation) parseDate(dateStr string) (domain.DateOrDateTime, error) {
	now := d.currentDateTime.Now().Time()

	// Handle relative dates: +1d, +7d, etc.
	if matched, _ := regexp.MatchString(`^\+\d+d$`, dateStr); matched {
		var days int
		if _, err := fmt.Sscanf(dateStr, "+%dd", &days); err != nil {
			return domain.DateOrDateTime{}, fmt.Errorf("parse relative date: %w", err)
		}
		t := libtime.ToDate(now.AddDate(0, 0, days)).Time()
		return domain.DateOrDateTime(t), nil
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
		t := libtime.ToDate(d.nextWeekday(now, weekday)).Time()
		return domain.DateOrDateTime(t), nil
	}

	// Handle ISO date: 2024-12-31
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return domain.DateOrDateTime(t), nil
	}

	// Handle RFC3339 datetime: 2026-03-19T16:00:00+01:00
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return domain.DateOrDateTime(t), nil
	}

	return domain.DateOrDateTime{}, fmt.Errorf(
		"invalid date format: %s (use +Nd, weekday, YYYY-MM-DD, or RFC3339)",
		dateStr,
	)
}

// nextWeekday returns the next occurrence of the specified weekday.
func (d *deferOperation) nextWeekday(from time.Time, targetWeekday time.Weekday) time.Time {
	daysUntil := (int(targetWeekday) - int(from.Weekday()) + 7) % 7
	if daysUntil == 0 {
		daysUntil = 7 // Next week's occurrence
	}
	return from.AddDate(0, 0, daysUntil)
}

// removeFromDailyNote removes a task from a daily note.
func (d *deferOperation) removeFromDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	taskName string,
) error {
	content, err := d.dailyNoteStorage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return errors.Wrap(ctx, err, "read daily note")
	}

	if content == "" {
		return nil // No daily note exists
	}

	lines := strings.Split(content, "\n")
	filteredLines := make([]string, 0, len(lines))
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)

	for _, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			taskText := matches[3]
			if strings.Contains(strings.ToLower(taskText), strings.ToLower(taskName)) {
				continue // Skip this line
			}
		}
		filteredLines = append(filteredLines, line)
	}

	updatedContent := strings.Join(filteredLines, "\n")
	if err := d.dailyNoteStorage.WriteDailyNote(ctx, vaultPath, date, updatedContent); err != nil {
		return errors.Wrap(ctx, err, "write daily note")
	}

	return nil
}

// addToDailyNote adds a task to a daily note.
func (d *deferOperation) addToDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	taskName string,
) error {
	content, err := d.dailyNoteStorage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return errors.Wrap(ctx, err, "read daily note")
	}

	// Create task line
	taskLine := fmt.Sprintf("- [ ] [[%s]]", taskName)

	// If content is empty, create a basic daily note structure with Should section
	if content == "" {
		content = fmt.Sprintf("# %s\n\n## Should\n\n%s\n", date, taskLine)
		return d.dailyNoteStorage.WriteDailyNote(ctx, vaultPath, date, content)
	}

	// Check if task already exists
	if strings.Contains(content, taskLine) {
		return nil // Task already in daily note
	}

	// Insert task into appropriate section
	updatedContent := d.insertTaskIntoSection(content, taskLine)

	return d.dailyNoteStorage.WriteDailyNote(ctx, vaultPath, date, updatedContent)
}

// insertTaskIntoSection inserts a task into the appropriate section.
func (d *deferOperation) insertTaskIntoSection(content string, taskLine string) string {
	lines := strings.Split(content, "\n")

	// Try Should section first
	shouldIdx := d.findSectionIndex(lines, "should")
	if shouldIdx != -1 {
		endIdx := d.findSectionEnd(lines, shouldIdx)
		return d.insertTaskAtLine(lines, endIdx, taskLine)
	}

	// Try Must section
	mustIdx := d.findSectionIndex(lines, "must")
	if mustIdx != -1 {
		endIdx := d.findSectionEnd(lines, mustIdx)
		return d.insertTaskAtLine(lines, endIdx, taskLine)
	}

	// Fallback: append to end of file
	return strings.TrimRight(content, "\n") + "\n" + taskLine + "\n"
}

// findSectionIndex finds the index of a section heading (## Section or ### Section).
func (d *deferOperation) findSectionIndex(lines []string, sectionName string) int {
	sectionName = strings.ToLower(sectionName)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "## "+sectionName) ||
			strings.HasPrefix(lower, "### "+sectionName) {
			return i
		}
	}
	return -1
}

// findSectionEnd finds the end of a section (before next heading or end of file).
func (d *deferOperation) findSectionEnd(lines []string, sectionStartIdx int) int {
	// Start looking from the line after the section heading
	for i := sectionStartIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		// If we hit another heading, return the index before it
		if strings.HasPrefix(trimmed, "##") {
			return i
		}
	}
	// No next section found, return end of file
	return len(lines)
}

// insertTaskAtLine inserts a task line at the specified index.
func (d *deferOperation) insertTaskAtLine(lines []string, idx int, taskLine string) string {
	// Insert the task line at the specified index
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:idx]...)
	newLines = append(newLines, taskLine)
	newLines = append(newLines, lines[idx:]...)
	return strings.Join(newLines, "\n")
}
