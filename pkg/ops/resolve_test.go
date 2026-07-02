// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("ResolveOperation", func() {
	var (
		ctx             context.Context
		resolveOp       ops.ResolveOperation
		mockTaskStorage *mocks.TaskStorage
		mockGoalStorage *mocks.GoalStorage
		vaultPath       string
		inputName       string
		result          domain.ResolveResult
		err             error
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockTaskStorage = &mocks.TaskStorage{}
		mockGoalStorage = &mocks.GoalStorage{}
		resolveOp = ops.NewResolveOperation(mockTaskStorage, mockGoalStorage)
		vaultPath = "/path/to/vault"
		inputName = "my-entity"
	})

	JustBeforeEach(func() {
		result, err = resolveOp.Execute(ctx, vaultPath, inputName)
	})

	Context("task match", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(&domain.Task{}, nil)
		})

		It("returns found=true with type task", func() {
			Expect(err).To(BeNil())
			Expect(result.Type).To(Equal("task"))
			Expect(result.Found).To(BeTrue())
			Expect(result.Name).To(Equal(inputName))
		})

		It("does not consult goal storage (short-circuit)", func() {
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(0))
		})
	})

	Context("goal match (task not found)", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("file not found"))
			mockGoalStorage.FindGoalByNameReturns(&domain.Goal{}, nil)
		})

		It("returns found=true with type goal", func() {
			Expect(err).To(BeNil())
			Expect(result.Type).To(Equal("goal"))
			Expect(result.Found).To(BeTrue())
			Expect(result.Name).To(Equal(inputName))
		})
	})

	Context("task-first priority", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(&domain.Task{}, nil)
			mockGoalStorage.FindGoalByNameReturns(&domain.Goal{}, nil)
		})

		It("returns task type even when goal also matches", func() {
			Expect(err).To(BeNil())
			Expect(result.Type).To(Equal("task"))
			Expect(result.Found).To(BeTrue())
		})

		It("never consults goal storage", func() {
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(0))
		})
	})

	Context("not found", func() {
		BeforeEach(func() {
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("file not found"))
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("file not found"))
		})

		It("returns found=false with empty type", func() {
			Expect(err).To(BeNil())
			Expect(result.Type).To(Equal(""))
			Expect(result.Found).To(BeFalse())
		})

		It("echoes the input name", func() {
			Expect(result.Name).To(Equal(inputName))
		})
	})

	Context("name is echoed correctly", func() {
		BeforeEach(func() {
			inputName = "Does Not Exist"
			mockTaskStorage.FindTaskByNameReturns(nil, errors.New("file not found"))
			mockGoalStorage.FindGoalByNameReturns(nil, errors.New("file not found"))
		})

		It("result.Name equals the exact input name", func() {
			Expect(result.Name).To(Equal("Does Not Exist"))
		})
	})
})
