// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

//counterfeiter:generate -o ../../mocks/lint-operation.go --fake-name LintOperation . LintOperation
type LintOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		tasksDir string,
		fix bool,
		outputFormat string,
	) error
	ExecuteFile(
		ctx context.Context,
		filePath string,
		taskName string,
		vaultName string,
		outputFormat string,
	) error
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
	fix bool,
	outputFormat string,
) error {
	tasksDirPath := filepath.Join(vaultPath, tasksDir)

	// Walk through all .md files in the tasks directory
	var issues []LintIssue
	err := filepath.Walk(tasksDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileIssues, err := l.lintFile(vaultPath, path, fix)
		if err != nil {
			return errors.Wrap(ctx, err, fmt.Sprintf("lint file %s", path))
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	if err != nil {
		return errors.Wrap(ctx, err, "walk tasks directory")
	}

	return l.outputIssues(vaultPath, issues, fix, outputFormat)
}

// ExecuteFile lints a single file and returns error if issues found.
func (l *lintOperation) ExecuteFile(
	ctx context.Context,
	filePath string,
	taskName string,
	vaultName string,
	outputFormat string,
) error {
	// Lint the single file (read-only, no fix)
	// Pass empty vaultPath since we don't have vault context for single file validation
	issues, err := l.lintFile("", filePath, false)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("lint file %s", filePath))
	}

	// Output results
	if outputFormat == "json" {
		return l.outputValidateJSON(taskName, vaultName, issues)
	}
	return l.outputValidatePlain(taskName, issues)
}

// outputValidateJSON outputs validation results in JSON format.
func (l *lintOperation) outputValidateJSON(
	taskName string,
	vaultName string,
	issues []LintIssue,
) error {
	type ValidateIssue struct {
		Type        string `json:"type"`
		IssueType   string `json:"issue_type"`
		Description string `json:"description"`
	}

	result := map[string]interface{}{
		"name":  taskName,
		"vault": vaultName,
	}

	jsonIssues := make([]ValidateIssue, len(issues))
	for i, issue := range issues {
		issueTypeStr := "WARN"
		if !issue.Fixable {
			issueTypeStr = "ERROR"
		}
		jsonIssues[i] = ValidateIssue{
			Type:        issueTypeStr,
			IssueType:   string(issue.IssueType),
			Description: issue.Description,
		}
	}
	result["issues"] = jsonIssues

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	if len(issues) > 0 {
		os.Exit(1)
	}
	return nil
}

// outputValidatePlain outputs validation results in plain text format.
func (l *lintOperation) outputValidatePlain(taskName string, issues []LintIssue) error {
	if len(issues) == 0 {
		fmt.Printf("✅ %s: no lint issues found\n", taskName)
		return nil
	}

	for _, issue := range issues {
		issueTypeStr := "WARN"
		if !issue.Fixable {
			issueTypeStr = "ERROR"
		}
		fmt.Printf(
			"%-5s %s: %s %s\n",
			issueTypeStr,
			taskName+".md",
			string(issue.IssueType),
			issue.Description,
		)
	}

	os.Exit(1)
	return nil
}

// outputIssues prints lint issues in the requested format and returns an error if any unfixed issues exist.
func (l *lintOperation) outputIssues(
	vaultPath string,
	issues []LintIssue,
	fix bool,
	outputFormat string,
) error {
	if outputFormat == "json" {
		return l.outputIssuesJSON(vaultPath, issues, fix)
	}
	return l.outputIssuesPlain(vaultPath, issues, fix)
}

func (l *lintOperation) outputIssuesJSON(vaultPath string, issues []LintIssue, fix bool) error {
	jsonIssues := make([]LintIssueJSON, len(issues))
	for i, issue := range issues {
		relPath, _ := filepath.Rel(vaultPath, issue.FilePath)
		jsonIssues[i] = LintIssueJSON{
			File:        relPath,
			Type:        l.issueTypeName(issue, fix),
			Description: issue.Description,
			Fixed:       issue.Fixed,
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(jsonIssues); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return l.countUnfixedError(issues)
}

func (l *lintOperation) outputIssuesPlain(vaultPath string, issues []LintIssue, fix bool) error {
	for _, issue := range issues {
		relPath, _ := filepath.Rel(vaultPath, issue.FilePath)
		fmt.Printf(
			"%-5s %s: %s %s\n",
			l.issueTypeName(issue, fix),
			relPath,
			issue.IssueType,
			issue.Description,
		)
	}
	if len(issues) == 0 {
		fmt.Println("No lint issues found")
	}
	return l.countUnfixedError(issues)
}

func (l *lintOperation) issueTypeName(issue LintIssue, fix bool) string {
	if issue.Fixed {
		return "FIXED"
	}
	if issue.Fixable && !fix {
		return "WARN"
	}
	return "ERROR"
}

func (l *lintOperation) countUnfixedError(issues []LintIssue) error {
	unfixed := 0
	for _, issue := range issues {
		if !issue.Fixed {
			unfixed++
		}
	}
	if unfixed > 0 {
		return fmt.Errorf("found %d lint issue(s)", unfixed)
	}
	return nil
}

// lintFile checks a single file for lint issues and optionally fixes them.
func (l *lintOperation) lintFile(vaultPath string, filePath string, fix bool) ([]LintIssue, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Handle missing frontmatter first
	frontmatterRegex := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return l.handleMissingFrontmatterCase(filePath, content, fix)
	}

	// Collect all lint issues from the frontmatter and content
	issues := l.collectLintIssues(vaultPath, filePath, string(matches[1]), content)

	// Fix issues if requested
	if fix && len(issues) > 0 {
		issues, err = l.fixIssues(filePath, string(content), issues)
		if err != nil {
			return nil, fmt.Errorf("fix issues: %w", err)
		}
	}

	return issues, nil
}

// handleMissingFrontmatterCase handles files without frontmatter
func (l *lintOperation) handleMissingFrontmatterCase(
	filePath string,
	content []byte,
	fix bool,
) ([]LintIssue, error) {
	issue, updatedContent, shouldReturn := l.handleMissingFrontmatter(filePath, content, fix)
	issues := []LintIssue{issue}

	if shouldReturn {
		return issues, nil
	}

	// After fixing frontmatter, re-parse and continue with other checks
	frontmatterRegex := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	matches := frontmatterRegex.FindSubmatch(updatedContent)
	if len(matches) < 2 {
		return issues, nil
	}

	// Collect additional issues from the now-valid frontmatter
	additionalIssues := l.collectLintIssues("", filePath, string(matches[1]), updatedContent)
	return append(issues, additionalIssues...), nil
}

// collectLintIssues runs all lint checks and returns found issues
func (l *lintOperation) collectLintIssues(
	vaultPath string,
	filePath string,
	frontmatterYAML string,
	content []byte,
) []LintIssue {
	issues := make([]LintIssue, 0, 4)

	// Check for duplicate keys
	for _, key := range l.detectDuplicateKeys(frontmatterYAML) {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeDuplicateKey,
			Description: fmt.Sprintf("key %q defined multiple times", key),
			Fixable:     true,
			Fixed:       false,
		})
	}

	// Check for invalid priority
	if priorityIssue, invalidPriorityValue := l.detectInvalidPriority(frontmatterYAML); priorityIssue {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeInvalidPriority,
			Description: fmt.Sprintf("priority is %q, expected int", invalidPriorityValue),
			Fixable:     true,
			Fixed:       false,
		})
	}

	// Check for invalid status
	if statusIssue, invalidStatusValue, statusIsFixable := l.detectInvalidStatus(
		frontmatterYAML,
	); statusIssue {
		issues = append(issues, LintIssue{
			FilePath:  filePath,
			IssueType: IssueTypeInvalidStatus,
			Description: fmt.Sprintf(
				"status is %q, expected one of: todo, in_progress, backlog, completed, hold, aborted",
				invalidStatusValue,
			),
			Fixable: statusIsFixable,
			Fixed:   false,
		})
	}

	// Check for status/phase mismatch
	if mismatchIssue, mismatchDesc := l.detectStatusPhaseMismatch(frontmatterYAML); mismatchIssue {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeStatusPhaseMismatch,
			Description: mismatchDesc,
			Fixable:     false,
			Fixed:       false,
		})
	}

	// Check for orphan goals
	for _, goalName := range l.detectOrphanGoals(vaultPath, frontmatterYAML) {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeOrphanGoal,
			Description: fmt.Sprintf("goal not found: %s", goalName),
			Fixable:     false,
			Fixed:       false,
		})
	}

	// Check for status/checkbox mismatch
	if mismatchIssue, mismatchDesc, mismatchFixable := l.detectStatusCheckboxMismatch(
		frontmatterYAML,
		string(content),
	); mismatchIssue {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeStatusCheckboxMismatch,
			Description: mismatchDesc,
			Fixable:     mismatchFixable,
			Fixed:       false,
		})
	}

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

	keyRegex := regexp.MustCompile(`^([a-z_]+):\s*`)
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
	priorityRegex := regexp.MustCompile(`(?m)^priority:\s*['"]?([a-z]+)['"]?\s*$`)
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
// Returns: (issueFound, invalidValue, isFixable)
func (l *lintOperation) detectInvalidStatus(frontmatterYAML string) (bool, string, bool) {
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
	matches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		statusValue := matches[1]

		// Use domain package to check validity
		if domain.IsValidTaskStatus(domain.TaskStatus(statusValue)) {
			return false, "", false
		}

		// Check if it's fixable by seeing if normalization gives a different valid value
		normalizedStatus, ok := domain.NormalizeTaskStatus(statusValue)
		isFixable := ok && normalizedStatus != domain.TaskStatus(statusValue)
		return true, statusValue, isFixable
	}
	return false, "", false
}

// detectOrphanGoals detects goals that reference non-existent goal files.
// Returns list of missing goal names.
func (l *lintOperation) detectOrphanGoals(vaultPath string, frontmatterYAML string) []string {
	if vaultPath == "" {
		return nil // Skip if no vault path (single file validation)
	}

	// Extract goals field (YAML list) - try inline format first
	goalsRegex := regexp.MustCompile(`(?m)^goals:\s*\[(.*?)\]`)
	matches := goalsRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		return l.parseInlineGoalsList(vaultPath, matches[1])
	}

	// Try multi-line YAML list format
	goalsRegex = regexp.MustCompile(`(?ms)^goals:\s*\n((?:\s*-\s*.+\n?)+)`)
	matches = goalsRegex.FindStringSubmatch(frontmatterYAML)
	if len(matches) >= 2 {
		return l.parseMultilineGoalsList(vaultPath, matches[1])
	}

	return nil
}

// parseInlineGoalsList parses inline goals list format: [goal1, goal2]
func (l *lintOperation) parseInlineGoalsList(vaultPath string, goalsList string) []string {
	var orphanGoals []string
	for _, item := range strings.Split(goalsList, ",") {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, `"'`)
		goalName := l.extractGoalName(item)
		if goalName == "" {
			continue
		}
		if !l.goalFileExists(vaultPath, goalName) {
			orphanGoals = append(orphanGoals, goalName)
		}
	}
	return orphanGoals
}

// parseMultilineGoalsList parses multi-line goals list format
func (l *lintOperation) parseMultilineGoalsList(vaultPath string, yamlList string) []string {
	var orphanGoals []string
	itemRegex := regexp.MustCompile(`(?m)^\s*-\s*['"]?(.+?)['"]?\s*$`)
	for _, line := range strings.Split(yamlList, "\n") {
		itemMatches := itemRegex.FindStringSubmatch(line)
		if len(itemMatches) < 2 {
			continue
		}
		goalName := l.extractGoalName(itemMatches[1])
		if goalName == "" {
			continue
		}
		if !l.goalFileExists(vaultPath, goalName) {
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
func (l *lintOperation) goalFileExists(vaultPath string, goalName string) bool {
	goalsDir := filepath.Join(vaultPath, "Goals")
	goalPath := filepath.Join(goalsDir, goalName+".md")
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
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return false, "", false
	}
	status := statusMatches[1]

	// Find all checkboxes in content
	checkboxRegex := regexp.MustCompile(`(?m)^[\s]*[-*]\s+\[([ xX])\]`)
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
	phaseRegex := regexp.MustCompile(`(?m)^phase:\s*['"]?([a-z_]+)['"]?\s*$`)
	phaseMatches := phaseRegex.FindStringSubmatch(frontmatterYAML)
	if len(phaseMatches) < 2 {
		return false, "" // No phase key — no validation
	}
	phase := domain.TaskPhase(phaseMatches[1])

	// Parse status
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
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

// fixIssues fixes fixable issues in the file.
func (l *lintOperation) fixIssues(
	filePath string,
	content string,
	issues []LintIssue,
) ([]LintIssue, error) {
	modified := false
	updatedContent := content

	for i := range issues {
		if !issues[i].Fixable {
			continue
		}

		switch issues[i].IssueType {
		case IssueTypeInvalidPriority:
			// Fix invalid priority by converting string to int
			newContent, fixed := l.fixInvalidPriority(updatedContent)
			if fixed {
				updatedContent = newContent
				issues[i].Fixed = true
				modified = true
			}

		case IssueTypeDuplicateKey:
			// Fix duplicate keys by removing duplicates (keep first occurrence)
			newContent, fixed := l.fixDuplicateKeys(updatedContent)
			if fixed {
				updatedContent = newContent
				issues[i].Fixed = true
				modified = true
			}

		case IssueTypeInvalidStatus:
			// Fix invalid status by migrating to new value
			newContent, fixed := l.fixInvalidStatus(updatedContent)
			if fixed {
				updatedContent = newContent
				issues[i].Fixed = true
				modified = true
			}

		case IssueTypeStatusCheckboxMismatch:
			// Fix status/checkbox mismatch by setting status to completed
			newContent, fixed := l.fixStatusCheckboxMismatch(updatedContent)
			if fixed {
				updatedContent = newContent
				issues[i].Fixed = true
				modified = true
			}
		}
	}

	// Write fixed content back to file
	if modified {
		if err := os.WriteFile(filePath, []byte(updatedContent), 0600); err != nil { //#nosec G304,G703 -- user-controlled vault path
			return issues, fmt.Errorf("write file: %w", err)
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
	priorityRegex := regexp.MustCompile(`(?m)^priority:\s*['"]?([a-z]+)['"]?\s*$`)
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
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
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
	frontmatterRegex := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
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
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?([a-z_]+)['"]?\s*$`)
	statusMatches := statusRegex.FindStringSubmatch(frontmatterYAML)
	if len(statusMatches) < 2 {
		return content, false
	}
	status := statusMatches[1]

	if status == "completed" {
		return content, false // Already completed
	}

	// Check if all checkboxes are checked
	checkboxRegex := regexp.MustCompile(`(?m)^[\s]*[-*]\s+\[([ xX])\]`)
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

// fixMissingFrontmatter prepends minimal frontmatter to files without frontmatter.
func (l *lintOperation) fixMissingFrontmatter(content string) (string, bool) {
	minimalFrontmatter := "---\nstatus: backlog\n---\n"
	newContent := minimalFrontmatter + content
	return newContent, true
}

// fixDuplicateKeys removes duplicate YAML keys, keeping the first occurrence.
func (l *lintOperation) fixDuplicateKeys(content string) (string, bool) {
	// Extract frontmatter
	frontmatterRegex := regexp.MustCompile(`(?s)^(---\n)(.*?)(\n---\n)(.*)$`)
	matches := frontmatterRegex.FindStringSubmatch(content)
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

	keyRegex := regexp.MustCompile(`^([a-z_]+):\s*`)
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
