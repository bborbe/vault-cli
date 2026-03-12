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

	"github.com/bborbe/errors"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
)

// Config holds the configuration for storage paths.
type Config struct {
	TasksDir      string
	GoalsDir      string
	ThemesDir     string
	ObjectivesDir string
	VisionDir     string
	DailyDir      string
}

// NewConfigFromVault creates a Config from a Vault.
func NewConfigFromVault(vault *config.Vault) *Config {
	return &Config{
		TasksDir:      vault.GetTasksDir(),
		GoalsDir:      vault.GetGoalsDir(),
		ThemesDir:     vault.GetThemesDir(),
		ObjectivesDir: vault.GetObjectivesDir(),
		VisionDir:     vault.GetVisionDir(),
		DailyDir:      vault.GetDailyDir(),
	}
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() *Config {
	return &Config{
		TasksDir:      "Tasks",
		GoalsDir:      "Goals",
		ThemesDir:     "21 Themes",
		ObjectivesDir: "22 Objectives",
		VisionDir:     "20 Vision",
		DailyDir:      "Daily Notes",
	}
}

//counterfeiter:generate -o ../../mocks/storage.go --fake-name Storage . Storage
type Storage interface {
	// Task operations
	ReadTask(ctx context.Context, vaultPath string, taskID domain.TaskID) (*domain.Task, error)
	WriteTask(ctx context.Context, task *domain.Task) error
	FindTaskByName(ctx context.Context, vaultPath string, name string) (*domain.Task, error)
	ListTasks(ctx context.Context, vaultPath string) ([]*domain.Task, error)

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

	// Generic page operations
	ListPages(ctx context.Context, vaultPath string, pagesDir string) ([]*domain.Task, error)
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
	checkboxRegex    = regexp.MustCompile(`^(\s*)- \[([ x/])\] (.+)$`)
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
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Task, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	task := &domain.Task{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, task); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return task, nil
}

// WriteTask writes a task to a markdown file.
func (m *markdownStorage) WriteTask(ctx context.Context, task *domain.Task) error {
	content, err := m.serializeWithFrontmatter(task, task.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(task.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", task.FilePath))
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

// ListTasks returns all tasks from the vault.
func (m *markdownStorage) ListTasks(
	ctx context.Context,
	vaultPath string,
) ([]*domain.Task, error) {
	tasksDir := filepath.Join(vaultPath, m.config.TasksDir)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read tasks directory %s", tasksDir))
	}

	tasks := make([]*domain.Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(tasksDir, entry.Name())

		task, err := m.readTaskFromPath(ctx, filePath, fileName)
		if err != nil {
			// Log error but continue with other tasks
			fmt.Fprintf(os.Stderr, "Warning: failed to read task %s: %v\n", fileName, err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ListPages returns all pages from a specific directory in the vault.
func (m *markdownStorage) ListPages(
	ctx context.Context,
	vaultPath string,
	pagesDir string,
) ([]*domain.Task, error) {
	targetDir := filepath.Join(vaultPath, pagesDir)

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read directory %s", targetDir))
	}

	tasks := make([]*domain.Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileName := strings.TrimSuffix(entry.Name(), ".md")
		filePath := filepath.Join(targetDir, entry.Name())

		task, err := m.readTaskFromPath(ctx, filePath, fileName)
		if err != nil {
			// Log error but continue with other tasks
			fmt.Fprintf(os.Stderr, "Warning: failed to read page %s: %v\n", fileName, err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
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
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Goal, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	goal := &domain.Goal{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, goal); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	// Parse checkbox items from content
	goal.Tasks = m.parseCheckboxes(string(content))

	return goal, nil
}

// WriteGoal writes a goal to a markdown file.
func (m *markdownStorage) WriteGoal(ctx context.Context, goal *domain.Goal) error {
	content, err := m.serializeWithFrontmatter(goal, goal.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(goal.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", goal.FilePath))
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
	ctx context.Context,
	filePath string,
	name string,
) (*domain.Theme, error) {
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		return nil, errors.Wrap(ctx, err, fmt.Sprintf("read file %s", filePath))
	}

	theme := &domain.Theme{
		Name:     name,
		Content:  string(content),
		FilePath: filePath,
	}

	if err := m.parseFrontmatter(content, theme); err != nil {
		return nil, errors.Wrap(ctx, err, "parse frontmatter")
	}

	return theme, nil
}

// WriteTheme writes a theme to a markdown file.
func (m *markdownStorage) WriteTheme(ctx context.Context, theme *domain.Theme) error {
	content, err := m.serializeWithFrontmatter(theme, theme.Content)
	if err != nil {
		return errors.Wrap(ctx, err, "serialize frontmatter")
	}

	if err := os.WriteFile(theme.FilePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write file %s", theme.FilePath))
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
	content, err := os.ReadFile(filePath) //#nosec G304 -- user-controlled vault path
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Return empty content if file doesn't exist
		}
		return "", errors.Wrap(ctx, err, fmt.Sprintf("read daily note %s", filePath))
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
		return errors.Wrap(ctx, err, "create daily notes directory")
	}

	filePath := filepath.Join(dailyNotesDir, date+".md")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write daily note %s", filePath))
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
			state := matches[2]
			items = append(items, domain.CheckboxItem{
				Line:       i,
				Checked:    state == "x",
				InProgress: state == "/",
				Text:       matches[3],
				RawLine:    line,
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
