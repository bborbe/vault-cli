// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package domain_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/pkg/domain"
)

var _ = Describe("TaskStatus", func() {
	Describe("String", func() {
		It("returns string value for todo", func() {
			Expect(domain.TaskStatusTodo.String()).To(Equal("todo"))
		})
		It("returns string value for in_progress", func() {
			Expect(domain.TaskStatusInProgress.String()).To(Equal("in_progress"))
		})
		It("returns string value for completed", func() {
			Expect(domain.TaskStatusCompleted.String()).To(Equal("completed"))
		})
	})

	Describe("Validate", func() {
		ctx := context.Background()

		It("returns nil for todo", func() {
			Expect(domain.TaskStatusTodo.Validate(ctx)).To(BeNil())
		})
		It("returns nil for in_progress", func() {
			Expect(domain.TaskStatusInProgress.Validate(ctx)).To(BeNil())
		})
		It("returns nil for completed", func() {
			Expect(domain.TaskStatusCompleted.Validate(ctx)).To(BeNil())
		})
		It("returns nil for backlog", func() {
			Expect(domain.TaskStatusBacklog.Validate(ctx)).To(BeNil())
		})
		It("returns nil for hold", func() {
			Expect(domain.TaskStatusHold.Validate(ctx)).To(BeNil())
		})
		It("returns nil for aborted", func() {
			Expect(domain.TaskStatusAborted.Validate(ctx)).To(BeNil())
		})
		It("returns error for invalid status", func() {
			Expect(domain.TaskStatus("invalid").Validate(ctx)).NotTo(BeNil())
		})
		It("returns error for empty status", func() {
			Expect(domain.TaskStatus("").Validate(ctx)).NotTo(BeNil())
		})
	})

	Describe("NormalizeTaskStatus", func() {
		Context("canonical values", func() {
			It("returns todo unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("todo")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusTodo))
			})

			It("returns in_progress unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("in_progress")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusInProgress))
			})

			It("returns completed unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("completed")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusCompleted))
			})

			It("returns backlog unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("backlog")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusBacklog))
			})

			It("returns hold unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("hold")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusHold))
			})

			It("returns aborted unchanged", func() {
				status, ok := domain.NormalizeTaskStatus("aborted")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusAborted))
			})
		})

		Context("alias values", func() {
			It("normalizes done to completed", func() {
				status, ok := domain.NormalizeTaskStatus("done")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusCompleted))
			})

			It("normalizes current to in_progress", func() {
				status, ok := domain.NormalizeTaskStatus("current")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusInProgress))
			})

			It("normalizes next to todo", func() {
				status, ok := domain.NormalizeTaskStatus("next")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusTodo))
			})

			It("normalizes deferred to hold", func() {
				status, ok := domain.NormalizeTaskStatus("deferred")
				Expect(ok).To(BeTrue())
				Expect(status).To(Equal(domain.TaskStatusHold))
			})
		})

		Context("invalid values", func() {
			It("returns false for garbage", func() {
				status, ok := domain.NormalizeTaskStatus("garbage")
				Expect(ok).To(BeFalse())
				Expect(status).To(Equal(domain.TaskStatus("")))
			})

			It("returns false for empty string", func() {
				status, ok := domain.NormalizeTaskStatus("")
				Expect(ok).To(BeFalse())
				Expect(status).To(Equal(domain.TaskStatus("")))
			})

			It("returns false for unknown status", func() {
				status, ok := domain.NormalizeTaskStatus("invalid_status")
				Expect(ok).To(BeFalse())
				Expect(status).To(Equal(domain.TaskStatus("")))
			})
		})
	})

	Describe("IsValidTaskStatus", func() {
		It("returns true for todo", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusTodo)).To(BeTrue())
		})

		It("returns true for in_progress", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusInProgress)).To(BeTrue())
		})

		It("returns true for completed", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusCompleted)).To(BeTrue())
		})

		It("returns true for backlog", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusBacklog)).To(BeTrue())
		})

		It("returns true for hold", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusHold)).To(BeTrue())
		})

		It("returns true for aborted", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatusAborted)).To(BeTrue())
		})

		It("returns false for invalid status", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatus("invalid"))).To(BeFalse())
		})

		It("returns false for empty string", func() {
			Expect(domain.IsValidTaskStatus(domain.TaskStatus(""))).To(BeFalse())
		})
	})

})
