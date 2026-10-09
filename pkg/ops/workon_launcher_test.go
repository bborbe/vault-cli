// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"
	stderrors "errors"

	libtime "github.com/bborbe/time"
	libtimetest "github.com/bborbe/time/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("WorkOnOperation launcher resolution", func() {
	const vaultScript = "/s/cc-private"

	var (
		ctx                  context.Context
		err                  error
		result               ops.MutationResult
		mockTaskStorage      *mocks.TaskStorage
		mockDailyNoteStorage *mocks.DailyNoteStorage
		mockGoalStorage      *mocks.GoalStorage
		defaultStarter       *mocks.ClaudeSessionStarter
		defaultResumer       *mocks.ClaudeResumer
		factoryStarter       *mocks.ClaudeSessionStarter
		factoryResumer       *mocks.ClaudeResumer
		factoryCalls         []string
		factoryStarterNil    bool
		testVault            config.Vault
		vaultPath            string
		task                 *domain.Task
	)

	newTask := func(data map[string]any) *domain.Task {
		return domain.NewTask(
			data,
			domain.FileMetadata{Name: "my-task", FilePath: vaultPath + "/Tasks/my-task.md"},
			domain.Content(""),
		)
	}

	BeforeEach(func() {
		ctx = context.Background()
		vaultPath = "/path/to/vault"
		mockTaskStorage = &mocks.TaskStorage{}
		mockDailyNoteStorage = &mocks.DailyNoteStorage{}
		mockGoalStorage = &mocks.GoalStorage{}
		defaultStarter = &mocks.ClaudeSessionStarter{}
		defaultResumer = &mocks.ClaudeResumer{}
		factoryStarter = &mocks.ClaudeSessionStarter{}
		factoryResumer = &mocks.ClaudeResumer{}
		factoryCalls = nil
		factoryStarterNil = false

		testVault = config.Vault{
			Path:          vaultPath,
			Name:          "test-vault",
			WorkOnCommand: "/vault-cli:work-on-task",
			ClaudeScript:  vaultScript,
		}

		mockTaskStorage.WriteTaskReturns(nil)
		mockDailyNoteStorage.ReadDailyNoteReturns("", nil)
		defaultStarter.StartSessionReturns(nil)
		factoryStarter.StartSessionReturns(nil)
		defaultResumer.ResumeSessionReturns(nil)
		factoryResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		currentDateTime := libtime.NewCurrentDateTime()
		currentDateTime.SetNow(libtimetest.ParseDateTime("2026-03-03T12:00:00Z"))

		factory := func(script string) (ops.ClaudeSessionStarter, ops.ClaudeResumer) {
			factoryCalls = append(factoryCalls, script)
			if factoryStarterNil {
				return nil, factoryResumer
			}
			return factoryStarter, factoryResumer
		}

		workOnOp := ops.NewWorkOnOperation(
			mockTaskStorage,
			mockDailyNoteStorage,
			mockGoalStorage,
			currentDateTime,
			func() string { return pinnedSessionID },
			defaultStarter,
			defaultResumer,
			factory,
		)

		mockTaskStorage.FindTaskByNameReturns(task, nil)

		result, err = workOnOp.Execute(
			ctx, vaultPath, "my-task", "user@example.com", "test-vault",
			false, vaultPath, &testVault,
		)
	})

	Context("task carries its own launcher", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next", "launcher": "cc-private-claude"})
		})

		It("builds the pair for the resolved launcher", func() {
			Expect(err).To(BeNil())
			Expect(factoryCalls).To(Equal([]string{"/s/cc-private-claude"}))
		})

		It("starts the session through the resolved pair", func() {
			Expect(err).To(BeNil())
			Expect(factoryStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("never uses the vault-default pair", func() {
			Expect(defaultStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("still writes the task", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(BeNumerically(">=", 1))
		})
	})

	Context("task carries no launcher and no goal launcher", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next"})
		})

		It("uses the vault-default pair", func() {
			Expect(err).To(BeNil())
			Expect(defaultStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("never calls the factory", func() {
			Expect(factoryCalls).To(BeEmpty())
		})
	})

	Context("two goals name different launchers", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next", "goals": []any{"[[A]]", "[[B]]"}})
			mockGoalStorage.FindGoalByNameReturnsOnCall(
				0,
				domain.NewGoal(
					map[string]any{"launcher": "cc-a"},
					domain.FileMetadata{Name: "A"},
					domain.Content(""),
				),
				nil,
			)
			mockGoalStorage.FindGoalByNameReturnsOnCall(
				1,
				domain.NewGoal(
					map[string]any{"launcher": "cc-b"},
					domain.FileMetadata{Name: "B"},
					domain.Content(""),
				),
				nil,
			)
		})

		It("returns an error naming both launchers", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cc-a"))
			Expect(err.Error()).To(ContainSubstring("cc-b"))
		})

		It("leaves the task file untouched", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("starts no session", func() {
			Expect(defaultStarter.StartSessionCallCount()).To(Equal(0))
			Expect(factoryStarter.StartSessionCallCount()).To(Equal(0))
		})
	})

	Context("explicit launcher cannot be found", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next", "launcher": "cc-missing"})
			factoryStarterNil = true
		})

		It("returns an error naming the launcher path", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("/s/cc-missing"))
		})

		It("leaves the task file untouched", func() {
			Expect(mockTaskStorage.WriteTaskCallCount()).To(Equal(0))
		})

		It("never falls back to the vault default", func() {
			Expect(defaultStarter.StartSessionCallCount()).To(Equal(0))
		})
	})

	Context("goal link is stale", func() {
		BeforeEach(func() {
			task = newTask(map[string]any{"status": "next", "goals": []any{"[[Gone]]"}})
			mockGoalStorage.FindGoalByNameReturns(nil, stderrors.New("goal not found"))
		})

		It("still starts on the vault default", func() {
			Expect(err).To(BeNil())
			Expect(defaultStarter.StartSessionCallCount()).To(Equal(1))
			Expect(factoryCalls).To(BeEmpty())
		})

		It("warns naming the stale goal", func() {
			Expect(result.Warnings).To(HaveLen(1))
			Expect(result.Warnings[0]).To(ContainSubstring("Gone"))
		})
	})
})
