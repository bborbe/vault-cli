// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ops

import (
	"context"
	"strings"

	"github.com/bborbe/errors"
	libtime "github.com/bborbe/time"
	"github.com/bborbe/validation"

	"github.com/bborbe/vault-cli/pkg/domain"
	"github.com/bborbe/vault-cli/pkg/storage"
)

//counterfeiter:generate -o ../../mocks/frontmatter-get-operation.go --fake-name FrontmatterGetOperation . FrontmatterGetOperation
type FrontmatterGetOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) (string, error)
}

// NewFrontmatterGetOperation creates a new frontmatter get operation.
func NewFrontmatterGetOperation(taskStorage storage.TaskStorage) FrontmatterGetOperation {
	return &frontmatterGetOperation{
		taskStorage: taskStorage,
	}
}

type frontmatterGetOperation struct {
	taskStorage storage.TaskStorage
}

// Execute retrieves the value of a frontmatter field from a task.
func (o *frontmatterGetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) (string, error) {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return "", errors.Wrap(ctx, err, "find task")
	}

	return task.GetField(key), nil
}

//counterfeiter:generate -o ../../mocks/frontmatter-set-operation.go --fake-name FrontmatterSetOperation . FrontmatterSetOperation
type FrontmatterSetOperation interface {
	// Execute sets one frontmatter field on a task.
	//
	// actor names who is performing the write. It is consulted only for the
	// "flag" key, where it is recorded as flag_set_by in the same write that sets
	// the flag; every other key ignores it. A truthy flag write with a blank
	// actor is refused with nothing written, because a present-but-blank
	// flag_set_by is worse than no record at all.
	Execute(ctx context.Context, vaultPath, taskName, key, value, reason, gateSuccessor, actor string, force bool) error
}

// NewFrontmatterSetOperation creates a new frontmatter set operation.
func NewFrontmatterSetOperation(
	taskStorage storage.TaskStorage,
	currentDateTime libtime.CurrentDateTime,
	publisher EscalationPublisher,
	vaultName, tasksDir string,
) FrontmatterSetOperation {
	return &frontmatterSetOperation{
		taskStorage:     taskStorage,
		currentDateTime: currentDateTime,
		publisher:       publisher,
		vaultName:       vaultName,
		tasksDir:        tasksDir,
	}
}

type frontmatterSetOperation struct {
	taskStorage     storage.TaskStorage
	currentDateTime libtime.CurrentDateTime
	publisher       EscalationPublisher
	vaultName       string
	tasksDir        string
}

// Execute sets the value of a frontmatter field on a task.
func (o *frontmatterSetOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key, value, reason, gateSuccessor, actor string,
	force bool,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// Refuse a non-empty blocked_by before anything is mutated: `set` has no
	// list form for this field, and writing the scalar would record a dependency
	// every reader discards. Nothing below runs on this path, so the file stays
	// byte-identical. --force does not bypass this — it is scoped to the phase
	// guard, and the refusal is deliberately absolute (spec 050 Non-goal).
	if err := blockedBySetRefusal(ctx, "task", taskName, key, value); err != nil {
		return err
	}

	// Refuse metrics_sessions before anything is mutated — see
	// metricsSessionsWriteRefusal. Nothing below runs on this path, so the task
	// file stays byte-identical, and --force does not bypass the refusal.
	if err := metricsSessionsWriteRefusal(ctx, taskName, key); err != nil {
		return err
	}

	// Refuse the unrecorded exit from the approval inbox before anything is
	// mutated — see todoPlanningSetRefusal. Nothing below runs on this path, so
	// the task file stays byte-identical, and --force does not bypass the
	// refusal.
	if err := todoPlanningSetRefusal(ctx, task, taskName, key, value); err != nil {
		return err
	}

	// Refuse a direct provenance write before anything is mutated — see
	// flagProvenanceSetRefusal. Nothing below runs on this path, so the task file
	// stays byte-identical.
	if err := flagProvenanceSetRefusal(ctx, taskName, key); err != nil {
		return err
	}

	// Flag provenance is composed onto the same map as the flag itself, so the
	// single WriteTask below persists all three keys together and no reader can
	// observe a flag without its writer. The refusal for a blank actor leaves the
	// file byte-identical because it runs before any mutation.
	if key == "flag" {
		if err := o.writeFlagProvenance(ctx, task, taskName, value, actor); err != nil {
			return err
		}
	}

	// Read the assignee before the mutation. `task set <task> assignee ""` leaves
	// the key present and empty, so a read taken after the write can no longer
	// tell a cleared assignee from one that was never set.
	previousAssignee := task.Assignee()

	// One-step close-out: when this invocation sets a close-out status, persist
	// reason and successor first so both land in a single WriteTask. Fields are
	// written only when provided; non-close-out targets never receive them.
	if err := writeTaskCloseOutFieldsIfCloseOut(ctx, task, value, reason, gateSuccessor); err != nil {
		return err
	}

	// Phase-regression guard: an in-progress task may not silently move
	// backward out of execution into todo (observed 2026-09-02: a bulk
	// `task set phase todo` loop regressed 80 active tasks). --force overrides.
	if err := checkPhaseRegression(ctx, task, key, value, force); err != nil {
		return err
	}

	if err := task.SetField(ctx, key, value); err != nil {
		if key == "status" && strings.Contains(err.Error(), "missing close-out field(s)") {
			err = errors.Errorf(ctx,
				"%s\nTry: vault-cli task set \"%s\" status %s --reason \"<text>\" --gate-successor \"<successor|none>\"",
				err.Error(), taskName, value)
		}
		return errors.Wrap(ctx, err, "set field")
	}

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, value, previousAssignee, o.vaultName, o.tasksDir)

	return nil
}

// writeTaskCloseOutFieldsIfCloseOut writes the one-step close-out fields when
// the target status is a close-out (aborted or completed), returning the first
// write error. Fields are written only when provided; non-close-out targets
// never receive them.
func writeTaskCloseOutFieldsIfCloseOut(
	ctx context.Context,
	task *domain.Task,
	value, reason, gateSuccessor string,
) error {
	target, ok := domain.NormalizeTaskStatus(value)
	if !ok || (target != domain.TaskStatusAborted && target != domain.TaskStatusCompleted) {
		return nil
	}
	if reason != "" {
		if err := task.SetField(ctx, "aborted_reason", reason); err != nil {
			return errors.Wrap(ctx, err, "set aborted_reason")
		}
	}
	if gateSuccessor != "" {
		if err := task.SetField(ctx, "gate_successor", gateSuccessor); err != nil {
			return errors.Wrap(ctx, err, "set gate_successor")
		}
	}
	return nil
}

// writeFlagProvenance records who set the flag and when, on the same map the
// flag itself is written to — so the caller's single WriteTask persists the
// flag and its provenance together and no read can observe an unattributed
// flag. The value is normalised exactly as setFlagField normalises it
// (case-insensitive, surrounding whitespace trimmed), so `set flag TRUE` and
// `set flag yes` write provenance just as `set flag true` does.
//
// Three outcomes, mirroring the field contract:
//   - truthy: flag_set_by is the actor and flag_set_at is the injected instant,
//     written as a bare time.Time so yaml.v3 renders it unquoted. A blank actor
//     is refused here, before any mutation, because flag_set_by: "" is worse
//     than no record at all.
//   - falsy: flag_set_by and flag_set_at are deleted. The flag key itself is
//     left to SetField, which stores flag: false — a `flag: false` row has no
//     writer to attribute, so only the provenance keys go.
//   - empty: nothing is done here. SetField clears the flag, and ClearFlag
//     removes the two provenance keys with it.
//
// A value outside those three sets is left alone; SetField reports it as an
// invalid flag value and nothing is written.
func (o *frontmatterSetOperation) writeFlagProvenance(
	ctx context.Context,
	task *domain.Task,
	taskName, value, actor string,
) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes":
		if actor == "" {
			return errors.Errorf(
				ctx,
				"flag_set_by must not be empty: refusing to record a blank actor on task %q",
				taskName,
			)
		}
		task.Set("flag_set_by", actor)
		task.Set("flag_set_at", o.currentDateTime.Now().Time())
	case "false", "no":
		task.Delete("flag_set_by")
		task.Delete("flag_set_at")
	}
	return nil
}

// flagProvenanceSetRefusal returns an error when a `set` invocation would write
// flag_set_by or flag_set_at directly, and nil for every other key.
//
// The two keys exist only as evidence of the write that set the flag: they are
// composed onto the same map, in the same WriteTask, as `flag` itself. A direct
// `task set <name> flag_set_by <actor>` would attach provenance to a flag that
// is already on disk, and the window between the two writes is exactly the
// unattributed-flag bypass the field exists to close. The refusal is absolute:
// --force is scoped to the phase guard and does not bypass it, and nothing is
// written on this path because the check runs before any mutation.
func flagProvenanceSetRefusal(ctx context.Context, taskName, key string) error {
	if key != "flag_set_by" && key != "flag_set_at" {
		return nil
	}
	return errors.Errorf(
		ctx,
		"refusing to set %q on %q: %s is written only by the same invocation that sets the flag; use `vault-cli task set %q flag true --by <actor>`",
		key, taskName, key, taskName,
	)
}

//counterfeiter:generate -o ../../mocks/frontmatter-clear-operation.go --fake-name FrontmatterClearOperation . FrontmatterClearOperation
type FrontmatterClearOperation interface {
	Execute(ctx context.Context, vaultPath, taskName, key string) error
}

// NewFrontmatterClearOperation creates a new frontmatter clear operation.
func NewFrontmatterClearOperation(
	taskStorage storage.TaskStorage,
	publisher EscalationPublisher,
	vaultName, tasksDir string,
) FrontmatterClearOperation {
	return &frontmatterClearOperation{
		taskStorage: taskStorage,
		publisher:   publisher,
		vaultName:   vaultName,
		tasksDir:    tasksDir,
	}
}

type frontmatterClearOperation struct {
	taskStorage storage.TaskStorage
	publisher   EscalationPublisher
	vaultName   string
	tasksDir    string
}

// Execute clears (removes) the value of a frontmatter field on a task.
func (o *frontmatterClearOperation) Execute(
	ctx context.Context,
	vaultPath, taskName, key string,
) error {
	task, err := o.taskStorage.FindTaskByName(ctx, vaultPath, taskName)
	if err != nil {
		return errors.Wrap(ctx, err, "find task")
	}

	// Read the assignee before the deletion. ClearField removes the key outright,
	// so a read taken after the write always yields "".
	previousAssignee := task.Assignee()

	task.ClearField(key)

	if err := o.taskStorage.WriteTask(ctx, task); err != nil {
		return errors.Wrap(ctx, err, "write task")
	}

	publishAssigneeClearEscalation(ctx, o.publisher, task, key, "", previousAssignee, o.vaultName, o.tasksDir)

	return nil
}

// checkPhaseRegression rejects a backward phase write on a task that has already
// moved past that phase. Such a write silently regresses the lifecycle: a human
// or agent reader treats the task as un-started and may re-open subtasks or
// re-ask answered questions on work that already shipped.
//
// Two observed incidents motivated the two halves of this guard:
//   - 2026-09-02: a bulk `task set phase todo` loop regressed 80 active tasks in
//     the Personal vault.
//   - 2026-09-19: vault-ui's PATCH /api/tasks/{id}/phase shells out to
//     `vault-cli task set <id> phase <value>`, so a board drag back to the
//     planning column regressed six finished Personal-vault tasks in a single
//     autocommit (bd76e6b4b1). The original guard rejected only a `todo` target,
//     so `planning` passed unguarded.
//
// A deliberate reset must pass force=true. A forward move (`planning` -> `execution`) is
// never a regression and always passes.
// `todo` -> `planning` is likewise not a regression and passes here; it is refused
// earlier, by todoPlanningSetRefusal, because leaving the approval inbox without a
// record is a different defect from a backward move.
func checkPhaseRegression(ctx context.Context, task *domain.Task, key, value string, force bool) error {
	if force || key != "phase" {
		return nil
	}
	canonical, ok := domain.NormalizeTaskPhase(value)
	if !ok {
		return nil
	}

	// A completed task is terminal. Moving its phase back to planning re-opens
	// finished work without touching its status, which is the state a reader
	// trusts least: `status: completed` beside `phase: planning`.
	if task.Status() == domain.TaskStatusCompleted && canonical == domain.TaskPhasePlanning {
		return errors.Wrapf(ctx, validation.Error,
			"refusing to set phase %q on %q: task is completed; this regression would silently re-open finished work. Pass --force to override",
			canonical, task.Name)
	}

	if task.Status() != domain.TaskStatusInProgress {
		return nil
	}
	if canonical != domain.TaskPhaseTodo && canonical != domain.TaskPhasePlanning {
		return nil
	}
	current := task.Phase()
	if current == nil {
		return nil
	}
	switch *current {
	case domain.TaskPhaseExecution, domain.TaskPhaseAIReview, domain.TaskPhaseHumanReview:
		return errors.Wrapf(ctx, validation.Error,
			"refusing to set phase %q on %q: task is in_progress with phase %q; this regression would silently stall the execution pipeline. Pass --force to override",
			canonical, task.Name, *current)
	}
	return nil
}

// todoPlanningSetRefusal returns an error when a `set` invocation would move a task
// out of the operator's approval inbox by setting the phase to planning.
//
// "todo" is the operator's approval inbox. Leaving it is the operator's approval and
// is performed by `vault-cli task approve`, which records approved_by and approved_at
// in the same write. `task set <name> phase planning` writes the field and nothing
// else, and checkPhaseRegression guards only backward moves, so this forward move
// passes it — leaving the row at planning with no evidence that anyone approved it.
// The refusal closes that route.
//
// The guard is deliberately one (field, value, current-phase) combination: key
// "phase", the canonical value "planning", and a current phase of "todo". It is not a
// general phase gate — a row at any other phase, a missing or empty phase key, an
// unknown phase value and every non-phase key keep their existing behaviour, and
// status is never consulted. --force does not bypass it: --force is scoped to a
// backward phase move on a row already past planning, and this refusal exists because
// the unrecorded row is the defect rather than a warning to be overridden.
//
// Nothing is written on the refusal path: the check runs before any mutation, so the
// file on disk is byte-identical.
func todoPlanningSetRefusal(ctx context.Context, task *domain.Task, taskName, key, value string) error {
	if key != "phase" {
		return nil
	}
	canonical, ok := domain.NormalizeTaskPhase(value)
	if !ok || canonical != domain.TaskPhasePlanning {
		return nil
	}
	current := task.Phase()
	if current == nil || *current != domain.TaskPhaseTodo {
		return nil
	}
	return errors.Errorf(ctx,
		"refusing to set phase %q on %q: the task is at phase %q, the operator's approval inbox. Leaving it is the operator's approval and is performed by `vault-cli task approve %q`, which records approved_by and approved_at in the same write",
		canonical, taskName, string(domain.TaskPhaseTodo), taskName,
	)
}

// publishAssigneeClearEscalation emits one agent-escalation notification when a
// frontmatter write cleared a non-empty assignee. It is the single transition
// rule both clear paths share.
//
// previousAssignee MUST be read before the mutation: `task set <task> assignee ""`
// leaves the key present and empty while `task clear <task> assignee` deletes it,
// so a read taken after the mutation yields "" on both paths and the notification
// would lose the one value it exists to carry. value is always "" on the clear
// path, which has no value argument. vaultName and tasksDir are the vault's
// configured identity, carried so the body can render the same Obsidian link the
// agent-side peer renders.
func publishAssigneeClearEscalation(
	ctx context.Context,
	publisher EscalationPublisher,
	task *domain.Task,
	key, value, previousAssignee, vaultName, tasksDir string,
) {
	if key != "assignee" || value != "" || previousAssignee == "" {
		return
	}
	// An absent phase renders as the empty string, exactly as the peer renders
	// it: the two producers must agree on the body for a task with no phase, so
	// this is not defaulted, not skipped, and not replaced by a placeholder.
	escalatedPhase := ""
	if p := task.Phase(); p != nil {
		escalatedPhase = string(*p)
	}
	publisher.PublishEscalation(ctx, Escalation{
		TaskIdentifier:   task.TaskIdentifier(),
		TaskName:         task.Name,
		PreviousAssignee: previousAssignee,
		Status:           string(task.Status()),
		Phase:            escalatedPhase,
		VaultName:        vaultName,
		TasksDir:         tasksDir,
	})
}
