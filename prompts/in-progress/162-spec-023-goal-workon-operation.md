---
status: approved
spec: [023-goal-work-on]
created: "2026-07-11T08:41:00Z"
queued: "2026-07-11T08:29:33Z"
---

<summary>
- Adds the core library operation behind `vault-cli goal work-on`: it finds a goal, marks it `in_progress`, applies the same assignee-ownership rule as tasks, writes the goal, then starts (or resumes/returns) a Claude session and records the session id on the goal.
- Mirrors `task work-on` in every respect except the two steps goals do not have: no daily-note update and no phase advancement.
- A goal that already has a session id short-circuits — no new session is minted, the cached id is returned.
- A missing `claude` binary is a soft failure (warning, still succeeds); a zero-turn / rejected Claude run is a hard failure (non-zero, goal stays `in_progress`).
- Ships a Counterfeiter mock for the new operation interface plus a full Ginkgo unit suite; no stdout writes in this layer.
- `make precommit` passes.
</summary>

<objective>
Create `GoalWorkOnOperation` in `pkg/ops` — the structured library primitive that starts or resumes a Claude session for a named goal — mirroring `WorkOnOperation` minus the daily-note and phase steps. Return a `MutationResult`; write no output. Add its Counterfeiter mock and a unit suite. No CLI wiring in this prompt.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, Counterfeiter mocks, external `_test` package.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — `github.com/bborbe/errors` wrapping idiom.
Read `/home/node/.claude/plugins/marketplaces/coding/docs/go-factory-pattern.md` — pure factory functions (no conditionals / I-O in the constructor).

Read these files before implementing:
- `pkg/ops/workon.go` — the reference implementation to mirror. Copy the shape of `Execute`, `applyAssigneeMatrix`, and `handleClaudeSession`. Key differences for goals: NO `dailyNoteStorage`, NO `currentDateTime`, NO daily-note step, NO phase advancement. Specifically reuse the soft/hard-fail branching around `ErrStarterUnavailable` (lines 113-126) and the interactive-resume tail (lines 128-136) verbatim in shape.
- `pkg/ops/workon_test.go` — the reference test suite. Mirror the Context blocks that still apply (success/assignee-matrix/custom-command/nil-starter/cached-session/hard-fail/zero-turns/not-found/write-error/interactive), and DROP every "daily note" and "phase advancement" Context (goals have neither).
- `pkg/ops/complete.go` — `MutationResult` struct (lines 53-68); reuse it as-is (fields `Success`, `Name`, `Vault`, `Error`, `Warnings`, `SessionID`).
- `pkg/ops/errors.go` — `ErrStarterUnavailable` sentinel (matched with `errors.Is`).
- `pkg/ops/claude_session.go` — `ClaudeSessionStarter.StartSession(ctx, prompt, cwd, name) (string, error)`; the starter itself enforces the 5m timeout and returns the "claude returned 0 turns" / timeout errors. Do NOT modify.
- `pkg/ops/claude_resume.go` — `ClaudeResumer.ResumeSession(ctx, sessionID, cwd) error`. Do NOT modify.
- `pkg/storage/storage.go` (lines 58-60) — `GoalStorage` interface: `WriteGoal(ctx, *domain.Goal) error` and `FindGoalByName(ctx, vaultPath, name) (*domain.Goal, error)`.
- `pkg/domain/goal_frontmatter.go` — after prompt 1: `Assignee()`, `SetAssignee(v)`, `SetStatus(s GoalStatus) error` (validates — returns an error you MUST handle; note `workon.go:87` discards Task's with `_ =`, but here you must propagate it), `ClaudeSessionID()`, `SetClaudeSessionID(v)`. `Goal` embeds `GoalFrontmatter` and `FileMetadata` (which provides `.FilePath` and `.Name`).
- `pkg/domain/goal.go` — `GoalStatusInProgress GoalStatus = "in_progress"` (line ~50); `NewGoal(data, meta, content)`.
- `pkg/config/config.go` accessor from prompt 1: `vault.GetWorkOnGoalCommand()` (mirrors existing `GetWorkOnCommand` at ~L114).
- `mocks/goal-storage.go` (`mocks.GoalStorage`), `mocks/claude-session-starter.go` (`mocks.ClaudeSessionStarter`), `mocks/claude-resumer.go` (`mocks.ClaudeResumer`) — reuse existing mocks; do not regenerate them.

Depends on prompt 1 (`ClaudeSessionID`/`SetClaudeSessionID` on `GoalFrontmatter`, `GetWorkOnGoalCommand` on `Vault`) — that prompt lands first.
</context>

<requirements>
1. Create `pkg/ops/goal_workon.go` with the interface + counterfeiter directive + pure constructor + struct:
   ```go
   //counterfeiter:generate -o ../../mocks/goal-workon-operation.go --fake-name GoalWorkOnOperation . GoalWorkOnOperation
   type GoalWorkOnOperation interface {
       Execute(
           ctx context.Context,
           vaultPath string,
           goalName string,
           assignee string,
           vaultName string,
           isInteractive bool,
           sessionDir string,
           vault *config.Vault,
       ) (MutationResult, error)
   }

   func NewGoalWorkOnOperation(
       goalStorage storage.GoalStorage,
       starter ClaudeSessionStarter,
       resumer ClaudeResumer,
   ) GoalWorkOnOperation {
       return &goalWorkOnOperation{
           goalStorage: goalStorage,
           starter:     starter,
           resumer:     resumer,
       }
   }

   type goalWorkOnOperation struct {
       goalStorage storage.GoalStorage
       starter     ClaudeSessionStarter
       resumer     ClaudeResumer
   }
   ```
   Note the constructor omits `dailyNoteStorage` and `currentDateTime` — goals have neither step.

2. Implement `Execute` mirroring `workOnOperation.Execute` MINUS daily note and phase:
   1. `goal, err := w.goalStorage.FindGoalByName(ctx, vaultPath, goalName)` — on error return `MutationResult{Success: false, Error: err.Error()}` and `errors.Wrap(ctx, err, "find goal")`.
   2. `if err := goal.SetStatus(domain.GoalStatusInProgress); err != nil { return MutationResult{Success: false, Error: err.Error()}, errors.Wrap(ctx, err, "set goal status") }` (SetStatus validates and returns an error — do not discard it).
   3. Apply the assignee matrix via a new `applyGoalAssigneeMatrix(goal, assignee)` helper (requirement 3); append its non-empty return to `warnings`.
   4. `if err := w.goalStorage.WriteGoal(ctx, goal); err != nil { return MutationResult{Success:false, Error: err.Error()}, errors.Wrap(ctx, err, "write goal") }`.
   5. `sessionID, sessionErr := w.handleClaudeSession(ctx, goal, sessionDir, vault)` (requirement 4); on `sessionErr`:
      - `if errors.Is(sessionErr, ErrStarterUnavailable)`: soft failure — append `fmt.Sprintf("claude session: %v", sessionErr)` to warnings, `slog.Warn(...)`, continue.
      - else: hard failure — `return MutationResult{Success:false, Name: goal.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID, Error: sessionErr.Error()}, errors.Wrap(ctx, sessionErr, "start work-on session")`.
   6. Interactive resume tail: `if isInteractive && w.resumer != nil && sessionID != "" { return MutationResult{Success:true, Name: goal.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID}, w.resumer.ResumeSession(ctx, sessionID, sessionDir) }`.
   7. Final success: `return MutationResult{Success:true, Name: goal.Name, Vault: vaultName, Warnings: warnings, SessionID: sessionID}, nil`.

3. Add `applyGoalAssigneeMatrix(goal *domain.Goal, assignee string) string` mirroring `applyAssigneeMatrix` (workon.go lines 162-176): blank existing → `SetAssignee(assignee)`, return `""`; equals assignee → return `""`; different non-blank → return `fmt.Sprintf("assignee not updated: goal owned by %s (current user: %s)", existing, assignee)` and leave unchanged. Status was already set in step 2 regardless of the assignee outcome.

4. Add `handleClaudeSession(ctx, goal *domain.Goal, vaultPath string, vault *config.Vault) (string, error)` mirroring workon.go lines 179-205:
   - `if existing := goal.ClaudeSessionID(); existing != "" { return existing, nil }` (short-circuit — no starter call).
   - `if w.starter == nil { return "", ErrStarterUnavailable }`.
   - `prompt := fmt.Sprintf(`%s "%s" --non-interactive`, vault.GetWorkOnGoalCommand(), goal.FilePath)` — note `GetWorkOnGoalCommand()` (not the task variant) and the mandatory trailing ` --non-interactive`.
   - `slog.Info("starting claude session", "goal", goal.Name)`.
   - `sessionID, err := w.starter.StartSession(ctx, prompt, vaultPath, goal.Name)`; on err `return "", errors.Wrap(ctx, err, "start claude session")`.
   - `goal.SetClaudeSessionID(sessionID)`; `if err := w.goalStorage.WriteGoal(ctx, goal); err != nil { return sessionID, errors.Wrap(ctx, err, "save session id to goal") }`; `return sessionID, nil`.

5. Run `go generate ./...` (or `make generate`) to produce `mocks/goal-workon-operation.go`. Commit the generated file as part of the change set (dark-factory stages it).

6. Create `pkg/ops/goal_workon_test.go` (package `ops_test`) — a Ginkgo suite mirroring the applicable `workon_test.go` Contexts. Cover at minimum:
   - success: `FindGoalByName` called with the right args; written goal `Status()` == `domain.GoalStatusInProgress`; `Assignee()` == current user; `StartSession` called once; session name arg == goal name.
   - assignee equals current user → assignee preserved, no "assignee not updated" warning, still `in_progress`.
   - assignee different user → assignee preserved, warning containing "assignee not updated" and both usernames, still `in_progress`.
   - custom `WorkOnGoalCommand`: prompt matches `^/custom-cmd "`, ends with ` --non-interactive$`, and contains the goal file path.
   - starter nil + no cached id → no error, `StartSession` count 0, warning containing "unavailable", empty `SessionID`.
   - goal already has a session id → `StartSession` count 0, `SessionID` == cached id, no error, no warnings.
   - hard failure: `StartSession` returns an error → `Execute` returns non-nil error containing "start work-on session", `result.Success` == false.
   - zero turns: `StartSession` returns `errors.New(ctx, "claude returned 0 turns: ...")` → error contains "start work-on session" AND the zero-turns text, `Success` false, and the written goal is still `in_progress` (assert via `WriteGoalArgsForCall`).
   - interactive mode (`isInteractive = true`): `ResumeSession` called once with the returned session id and `sessionDir`.
   - goal not found: `FindGoalByName` returns error → `Execute` errors, `WriteGoal` count 0.
   - write error: `WriteGoal` returns error → `Execute` errors.
   Build the goal fixture with `domain.NewGoal(map[string]any{"status": "next"}, domain.FileMetadata{Name: goalName, FilePath: "/path/to/vault/Goals/my-goal.md"}, domain.Content(""))`. Use `mocks.GoalStorage`, `mocks.ClaudeSessionStarter`, `mocks.ClaudeResumer`. Reuse the shared `ErrTest` sentinel from the existing `ops_test` package.
</requirements>

<constraints>
- `pkg/ops` is a library layer — the operation returns a structured `MutationResult` and MUST NOT write to stdout/stderr (except `slog` diagnostics, as `workon.go` does). The CLI layer (prompt 3) owns all user-facing output.
- Reuse `FindGoalByName` / `WriteGoal` and `ClaudeSessionStarter` / `ClaudeResumer` unchanged — do NOT modify storage, the starter, the resumer, or `workon.go`.
- Do NOT add daily-note handling or phase advancement — goals have neither (spec Non-goals; hard veto).
- Do NOT add a per-goal opt-out / no-session flag — invariant (spec Non-goals; hard veto).
- The zero-turn / rejected-run hard-fail invariant MUST match `task work-on`: a zero-turn run is a non-nil error and must not be swallowed.
- Error handling: `github.com/bborbe/errors` wrapping with `ctx` everywhere; never `fmt.Errorf`; never bare `return err`; never `context.Background()` in non-test code.
- Factory function is pure composition — no conditionals or I/O in `NewGoalWorkOnOperation`.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass; `task work-on` behavior is unchanged.
</constraints>

<verification>
Run `go generate ./...` — regenerates `mocks/goal-workon-operation.go` with no unexpected diff.
Run `make precommit` — must pass (lint + format + generate + test + version checks).
Run `go test ./pkg/ops/` — the new suite and the existing `workon` suite both pass.
Run `test -f mocks/goal-workon-operation.go && echo OK` — prints OK.
Run `grep -n "GetWorkOnGoalCommand\|--non-interactive" pkg/ops/goal_workon.go` — ≥2 lines.
Run `grep -n "dailyNote\|Phase\|currentDateTime" pkg/ops/goal_workon.go` — 0 lines (confirms the excluded steps are absent).
</verification>
