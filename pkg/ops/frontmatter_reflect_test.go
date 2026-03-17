// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type reflectTestEntity struct {
	Status   string     `yaml:"status"`
	Priority int        `yaml:"priority,omitempty"`
	Tags     []string   `yaml:"tags,omitempty"`
	DueDate  *time.Time `yaml:"due_date,omitempty"`
	Name     string     `yaml:"-"`
}

var _ = Describe("fieldByYAMLTag", func() {
	var entity *reflectTestEntity

	BeforeEach(func() {
		entity = &reflectTestEntity{
			Status:   "active",
			Priority: 2,
			Tags:     []string{"a", "b"},
			Name:     "my-name",
		}
	})

	Context("known field", func() {
		It("finds the status field", func() {
			field, val, found := fieldByYAMLTag(entity, "status")
			Expect(found).To(BeTrue())
			Expect(field.Name).To(Equal("Status"))
			Expect(val.String()).To(Equal("active"))
		})

		It("finds a field with omitempty option", func() {
			_, val, found := fieldByYAMLTag(entity, "priority")
			Expect(found).To(BeTrue())
			Expect(val.Int()).To(Equal(int64(2)))
		})
	})

	Context("unknown field", func() {
		It("returns not-found", func() {
			_, _, found := fieldByYAMLTag(entity, "xyz")
			Expect(found).To(BeFalse())
		})
	})

	Context("metadata field with yaml:\"-\"", func() {
		It("finds field by literal tag value \"-\"", func() {
			field, _, found := fieldByYAMLTag(entity, "-")
			Expect(found).To(BeTrue())
			// The first field with yaml:"-" is Name
			Expect(field.Name).To(Equal("Name"))
		})
	})
})

var _ = Describe("getFieldAsString", func() {
	Context("string field", func() {
		It("returns the string value", func() {
			entity := &reflectTestEntity{Status: "active"}
			_, val, _ := fieldByYAMLTag(entity, "status")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("active"))
		})
	})

	Context("int field", func() {
		It("returns the int as string", func() {
			entity := &reflectTestEntity{Priority: 3}
			_, val, _ := fieldByYAMLTag(entity, "priority")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("3"))
		})
	})

	Context("slice field", func() {
		It("returns comma-joined string", func() {
			entity := &reflectTestEntity{Tags: []string{"x", "y"}}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("x,y"))
		})

		It("returns empty string for nil slice", func() {
			entity := &reflectTestEntity{Tags: nil}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})

	Context("pointer field", func() {
		It("returns empty string for nil *time.Time", func() {
			entity := &reflectTestEntity{DueDate: nil}
			_, val, _ := fieldByYAMLTag(entity, "due_date")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})

		It("returns formatted date for non-nil *time.Time", func() {
			t := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
			entity := &reflectTestEntity{DueDate: &t}
			_, val, _ := fieldByYAMLTag(entity, "due_date")
			result, err := getFieldAsString(val)
			Expect(err).To(BeNil())
			Expect(result).To(Equal("2025-06-15"))
		})
	})

	Context("invalid reflect.Value", func() {
		It("returns empty string", func() {
			result, err := getFieldAsString(reflect.Value{})
			Expect(err).To(BeNil())
			Expect(result).To(Equal(""))
		})
	})
})

var _ = Describe("setFieldFromString", func() {
	var (
		ctx    context.Context
		entity *reflectTestEntity
	)

	BeforeEach(func() {
		ctx = context.Background()
		entity = &reflectTestEntity{}
	})

	Context("string field", func() {
		It("sets the value", func() {
			field, val, _ := fieldByYAMLTag(entity, "status")
			err := setFieldFromString(ctx, val, field.Type, "done")
			Expect(err).To(BeNil())
			Expect(entity.Status).To(Equal("done"))
		})
	})

	Context("int field", func() {
		It("sets the value", func() {
			field, val, _ := fieldByYAMLTag(entity, "priority")
			err := setFieldFromString(ctx, val, field.Type, "5")
			Expect(err).To(BeNil())
			Expect(entity.Priority).To(Equal(5))
		})

		It("returns error for non-integer string", func() {
			field, val, _ := fieldByYAMLTag(entity, "priority")
			err := setFieldFromString(ctx, val, field.Type, "notanint")
			Expect(err).To(MatchError(ContainSubstring("invalid integer value")))
		})
	})

	Context("slice field", func() {
		It("sets from comma-separated string", func() {
			field, val, _ := fieldByYAMLTag(entity, "tags")
			err := setFieldFromString(ctx, val, field.Type, "foo,bar")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(Equal([]string{"foo", "bar"}))
		})

		It("sets nil for empty string", func() {
			entity.Tags = []string{"old"}
			field, val, _ := fieldByYAMLTag(entity, "tags")
			err := setFieldFromString(ctx, val, field.Type, "")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(BeNil())
		})
	})

	Context("*time.Time field", func() {
		It("sets from YYYY-MM-DD string", func() {
			field, val, _ := fieldByYAMLTag(entity, "due_date")
			err := setFieldFromString(ctx, val, field.Type, "2025-06-15")
			Expect(err).To(BeNil())
			Expect(entity.DueDate).NotTo(BeNil())
			Expect(entity.DueDate.Format("2006-01-02")).To(Equal("2025-06-15"))
		})

		It("sets nil for empty string", func() {
			t := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			entity.DueDate = &t
			field, val, _ := fieldByYAMLTag(entity, "due_date")
			err := setFieldFromString(ctx, val, field.Type, "")
			Expect(err).To(BeNil())
			Expect(entity.DueDate).To(BeNil())
		})

		It("returns error for invalid date format", func() {
			field, val, _ := fieldByYAMLTag(entity, "due_date")
			err := setFieldFromString(ctx, val, field.Type, "not-a-date")
			Expect(err).To(MatchError(ContainSubstring("invalid date format")))
		})
	})
})

var _ = Describe("clearField", func() {
	Context("string field", func() {
		It("zeros the value", func() {
			entity := &reflectTestEntity{Status: "active"}
			field, val, _ := fieldByYAMLTag(entity, "status")
			clearField(val, field.Type)
			Expect(entity.Status).To(Equal(""))
		})
	})

	Context("pointer field", func() {
		It("sets to nil", func() {
			t := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			entity := &reflectTestEntity{DueDate: &t}
			field, val, _ := fieldByYAMLTag(entity, "due_date")
			clearField(val, field.Type)
			Expect(entity.DueDate).To(BeNil())
		})
	})

	Context("slice field", func() {
		It("sets to nil", func() {
			entity := &reflectTestEntity{Tags: []string{"a"}}
			field, val, _ := fieldByYAMLTag(entity, "tags")
			clearField(val, field.Type)
			Expect(entity.Tags).To(BeNil())
		})
	})
})

var _ = Describe("isListField", func() {
	Context("[]string field", func() {
		It("returns true", func() {
			entity := &reflectTestEntity{Tags: []string{"a"}}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			Expect(isListField(val)).To(BeTrue())
		})
	})

	Context("string field", func() {
		It("returns false", func() {
			entity := &reflectTestEntity{Status: "active"}
			_, val, _ := fieldByYAMLTag(entity, "status")
			Expect(isListField(val)).To(BeFalse())
		})
	})

	Context("*time.Time field", func() {
		It("returns false", func() {
			entity := &reflectTestEntity{}
			_, val, _ := fieldByYAMLTag(entity, "due_date")
			Expect(isListField(val)).To(BeFalse())
		})
	})
})

var _ = Describe("appendToList", func() {
	var entity *reflectTestEntity

	BeforeEach(func() {
		entity = &reflectTestEntity{}
	})

	Context("appending to empty list", func() {
		It("results in a list with one element", func() {
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := appendToList(val, "foo")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(Equal([]string{"foo"}))
		})
	})

	Context("appending to non-empty list", func() {
		It("results in list with N+1 elements", func() {
			entity.Tags = []string{"a", "b"}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := appendToList(val, "c")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(Equal([]string{"a", "b", "c"}))
		})
	})

	Context("appending duplicate value", func() {
		It("returns error containing 'already exists'", func() {
			entity.Tags = []string{"a", "b"}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := appendToList(val, "a")
			Expect(err).To(MatchError(ContainSubstring("already exists")))
			Expect(entity.Tags).To(Equal([]string{"a", "b"}))
		})
	})

	Context("calling on non-slice field", func() {
		It("returns error containing 'not a list field'", func() {
			_, val, _ := fieldByYAMLTag(entity, "status")
			err := appendToList(val, "foo")
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
		})
	})
})

var _ = Describe("removeFromList", func() {
	var entity *reflectTestEntity

	BeforeEach(func() {
		entity = &reflectTestEntity{}
	})

	Context("removing existing value", func() {
		It("results in list without the value", func() {
			entity.Tags = []string{"a", "b", "c"}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := removeFromList(val, "b")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(Equal([]string{"a", "c"}))
		})
	})

	Context("removing non-existent value", func() {
		It("returns error containing 'not found'", func() {
			entity.Tags = []string{"a", "b"}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := removeFromList(val, "x")
			Expect(err).To(MatchError(ContainSubstring("not found")))
			Expect(entity.Tags).To(Equal([]string{"a", "b"}))
		})
	})

	Context("removing last element", func() {
		It("results in an empty (not nil) slice", func() {
			entity.Tags = []string{"only"}
			_, val, _ := fieldByYAMLTag(entity, "tags")
			err := removeFromList(val, "only")
			Expect(err).To(BeNil())
			Expect(entity.Tags).To(HaveLen(0))
		})
	})

	Context("calling on non-slice field", func() {
		It("returns error containing 'not a list field'", func() {
			_, val, _ := fieldByYAMLTag(entity, "status")
			err := removeFromList(val, "foo")
			Expect(err).To(MatchError(ContainSubstring("not a list field")))
		})
	})
})
