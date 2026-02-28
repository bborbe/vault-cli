// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("Priority", func() {
	Describe("UnmarshalYAML", func() {
		var (
			result struct {
				Priority domain.Priority `yaml:"priority"`
			}
			err error
		)

		Context("valid int value", func() {
			BeforeEach(func() {
				yamlData := []byte("priority: 1")
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to the int value", func() {
				Expect(result.Priority).To(Equal(domain.Priority(1)))
			})
		})

		Context("another valid int value", func() {
			BeforeEach(func() {
				yamlData := []byte("priority: 5")
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to the int value", func() {
				Expect(result.Priority).To(Equal(domain.Priority(5)))
			})
		})

		Context("zero value", func() {
			BeforeEach(func() {
				yamlData := []byte("priority: 0")
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to zero", func() {
				Expect(result.Priority).To(Equal(domain.Priority(0)))
			})
		})

		Context("string value", func() {
			BeforeEach(func() {
				yamlData := []byte("priority: medium")
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to -1 for invalid string", func() {
				Expect(result.Priority).To(Equal(domain.Priority(-1)))
			})
		})

		Context("quoted string value", func() {
			BeforeEach(func() {
				yamlData := []byte(`priority: "high"`)
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to -1 for invalid string", func() {
				Expect(result.Priority).To(Equal(domain.Priority(-1)))
			})
		})

		Context("missing field", func() {
			BeforeEach(func() {
				yamlData := []byte("other: value")
				err = yaml.Unmarshal(yamlData, &result)
			})

			It("returns no error", func() {
				Expect(err).To(BeNil())
			})

			It("sets priority to -1 (UnmarshalYAML is called with null node)", func() {
				// YAML unmarshaler calls UnmarshalYAML even for missing/null fields,
				// and value.Decode(&i) fails for null, so we get -1
				Expect(result.Priority).To(Equal(domain.Priority(-1)))
			})
		})
	})
})
