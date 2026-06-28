// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bborbe/errors"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

// Package-level compiled regex patterns for performance.
var (
	frontmatterRegex      = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	fixDuplicateKeysRegex = regexp.MustCompile(`(?s)^(---\n)(.*?)(\n---\n)(.*)$`)
	keyRegex              = regexp.MustCompile(`^([a-z_]+):\s*`)
	priorityRegex         = regexp.MustCompile(`(?m)^priority:\s*['"]?([a-z]+)['"]?\s*$`)
	statusRegex           = regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
	phaseRegex            = regexp.MustCompile(`(?m)^phase:\s*['"]?([a-z_]+)['"]?\s*$`)
	checkboxRegex         = regexp.MustCompile(`(?m)^[\s]*[-*]\s+\[([ xX])\]`)
	goalsInlineRegex      = regexp.MustCompile(`(?m)^goals:\s*\[(.*?)\]`)
	goalsMultilineRegex   = regexp.MustCompile(`(?ms)^goals:\s*\n((?:\s*-\s*.+\n?)+)`)
	goalItemRegex         = regexp.MustCompile(`(?m)^\s*-\s*['"]?(.+?)['"]?\s*$`)
	dateRegex             = regexp.MustCompile(
		`(?m)^(planned_date|defer_date|due_date):\s*['"]?([^\s'"]+)?['"]?\s*$`,
	)
)

//counterfeiter:generate -o ../../mocks/lint-operation.go --fake-name LintOperation . LintOperation
type LintOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		tasksDir string,
		goalsDir string,
		fix bool,
	) ([]LintIssue, error)
	ExecuteFile(
		ctx context.Context,
		filePath string,
		taskName string,
		vaultName string,
	) ([]LintIssue, error)
}

// NewLintOperation creates a new lint operation.
func NewLintOperation() LintOperation {
	return &lintOperation{}
}

type lintOperation struct{}

// IssueType represents the type of lint issue found.
type IssueType string

const (
	IssueTypeMissingFrontmatter     IssueType = "MISSING_FRONTMATTER"
	IssueTypeInvalidPriority        IssueType = "INVALID_PRIORITY"
	IssueTypeDuplicateKey           IssueType = "DUPLICATE_KEY"
	IssueTypeInvalidStatus          IssueType = "INVALID_STATUS"
	IssueTypeOrphanGoal             IssueType = "ORPHAN_GOAL"
	IssueTypeStatusCheckboxMismatch IssueType = "STATUS_CHECKBOX_MISMATCH"
	IssueTypeStatusPhaseMismatch    IssueType = "STATUS_PHASE_MISMATCH"
	IssueTypeMissingTaskIdentifier  IssueType = "MISSING_TASK_IDENTIFIER"
	IssueTypeStatusDateMismatch     IssueType = "STATUS_DATE_MISMATCH"
	IssueTypeInvalidTaskIdentifier  IssueType = "INVALID_TASK_IDENTIFIER"
)

// LintIssue represents a single lint issue found in a file.
type LintIssue struct {
	FilePath    string
	IssueType   IssueType
	Description string
	Fixable     bool
	Fixed       bool
}

// LintIssueJSON represents a lint issue in JSON format.
type LintIssueJSON struct {
	File        string `json:"file"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Fixed       bool   `json:"fixed,omitempty"`
}

// Execute scans all task files for lint issues and optionally fixes them.
func (l *lintOperation) Execute(
	ctx context.Context,
	vaultPath string,
	tasksDir string,
	goalsDir string,
	fix bool,
) ([]LintIssue, error) {
	tasksDirPath := filepath.Join(vaultPath, tasksDir)

	// Walk through all .md files in the tasks directory
	var issues []LintIssue
	err := filepath.Walk(tasksDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(ctx, err, "walk %s", tasksDirPath)
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileIssues, err := l.lintFile(ctx, vaultPath, goalsDir, path, fix)
		if err != nil {
			return errors.Wrap(ctx, err, fmt.Sprintf("lint file %s", path))
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(ctx, err, "walk tasks directory")
	}

	return issues, nil
}

// ExecuteFile lints a single file and returns any lint issues found.
func (l *lintOperation) ExecuteFile(
	ctx context.Context,
	filePath string,
	taskName string,
	vaultName string,
) ([]LintIssue, error) {
	issues, err := l.lintFile(ctx, "", "", filePath, false)
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("lint file %s", filePath))
	}
	return issues, nil
}

// ValidateIssueJSON is the per-issue structure used in validate JSON output.
type ValidateIssueJSON struct {
	Type        string `json:"type"`
	IssueType   string `json:"issue_type"`
	Description string `json:"description"`
}

// ValidateResult is the structured result from ExecuteFile used in CLI JSON output.
type ValidateResult struct {
	Name   string              `json:"name"`
	Vault  string              `json:"vault"`
	Issues []ValidateIssueJSON `json:"issues"`
}

// lintFile checks a single file for lint issues and optionally fixes them.
func (l *lintOperation) lintFile(
	ctx context.Context,
	vaultPath string,
	goalsDir string,
	filePath string,
	fix bool,
) ([]LintIssue, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, "read file")
	}

	// Handle missing frontmatter first
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return l.handleMissingFrontmatterCase(vaultPath, goalsDir, filePath, content, fix)
	}

	// Collect all lint issues from the frontmatter and content
	issues := l.collectLintIssues(vaultPath, goalsDir, filePath, string(matches[1]), content)

	// Fix issues if requested
	if fix && len(issues) > 0 {
		issues, err = l.fixIssues(ctx, filePath, string(content), issues)
		if err != nil {
			return nil, errors.Wrap(ctx, err, "fix issues")
		}
	}

	return issues, nil
}

// handleMissingFrontmatterCase handles files without frontmatter
func (l *lintOperation) handleMissingFrontmatterCase(
	vaultPath string,
	goalsDir string,
	filePath string,
	content []byte,
	fix bool,
) ([]LintIssue, error) {
	issue, updatedContent, shouldReturn := l.handleMissingFrontmatter(filePath, content, fix)
	issues := make([]LintIssue, 0, 2) //nolint:mnd
	issues = append(issues, issue)

	if shouldReturn {
		return issues, nil
	}

	// After fixing frontmatter, re-parse and continue with other checks
	matches := frontmatterRegex.FindSubmatch(updatedContent)
	if len(matches) < 2 {
		return issues, nil
	}

	// Collect additional issues from the now-valid frontmatter
	additionalIssues := l.collectLintIssues(
		vaultPath,
		goalsDir,
		filePath,
		string(matches[1]),
		updatedContent,
	)
	return append(issues, additionalIssues...), nil
}

// collectLintIssues runs all lint checks and returns found issues
func (l *lintOperation) collectLintIssues(
	vaultPath string,
	goalsDir string,
	filePath string,
	frontmatterYAML string,
	content []byte,
) []LintIssue {
	issues := make([]LintIssue, 0, 4)
	add := func(issueType IssueType, desc string, fixable bool) {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   issueType,
			Description: desc,
			Fixable:     fixable,
			Fixed:       false,
		})
	}

	// Check for duplicate keys
	for _, key := range l.detectDuplicateKeys(frontmatterYAML) {
		add(IssueTypeDuplicateKey, fmt.Sprintf("key %q defined multiple times", key), true)
	}

	// Check for invalid priority
	if priorityIssue, invalidPriorityValue := l.detectInvalidPriority(frontmatterYAML); priorityIssue {
		add(
			IssueTypeInvalidPriority,
			fmt.Sprintf("priority is %q, expected int", invalidPriorityValue),
			true,
		)
	}

	// Check for invalid status
	if statusIssue, invalidStatusValue := l.detectInvalidStatus(frontmatterYAML); statusIssue {
		add(IssueTypeInvalidStatus, fmt.Sprintf(
			"status is %q, expected one of: next, in_progress, backlog, completed, hold, aborted",
			invalidStatusValue,
		), false)
	}

	// Check for status/phase mismatch
	if mismatchIssue, mismatchDesc := l.detectStatusPhaseMismatch(frontmatterYAML); mismatchIssue {
		add(IssueTypeStatusPhaseMismatch, mismatchDesc, false)
	}

	// Check for status/date mismatch (calendar-as-commitment rule)
	if mismatchIssue, mismatchDesc := l.detectStatusDateMismatch(frontmatterYAML); mismatchIssue {
		add(IssueTypeStatusDateMismatch, mismatchDesc, true)
	}

	// Check for orphan goals
	for _, goalName := range l.detectOrphanGoals(vaultPath, goalsDir, frontmatterYAML) {
		add(IssueTypeOrphanGoal, fmt.Sprintf("goal not found: %s", goalName), false)
	}

	// Check for status/checkbox mismatch
	if mismatchIssue, mismatchDesc, mismatchFixable := l.detectStatusCheckboxMismatch(
		frontmatterYAML,
		string(content),
	); mismatchIssue {
		add(IssueTypeStatusCheckboxMismatch, mismatchDesc, mismatchFixable)
	}

	// Check for missing task_identifier
	issues = append(issues, l.missingTaskIdentifierIssues(filePath, frontmatterYAML)...)

	// Check for invalid (non-UUID) task_identifier values
	issues = append(issues, l.invalidTaskIdentifierIssues(filePath, frontmatterYAML)...)

	return issues
}

// handleMissingFrontmatter handles the case when a file is missing frontmatter.
// Returns: (issue, updatedContent, shouldReturn)
func (l *lintOperation) handleMissingFrontmatter(
	filePath string,
	content []byte,
	fix bool,
) (LintIssue, []byte, bool) {
	issue := LintIssue{
		FilePath:    filePath,
		IssueType:   IssueTypeMissingFrontmatter,
		Description: "no frontmatter block found",
		Fixable:     true,
		Fixed:       false,
	}

	if !fix {
		return issue, content, true // Can't check further without frontmatter
	}

	// Fix missing frontmatter
	newContent, fixed := l.fixMissingFrontmatter(string(content))
	if !fixed {
		return issue, content, true
	}

	content = []byte(newContent)
	issue.Fixed = true

	// Write the fixed content to file
	//#nosec G304,G703 -- user-controlled vault path
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		// If write fails, return the issue as unfixed
		issue.Fixed = false
		return issue, content, true
	}

	return issue, content, false // Continue checking other issues
}

// detectDuplicateKeys detects duplicate YAML keys in frontmatter.
func (l *lintOperation) detectDuplicateKeys(frontmatterYAML string) []string {
	lines := strings.Split(frontmatterYAML, "\n")
	keysSeen := make(map[string]int)
	var duplicates []string

	for _, line := range lines {
		matches := keyRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			key := matches[1]
			keysSeen[key]++
			if keysSeen[key] == 2 {
				duplicates = append(duplicates, key)
			}
		}
	}

	return duplicates
}

// detectInvalidPriority detects if priority field is a string instead of int.
func (l *lintOperation) detectInvalidPriority(frontmatterYAML string) (bool, string) {
	matches := priorityRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		priorityValue := matches[1]
		// Check if it's a known string value
		validStringPriorities := []string{"high", "must", "should", "medium", "low"}
		for _, valid := range validStringPriorities {
			if priorityValue == valid {
				return true, priorityValue
			}
		}
	}
	return false, ""
}

// detectInvalidStatus detects if status field has an invalid value.
// Returns: (issueFound, invalidValue)
func (l *lintOperation) detectInvalidStatus(frontmatterYAML string) (bool, string) {
	matches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		statusValue := matches[1]
		_, ok := domain.NormalizeTaskStatus(statusValue)
		if ok {
			return false, "" // canonical or known alias — accepted silently
		}
		return true, statusValue // truly unknown, not fixable
	}
	return false, ""
}

// detectOrphanGoals detects goals that reference non-existent goal files.
// Returns list of missing goal names.
func (l *lintOperation) detectOrphanGoals(
	vaultPath string,
	goalsDir string,
	frontmatterYAML string,
) []string {
	if vaultPath == "" || goalsDir == "" {
		return nil // Skip if no vault path or no goals dir (single file validation)
	}

	// Extract goals field (YAML list) - try inline format first
	matches := goalsInlineRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		return l.parseInlineGoalsList(vaultPath, goalsDir, matches[1])
	}

	// Try multi-line YAML list format
	matches = goalsMultilineRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		return l.parseMultilineGoalsList(vaultPath, goalsDir, matches[1])
	}

	return nil
}

// parseInlineGoalsList parses inline goals list format: [goal1, goal2]
func (l *lintOperation) parseInlineGoalsList(
	vaultPath string,
	goalsDir string,
	goalsList string,
) []string {
	var orphanGoals []string
	for _, item := range strings.Split(goalsList, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		goalName := l.extractGoalName(item)
		if goalName == "" {
			continue
		}
		if !l.goalFileExists(vaultPath, goalsDir, goalName) {
			orphanGoals = append(orphanGoals, goalName)
		}
	}
	return orphanGoals
}

// parseMultilineGoalsList parses multi-line goals list format
func (l *lintOperation) parseMultilineGoalsList(
	vaultPath string,
	goalsDir string,
	yamlList string,
) []string {
	var orphanGoals []string
	for _, line := range strings.Split(yamlList, "\n") {
		itemMatches := goalItemRegex.FindStringSubmatch(line)
		if len(itemMatches) < 2 {
			continue
		}
		goalName := l.extractGoalName(itemMatches[1])
		if goalName == "" {
			continue
		}
		if !l.goalFileExists(vaultPath, goalsDir, goalName) {
			orphanGoals = append(orphanGoals, goalName)
		}
	}
	return orphanGoals
}

// extractGoalName strips wikilink brackets and whitespace from a goal name
func (l *lintOperation) extractGoalName(raw string) string {
	goalName := strings.TrimPrefix(raw, "[[")
	goalName = strings.TrimSuffix(goalName, "]]")
	return strings.TrimSpace(goalName)
}

// goalFileExists checks if a goal file exists in the vault
func (l *lintOperation) goalFileExists(vaultPath string, goalsDir string, goalName string) bool {
	goalPath := filepath.Join(vaultPath, goalsDir, goalName+".md")
	//#nosec G304,G703 -- user-controlled vault path
	_, err := os.Stat(goalPath)
	return !os.IsNotExist(err)
}

// detectStatusCheckboxMismatch detects mismatches between status and checkbox completion.
// Returns: (issueFound, description, isFixable)
func (l *lintOperation) detectStatusCheckboxMismatch(
	frontmatterYAML string,
	content string,
) (bool, string, bool) {
	// Skip if task is recurring
	if strings.Contains(frontmatterYAML, "recurring:") {
		return false, "", false
	}

	// Extract status
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return false, "", false
	}
	status := statusMatches[1]

	// Find all checkboxes in content
	checkboxMatches := checkboxRegex.FindAllStringSubmatch(content, -1)

	if len(checkboxMatches) == 0 {
		return false, "", false // No checkboxes
	}

	// Count checked and total checkboxes
	totalCheckboxes := len(checkboxMatches)
	checkedCheckboxes := 0
	for _, match := range checkboxMatches {
		if len(match) >= 2 && (match[1] == "x" || match[1] == "X") {
			checkedCheckboxes++
		}
	}

	// Case 1: status=completed but not all checkboxes are checked
	if status == "completed" && checkedCheckboxes < totalCheckboxes {
		unchecked := totalCheckboxes - checkedCheckboxes
		return true, fmt.Sprintf(
			"status is completed but %d/%d checkboxes unchecked",
			unchecked,
			totalCheckboxes,
		), false
	}

	// Case 2: all checkboxes checked but status is not completed
	if checkedCheckboxes == totalCheckboxes && status != "completed" {
		return true, fmt.Sprintf(
			"all checkboxes checked but status is %s",
			status,
		), true
	}

	return false, "", false
}

// detectStatusPhaseMismatch detects mismatches between status and phase fields.
// Returns: (issueFound, description)
func (l *lintOperation) detectStatusPhaseMismatch(frontmatterYAML string) (bool, string) {
	// Parse phase
	phaseMatches := phaseRegex.FindStringSubmatch(frontmatterYAML)
	if len(phaseMatches) < 2 {
		return false, "" // No phase key — no validation
	}
	phase := domain.TaskPhase(phaseMatches[1])

	// Parse status
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return false, ""
	}
	status := domain.TaskStatus(statusMatches[1])

	// Rule 1: completed/aborted status requires phase=done
	if (status == domain.TaskStatusCompleted || status == domain.TaskStatusAborted) &&
		phase != domain.TaskPhaseDone {
		return true, fmt.Sprintf(
			"status is %s but phase is %s (expected done or no phase)",
			status,
			phase,
		)
	}

	// Rule 2: phase=done requires completed status
	if phase == domain.TaskPhaseDone && status != domain.TaskStatusCompleted {
		return true, fmt.Sprintf("phase is done but status is %s (expected completed)", status)
	}

	// Rule 3: backlog/hold status incompatible with active phases
	activePhases := []domain.TaskPhase{
		domain.TaskPhaseExecution,
		domain.TaskPhaseInProgress,
		domain.TaskPhaseAIReview,
		domain.TaskPhaseHumanReview,
	}
	if status == domain.TaskStatusBacklog || status == domain.TaskStatusHold {
		for _, active := range activePhases {
			if phase == active {
				return true, fmt.Sprintf(
					"status is %s but phase is %s (active phase incompatible with inactive status)",
					status,
					phase,
				)
			}
		}
	}

	return false, ""
}

// detectStatusDateMismatch detects tasks whose status is next or backlog
// while any of planned_date, defer_date, or due_date is set.
// Per spec 017: calendar dates are commitments; only in_progress and terminal
// statuses are compatible with a date on an unstarted task.
// Returns: (issueFound, description)
func (l *lintOperation) detectStatusDateMismatch(frontmatterYAML string) (bool, string) {
	// Parse status
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return false, ""
	}
	status := domain.TaskStatus(statusMatches[1])

	// Only flag inactive statuses; completed/aborted/hold/in_progress are out of scope
	if status != domain.TaskStatusNext && status != domain.TaskStatusBacklog {
		return false, ""
	}

	// Check for any date field with a non-empty value
	// Match: `field: <value>` where value is non-empty (not just whitespace, not empty)
	matches := dateRegex.FindAllStringSubmatch(frontmatterYAML, -1)
	for _, m := range matches {
		if len(m) >= 3 && m[2] != "" {
			return true, fmt.Sprintf(
				"status is %s but %s is set (calendar dates are commitments; expected in_progress)",
				status, m[1],
			)
		}
	}
	return false, ""
}

// missingTaskIdentifierIssues returns a lint issue if task_identifier is absent or empty.
func (l *lintOperation) missingTaskIdentifierIssues(filePath, frontmatterYAML string) []LintIssue {
	if !l.detectMissingTaskIdentifier(frontmatterYAML) {
		return nil
	}
	return []LintIssue{{
		FilePath:    filePath,
		IssueType:   IssueTypeMissingTaskIdentifier,
		Description: "task_identifier is missing; run backfill to assign one",
		Fixable:     false,
		Fixed:       false,
	}}
}

// detectMissingTaskIdentifier returns true if task_identifier is absent or empty.
func (l *lintOperation) detectMissingTaskIdentifier(frontmatterYAML string) bool {
	var fm struct {
		TaskIdentifier string `yaml:"task_identifier"`
	}
	if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
		return false // Cannot parse; other checks will surface the error
	}
	return fm.TaskIdentifier == ""
}

// invalidTaskIdentifierIssues returns a lint issue if task_identifier is set
// to a value that does not parse as a UUID. Empty values are out of scope —
// they are covered by IssueTypeMissingTaskIdentifier (see missingTaskIdentifierIssues).
// Non-fixable on purpose: auto-fix would silently mint a fresh UUID, recreating
// the hidden creation site that causes concurrent-write merge conflicts on
// legacy tasks. Operator must replace the value with a real UUIDv4.
func (l *lintOperation) invalidTaskIdentifierIssues(filePath, frontmatterYAML string) []LintIssue {
	var fm struct {
		TaskIdentifier string `yaml:"task_identifier"`
	}
	if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
		return nil // Cannot parse; other checks will surface the error
	}
	if fm.TaskIdentifier == "" {
		return nil // Empty value is covered by MISSING_TASK_IDENTIFIER
	}
	if _, err := uuid.Parse(fm.TaskIdentifier); err == nil {
		return nil // Valid UUID — no issue
	}
	return []LintIssue{{
		FilePath:  filePath,
		IssueType: IssueTypeInvalidTaskIdentifier,
		Description: fmt.Sprintf(
			"task_identifier %q is not a valid UUID; replace with a fresh UUIDv4",
			fm.TaskIdentifier,
		),
		Fixable: false,
		Fixed:   false,
	}}
}

// fixIssues fixes fixable issues in the file.
func (l *lintOperation) fixIssues(
	ctx context.Context,
	filePath string,
	content string,
	issues []LintIssue,
) ([]LintIssue, error) {
	modified := false
	updatedContent := content

	apply := func(i int, fixFn func(string) (string, bool)) {
		newContent, fixed := fixFn(updatedContent)
		if !fixed {
			return
		}
		updatedContent = newContent
		issues[i].Fixed = true
		modified = true
	}

	for i := range issues {
		if !issues[i].Fixable {
			continue
		}
		switch issues[i].IssueType {
		case IssueTypeInvalidPriority:
			apply(i, l.fixInvalidPriority)
		case IssueTypeDuplicateKey:
			apply(i, l.fixDuplicateKeys)
		case IssueTypeInvalidStatus:
			apply(i, l.fixInvalidStatus)
		case IssueTypeStatusCheckboxMismatch:
			apply(i, l.fixStatusCheckboxMismatch)
		case IssueTypeStatusDateMismatch:
			apply(i, l.fixStatusDateMismatch)
		}
	}

	// Write fixed content back to file
	if modified {
		if err := os.WriteFile(filePath, []byte(updatedContent), 0600); err != nil { //#nosec G304,G703 -- user-controlled vault path
			return issues, errors.Wrapf(ctx, err, "write file %s", filePath)
		}
	}

	return issues, nil
}

// fixInvalidPriority converts string priority values to integers.
func (l *lintOperation) fixInvalidPriority(content string) (string, bool) {
	priorityMap := map[string]int{
		"high":   1,
		"must":   1,
		"medium": 2,
		"should": 2,
		"low":    3,
	}

	// Match priority field with string value
	matches := priorityRegex.FindStringSubmatch(content)
	if len(matches) >= 2 {
		oldValue := matches[1]
		if newValue, ok := priorityMap[oldValue]; ok {
			newContent := priorityRegex.ReplaceAllString(
				content,
				fmt.Sprintf("priority: %d", newValue),
			)
			return newContent, true
		}
	}

	return content, false
}

// fixInvalidStatus migrates old status values to new ones.
func (l *lintOperation) fixInvalidStatus(content string) (string, bool) {
	// Match status field with any value
	matches := statusRegex.FindStringSubmatch(content)
	if len(matches) >= 2 {
		oldValue := matches[1]
		newValue, ok := domain.NormalizeTaskStatus(oldValue)

		// Only fix if normalization gives a different valid value
		if ok && newValue != domain.TaskStatus(oldValue) {
			newContent := statusRegex.ReplaceAllString(
				content,
				fmt.Sprintf("status: %s", newValue),
			)
			return newContent, true
		}
	}

	return content, false
}

// fixStatusCheckboxMismatch sets status to completed when all checkboxes are checked.
func (l *lintOperation) fixStatusCheckboxMismatch(content string) (string, bool) {
	// Extract frontmatter
	matches := frontmatterRegex.FindSubmatch([]byte(content))
	if len(matches) < 2 {
		return content, false
	}

	frontmatterYAML := string(matches[1])

	// Skip if recurring
	if strings.Contains(frontmatterYAML, "recurring:") {
		return content, false
	}

	// Extract status
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return content, false
	}
	status := statusMatches[1]

	if status == "completed" {
		return content, false // Already completed
	}

	// Check if all checkboxes are checked
	checkboxMatches := checkboxRegex.FindAllStringSubmatch(content, -1)

	if len(checkboxMatches) == 0 {
		return content, false // No checkboxes
	}

	allChecked := true
	for _, match := range checkboxMatches {
		if len(match) >= 2 && match[1] != "x" && match[1] != "X" {
			allChecked = false
			break
		}
	}

	if !allChecked {
		return content, false
	}

	// All checkboxes are checked, set status to completed
	newContent := statusRegex.ReplaceAllString(
		content,
		"status: completed",
	)
	return newContent, true
}

// fixStatusDateMismatch promotes status from next/backlog to in_progress
// when a date field is set. Per spec 017: calendar-as-commitment rule auto-fixes
// the status, never strips the date. Idempotent on in_progress (no rewrite).
func (l *lintOperation) fixStatusDateMismatch(content string) (string, bool) {
	matches := statusRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return content, false
	}
	current := matches[1]
	if current != "next" && current != "backlog" {
		return content, false
	}
	newContent := statusRegex.ReplaceAllString(content, "status: in_progress")
	return newContent, true
}

// fixMissingFrontmatter prepends minimal frontmatter to files without frontmatter.
func (l *lintOperation) fixMissingFrontmatter(content string) (string, bool) {
	minimalFrontmatter := "---\nstatus: backlog\n---\n"
	newContent := minimalFrontmatter + content
	return newContent, true
}

// fixDuplicateKeys removes duplicate YAML keys, keeping the first occurrence.
func (l *lintOperation) fixDuplicateKeys(content string) (string, bool) {
	// Extract frontmatter
	matches := fixDuplicateKeysRegex.FindStringSubmatch(content)
	if len(matches) < 5 {
		return content, false
	}

	frontmatterStart := matches[1]
	frontmatterYAML := matches[2]
	frontmatterEnd := matches[3]
	body := matches[4]

	// Parse frontmatter line by line, keeping only first occurrence of each key
	lines := strings.Split(frontmatterYAML, "\n")
	keysSeen := make(map[string]bool)
	newLines := make([]string, 0, len(lines))
	modified := false

	for _, line := range lines {
		matches := keyRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			key := matches[1]
			if keysSeen[key] {
				// Skip duplicate key
				modified = true
				continue
			}
			keysSeen[key] = true
		}
		newLines = append(newLines, line)
	}

	if !modified {
		return content, false
	}

	// Reconstruct content
	newFrontmatter := strings.Join(newLines, "\n")
	newContent := frontmatterStart + newFrontmatter + frontmatterEnd + body

	// Validate that the new YAML is still valid
	var testMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(newFrontmatter), &testMap); err != nil {
		return content, false // Don't apply fix if it creates invalid YAML
	}

	return newContent, true
}
