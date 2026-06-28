---
status: approved
created: "2026-06-28T22:00:00Z"
queued: "2026-06-28T20:59:31Z"
---

<summary>
- `pkg/ops/errors.go` sentinel uses `errors.New(context.Background(), ...)` — should use `stderrors.New` (stdlib) without context
- 5 domain type files use `fmt.Errorf("%w: ...", validation.Error, ...)` in `Validate` methods — wrong for two reasons: `fmt.Errorf` doesn't stack-trace wrap, and `github.com/bborbe/errors.Errorf` doesn't support `%w` (see go-error-wrapping-guide.md)
- Correct pattern: `errors.Wrapf(ctx, validation.Error, "unknown X status '%s'", s)` as already established in `pkg/domain/task_phase.go:64`
- `pkg/cli/cli.go` user-facing errors stay as `fmt.Errorf` — correct convention for user messages
</summary>

<objective>
Fix inconsistent error construction patterns in domain validation: replace `fmt.Errorf("%w: ...", ...)` with `errors.Wrapf(ctx, ...)` for stack trace preservation and correct error wrapping, and replace the sentinel error to use standard-library `stderrors.New` without context.
</objective>

<context>
Read:
- `pkg/ops/errors.go` — `ErrStarterUnavailable` at line 17 uses `errors.New(context.Background(), "...")` — needs `stderrors.New` instead
- `pkg/domain/goal.go` — `GoalStatus.Validate` at line 88 uses `fmt.Errorf("%w: unknown goal status '%s'", validation.Error, s)`
- `pkg/domain/theme.go` — `ThemeStatus.Validate` at line 61 uses the same `fmt.Errorf("%w: ...")` pattern
- `pkg/domain/vision.go` — `VisionStatus.Validate` at line 61 uses the same pattern
- `pkg/domain/objective.go` — `ObjectiveStatus.Validate` at line 61 uses the same pattern
- `pkg/domain/task_status.go` — `TaskStatus.Validate` at line 64 uses the same pattern (same sibling, must be fixed too)
- `pkg/domain/task_phase.go` — at line 64, the established correct pattern: `errors.Wrapf(ctx, validation.Error, "unknown task phase '%s'", t)`
- `pkg/cli/cli.go` — `resolveSessionMode` at line 386 uses `fmt.Errorf` for user-facing error (this stays)
- `~/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — project conventions for error wrapping and sentinels
</context>

<requirements>
1. **Fix sentinel error in `pkg/ops/errors.go`**:
   - Replace `var ErrStarterUnavailable = errors.New(context.Background(), "...")`
   - With `var ErrStarterUnavailable = stderrors.New("...")` — import `"errors"` aliased as `stderrors`, matching the convention in `go-error-wrapping-guide.md` (sentinel errors use stdlib to avoid collision with `github.com/bborbe/errors`)
   - Remove the `"context"` and `"github.com/bborbe/errors"` imports if they become unused

2. **Fix `pkg/domain/goal.go`**:
   - `GoalStatus.Validate` at line 88: change `fmt.Errorf("%w: unknown goal status '%s'", validation.Error, s)` to `errors.Wrapf(ctx, validation.Error, "unknown goal status '%s'", s)`
   - Add `"github.com/bborbe/errors"` import
   - Remove `"fmt"` import if it becomes unused

3. **Fix `pkg/domain/theme.go`**:
   - Same fix: `fmt.Errorf` → `errors.Wrapf(ctx, validation.Error, ...)`

4. **Fix `pkg/domain/vision.go`**:
   - Same fix.

5. **Fix `pkg/domain/objective.go`**:
   - Same fix.

6. **Fix `pkg/domain/task_status.go`**:
   - `TaskStatus.Validate` at line 64: same fix — `errors.Wrapf(ctx, validation.Error, "unknown task status '%s'", s)`
   - Add `"github.com/bborbe/errors"` import
   - Remove `"fmt"` import if it becomes unused

7. **Do NOT change `pkg/cli/cli.go`** — user-facing errors with `fmt.Errorf` are the correct convention there

8. **Existing tests must still pass** — run `make precommit`
</requirements>

<constraints>
- MUST use `errors.Wrapf(ctx, validation.Error, ...)` — NOT `errors.Errorf(ctx, "%w: ...")` — because `github.com/bborbe/errors.Errorf` delegates to `github.com/pkg/errors.Errorf` which uses `fmt.Sprintf` internally and does NOT support `%w` wrapping
- Keep `validation.Error` wrapping intact in all `Validate` methods — `errors.Wrapf` preserves it and `errors.Is` will still work
- Only import cleanup: remove unused `"fmt"` when replaced, add `"github.com/bborbe/errors"`
- `errors.Wrapf` in domain files uses the existing `ctx` parameter from the `Validate` method signature
- Refer to `pkg/domain/task_phase.go:64` for the canonical correct pattern
</constraints>

<verification>
Run `make precommit` — must pass.
</verification>
