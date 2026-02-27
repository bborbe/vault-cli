// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
)

// Config holds the configuration for storage paths.
type Config struct {
	TasksDir string
	GoalsDir string
	DailyDir string
}

// NewConfigFromVault creates a Config from a Vault.
func NewConfigFromVault(vault *config.Vault) *Config {
	return &Config{
		TasksDir: vault.GetTasksDir(),
		GoalsDir: vault.GetGoalsDir(),
		DailyDir: vault.GetDailyDir(),
	}
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() *Config {
	return &Config{
		TasksDir: "Tasks",
		GoalsDir: "Goals",
		DailyDir: "Daily Notes",
	}
}

//counterfeiter:generate -o ../../mocks/storage.go --fake-name Storage . Storage
type Storage interface {
	// Task operations
	ReadTask(ctx context.Context, vaultPath string, taskID domain.TaskID) (*domain.Task, error)
	WriteTask(ctx context.Context, task *domain.Task) error
	FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)

	// Goal operations
	ReadGoal(ctx context.Context, vaultPath string, goalID domain.GoalID) (*domain.Goal, error)
	WriteGoal(ctx context.Context, goal *domain.Goal) error
	FindGoalByName(ctx context.Context, vaultPath string, name string) (*domain.Goal, error)

	// Theme operations
	ReadTheme(ctx context.Context, vaultPath string, themeID domain.ThemeID) (*domain.Theme, error)
	WriteTheme(ctx context.Context, theme *domain.Theme) error

	// Daily note operations
	ReadDailyNote(ctx context.Context, vaultPath string, date string) (string, error)
	WriteDailyNote(ctx context.Context, vaultPath string, date string, content string) error
}

// NewStorage creates a new markdown storage instance with custom configuration.
func NewStorage(storageConfig *Config) Storage {
	if storageConfig == nil {
		storageConfig = DefaultConfig()
	}
	return &markdownStorage{
		config: storageConfig,
	}
}

type markdownStorage struct {
	config *Config
}

var (
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n(.*)$`)
	checkboxRegex    = regexp.MustCompile(`^(\s*)- \[([ x])\] (.+)$`)
)

// ReadTask reads a task from a markdown file.
func (m *markdownStorage) ReadTask(
	ctx context.Context,
	vaultPath string,
	taskID domain.TaskID,
) (*domain.Task, error) {
	filePath := filepath.Join(vaultPath, m.config.TasksDir, taskID.String()+".md")
	return m.readTaskFromPath(ctx, filePath, taskID.String())
}

func (m *markdownStorage) readTaskFromPath(
	_ context.Context,
	filePath string,
	name string,
) (*domain.Task, error) {
	content, err := os.ReadFile(filePath) //nolint:gosec // User-controlled vault path
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	task := &domain.Task{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, task); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return task, nil
}

// WriteTask writes a task to a markdown file.
func (m *markdownStorage) WriteTask(ctx context.Context, task *domain.Task) error {
	content, err := m.serializeWithFrontmatter(task, task.Content)
	if err != nil {
		return fmt.Errorf("serialize frontmatter: %w", err)
	}

	if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write file %s: %w", task.FilePath, err)
	}

	return nil
}

// FindTaskByName searches for a task by name in the vault.
func (m *markdownStorage) FindTaskByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Task, error) {
	tasksDir := filepath.Join(vaultPath, m.config.TasksDir)
	matchedPath, matchedName, err := m.findFileByName(tasksDir, name)
	if err != nil {
		return nil, err
	}
	return m.readTaskFromPath(ctx, matchedPath, matchedName)
}

// ReadGoal reads a goal from a markdown file.
func (m *markdownStorage) ReadGoal(
	ctx context.Context,
	vaultPath string,
	goalID domain.GoalID,
) (*domain.Goal, error) {
	filePath := filepath.Join(vaultPath, m.config.GoalsDir, goalID.String()+".md")
	return m.readGoalFromPath(ctx, filePath, goalID.String())
}

func (m *markdownStorage) readGoalFromPath(
	_ context.Context,
	filePath string,
	name string,
) (*domain.Goal, error) {
	content, err := os.ReadFile(filePath) //nolint:gosec // User-controlled vault path
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	goal := &domain.Goal{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, goal); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	// Parse checkbox items from content
	goal.Tasks = m.parseCheckboxes(string(content))

	return goal, nil
}

// WriteGoal writes a goal to a markdown file.
func (m *markdownStorage) WriteGoal(ctx context.Context, goal *domain.Goal) error {
	content, err := m.serializeWithFrontmatter(goal, goal.Content)
	if err != nil {
		return fmt.Errorf("serialize frontmatter: %w", err)
	}

	if err := os.WriteFile(goal.FilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write file %s: %w", goal.FilePath, err)
	}

	return nil
}

// FindGoalByName searches for a goal by name in the vault.
func (m *markdownStorage) FindGoalByName(
	ctx context.Context,
	vaultPath string,
	name string,
) (*domain.Goal, error) {
	goalsDir := filepath.Join(vaultPath, m.config.GoalsDir)
	matchedPath, matchedName, err := m.findFileByName(goalsDir, name)
	if err != nil {
		return nil, err
	}
	return m.readGoalFromPath(ctx, matchedPath, matchedName)
}

// ReadTheme reads a theme from a markdown file.
func (m *markdownStorage) ReadTheme(
	ctx context.Context,
	vaultPath string,
	themeID domain.ThemeID,
) (*domain.Theme, error) {
	filePath := filepath.Join(vaultPath, "Themes", themeID.String()+".md")
	return m.readThemeFromPath(ctx, filePath, themeID.String())
}

func (m *markdownStorage) readThemeFromPath(
	_ context.Context,
	filePath string,
	name string,
) (*domain.Theme, error) {
	content, err := os.ReadFile(filePath) //nolint:gosec // User-controlled vault path
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	theme := &domain.Theme{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, theme); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	return theme, nil
}

// WriteTheme writes a theme to a markdown file.
func (m *markdownStorage) WriteTheme(ctx context.Context, theme *domain.Theme) error {
	content, err := m.serializeWithFrontmatter(theme, theme.Content)
	if err != nil {
		return fmt.Errorf("serialize frontmatter: %w", err)
	}

	if err := os.WriteFile(theme.FilePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write file %s: %w", theme.FilePath, err)
	}

	return nil
}

// ReadDailyNote reads a daily note from the vault.
func (m *markdownStorage) ReadDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
) (string, error) {
	filePath := filepath.Join(vaultPath, m.config.DailyDir, date+".md")
	content, err := os.ReadFile(filePath) //nolint:gosec // User-controlled vault path
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Return empty content if file doesn't exist
		}
		return "", fmt.Errorf("read daily note %s: %w", filePath, err)
	}
	return string(content), nil
}

// WriteDailyNote writes a daily note to the vault.
func (m *markdownStorage) WriteDailyNote(
	ctx context.Context,
	vaultPath string,
	date string,
	content string,
) error {
	dailyNotesDir := filepath.Join(vaultPath, m.config.DailyDir)
	if err := os.MkdirAll(dailyNotesDir, 0750); err != nil {
		return fmt.Errorf("create daily notes directory: %w", err)
	}

	filePath := filepath.Join(dailyNotesDir, date+".md")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write daily note %s: %w", filePath, err)
	}

	return nil
}

// parseFrontmatter extracts and parses YAML frontmatter from markdown content.
func (m *markdownStorage) parseFrontmatter(content []byte, target interface{}) error {
	matches := frontmatterRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return fmt.Errorf("no frontmatter found")
	}

	frontmatter := matches[1]
	if err := yaml.Unmarshal(frontmatter, target); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	return nil
}

// serializeWithFrontmatter serializes an object to YAML frontmatter and combines with body content.
func (m *markdownStorage) serializeWithFrontmatter(
	frontmatter interface{},
	originalContent string,
) (string, error) {
	// Extract body content (everything after frontmatter)
	matches := frontmatterRegex.FindStringSubmatch(originalContent)
	var bodyContent string
	if len(matches) >= 3 {
		bodyContent = matches[2]
	}

	// Serialize frontmatter to YAML
	yamlBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}

	// Combine frontmatter and body
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(bodyContent)

	return buf.String(), nil
}

// parseCheckboxes extracts checkbox items from markdown content.
func (m *markdownStorage) parseCheckboxes(content string) []domain.CheckboxItem {
	var items []domain.CheckboxItem
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if matches := checkboxRegex.FindStringSubmatch(line); len(matches) == 4 {
			items = append(items, domain.CheckboxItem{
				Line:    i,
				Checked: matches[2] == "x",
				Text:    matches[3],
				RawLine: line,
			})
		}
	}

	return items
}

// findFileByName searches for a file by exact or partial name match in a directory.
// Returns the matched file path and name (without .md extension).
func (m *markdownStorage) findFileByName(dir string, name string) (string, string, error) {
	// Try exact match first
	exactPath := filepath.Join(dir, name+".md")
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath, name, nil
	}

	// Search for partial match
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		if strings.Contains(strings.ToLower(fileName), strings.ToLower(name)) {
			filePath := filepath.Join(dir, entry.Name())
			return filePath, fileName, nil
		}
	}

	return "", "", fmt.Errorf("file not found: %s", name)
}
