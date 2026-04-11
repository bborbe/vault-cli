// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("FrontmatterMap", func() {
	Describe("NewFrontmatterMap(nil)", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(nil)
		})

		It("returns nil for Keys", func() {
			Expect(fm.Keys()).To(BeNil())
		})

		It("returns nil for Get", func() {
			Expect(fm.Get("missing")).To(BeNil())
		})
	})

	Describe("NewFrontmatterMap with data", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"status": "todo"})
		})

		It("GetString returns value", func() {
			Expect(fm.GetString("status")).To(Equal("todo"))
		})
	})

	Describe("Set and Get round-trips", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(nil)
		})

		It("round-trips a string value", func() {
			fm.Set("key", "value")
			Expect(fm.Get("key")).To(Equal("value"))
		})

		It("round-trips an int value", func() {
			fm.Set("num", 42)
			Expect(fm.Get("num")).To(Equal(42))
		})

		It("round-trips a []string value", func() {
			fm.Set("tags", []string{"a", "b"})
			Expect(fm.Get("tags")).To(Equal([]string{"a", "b"}))
		})
	})

	Describe("GetString on int value", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"priority": 3})
		})

		It("returns decimal string representation", func() {
			Expect(fm.GetString("priority")).To(Equal("3"))
		})
	})

	Describe("GetStringSlice", func() {
		Context("on []any value", func() {
			var fm domain.FrontmatterMap

			BeforeEach(func() {
				fm = domain.NewFrontmatterMap(map[string]any{"tags": []any{"a", "b"}})
			})

			It("returns []string", func() {
				Expect(fm.GetStringSlice("tags")).To(Equal([]string{"a", "b"}))
			})
		})

		Context("on nil key", func() {
			var fm domain.FrontmatterMap

			BeforeEach(func() {
				fm = domain.NewFrontmatterMap(nil)
			})

			It("returns nil", func() {
				Expect(fm.GetStringSlice("missing")).To(BeNil())
			})
		})

		Context("on comma-separated string", func() {
			var fm domain.FrontmatterMap

			BeforeEach(func() {
				fm = domain.NewFrontmatterMap(map[string]any{"tags": "a,b"})
			})

			It("splits on comma", func() {
				Expect(fm.GetStringSlice("tags")).To(Equal([]string{"a", "b"}))
			})
		})
	})

	Describe("Delete", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"key": "val"})
		})

		It("removes key; subsequent Get returns nil", func() {
			fm.Delete("key")
			Expect(fm.Get("key")).To(BeNil())
		})
	})

	Describe("Set with nil value", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"key": "val"})
		})

		It("is equivalent to Delete", func() {
			fm.Set("key", nil)
			Expect(fm.Get("key")).To(BeNil())
		})
	})

	Describe("Keys", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"a": 1, "b": 2, "c": 3})
		})

		It("returns all stored keys in any order", func() {
			Expect(fm.Keys()).To(ConsistOf("a", "b", "c"))
		})
	})

	Describe("RawMap", func() {
		var fm domain.FrontmatterMap

		BeforeEach(func() {
			fm = domain.NewFrontmatterMap(map[string]any{"x": "y"})
		})

		It("returns the underlying map", func() {
			raw := fm.RawMap()
			Expect(raw).To(HaveKeyWithValue("x", "y"))
		})
	})

	Describe("GetTime", func() {
		It("returns non-nil for time.Time value", func() {
			t := time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)
			fm := domain.NewFrontmatterMap(map[string]any{"d": t})
			result := fm.GetTime("d")
			Expect(result).NotTo(BeNil())
			Expect(result.UTC().Format("2006-01-02")).To(Equal("2026-04-13"))
		})

		It("parses ISO-8601 date-only string", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": "2026-04-13"})
			result := fm.GetTime("d")
			Expect(result).NotTo(BeNil())
			Expect(result.UTC().Year()).To(Equal(2026))
			Expect(result.UTC().Month()).To(Equal(time.April))
			Expect(result.UTC().Day()).To(Equal(13))
		})

		It("parses RFC3339 datetime string", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": "2026-03-08T00:00:00Z"})
			result := fm.GetTime("d")
			Expect(result).NotTo(BeNil())
			Expect(result.UTC().Format("2006-01-02")).To(Equal("2026-03-08"))
		})

		It("returns nil for nil value", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": nil})
			Expect(fm.GetTime("d")).To(BeNil())
		})

		It("returns nil for empty string", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": ""})
			Expect(fm.GetTime("d")).To(BeNil())
		})

		It("returns nil for wrong type (integer)", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": 42})
			Expect(fm.GetTime("d")).To(BeNil())
		})

		It("returns nil for missing key", func() {
			fm := domain.NewFrontmatterMap(nil)
			Expect(fm.GetTime("absent")).To(BeNil())
		})

		It("returns nil for unparseable string", func() {
			fm := domain.NewFrontmatterMap(map[string]any{"d": "not-a-date"})
			Expect(fm.GetTime("d")).To(BeNil())
		})
	})
})
