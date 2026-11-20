// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"sort"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/goal-complete-operation.go --fake-name GoalCompleteOperation . GoalCompleteOperation
type GoalCompleteOperation interface {
	Execute(
		ctx context.Context,
		vaultPath string,
		goalName string,
		vaultName string,
		force bool,
		reason string,
		gateSuccessor string,
	) (MutationResult, error)
}

// NewGoalCompleteOperation creates a new goal complete operation.
//
// The parent-topic close runs through topicCompleteOperation, so the closure
// rule and the topic close sequence exist once: the goal close never writes a
// topic itself.
func NewGoalCompleteOperation(
	goalStorage storage.GoalStorage,
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
	topicCompleteOperation TopicCompleteOperation,
	topicStorage storage.TopicStorage,
) GoalCompleteOperation {
	return &goalCompleteOperation{
		goalStorage:            goalStorage,
		taskStorage:            taskStorage,
		currentDateTime:        currentDateTime,
		topicCompleteOperation: topicCompleteOperation,
		topicStorage:           topicStorage,
	}
}

type goalCompleteOperation struct {
	goalStorage            storage.GoalStorage
	taskStorage            storage.TaskStorage
	currentDateTime        libtime.CurrentDateTime
	topicCompleteOperation TopicCompleteOperation
	topicStorage           storage.TopicStorage
}

// Execute marks a goal as completed, optionally blocking if open tasks exist.
func (g *goalCompleteOperation) Execute(
	ctx context.Context,
	vaultPath string,
	goalName string,
	vaultName string,
	force bool,
	reason string,
	gateSuccessor string,
) (MutationResult, error) {
	goal, err := g.goalStorage.FindGoalByName(ctx, vaultPath, goalName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"find goal",
		)
	}

	if goal.Status() == domain.GoalStatusCompleted || goal.Status() == domain.GoalStatusAborted {
		result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
		result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
		return result, nil
	}

	var members []closureMember
	if !force {
		members, err = g.linkedTaskMembers(ctx, vaultPath, goalName)
		if err != nil {
			return MutationResult{Success: false, Error: err.Error()}, err
		}
	}

	verdict, err := checkClosureGate(ctx, closureKindGoal, string(goal.Content), members)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}
	if verdict.Outcome == closureOutcomeTerminal {
		result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
		result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
		return result, nil
	}
	if verdict.Outcome != closureOutcomeClose {
		msg := closureRefusalMessage("goal", verdict.OpenItems)
		return MutationResult{Success: false, Error: msg, OpenItems: verdict.OpenItems},
			errors.Errorf(ctx, "%s", msg)
	}

	// One-step close-out: persist reason and successor before the status
	// transition so both land in a single WriteGoal. Fields are written only
	// when provided — the two-step close-out (fields already in the file) works
	// unchanged.
	if result, err := g.setCompletedStatus(ctx, goal, reason, gateSuccessor); err != nil {
		return result, err
	}
	goal.SetCompleted(libtime.ToDate(g.currentDateTime.Now().Time()).Ptr())
	// The terminal transition ends the phase walk: land `phase: done` in the same
	// in-memory mutation as the status so the single WriteGoal below persists both.
	goal.SetPhase(domain.GoalPhaseDone.Ptr())

	if err := g.goalStorage.WriteGoal(ctx, goal); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(
			ctx,
			err,
			"write goal",
		)
	}

	result := MutationResult{Success: true, Name: goal.Name, Vault: vaultName}
	result.Parents = g.cascadeTopics(ctx, vaultPath, vaultName, goal)
	return result, nil
}

// cascadeTopics attempts every topic that names the goal, either through the
// goal's declared `topics:` list or through the topic page's `## Goals` section.
// Declared topics run first in document order, then discovered topics sorted by
// name, with duplicates removed. It runs only once the goal is terminal: a
// topic's gate requires its member goals terminal, so an open goal could only
// refuse. The topics of an already-terminal goal are still attempted, which is
// how a topic closes on a later run after its goal closed earlier.
//
// A failure to scan the topics directory never fails the goal close: the
// declared names are still attempted and the scan failure is appended as a
// refused `<topics scan>` entry so it is visible rather than silent.
func (g *goalCompleteOperation) cascadeTopics(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	goal *domain.Goal,
) []ParentAttempt {
	names := goal.Topics()

	var scanErr error
	topics, err := g.topicStorage.ListTopics(ctx, vaultPath)
	if err != nil {
		scanErr = err
	} else {
		names = append(names, discoveredTopicNames(goal, topics, names)...)
	}

	attempts := cascadeParents(
		ctx,
		names,
		func(ctx context.Context, topicName string) (MutationResult, error) {
			return g.topicCompleteOperation.Execute(ctx, vaultPath, topicName, vaultName)
		},
	)
	if scanErr != nil {
		attempts = append(attempts, ParentAttempt{
			Name:      "<topics scan>",
			Result:    ParentResultRefused,
			OpenItems: []string{scanErr.Error()},
		})
	}
	return attempts
}

// discoveredTopicNames returns the topics whose `## Goals` section names goal,
// sorted by name. A name already declared in declared is skipped so the
// declared order stays observable and no topic is attempted twice.
func discoveredTopicNames(
	goal *domain.Goal,
	topics []*domain.Topic,
	declared []string,
) []string {
	goalName := normaliseGoalLink(goal.Name)
	seen := make(map[string]bool, len(declared))
	for _, name := range declared {
		seen[normaliseGoalLink(name)] = true
	}

	var names []string
	for _, topic := range topics {
		if !goalSectionNames(goalName, string(topic.Content)) {
			continue
		}
		normalised := normaliseGoalLink(topic.Name)
		if seen[normalised] {
			continue
		}
		seen[normalised] = true
		names = append(names, topic.Name)
	}
	sort.Strings(names)
	return names
}

// goalSectionNames reports whether the topic content's Goals section names goal.
func goalSectionNames(goalName string, content string) bool {
	for _, member := range parseGoalSectionMembers(content) {
		if member == goalName {
			return true
		}
	}
	return false
}

// setCompletedStatus persists the one-step close-out fields (when provided)
// and transitions the goal to completed. completed never consults the
// close-out fields (spec 039), so this cannot fail on the guard; the error
// return is kept for status-set failure propagation.
//
//nolint:dupl // Structurally parallel to the task variant; frozen field names prevent dedup
func (g *goalCompleteOperation) setCompletedStatus(
	ctx context.Context,
	goal *domain.Goal,
	reason, gateSuccessor string,
) (MutationResult, error) {
	if reason != "" {
		if err := goal.SetField(ctx, "aborted_reason", reason); err != nil {
			return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(ctx, err, "set aborted_reason")
		}
	}
	if gateSuccessor != "" {
		if err := goal.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
			return MutationResult{
				Success: false,
				Error:   err.Error(),
			}, errors.Wrap(ctx, err, "set gate_successor")
		}
	}
	if err := goal.SetStatus(domain.GoalStatusCompleted); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set status")
	}
	return MutationResult{}, nil
}

// linkedTaskMembers returns every task linked to the goal, as closure members.
// It is the only reader of the task relation; the gate decides which statuses
// are open.
func (g *goalCompleteOperation) linkedTaskMembers(
	ctx context.Context,
	vaultPath string,
	goalName string,
) ([]closureMember, error) {
	tasks, err := g.taskStorage.ListTasks(ctx, vaultPath)
	if err != nil {
		return nil, errors.Wrap(ctx, err, "list tasks")
	}

	members := make([]closureMember, 0, len(tasks))
	for _, task := range tasks {
		if !taskLinkedToGoal(task, goalName) {
			continue
		}
		members = append(members, closureMember{Name: task.Name, Status: string(task.Status())})
	}
	return members, nil
}

func taskLinkedToGoal(task *domain.Task, goalName string) bool {
	for _, goal := range task.Goals() {
		if normaliseGoalLink(goal) == goalName {
			return true
		}
	}
	return false
}
