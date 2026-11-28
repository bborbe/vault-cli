// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"slices"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/task-approve-operation.go --fake-name TaskApproveOperation . TaskApproveOperation
type TaskApproveOperation interface {
	// Execute approves the named task: it moves a task waiting in the approval
	// inbox (phase todo) to phase planning and records, in the same write, who
	// approved it, the instant of the transition, and the task's owner. Any task
	// that is not waiting in the inbox is refused with nothing written. Any task
	// that already carries `approved_by` and/or `approved_at` is advanced to
	// planning with the record preserved and is not re-approved; no owner is
	// resolved or written on that path.
	//
	// The owner is resolved once, at approval time, by precedence: assignee when
	// non-empty, else the task's existing assignee, else currentUser. An empty
	// assignee means the --assignee flag was not given (a blank flag value is
	// never a request for an unowned task); an empty currentUser means no current
	// user could be resolved. When none of the three names an owner the approval
	// is refused and nothing is written, so an approval never enters planning
	// ownerless. An existing assignee that neither the flag nor currentUser
	// matches is preserved untouched and silently.
	//
	// This rule governs the approval act only. A row that already carries an
	// approval record is advanced to planning without resolving or writing an
	// owner, so such a row may reach planning unassigned — the approval that
	// would have fixed an owner already happened.
	//
	// Approval is unbounded: `task approve` never refuses on a count of approved
	// rows. What opens a task is governed by flag provenance, not by a count — the
	// `/supervisor:open --flagged` selector opens only rows whose `flag_set_by` is
	// `operator` or `legacy`, and that selector is the control the approval path
	// relies on. An agent cannot cause an open by approving a row, however many
	// rows are approved.
	Execute(
		ctx context.Context,
		vaultPath string,
		taskName string,
		vaultName string,
		approvedBy string,
		assignee string,
		currentUser string,
	) (MutationResult, error)
}

// NewTaskApproveOperation creates a new task approve operation.
func NewTaskApproveOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
	managerResolver ManagerResolver,
) TaskApproveOperation {
	return &taskApproveOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
		managerResolver: managerResolver,
	}
}

type taskApproveOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
	// managerResolver is nil when require_manager_on_approve is off, which
	// leaves the approval semantics exactly as they were before the gate.
	managerResolver ManagerResolver
}

// Execute approves a task waiting in the approval inbox.
func (o *taskApproveOperation) Execute(
	ctx context.Context,
	vaultPath string,
	taskName string,
	vaultName string,
	approvedBy string,
	assignee string,
	currentUser string,
) (MutationResult, error) {
	// Refuse an empty approver before reading the task: a present-but-blank
	// approved_by is worse than no record at all, and nothing is written here.
	if approvedBy == "" {
		err := errors.Errorf(
			ctx,
			"approved_by must not be empty: refusing to record a blank approver on task %q",
			taskName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "find task")
	}

	// Read and validate the clock before any guard: a zero instant would be
	// recorded as a placeholder rather than a measurement, so it is refused.
	now := o.currentDateTime.Now()
	if now.Time().IsZero() {
		err := errors.Errorf(
			ctx,
			"approved_at is not a valid instant: refusing to record a zero timestamp on task %q",
			taskName,
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The inbox guard: only a task at phase todo is waiting for approval. The
	// message names both the current phase and the expected one so the operator
	// can see what was found and what was required.
	if phase := task.Phase(); phase == nil || *phase != domain.TaskPhaseTodo {
		current := "(none)"
		if phase != nil {
			current = string(*phase)
		}
		err := errors.Errorf(
			ctx,
			"refusing to approve %q: task is at phase %q, not %q; only a task in the approval inbox (phase %q) can be approved",
			taskName, current, string(domain.TaskPhaseTodo), string(domain.TaskPhaseTodo),
		)
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// A todo row that already carries an approval record has already been
	// approved; only the phase half of the transition is missing. Advance it and
	// preserve the record exactly as it is: no approval is written, no missing
	// key is invented, and no owner is resolved or required, because the owner
	// rule governs the approval act, which already happened. The act of
	// re-approval is still recorded — its instant goes into reapproved_at —
	// while the approval record itself is left untouched.
	if hasApprovalRecord(task) {
		return o.advanceApprovedTask(ctx, task, taskName, now, vaultName)
	}

	// The manager gate. When require_manager_on_approve is on, an approval may
	// leave the inbox only when its goal resolves a live manager, and an
	// unreadable fleet is refused rather than approved. Nothing is written on
	// either refusal.
	//
	// The operator-flag path is deliberately exempt: `task work-on` advances a
	// `flag: true` row to planning without this operation, and that flag is the
	// operator's own dispatch, not dispatch-by-omission. The exemption lives in
	// pkg/ops/workon.go and is intentionally unchanged here.
	managerAtApproval := ""
	if o.managerResolver != nil {
		managerName, err := o.requireLiveManager(ctx, taskName, task, vaultPath)
		if err != nil {
			return MutationResult{Success: false, Error: err.Error()}, err
		}
		managerAtApproval = managerName
	}

	// Resolve the owner after every guard and before any mutation. Refusing here
	// — and only here — keeps the four earlier refusals' messages intact while
	// making approval the one moment an owner is fixed.
	owner, err := resolveOwner(ctx, taskName, task, assignee, currentUser)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All three keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	task.SetPhase(domain.TaskPhasePlanning.Ptr())
	task.Set("approved_by", approvedBy)
	task.Set("approved_at", now.Time())
	// Record the manager that satisfied the gate in the same single write. The
	// key is absent when the gate is off.
	if o.managerResolver != nil {
		task.Set("manager_at_approval", managerAtApproval)
	}
	// Only write the owner when it differs: a task already carrying the owner
	// the flag or the config would produce is not dirtied by an assignee write.
	if owner != task.Assignee() {
		task.SetAssignee(owner)
	}

	return o.write(ctx, task, vaultName)
}

// write persists the task and returns the success result, so the approval
// write and the already-approved repair share exactly one write path.
func (o *taskApproveOperation) write(
	ctx context.Context,
	task *domain.Task,
	vaultName string,
) (MutationResult, error) {
	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return MutationResult{
			Success: false,
			Error:   err.Error(),
		}, errors.Wrap(ctx, err, "write task")
	}
	return MutationResult{Success: true, Name: task.Name, Vault: vaultName}, nil
}

// hasApprovalRecord reports whether the task already carries an approved_by
// or an approved_at key. Presence, not value, is the test: a key carrying an
// empty value still means an approval was recorded, and the repair advances
// the phase without rewriting or completing that record.
//
// Presence is read from the key set, never from Get. Get is a raw map lookup
// that returns nil for a key present with a null value — `approved_by:` with
// nothing after the colon — which the lint rule's line-anchored matcher counts
// as a record. Testing Get would let approve and lint disagree on exactly that
// shape: lint would advance the row while approve treated it as unrecorded,
// resolved an owner, and either wrote a fabricated approval or refused for want
// of one, which is the deadlock this repair exists to close.
func hasApprovalRecord(task *domain.Task) bool {
	keys := task.Keys()
	return slices.Contains(keys, "approved_by") || slices.Contains(keys, "approved_at")
}

// requireLiveManager resolves the task's subjects in order — `managed_by`
// first, then each `goals:` entry — and returns the first live manager found.
// It refuses when the fleet cannot be read, and refuses when no subject
// resolves a manager, so an approval never leaves the inbox unowned.
func (o *taskApproveOperation) requireLiveManager(
	ctx context.Context,
	taskName string,
	task *domain.Task,
	vaultPath string,
) (string, error) {
	subjects := approvalSubjects(task)
	named := subjects
	if len(named) == 0 {
		named = []string{"(no goal)"}
	}

	for _, subject := range named {
		name, found, err := o.managerResolver.LiveManagerForGoal(ctx, subject, vaultPath)
		if err != nil {
			return "", errors.Errorf(
				ctx,
				"refusing to approve %q: cannot read live manager state for %q: %v; nothing was written",
				taskName, subject, err,
			)
		}
		if found {
			return name, nil
		}
	}

	return "", errors.Errorf(
		ctx,
		"refusing to approve %q: goal %q resolves no manager; nothing was written; start a manager for the goal first",
		taskName, named[0],
	)
}

// advanceApprovedTask completes the phase half of a transition for a todo row
// that already carries an approval record. It preserves the record exactly as
// it is, invents no missing key, and resolves no owner.
func (o *taskApproveOperation) advanceApprovedTask(
	ctx context.Context,
	task *domain.Task,
	taskName string,
	now libtime.DateTime,
	vaultName string,
) (MutationResult, error) {
	// A terminal row is refused rather than advanced: moving it to planning
	// would produce the status/phase pair checkPhaseRegression refuses and
	// detectStatusPhaseMismatch reports — the same pair the lint rule marks
	// non-fixable. Refusing keeps approve and lint in agreement about a
	// finished row instead of having each repair what the other refuses.
	if status := task.Status(); status == domain.TaskStatusCompleted || status == domain.TaskStatusAborted {
		terminalErr := errors.Errorf(
			ctx,
			"refusing to advance %q: it carries an approval record at phase %q but its status is %q; advancing the phase would produce the status/phase pair checkPhaseRegression refuses. A finished row is not re-opened by an approval — decide deliberately whether it is mis-phased",
			taskName, string(domain.TaskPhaseTodo), status,
		)
		return MutationResult{Success: false, Error: terminalErr.Error()}, terminalErr
	}
	task.SetPhase(domain.TaskPhasePlanning.Ptr())
	// The re-approval is an event with a time of its own: record it in a
	// separate key, beside the preserved approval record, in the same single
	// write. A first approval never writes this key, so a row that carries it
	// was re-approved and the value is when.
	task.Set("reapproved_at", now.Time())
	return o.write(ctx, task, vaultName)
}

// approvalSubjects returns the task's manager subjects in precedence order:
// a normalised `managed_by` first, then each normalised `goals:` entry. Empty
// entries are dropped, so a malformed link never becomes a subject.
func approvalSubjects(task *domain.Task) []string {
	var subjects []string
	if managedBy := normaliseGoalLink(task.GetString("managed_by")); managedBy != "" {
		subjects = append(subjects, managedBy)
	}
	for _, goal := range task.Goals() {
		if name := normaliseGoalLink(goal); name != "" {
			subjects = append(subjects, name)
		}
	}
	return subjects
}

// resolveOwner fixes the task's owner by the precedence the approval contract
// states: the --assignee flag, else the task's own assignee (preserved as it
// is), else the configured current user. It returns a refusal error when none
// of the three names an owner, so the caller writes nothing.
func resolveOwner(
	ctx context.Context,
	taskName string,
	task *domain.Task,
	assignee string,
	currentUser string,
) (string, error) {
	owner := assignee
	if owner == "" {
		owner = task.Assignee()
	}
	if owner == "" {
		owner = currentUser
	}
	if owner == "" {
		return "", errors.Errorf(
			ctx,
			"assignee must not be empty: task %q has no assignee, no --assignee was given, and no current_user is configured; refusing to approve an unowned task",
			taskName,
		)
	}
	return owner, nil
}
