// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops_test

import (
	"context"

	"github.com/bborbe/errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/vault-cli/mocks"
	"github.com/bborbe/vault-cli/pkg/config"
	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/ops"
)

var _ = Describe("GoalWorkOnOperation", func() {
	var (
		ctx             context.Context
		err             error
		result          ops.MutationResult
		goalWorkOnOp    ops.GoalWorkOnOperation
		mockGoalStorage *mocks.GoalStorage
		mockStarter     *mocks.ClaudeSessionStarter
		mockResumer     *mocks.ClaudeResumer
		vaultPath       string
		goalName        string
		assignee        string
		goal            *domain.Goal
		isInteractive   bool
		testVault       config.Vault
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockGoalStorage = &mocks.GoalStorage{}
		mockStarter = &mocks.ClaudeSessionStarter{}
		mockResumer = &mocks.ClaudeResumer{}
		goalWorkOnOp = ops.NewGoalWorkOnOperation(
			mockGoalStorage,
			mockStarter,
			mockResumer,
		)
		vaultPath = "/path/to/vault"
		goalName = "my-goal"
		assignee = "user@example.com"
		isInteractive = false
		testVault = config.Vault{
			Path:              vaultPath,
			Name:              "test-vault",
			WorkOnGoalCommand: "/vault-cli:work-on-goal",
		}

		goal = domain.NewGoal(
			map[string]any{"status": "next"},
			domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
			domain.Content(""),
		)
		mockGoalStorage.FindGoalByNameReturns(goal, nil)
		mockGoalStorage.WriteGoalReturns(nil)
		mockStarter.StartSessionReturns("session-123", nil)
		mockResumer.ResumeSessionReturns(nil)
	})

	JustBeforeEach(func() {
		result, err = goalWorkOnOp.Execute(
			ctx,
			vaultPath,
			goalName,
			assignee,
			"test-vault",
			isInteractive,
			vaultPath,
			&testVault,
		)
	})

	Context("success", func() {
		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("calls FindGoalByName", func() {
			Expect(mockGoalStorage.FindGoalByNameCallCount()).To(Equal(1))
			actualCtx, actualVaultPath, actualGoalName := mockGoalStorage.FindGoalByNameArgsForCall(
				0,
			)
			Expect(actualCtx).To(Equal(ctx))
			Expect(actualVaultPath).To(Equal(vaultPath))
			Expect(actualGoalName).To(Equal(goalName))
		})

		It("marks goal as in_progress", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})

		It("sets assignee correctly", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(assignee))
		})

		It("starts a claude session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
		})

		It("passes goal name to session starter", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, _, _, name := mockStarter.StartSessionArgsForCall(0)
			Expect(name).To(Equal(goalName))
		})
	})

	Context("when assignee already equals current user", func() {
		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "next", "assignee": assignee},
				domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the existing assignee", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(assignee))
		})

		It("emits no assignee warning", func() {
			Expect(result.Warnings).NotTo(ContainElement(ContainSubstring("assignee not updated")))
		})
	})

	Context("when assignee is set to a different user", func() {
		const otherUser = "alice@example.com"

		BeforeEach(func() {
			goal = domain.NewGoal(
				map[string]any{"status": "next", "assignee": otherUser},
				domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"},
				domain.Content(""),
			)
			mockGoalStorage.FindGoalByNameReturns(goal, nil)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("preserves the other user's assignment", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Assignee()).To(Equal(otherUser))
		})

		It("emits an assignee-not-updated warning naming both users", func() {
			Expect(result.Warnings).To(ContainElement(ContainSubstring("assignee not updated")))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(otherUser)))
			Expect(result.Warnings).To(ContainElement(ContainSubstring(assignee)))
		})

		It("still marks the goal in_progress (status is independent of assignee)", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})
	})

	Context("custom work on command", func() {
		BeforeEach(func() {
			testVault.WorkOnGoalCommand = "/custom-cmd"
		})

		It("uses the configured work on command in the prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, prompt, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(`^/custom-cmd "`))
		})

		It("appends --non-interactive to the bootstrap prompt", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(1))
			_, prompt, _, _ := mockStarter.StartSessionArgsForCall(0)
			Expect(prompt).To(MatchRegexp(` --non-interactive$`))
			Expect(prompt).To(MatchRegexp(`/path/to/vault/Goals/my-goal\.md`))
		})
	})

	Context("when starter is nil and goal has no cached session ID", func() {
		BeforeEach(func() {
			goalWorkOnOp = ops.NewGoalWorkOnOperation(
				mockGoalStorage,
				nil,
				nil,
			)
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("skips session start", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("emits warning about missing starter", func() {
			Expect(
				result.Warnings,
			).To(ContainElement(ContainSubstring("claude session: claude session starter unavailable")))
		})

		It("returns empty session ID", func() {
			Expect(result.SessionID).To(Equal(""))
		})
	})

	Context("when goal already has a session ID", func() {
		BeforeEach(func() {
			goal.SetClaudeSessionID("existing-session")
		})

		It("does not start a new session", func() {
			Expect(mockStarter.StartSessionCallCount()).To(Equal(0))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})

		It("returns cached session ID", func() {
			Expect(result.SessionID).To(Equal("existing-session"))
		})
	})

	Context("when session start fails (hard failure)", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns("", ErrTest)
		})

		It("returns wrapped error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
		})

		It("returns Success=false", func() {
			Expect(result.Success).To(BeFalse())
		})
	})

	Context("when claude returns zero turns", func() {
		BeforeEach(func() {
			mockStarter.StartSessionReturns(
				"",
				errors.New(ctx, "claude returned 0 turns: Unknown command: /x"),
			)
		})

		It("returns non-nil error wrapped with start work-on session and Success=false", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("start work-on session"))
			Expect(err.Error()).To(ContainSubstring("claude returned 0 turns: Unknown command: /x"))
			Expect(result.Success).To(BeFalse())
		})

		It("still marks goal as in_progress", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(BeNumerically(">=", 1))
			_, writtenGoal := mockGoalStorage.WriteGoalArgsForCall(0)
			Expect(writtenGoal.Status()).To(Equal(domain.GoalStatusInProgress))
		})
	})

	Context("interactive mode", func() {
		BeforeEach(func() {
			isInteractive = true
		})

		It("calls ResumeSession", func() {
			Expect(mockResumer.ResumeSessionCallCount()).To(Equal(1))
			_, sessionID, cwd := mockResumer.ResumeSessionArgsForCall(0)
			Expect(sessionID).To(Equal("session-123"))
			Expect(cwd).To(Equal(vaultPath))
		})

		It("returns no error", func() {
			Expect(err).To(BeNil())
		})
	})

	Context("goal not found", func() {
		BeforeEach(func() {
			mockGoalStorage.FindGoalByNameReturns(nil, ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})

		It("does not call WriteGoal", func() {
			Expect(mockGoalStorage.WriteGoalCallCount()).To(Equal(0))
		})
	})

	Context("write error", func() {
		BeforeEach(func() {
			mockGoalStorage.WriteGoalReturns(ErrTest)
		})

		It("returns error", func() {
			Expect(err).NotTo(BeNil())
		})
	})
})
