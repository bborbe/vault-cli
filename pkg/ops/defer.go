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

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/defer-operation.go --fake-name DeferOperation . DeferOperation
type DeferOperation interface {
	Execute(ctx context.Context, vaultPath string, taskName string, dateStr string) error
}

// NewDeferOperation creates a new defer operation.
func NewDeferOperation(
	storage storage.Storage,
) DeferOperation {
	return &deferOperation{
		storage: storage,
	}
}

type deferOperation struct {
	storage storage.Storage
}

// Execute defers a task to a specific date.
func (d *deferOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	dateStr string,
) error {
	// Parse the date string
	targetDate, err := d.parseDate(dateStr)
	if err != nil {
		return fmt.Errorf("parse date: %w", err)
	}

	// Find and read the task
	task, err := d.storage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return fmt.Errorf("find task: %w", err)
	}

	// Update task status and defer date
	task.Status = domain.TaskStatusDeferred
	task.DeferDate = &targetDate

	// Write updated task
	if err := d.storage.WriteTask(ctx, task); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	// Remove from today's daily note
	today := time.Now().Format("2006-01-02")
	if err := d.removeFromDailyNote(ctx, vaultPath, today, task.Name); err != nil {
		fmt.Printf("Warning: failed to update today's daily note: %v\n", err)
	}

	// Add to target date's daily note
	targetDateStr := targetDate.Format("2006-01-02")
	if err := d.addToDailyNote(ctx, vaultPath, targetDateStr, task.Name); err != nil {
		fmt.Printf("Warning: failed to update target daily note: %v\n", err)
	}

	fmt.Printf("📅 Task deferred to %s: %s\n", targetDateStr, task.Name)
	return nil
}

// parseDate parses various date formats: +Nd, weekday names, ISO dates.
func (d *deferOperation) parseDate(dateStr string) (time.Time, error) {
	now := time.Now()

	// Handle relative dates: +1d, +7d, etc.
	if matched, _ := regexp.MatchString(`^\+\d+d$`, dateStr); matched {
		var days int
		if _, err := fmt.Sscanf(dateStr, "+%dd", &days); err != nil {
			return time.Time{}, fmt.Errorf("parse relative date: %w", err)
		}
		return now.AddDate(0, 0, days), nil
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
		return d.nextWeekday(now, weekday), nil
	}

	// Handle ISO date: 2024-12-31
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf(
		"invalid date format: %s (use +Nd, weekday, or YYYY-MM-DD)",
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
	content, err := d.storage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return fmt.Errorf("read daily note: %w", err)
	}

	if content == "" {
		return nil // No daily note exists
	}

	lines := strings.Split(content, "\n")
	filteredLines := make([]string, 0, len(lines))
	checkboxRegex := regexp.MustCompile(`^(\s*)- \[([ x])\] (.+)$`)

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
	if err := d.storage.WriteDailyNote(ctx, vaultPath, date, updatedContent); err != nil {
		return fmt.Errorf("write daily note: %w", err)
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
	content, err := d.storage.ReadDailyNote(ctx, vaultPath, date)
	if err != nil {
		return fmt.Errorf("read daily note: %w", err)
	}

	// Create task line
	taskLine := fmt.Sprintf("- [ ] [[%s]]", taskName)

	// If content is empty, create a basic daily note structure
	if content == "" {
		content = fmt.Sprintf("# %s\n\n## Tasks\n\n%s\n", date, taskLine)
	} else {
		// Append to tasks section or end of file
		content = strings.TrimRight(content, "\n") + "\n" + taskLine + "\n"
	}

	if err := d.storage.WriteDailyNote(ctx, vaultPath, date, content); err != nil {
		return fmt.Errorf("write daily note: %w", err)
	}

	return nil
}
