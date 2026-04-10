// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/bborbe/errors"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

// EntityGetOperation retrieves a single frontmatter field value from an entity.
//
//counterfeiter:generate -o ../../mocks/entity-get-operation.go --fake-name EntityGetOperation . EntityGetOperation
type EntityGetOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, key string) (string, error)
}

type entityGetOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
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
	field, fieldVal, found := fieldByYAMLTag(entity, key)
	if !found {
		return "", fmt.Errorf("unknown field %q for %s", key, o.entityType)
	}
	if isReadOnlyTag(field) {
		return "", fmt.Errorf("field %q is read-only", key)
	}
	return getFieldAsString(fieldVal)
}

// NewGoalGetOperation creates an EntityGetOperation for goals.
func NewGoalGetOperation(goalStorage storage.GoalStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		entityType: "goal",
	}
}

// NewThemeGetOperation creates an EntityGetOperation for themes.
func NewThemeGetOperation(themeStorage storage.ThemeStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		entityType: "theme",
	}
}

// NewObjectiveGetOperation creates an EntityGetOperation for objectives.
func NewObjectiveGetOperation(objectiveStorage storage.ObjectiveStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		entityType: "objective",
	}
}

// NewVisionGetOperation creates an EntityGetOperation for visions.
func NewVisionGetOperation(visionStorage storage.VisionStorage) EntityGetOperation {
	return &entityGetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
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

type entitySetOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
	writeFn    func(ctx context.Context, entity any) error
	entityType string
}

// Execute sets the value of a frontmatter field on the named entity.
func (o *entitySetOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key, value string,
) error {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
	}
	field, fieldVal, found := fieldByYAMLTag(entity, key)
	if !found {
		return fmt.Errorf("unknown field %q for %s", key, o.entityType)
	}
	if isReadOnlyTag(field) {
		return fmt.Errorf("field %q is read-only", key)
	}
	if err := setFieldFromString(ctx, fieldVal, field.Type, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("set field %q", key))
	}
	if err := o.writeFn(ctx, entity); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write %s", o.entityType))
	}
	return nil
}

// NewGoalSetOperation creates an EntitySetOperation for goals.
func NewGoalSetOperation(goalStorage storage.GoalStorage) EntitySetOperation {
	return &entitySetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			goal, ok := entity.(*domain.Goal)
			if !ok {
				return fmt.Errorf("unexpected entity type for goal")
			}
			return goalStorage.WriteGoal(ctx, goal)
		},
		entityType: "goal",
	}
}

// NewThemeSetOperation creates an EntitySetOperation for themes.
func NewThemeSetOperation(themeStorage storage.ThemeStorage) EntitySetOperation {
	return &entitySetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			theme, ok := entity.(*domain.Theme)
			if !ok {
				return fmt.Errorf("unexpected entity type for theme")
			}
			return themeStorage.WriteTheme(ctx, theme)
		},
		entityType: "theme",
	}
}

// NewObjectiveSetOperation creates an EntitySetOperation for objectives.
func NewObjectiveSetOperation(objectiveStorage storage.ObjectiveStorage) EntitySetOperation {
	return &entitySetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			objective, ok := entity.(*domain.Objective)
			if !ok {
				return fmt.Errorf("unexpected entity type for objective")
			}
			return objectiveStorage.WriteObjective(ctx, objective)
		},
		entityType: "objective",
	}
}

// NewVisionSetOperation creates an EntitySetOperation for visions.
func NewVisionSetOperation(visionStorage storage.VisionStorage) EntitySetOperation {
	return &entitySetOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			vision, ok := entity.(*domain.Vision)
			if !ok {
				return fmt.Errorf("unexpected entity type for vision")
			}
			return visionStorage.WriteVision(ctx, vision)
		},
		entityType: "vision",
	}
}

// EntityClearOperation clears a single frontmatter field value on an entity.
//
//counterfeiter:generate -o ../../mocks/entity-clear-operation.go --fake-name EntityClearOperation . EntityClearOperation
type EntityClearOperation interface {
	Execute(ctx context.Context, vaultPath, entityName, key string) error
}

type entityClearOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
	writeFn    func(ctx context.Context, entity any) error
	entityType string
}

// Execute clears the value of a frontmatter field on the named entity.
func (o *entityClearOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, key string,
) error {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
	}
	field, fieldVal, found := fieldByYAMLTag(entity, key)
	if !found {
		return fmt.Errorf("unknown field %q for %s", key, o.entityType)
	}
	if isReadOnlyTag(field) {
		return fmt.Errorf("field %q is read-only", key)
	}
	clearField(fieldVal, field.Type)
	if err := o.writeFn(ctx, entity); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write %s", o.entityType))
	}
	return nil
}

// NewGoalClearOperation creates an EntityClearOperation for goals.
func NewGoalClearOperation(goalStorage storage.GoalStorage) EntityClearOperation {
	return &entityClearOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			goal, ok := entity.(*domain.Goal)
			if !ok {
				return fmt.Errorf("unexpected entity type for goal")
			}
			return goalStorage.WriteGoal(ctx, goal)
		},
		entityType: "goal",
	}
}

// NewThemeClearOperation creates an EntityClearOperation for themes.
func NewThemeClearOperation(themeStorage storage.ThemeStorage) EntityClearOperation {
	return &entityClearOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			theme, ok := entity.(*domain.Theme)
			if !ok {
				return fmt.Errorf("unexpected entity type for theme")
			}
			return themeStorage.WriteTheme(ctx, theme)
		},
		entityType: "theme",
	}
}

// NewObjectiveClearOperation creates an EntityClearOperation for objectives.
func NewObjectiveClearOperation(objectiveStorage storage.ObjectiveStorage) EntityClearOperation {
	return &entityClearOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			objective, ok := entity.(*domain.Objective)
			if !ok {
				return fmt.Errorf("unexpected entity type for objective")
			}
			return objectiveStorage.WriteObjective(ctx, objective)
		},
		entityType: "objective",
	}
}

// NewVisionClearOperation creates an EntityClearOperation for visions.
func NewVisionClearOperation(visionStorage storage.VisionStorage) EntityClearOperation {
	return &entityClearOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			vision, ok := entity.(*domain.Vision)
			if !ok {
				return fmt.Errorf("unexpected entity type for vision")
			}
			return visionStorage.WriteVision(ctx, vision)
		},
		entityType: "vision",
	}
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

// entityListOperation is a shared implementation for both add and remove list operations.
type entityListOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
	writeFn    func(ctx context.Context, entity any) error
	listFn     func(fieldVal reflect.Value, value string) error
	opLabel    string
	entityType string
}

// Execute applies the list operation (add or remove) to the named field on the entity.
func (o *entityListOperation) Execute(
	ctx context.Context,
	vaultPath, entityName, field, value string,
) error {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
	}
	sf, fieldVal, found := fieldByYAMLTag(entity, field)
	if !found {
		return fmt.Errorf("unknown field %q for %s", field, o.entityType)
	}
	if isReadOnlyTag(sf) {
		return fmt.Errorf("field %q is read-only", field)
	}
	if !isListField(fieldVal) {
		return fmt.Errorf("field %q is not a list field", field)
	}
	if err := o.listFn(fieldVal, value); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("%s field %q", o.opLabel, field))
	}
	if err := o.writeFn(ctx, entity); err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("write %s", o.entityType))
	}
	return nil
}

// NewGoalListAddOperation creates an EntityListAddOperation for goals.
func NewGoalListAddOperation(goalStorage storage.GoalStorage) EntityListAddOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			goal, ok := entity.(*domain.Goal)
			if !ok {
				return fmt.Errorf("unexpected entity type for goal")
			}
			return goalStorage.WriteGoal(ctx, goal)
		},
		listFn:     appendToList,
		opLabel:    "append to",
		entityType: "goal",
	}
}

// NewThemeListAddOperation creates an EntityListAddOperation for themes.
func NewThemeListAddOperation(themeStorage storage.ThemeStorage) EntityListAddOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			theme, ok := entity.(*domain.Theme)
			if !ok {
				return fmt.Errorf("unexpected entity type for theme")
			}
			return themeStorage.WriteTheme(ctx, theme)
		},
		listFn:     appendToList,
		opLabel:    "append to",
		entityType: "theme",
	}
}

// NewObjectiveListAddOperation creates an EntityListAddOperation for objectives.
func NewObjectiveListAddOperation(
	objectiveStorage storage.ObjectiveStorage,
) EntityListAddOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			objective, ok := entity.(*domain.Objective)
			if !ok {
				return fmt.Errorf("unexpected entity type for objective")
			}
			return objectiveStorage.WriteObjective(ctx, objective)
		},
		listFn:     appendToList,
		opLabel:    "append to",
		entityType: "objective",
	}
}

// NewVisionListAddOperation creates an EntityListAddOperation for visions.
func NewVisionListAddOperation(visionStorage storage.VisionStorage) EntityListAddOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			vision, ok := entity.(*domain.Vision)
			if !ok {
				return fmt.Errorf("unexpected entity type for vision")
			}
			return visionStorage.WriteVision(ctx, vision)
		},
		listFn:     appendToList,
		opLabel:    "append to",
		entityType: "vision",
	}
}

// NewTaskListAddOperation creates an EntityListAddOperation for tasks.
func NewTaskListAddOperation(taskStorage storage.TaskStorage) EntityListAddOperation {
	return &taskListOperation{
		taskStorage: taskStorage,
		mode:        "add",
	}
}

// NewGoalListRemoveOperation creates an EntityListRemoveOperation for goals.
func NewGoalListRemoveOperation(goalStorage storage.GoalStorage) EntityListRemoveOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return goalStorage.FindGoalByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			goal, ok := entity.(*domain.Goal)
			if !ok {
				return fmt.Errorf("unexpected entity type for goal")
			}
			return goalStorage.WriteGoal(ctx, goal)
		},
		listFn:     removeFromList,
		opLabel:    "remove from",
		entityType: "goal",
	}
}

// NewThemeListRemoveOperation creates an EntityListRemoveOperation for themes.
func NewThemeListRemoveOperation(themeStorage storage.ThemeStorage) EntityListRemoveOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return themeStorage.FindThemeByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			theme, ok := entity.(*domain.Theme)
			if !ok {
				return fmt.Errorf("unexpected entity type for theme")
			}
			return themeStorage.WriteTheme(ctx, theme)
		},
		listFn:     removeFromList,
		opLabel:    "remove from",
		entityType: "theme",
	}
}

// NewObjectiveListRemoveOperation creates an EntityListRemoveOperation for objectives.
func NewObjectiveListRemoveOperation(
	objectiveStorage storage.ObjectiveStorage,
) EntityListRemoveOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return objectiveStorage.FindObjectiveByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			objective, ok := entity.(*domain.Objective)
			if !ok {
				return fmt.Errorf("unexpected entity type for objective")
			}
			return objectiveStorage.WriteObjective(ctx, objective)
		},
		listFn:     removeFromList,
		opLabel:    "remove from",
		entityType: "objective",
	}
}

// NewVisionListRemoveOperation creates an EntityListRemoveOperation for visions.
func NewVisionListRemoveOperation(visionStorage storage.VisionStorage) EntityListRemoveOperation {
	return &entityListOperation{
		findFn: func(ctx context.Context, vaultPath, name string) (any, error) {
			return visionStorage.FindVisionByName(ctx, vaultPath, name)
		},
		writeFn: func(ctx context.Context, entity any) error {
			vision, ok := entity.(*domain.Vision)
			if !ok {
				return fmt.Errorf("unexpected entity type for vision")
			}
			return visionStorage.WriteVision(ctx, vision)
		},
		listFn:     removeFromList,
		opLabel:    "remove from",
		entityType: "vision",
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

	v := reflect.ValueOf(entity).Elem()
	t := v.Type()
	fields := make(map[string]string)
	var fieldOrder []string
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		yamlTag := sf.Tag.Get("yaml")
		name := strings.Split(yamlTag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fieldStr, fieldErr := getFieldAsString(v.Field(i))
		if fieldErr != nil {
			continue
		}
		fields[name] = fieldStr
		fieldOrder = append(fieldOrder, name)
	}

	nameVal := v.FieldByName("Name").String()
	filePathVal := v.FieldByName("FilePath").String()
	contentVal := v.FieldByName("Content").String()

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
