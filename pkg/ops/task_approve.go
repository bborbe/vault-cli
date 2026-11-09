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

// DefaultMaxApprovedNotStarted is the admission cap for task approval: the
// number of approved rows still on the board at or above which `task approve`
// refuses to create another approved-but-unstarted row. It is 110, the same
// operator-attention WIP limit used elsewhere — deliberately not a second,
// independently chosen number.
//
// The count it bounds is a deliberate conservative superset of the strictly
// unstarted set: it counts every non-terminal row carrying `approved_at`,
// including rows already holding a live session. It is therefore not the same
// instrument as claude-supervisor/scripts/approved-not-started.py, which also
// excludes rows with a live session; this cap is intentionally the wider of the
// two so the intake cannot refill a queue that is already over its limit.
const DefaultMaxApprovedNotStarted = 110

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
	// A new approval is also refused, with nothing written, when the vault's
	// count of approved rows still on the board (non-terminal rows carrying
	// `approved_at`) is at or above the admission cap. The count is a
	// conservative superset of the strictly-unstarted set, because it includes
	// rows already holding a live session. The cap does not gate the
	// already-approved advance path above, which writes no approval.
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
//
// maxApprovedNotStarted is the admission cap; a value of 0 means
// DefaultMaxApprovedNotStarted. The sentinel exists so a zero-valued
// construction can never admit everything: the struct always holds a positive
// cap.
func NewTaskApproveOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
	maxApprovedNotStarted int,
) TaskApproveOperation {
	if maxApprovedNotStarted == 0 {
		maxApprovedNotStarted = DefaultMaxApprovedNotStarted
	}
	return &taskApproveOperation{
		taskStorage:           taskStorage,
		currentDateTime:       currentDateTime,
		maxApprovedNotStarted: maxApprovedNotStarted,
	}
}

type taskApproveOperation struct {
	taskStorage           storage.TaskStorage
	currentDateTime       libtime.CurrentDateTime
	maxApprovedNotStarted int
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
	// rule governs the approval act, which already happened.
	if hasApprovalRecord(task) {
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
		return o.write(ctx, task, vaultName)
	}

	// Resolve the owner after every guard and before any mutation. Refusing here
	// — and only here — keeps the four earlier refusals' messages intact while
	// making approval the one moment an owner is fixed.
	owner, err := resolveOwner(ctx, taskName, task, assignee, currentUser)
	if err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// The admission cap: refuse a new approval when the vault already holds at
	// or above the cap of approved rows still on the board. This sits after
	// every guard that writes nothing and after the already-approved advance
	// path has returned, so an already-approved row is never gated.
	if err := o.refuseAdmissionOverCap(ctx, vaultPath, vaultName, taskName); err != nil {
		return MutationResult{Success: false, Error: err.Error()}, err
	}

	// All three keys are composed onto the existing map, then handed to storage
	// in exactly one write — so a row can never end up planned without an
	// approval record beside it. Every other key, known or unknown, survives
	// because the map itself is preserved.
	task.SetPhase(domain.TaskPhasePlanning.Ptr())
	task.Set("approved_by", approvedBy)
	task.Set("approved_at", now.Time())
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

// refuseAdmissionOverCap refuses a new approval when the vault's count of
// approved rows still on the board is at or above the admission cap.
//
// The count is every task whose `approved_at` is present and non-empty and
// whose status is neither completed nor aborted. The date is read with GetTime
// — the project's date-shaped accessor — rather than a Get type assertion, so
// the quoted-string and libtime.DateOrDateTime shapes a real task file decodes
// to are counted rather than silently dropped.
//
// The count is a deliberate conservative superset of
// claude-supervisor/scripts/approved-not-started.py, which additionally
// excludes rows holding a live session; the two are not one instrument.
//
// It fails closed: an unreadable task list refuses, because a silent undercount
// would admit exactly the rows the cap exists to refuse.
func (o *taskApproveOperation) refuseAdmissionOverCap(
	ctx context.Context,
	vaultPath string,
	vaultName string,
	taskName string,
) error {
	tasks, err := o.taskStorage.ListTasksStrict(ctx, vaultPath)
	if err != nil {
		return errors.Wrapf(
			ctx,
			err,
			"count approved rows: the approved-row count could not be read; refusing to approve %q in vault %q",
			taskName,
			vaultName,
		)
	}

	count := 0
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.GetTime("approved_at") == nil {
			continue
		}
		status := task.Status()
		if status == domain.TaskStatusCompleted || status == domain.TaskStatusAborted {
			continue
		}
		count++
	}

	if count >= o.maxApprovedNotStarted {
		//nolint:revive // the refusal message shape is part of the admission-cap contract
		return errors.Errorf(
			ctx,
			"refusing to approve %q in vault %q: the vault's count of approved rows still on the board is %d (a conservative superset of the unstarted set — it includes rows already holding a live session), at or above the admission cap of %d; no approval was written. Drain the queue, or raise the cap in code deliberately.",
			taskName,
			vaultName,
			count,
			o.maxApprovedNotStarted,
		)
	}
	return nil
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
