// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"encoding/json"
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

// EntityShowOperation returns full detail for a single entity.
//
//counterfeiter:generate -o ../../mocks/entity-show-operation.go --fake-name EntityShowOperation . EntityShowOperation
type EntityShowOperation interface {
	Execute(ctx context.Context, vaultPath, vaultName, entityName, outputFormat string) error
}

type entityShowOperation struct {
	findFn     func(ctx context.Context, vaultPath, name string) (any, error)
	entityType string
}

// Execute finds an entity by name and outputs its full detail.
func (o *entityShowOperation) Execute(
	ctx context.Context,
	vaultPath, vaultName, entityName, outputFormat string,
) error {
	entity, err := o.findFn(ctx, vaultPath, entityName)
	if err != nil {
		return errors.Wrap(ctx, err, fmt.Sprintf("find %s", o.entityType))
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

	if outputFormat == "json" {
		result := map[string]any{
			"name":      nameVal,
			"file_path": filePathVal,
			"vault":     vaultName,
			"fields":    fields,
			"content":   contentVal,
		}
		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return errors.Wrap(ctx, marshalErr, "marshal json")
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("%s: %s\n", o.entityType, nameVal)
	for _, name := range fieldOrder {
		if fields[name] != "" {
			fmt.Printf("%s: %s\n", name, fields[name])
		}
	}
	return nil
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
