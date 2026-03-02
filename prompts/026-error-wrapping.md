
<objective>
Adopt `github.com/bborbe/errors` for error wrapping across `pkg/ops/` and `pkg/storage/`.
Replace `fmt.Errorf("msg: %w", err)` with `errors.Wrap(ctx, err, "msg")` per YOLO go-patterns.md.
</objective>

<context>
Go CLI project at ~/Documents/workspaces/vault-cli.
Read CLAUDE.md for project conventions.
Read ~/.claude/docs/go-patterns.md — error wrapping section.

Currently all errors use `fmt.Errorf("...: %w", err)`. The guide requires:
```go
import "github.com/bborbe/errors"
return errors.Wrap(ctx, err, "operation failed")
```

`github.com/bborbe/errors` is NOT yet in go.mod — must be added.
</context>

<requirements>
1. Add dependency: `go get github.com/bborbe/errors`

2. Replace in `pkg/ops/*.go` (all files, excluding test files):
   - `fmt.Errorf("msg: %w", err)` → `errors.Wrap(ctx, err, "msg")`
   - Keep `fmt.Errorf` only for errors that don't wrap another error (e.g., `fmt.Errorf("unknown field: %s", key)`)
   - Ensure `ctx context.Context` is available in each function (it is — all Execute methods receive ctx)

3. Replace in `pkg/storage/markdown.go`:
   - Same pattern — wrap with ctx where ctx is available
   - For private functions without ctx parameter: keep `fmt.Errorf` or thread ctx through if simple

4. Update imports: replace `"fmt"` with `"github.com/bborbe/errors"` where fmt is only used for Errorf.
   Keep `"fmt"` if it's also used for Sprintf/Println/etc.

5. Run `go mod tidy` after adding dependency.
</requirements>

<constraints>
- Only replace wrapping errors (those with `%w`) — not format strings without wrapping
- Do NOT change error messages — only the wrapping mechanism
- All existing tests must continue to pass
- Do NOT run make precommit iteratively — use make test; run make precommit once at the end
</constraints>

<verification>
Run: `go mod tidy`
Run: `make test`
Run: `make precommit`
</verification>

<success_criteria>
- github.com/bborbe/errors in go.mod
- make test passes
- make precommit passes
- No `fmt.Errorf("...: %w", err)` patterns remain in pkg/ops/ or pkg/storage/
- All error wrapping uses errors.Wrap(ctx, err, "msg")
</success_criteria>
