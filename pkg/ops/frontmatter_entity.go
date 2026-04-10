// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// FrontmatterEntity is implemented by all refactored entity types (Goal, Theme, Objective, Vision, Task).
type FrontmatterEntity interface {
	GetField(key string) string
	SetField(ctx context.Context, key, value string) error
	ClearField(key string)
	Keys() []string
}

// EntityGetOperation retrieves a single frontmatter field value from an entity.
//
//counterfeiter:generate -o ../../mocks/entity-get-operation.go --fake-name EntityGetOperation . EntityGetOperation
type EntityGetOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, key string) (string, error)
}

type entityGetOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error)
	entityType string
}

// Execute retrieves the value of a frontmatter field from the named entity.
func (o *entityGetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) (string, error) {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return "", errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
	}
	return entity.GetField(key), nil
}

// NewGoalGetOperation creates an EntityGetOperation for goals.
func NewGoalGetOperation(goalStorage storage.GoalStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		entityType: "goal",
	}
}

// NewThemeGetOperation creates an EntityGetOperation for themes.
func NewThemeGetOperation(themeStorage storage.ThemeStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		entityType: "theme",
	}
}

// NewObjectiveGetOperation creates an EntityGetOperation for objectives.
func NewObjectiveGetOperation(objectiveStorage storage.ObjectiveStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		entityType: "objective",
	}
}

// NewVisionGetOperation creates an EntityGetOperation for visions.
func NewVisionGetOperation(visionStorage storage.VisionStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (FrontmatterEntity, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		entityType: "vision",
	}
}

// EntitySetOperation sets a single frontmatter field value on an entity.
//
//counterfeiter:generate -o ../../mocks/entity-set-operation.go --fake-name EntitySetOperation . EntitySetOperation
type EntitySetOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, key, value string) error
}

type goalSetOperation struct {
	goalStorage storage.GoalStorage
}

// Execute sets the value of a frontmatter field on the named goal.
func (o *goalSetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find goal")
	}
	if err := goal.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.goalStorage.WriteGoal(ctx, goal); err != nil {
		return errors.Wrap(ctx, err, "write goal")
	}
	return nil
}

// NewGoalSetOperation creates an EntitySetOperation for goals.
func NewGoalSetOperation(goalStorage storage.GoalStorage) EntitySetOperation {
	return &goalSetOperation{goalStorage: goalStorage}
}

type themeSetOperation struct {
	themeStorage storage.ThemeStorage
}

// Execute sets the value of a frontmatter field on the named theme.
func (o *themeSetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	theme, err := o.themeStorage.FindThemeByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find theme")
	}
	if err := theme.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.themeStorage.WriteTheme(ctx, theme); err != nil {
		return errors.Wrap(ctx, err, "write theme")
	}
	return nil
}

// NewThemeSetOperation creates an EntitySetOperation for themes.
func NewThemeSetOperation(themeStorage storage.ThemeStorage) EntitySetOperation {
	return &themeSetOperation{themeStorage: themeStorage}
}

type objectiveSetOperation struct {
	objectiveStorage storage.ObjectiveStorage
}

// Execute sets the value of a frontmatter field on the named objective.
func (o *objectiveSetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	objective, err := o.objectiveStorage.FindObjectiveByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find objective")
	}
	if err := objective.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.objectiveStorage.WriteObjective(ctx, objective); err != nil {
		return errors.Wrap(ctx, err, "write objective")
	}
	return nil
}

// NewObjectiveSetOperation creates an EntitySetOperation for objectives.
func NewObjectiveSetOperation(objectiveStorage storage.ObjectiveStorage) EntitySetOperation {
	return &objectiveSetOperation{objectiveStorage: objectiveStorage}
}

type visionSetOperation struct {
	visionStorage storage.VisionStorage
}

// Execute sets the value of a frontmatter field on the named vision.
func (o *visionSetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	vision, err := o.visionStorage.FindVisionByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find vision")
	}
	if err := vision.SetField(ctx, key, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.visionStorage.WriteVision(ctx, vision); err != nil {
		return errors.Wrap(ctx, err, "write vision")
	}
	return nil
}

// NewVisionSetOperation creates an EntitySetOperation for visions.
func NewVisionSetOperation(visionStorage storage.VisionStorage) EntitySetOperation {
	return &visionSetOperation{visionStorage: visionStorage}
}

// EntityClearOperation clears a single frontmatter field value on an entity.
//
//counterfeiter:generate -o ../../mocks/entity-clear-operation.go --fake-name EntityClearOperation . EntityClearOperation
type EntityClearOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, key string) error
}

type goalClearOperation struct {
	goalStorage storage.GoalStorage
}

// Execute clears the value of a frontmatter field on the named goal.
func (o *goalClearOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) error {
	goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find goal")
	}
	goal.ClearField(key)
	if err := o.goalStorage.WriteGoal(ctx, goal); err != nil {
		return errors.Wrap(ctx, err, "write goal")
	}
	return nil
}

// NewGoalClearOperation creates an EntityClearOperation for goals.
func NewGoalClearOperation(goalStorage storage.GoalStorage) EntityClearOperation {
	return &goalClearOperation{goalStorage: goalStorage}
}

type themeClearOperation struct {
	themeStorage storage.ThemeStorage
}

// Execute clears the value of a frontmatter field on the named theme.
func (o *themeClearOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) error {
	theme, err := o.themeStorage.FindThemeByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find theme")
	}
	theme.ClearField(key)
	if err := o.themeStorage.WriteTheme(ctx, theme); err != nil {
		return errors.Wrap(ctx, err, "write theme")
	}
	return nil
}

// NewThemeClearOperation creates an EntityClearOperation for themes.
func NewThemeClearOperation(themeStorage storage.ThemeStorage) EntityClearOperation {
	return &themeClearOperation{themeStorage: themeStorage}
}

type objectiveClearOperation struct {
	objectiveStorage storage.ObjectiveStorage
}

// Execute clears the value of a frontmatter field on the named objective.
func (o *objectiveClearOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) error {
	objective, err := o.objectiveStorage.FindObjectiveByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find objective")
	}
	objective.ClearField(key)
	if err := o.objectiveStorage.WriteObjective(ctx, objective); err != nil {
		return errors.Wrap(ctx, err, "write objective")
	}
	return nil
}

// NewObjectiveClearOperation creates an EntityClearOperation for objectives.
func NewObjectiveClearOperation(objectiveStorage storage.ObjectiveStorage) EntityClearOperation {
	return &objectiveClearOperation{objectiveStorage: objectiveStorage}
}

type visionClearOperation struct {
	visionStorage storage.VisionStorage
}

// Execute clears the value of a frontmatter field on the named vision.
func (o *visionClearOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) error {
	vision, err := o.visionStorage.FindVisionByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find vision")
	}
	vision.ClearField(key)
	if err := o.visionStorage.WriteVision(ctx, vision); err != nil {
		return errors.Wrap(ctx, err, "write vision")
	}
	return nil
}

// NewVisionClearOperation creates an EntityClearOperation for visions.
func NewVisionClearOperation(visionStorage storage.VisionStorage) EntityClearOperation {
	return &visionClearOperation{visionStorage: visionStorage}
}

// EntityListAddOperation appends a value to a list frontmatter field on an entity.
//
//counterfeiter:generate -o ../../mocks/entity-list-add-operation.go --fake-name EntityListAddOperation . EntityListAddOperation
type EntityListAddOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, field, value string) error
}

// EntityListRemoveOperation removes a value from a list frontmatter field on an entity.
//
//counterfeiter:generate -o ../../mocks/entity-list-remove-operation.go --fake-name EntityListRemoveOperation . EntityListRemoveOperation
type EntityListRemoveOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, field, value string) error
}

// knownGoalScalarFields are goal fields that hold a scalar (not a list).
var knownGoalScalarFields = map[string]bool{
	"status": true, "page_type": true, "theme": true, "priority": true,
	"assignee": true, "start_date": true, "target_date": true,
	"completed": true, "defer_date": true,
}

type goalTagsListOperation struct {
	goalStorage storage.GoalStorage
	mode        string // "add" or "remove"
}

func (o *goalTagsListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	goal, err := o.goalStorage.FindGoalByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find goal")
	}
	if knownGoalScalarFields[key] {
		return fmt.Errorf("not a list field: %q", key)
	}
	if key != "tags" {
		return fmt.Errorf("unknown field: %q", key)
	}
	current := goal.Tags()
	updated, err := applyListMutation(current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}
	goal.SetTags(updated)
	if err := o.goalStorage.WriteGoal(ctx, goal); err != nil {
		return errors.Wrap(ctx, err, "write goal")
	}
	return nil
}

// NewGoalListAddOperation creates an EntityListAddOperation for goals.
func NewGoalListAddOperation(goalStorage storage.GoalStorage) EntityListAddOperation {
	return &goalTagsListOperation{goalStorage: goalStorage, mode: "add"}
}

// NewGoalListRemoveOperation creates an EntityListRemoveOperation for goals.
func NewGoalListRemoveOperation(goalStorage storage.GoalStorage) EntityListRemoveOperation {
	return &goalTagsListOperation{goalStorage: goalStorage, mode: "remove"}
}

// knownThemeScalarFields are theme fields that hold a scalar (not a list).
var knownThemeScalarFields = map[string]bool{
	"status": true, "page_type": true, "priority": true,
	"assignee": true, "start_date": true, "target_date": true,
}

type themeTagsListOperation struct {
	themeStorage storage.ThemeStorage
	mode         string
}

func (o *themeTagsListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	theme, err := o.themeStorage.FindThemeByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find theme")
	}
	if knownThemeScalarFields[key] {
		return fmt.Errorf("not a list field: %q", key)
	}
	if key != "tags" {
		return fmt.Errorf("unknown field: %q", key)
	}
	current := theme.Tags()
	updated, err := applyListMutation(current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}
	theme.SetTags(updated)
	if err := o.themeStorage.WriteTheme(ctx, theme); err != nil {
		return errors.Wrap(ctx, err, "write theme")
	}
	return nil
}

// NewThemeListAddOperation creates an EntityListAddOperation for themes.
func NewThemeListAddOperation(themeStorage storage.ThemeStorage) EntityListAddOperation {
	return &themeTagsListOperation{themeStorage: themeStorage, mode: "add"}
}

// NewThemeListRemoveOperation creates an EntityListRemoveOperation for themes.
func NewThemeListRemoveOperation(themeStorage storage.ThemeStorage) EntityListRemoveOperation {
	return &themeTagsListOperation{themeStorage: themeStorage, mode: "remove"}
}

// knownObjectiveScalarFields are objective fields that hold a scalar (not a list).
var knownObjectiveScalarFields = map[string]bool{
	"status": true, "page_type": true, "priority": true,
	"assignee": true, "start_date": true, "target_date": true, "completed": true,
}

type objectiveTagsListOperation struct {
	objectiveStorage storage.ObjectiveStorage
	mode             string
}

func (o *objectiveTagsListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	objective, err := o.objectiveStorage.FindObjectiveByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find objective")
	}
	if knownObjectiveScalarFields[key] {
		return fmt.Errorf("not a list field: %q", key)
	}
	if key != "tags" {
		return fmt.Errorf("unknown field: %q", key)
	}
	current := objective.Tags()
	updated, err := applyListMutation(current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}
	objective.SetTags(updated)
	if err := o.objectiveStorage.WriteObjective(ctx, objective); err != nil {
		return errors.Wrap(ctx, err, "write objective")
	}
	return nil
}

// NewObjectiveListAddOperation creates an EntityListAddOperation for objectives.
func NewObjectiveListAddOperation(
	objectiveStorage storage.ObjectiveStorage,
) EntityListAddOperation {
	return &objectiveTagsListOperation{objectiveStorage: objectiveStorage, mode: "add"}
}

// NewObjectiveListRemoveOperation creates an EntityListRemoveOperation for objectives.
func NewObjectiveListRemoveOperation(
	objectiveStorage storage.ObjectiveStorage,
) EntityListRemoveOperation {
	return &objectiveTagsListOperation{objectiveStorage: objectiveStorage, mode: "remove"}
}

// knownVisionScalarFields are vision fields that hold a scalar (not a list).
var knownVisionScalarFields = map[string]bool{
	"status": true, "page_type": true, "priority": true, "assignee": true,
}

type visionTagsListOperation struct {
	visionStorage storage.VisionStorage
	mode          string
}

func (o *visionTagsListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	vision, err := o.visionStorage.FindVisionByName(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, "find vision")
	}
	if knownVisionScalarFields[key] {
		return fmt.Errorf("not a list field: %q", key)
	}
	if key != "tags" {
		return fmt.Errorf("unknown field: %q", key)
	}
	current := vision.Tags()
	updated, err := applyListMutation(current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}
	vision.SetTags(updated)
	if err := o.visionStorage.WriteVision(ctx, vision); err != nil {
		return errors.Wrap(ctx, err, "write vision")
	}
	return nil
}

// NewVisionListAddOperation creates an EntityListAddOperation for visions.
func NewVisionListAddOperation(visionStorage storage.VisionStorage) EntityListAddOperation {
	return &visionTagsListOperation{visionStorage: visionStorage, mode: "add"}
}

// NewVisionListRemoveOperation creates an EntityListRemoveOperation for visions.
func NewVisionListRemoveOperation(visionStorage storage.VisionStorage) EntityListRemoveOperation {
	return &visionTagsListOperation{visionStorage: visionStorage, mode: "remove"}
}

// NewTaskListAddOperation creates an EntityListAddOperation for tasks.
func NewTaskListAddOperation(taskStorage storage.TaskStorage) EntityListAddOperation {
	return &taskListOperation{
		taskStorage: taskStorage,
		mode:        "add",
	}
}

// NewTaskListRemoveOperation creates an EntityListRemoveOperation for tasks.
func NewTaskListRemoveOperation(taskStorage storage.TaskStorage) EntityListRemoveOperation {
	return &taskListOperation{
		taskStorage: taskStorage,
		mode:        "remove",
	}
}

// taskListOperation implements list add/remove for Task entities using typed accessors.
// It avoids the reflection-based fieldByYAMLTag approach since Task no longer has YAML tags.
type taskListOperation struct {
	taskStorage storage.TaskStorage
	mode        string // "add" or "remove"
}

// knownTaskListFields are task fields that hold a list.
var knownTaskListFields = map[string]bool{
	"goals": true,
	"tags":  true,
}

// knownTaskScalarFields are task fields that hold a scalar (not a list).
var knownTaskScalarFields = map[string]bool{
	"status": true, "page_type": true, "priority": true, "assignee": true,
	"defer_date": true, "planned_date": true, "due_date": true,
	"phase": true, "claude_session_id": true, "recurring": true,
	"last_completed": true, "completed_date": true, "task_identifier": true,
}

// Execute applies the list operation (add or remove) to the named field on the task.
func (o *taskListOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key, value string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	if knownTaskScalarFields[key] {
		return fmt.Errorf("not a list field: %q", key)
	}
	if !knownTaskListFields[key] {
		return fmt.Errorf("unknown field: %q", key)
	}

	var current []string
	switch key {
	case "goals":
		current = task.Goals()
	case "tags":
		current = task.Tags()
	}

	updated, err := applyListMutation(current, value, o.mode)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.mode, key))
	}

	switch key {
	case "goals":
		task.SetGoals(updated)
	case "tags":
		task.SetTags(updated)
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}
	return nil
}

// applyListMutation adds or removes a value from a string slice.
// For mode "add": appends value; returns error if already present.
// For mode "remove": filters out value; returns error if not found.
func applyListMutation(current []string, value, mode string) ([]string, error) {
	switch mode {
	case "add":
		for _, v := range current {
			if v == value {
				return nil, fmt.Errorf("value %q already exists in list", value)
			}
		}
		return append(current, value), nil
	case "remove":
		result := make([]string, 0, len(current))
		found := false
		for _, v := range current {
			if v == value {
				found = true
				continue
			}
			result = append(result, v)
		}
		if !found {
			return nil, fmt.Errorf("value %q not found in list", value)
		}
		return result, nil
	default:
		return current, nil
	}
}

// EntityShowResult is the structured result from EntityShowOperation.
type EntityShowResult struct {
	Name       string            `json:"name"`
	FilePath   string            `json:"file_path"`
	Vault      string            `json:"vault"`
	Fields     map[string]string `json:"fields"`
	FieldOrder []string          `json:"field_order,omitempty"`
	Content    string            `json:"content"`
}

// EntityShowOperation returns full detail for a single entity.
//
//counterfeiter:generate -o ../../mocks/entity-show-operation.go --fake-name EntityShowOperation . EntityShowOperation
type EntityShowOperation interface {
	Execute(ctx context.Context, vaultPath, vaultName, entityName string) (EntityShowResult, error)
}

type entityShowOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
	entityType string
}

// Execute finds an entity by name and returns its full detail.
func (o *entityShowOperation) Execute(
	ctx context.Context,
	vaultPath, vaultName, entityName string,
) (EntityShowResult, error) {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return EntityShowResult{}, errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
	}

	fields := make(map[string]string)
	var fieldOrder []string
	var nameVal, filePathVal, contentVal string

	switch e := entity.(type) {
	case *domain.Goal:
		nameVal = e.Name
		filePathVal = e.FilePath
		contentVal = string(e.Content)
		for _, k := range e.Keys() {
			fields[k] = e.GetField(k)
			fieldOrder = append(fieldOrder, k)
		}
	case *domain.Theme:
		nameVal = e.Name
		filePathVal = e.FilePath
		contentVal = string(e.Content)
		for _, k := range e.Keys() {
			fields[k] = e.GetField(k)
			fieldOrder = append(fieldOrder, k)
		}
	case *domain.Objective:
		nameVal = e.Name
		filePathVal = e.FilePath
		contentVal = string(e.Content)
		for _, k := range e.Keys() {
			fields[k] = e.GetField(k)
			fieldOrder = append(fieldOrder, k)
		}
	case *domain.Vision:
		nameVal = e.Name
		filePathVal = e.FilePath
		contentVal = string(e.Content)
		for _, k := range e.Keys() {
			fields[k] = e.GetField(k)
			fieldOrder = append(fieldOrder, k)
		}
	default:
		return EntityShowResult{}, errors.Errorf(ctx, "unsupported entity type %T", entity)
	}

	return EntityShowResult{
		Name:       nameVal,
		FilePath:   filePathVal,
		Vault:      vaultName,
		Fields:     fields,
		FieldOrder: fieldOrder,
		Content:    contentVal,
	}, nil
}

// NewGoalShowOperation creates an EntityShowOperation for goals.
func NewGoalShowOperation(goalStorage storage.GoalStorage) EntityShowOperation {
	return &entityShowOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		entityType: "goal",
	}
}

// NewThemeShowOperation creates an EntityShowOperation for themes.
func NewThemeShowOperation(themeStorage storage.ThemeStorage) EntityShowOperation {
	return &entityShowOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		entityType: "theme",
	}
}

// NewObjectiveShowOperation creates an EntityShowOperation for objectives.
func NewObjectiveShowOperation(objectiveStorage storage.ObjectiveStorage) EntityShowOperation {
	return &entityShowOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		entityType: "objective",
	}
}

// NewVisionShowOperation creates an EntityShowOperation for visions.
func NewVisionShowOperation(visionStorage storage.VisionStorage) EntityShowOperation {
	return &entityShowOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		entityType: "vision",
	}
}
