// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/bborbe/errors"
)

// fieldByYAMLTag finds a struct field by its yaml tag name.
// Returns the field, its value, and whether it was found.
func fieldByYAMLTag(entityPtr any, tagName string) (reflect.StructField, reflect.Value, bool) {
	v := reflect.ValueOf(entityPtr).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		// Strip options like ",omitempty"
		name := strings.Split(yamlTag, ",")[0]
		if name == tagName {
			return field, v.Field(i), true
		}
	}
	return reflect.StructField{}, reflect.Value{}, false
}

// getFieldAsString reads a struct field value as a string.
// Handles: string, string-alias, int-alias (Priority), *time.Time, []string.
func getFieldAsString(fieldVal reflect.Value) (string, error) {
	if !fieldVal.IsValid() {
		return "", nil
	}
	switch fieldVal.Kind() {
	case reflect.String:
		return fieldVal.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(fieldVal.Int(), 10), nil
	case reflect.Slice:
		if fieldVal.IsNil() {
			return "", nil
		}
		strs := make([]string, fieldVal.Len())
		for i := 0; i < fieldVal.Len(); i++ {
			strs[i] = fieldVal.Index(i).String()
		}
		return strings.Join(strs, ","), nil
	case reflect.Ptr:
		if fieldVal.IsNil() {
			return "", nil
		}
		// Handle *time.Time
		if t, ok := fieldVal.Interface().(*time.Time); ok {
			return t.Format("2006-01-02"), nil
		}
		return "", fmt.Errorf("unsupported pointer type: %s", fieldVal.Type())
	default:
		return "", fmt.Errorf("unsupported field type: %s", fieldVal.Kind())
	}
}

// setFieldFromString sets a struct field from a string value.
// Type coercion is based on the field's reflect.Kind and type.
func setFieldFromString(
	ctx context.Context,
	fieldVal reflect.Value,
	fieldType reflect.Type,
	value string,
) error {
	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return errors.Wrap(ctx, err, "invalid integer value")
		}
		fieldVal.SetInt(n)
		return nil
	case reflect.Slice:
		if value == "" {
			fieldVal.Set(reflect.Zero(fieldType))
			return nil
		}
		parts := strings.Split(value, ",")
		slice := reflect.MakeSlice(fieldType, len(parts), len(parts))
		for i, p := range parts {
			slice.Index(i).SetString(p)
		}
		fieldVal.Set(slice)
		return nil
	case reflect.Ptr:
		if value == "" {
			fieldVal.Set(reflect.Zero(fieldType))
			return nil
		}
		// Handle *time.Time
		if fieldType == reflect.TypeOf((*time.Time)(nil)) {
			t, err := time.Parse("2006-01-02", value)
			if err != nil {
				return errors.Wrap(ctx, err, "invalid date format (expected YYYY-MM-DD)")
			}
			fieldVal.Set(reflect.ValueOf(&t))
			return nil
		}
		return fmt.Errorf("unsupported pointer type: %s", fieldType)
	default:
		return fmt.Errorf("unsupported field type: %s", fieldVal.Kind())
	}
}

// clearField zeros a struct field.
func clearField(fieldVal reflect.Value, fieldType reflect.Type) {
	fieldVal.Set(reflect.Zero(fieldType))
}

// isListField returns true if the struct field is a slice type.
func isListField(fieldVal reflect.Value) bool {
	return fieldVal.Kind() == reflect.Slice
}

// appendToList appends value to a []string slice field.
// Returns an error if the value already exists in the list.
func appendToList(fieldVal reflect.Value, value string) error {
	if fieldVal.Kind() != reflect.Slice {
		return fmt.Errorf("field is not a list field")
	}
	for i := 0; i < fieldVal.Len(); i++ {
		if fieldVal.Index(i).String() == value {
			return fmt.Errorf("value %q already exists in list", value)
		}
	}
	newSlice := reflect.Append(fieldVal, reflect.ValueOf(value))
	fieldVal.Set(newSlice)
	return nil
}

// removeFromList removes value from a []string slice field.
// Returns an error if the value is not found in the list.
func removeFromList(fieldVal reflect.Value, value string) error {
	if fieldVal.Kind() != reflect.Slice {
		return fmt.Errorf("field is not a list field")
	}
	for i := 0; i < fieldVal.Len(); i++ {
		if fieldVal.Index(i).String() == value {
			// Remove element at index i by appending the two slices around it
			newSlice := reflect.AppendSlice(
				fieldVal.Slice(0, i),
				fieldVal.Slice(i+1, fieldVal.Len()),
			)
			fieldVal.Set(newSlice)
			return nil
		}
	}
	return fmt.Errorf("value %q not found in list", value)
}

// isReadOnlyTag returns true if the yaml tag marks the field as metadata (yaml:"-").
func isReadOnlyTag(field reflect.StructField) bool {
	return field.Tag.Get("yaml") == "-"
}
