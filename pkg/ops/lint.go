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

	"gopkg.in/yaml.v3"
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
	IssueTypeMissingFrontmatter IssueType = "MISSING_FRONTMATTER"
	IssueTypeInvalidPriority    IssueType = "INVALID_PRIORITY"
	IssueTypeDuplicateKey       IssueType = "DUPLICATE_KEY"
	IssueTypeInvalidStatus      IssueType = "INVALID_STATUS"
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

		fileIssues, err := l.lintFile(path, fix)
		if err != nil {
			return fmt.Errorf("lint file %s: %w", path, err)
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk tasks directory: %w", err)
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
	issues, err := l.lintFile(filePath, false)
	if err != nil {
		return fmt.Errorf("lint file %s: %w", filePath, err)
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
func (l *lintOperation) lintFile(filePath string, fix bool) ([]LintIssue, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	issues := make([]LintIssue, 0, 4)

	// Check for frontmatter existence
	frontmatterRegex := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		issue, updatedContent, shouldReturn := l.handleMissingFrontmatter(
			filePath,
			content,
			fix,
		)
		issues = append(issues, issue)

		if shouldReturn {
			return issues, nil
		}

		// Update content and re-parse for further checks
		content = updatedContent
		matches = frontmatterRegex.FindSubmatch(content)
	}

	frontmatterYAML := string(matches[1])

	// Check for duplicate keys by parsing lines manually
	duplicateIssues := l.detectDuplicateKeys(frontmatterYAML)
	for _, key := range duplicateIssues {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeDuplicateKey,
			Description: fmt.Sprintf("key %q defined multiple times", key),
			Fixable:     true,
			Fixed:       false,
		})
	}

	// Check for invalid priority (string instead of int)
	priorityIssue, invalidPriorityValue := l.detectInvalidPriority(frontmatterYAML)
	if priorityIssue {
		issues = append(issues, LintIssue{
			FilePath:    filePath,
			IssueType:   IssueTypeInvalidPriority,
			Description: fmt.Sprintf("priority is %q, expected int", invalidPriorityValue),
			Fixable:     true,
			Fixed:       false,
		})
	}

	// Check for invalid status
	statusIssue, invalidStatusValue, statusIsFixable := l.detectInvalidStatus(frontmatterYAML)
	if statusIssue {
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

	// Fix issues if requested
	if fix && len(issues) > 0 {
		issues, err = l.fixIssues(filePath, string(content), issues)
		if err != nil {
			return nil, fmt.Errorf("fix issues: %w", err)
		}
	}

	return issues, nil
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
		validStatuses := []string{
			"todo",
			"in_progress",
			"backlog",
			"completed",
			"hold",
			"aborted",
		}
		for _, valid := range validStatuses {
			if statusValue == valid {
				return false, "", false
			}
		}

		// Check if it's a fixable migration status
		statusMigrationMap := map[string]string{
			"next":    "todo",
			"current": "in_progress",
			"done":    "completed",
		}
		_, isFixable := statusMigrationMap[statusValue]
		return true, statusValue, isFixable
	}
	return false, "", false
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
	statusMigrationMap := map[string]string{
		"next":    "todo",
		"current": "in_progress",
		"done":    "completed",
	}

	// Match status field with invalid value (next, current, or done)
	statusRegex := regexp.MustCompile(`(?m)^status:\s*['"]?(next|current|done)['"]?\s*$`)
	matches := statusRegex.FindStringSubmatch(content)
	if len(matches) >= 2 {
		oldValue := matches[1]
		if newValue, ok := statusMigrationMap[oldValue]; ok {
			newContent := statusRegex.ReplaceAllString(
				content,
				fmt.Sprintf("status: %s", newValue),
			)
			return newContent, true
		}
	}

	return content, false
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
