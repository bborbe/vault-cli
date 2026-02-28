// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("ID String methods", func() {
	Describe("TaskID.String", func() {
		It("converts TaskID to string", func() {
			id := domain.TaskID("my-task")
			Expect(id.String()).To(Equal("my-task"))
		})
	})

	Describe("GoalID.String", func() {
		It("converts GoalID to string", func() {
			id := domain.GoalID("my-goal")
			Expect(id.String()).To(Equal("my-goal"))
		})
	})

	Describe("ThemeID.String", func() {
		It("converts ThemeID to string", func() {
			id := domain.ThemeID("my-theme")
			Expect(id.String()).To(Equal("my-theme"))
		})
	})
})
