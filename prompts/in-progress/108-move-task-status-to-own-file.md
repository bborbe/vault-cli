---
status: committing
summary: Moved TaskStatus type, constants, helpers, and related functions from pkg/domain/task.go into new file pkg/domain/task_status.go, mirroring the task_phase.go pattern; pruned imports from task.go; added CHANGELOG Unreleased entry.
container: vault-cli-108-move-task-status-to-own-file
dark-factory-version: v0.151.2-4-g3dc5753
created: "2026-05-06T20:55:00Z"
queued: "2026-05-06T19:57:41Z"
started: "2026-05-06T19:58:19Z"
---

<summary>
- All `TaskStatus`-related code (type definition, constants, `AvailableTaskStatuses`, `TaskStatuses` slice helper, methods, `IsValidTaskStatus`, `NormalizeTaskStatus`) moves from `pkg/domain/task.go` into a new file `pkg/domain/task_status.go`
- Mirrors the existing layout for `TaskPhase` which already lives in its own file `pkg/domain/task_phase.go`
- `pkg/domain/task.go` keeps the `Task` struct, `NewTask`, and `TaskID`
- Imports in `task.go` are pruned (the moved code was the only consumer of `context`, `fmt`, and `github.com/bborbe/validation` in that file)
- No public API change; tests in `pkg/domain/task_status_test.go` continue to pass unchanged
- No semantic behavior change
</summary>

<objective>
Split `pkg/domain/task.go` so each domain enum lives in its own file (the codebase already does this for `TaskPhase`). Pure mechanical refactor — no public API change, no behavior change.
</objective>

<context>
Read CLAUDE.md for project conventions.

Files to read fully before making changes:
- `pkg/domain/task.go` — current home of `TaskStatus*` (the code to move) AND `Task`, `NewTask`, `TaskID` (which stay)
- `pkg/domain/task_phase.go` — canonical pattern to mirror for the new file (file header, package decl, imports, doc comments)
- `pkg/domain/task_status_test.go` — already exists; should keep passing without changes

The exact ranges in `pkg/domain/task.go` to move (per the file at HEAD):

| Lines | Symbol | Action |
|---|---|---|
| 35–51 | `TaskStatus` type + 6 const values | move |
| 53–61 | `AvailableTaskStatuses` var | move |
| 63–69 | `TaskStatuses` type + `Contains` method | move |
| 71–87 | `String`, `Validate`, `Ptr` methods on `TaskStatus` | move |
| 96–99 | `IsValidTaskStatus` function | move |
| 101–123 | `NormalizeTaskStatus` function (incl. migration map) | move |

What stays in `pkg/domain/task.go`:
- File header (lines 1–4) + `package domain`
- `Task` struct + `NewTask` (lines 15–33)
- `TaskID` type + `String` method (lines 89–94)

After the move, `pkg/domain/task.go`'s remaining code only uses `TaskFrontmatter`, `FileMetadata`, `Content` — no `context`, no `fmt`, no `validation`. Imports must be pruned accordingly.
</context>

<requirements>
**Execute steps in order. Run `make precommit` only at the final step.**

1. **Create `pkg/domain/task_status.go`** with the standard file header and `package domain`, then move the symbols listed in the table above into it. Preserve every doc comment verbatim.

   The new file's import block needs:
   ```go
   import (
       "context"
       "fmt"

       "github.com/bborbe/collection"
       "github.com/bborbe/validation"
   )
   ```

   Mirror `pkg/domain/task_phase.go` for: file-header copyright comment, blank line after package, import grouping (stdlib first, then third-party).

2. **Edit `pkg/domain/task.go`** to:
   - Delete the moved code blocks (lines 35–51, 53–61, 63–69, 71–87, 96–99, 101–123 per the table above)
   - Prune imports: remove `"context"`, `"fmt"`, `"github.com/bborbe/collection"`, and `"github.com/bborbe/validation"`. After the move, `task.go` references none of them. Verify with `grep -E 'context\.|fmt\.|collection\.|validation\.' pkg/domain/task.go` — expect zero hits.
   - Result: `task.go` defines only `Task`, `NewTask`, `TaskID`, `TaskID.String`. Roughly 30 lines.

3. **Verify symbols still resolve.** No symbol names change; consumers across the package and across the repo should compile without edits.

   ```bash
   cd ~/Documents/workspaces/vault-cli && go build ./...
   ```

4. **Run package tests.** `task_status_test.go` already exists; it should pass against the new file location with no edits.

   ```bash
   go test ./pkg/domain/... -count=1
   ```

5. **Run `make precommit`** in repo root:

   ```bash
   make precommit
   ```

6. **Add CHANGELOG entry.** `CHANGELOG.md` does not currently have an `## Unreleased` section — top entry is `## v0.58.1`. Insert a new `## Unreleased` section directly under the `# Changelog` header (above `## v0.58.1`) with this bullet:
   ```
   ## Unreleased

   - chore(domain): move `TaskStatus` and helpers to `pkg/domain/task_status.go` (mirrors `task_phase.go`); pure refactor, no API change
   ```
</requirements>

<constraints>
- Only edit `pkg/domain/task.go`, create `pkg/domain/task_status.go`, and update `CHANGELOG.md`
- Do NOT commit — dark-factory handles git
- Do NOT rename any symbol — public API stays byte-identical
- Do NOT change any doc comment text — copy verbatim
- Do NOT change the migration map in `NormalizeTaskStatus`
- Do NOT touch `pkg/domain/task_phase.go` — it's the model, not a target
- Do NOT touch `pkg/domain/task_status_test.go` — it should pass unchanged after the move
- Existing imports outside this file MUST stay valid (the symbols' fully-qualified names don't change because the file is in the same `domain` package)
- `make precommit` runs from repo root, must exit 0
</constraints>

<verification>
make precommit

# Confirm the new file exists with the moved symbols:
grep -E "^(type TaskStatus|const|var AvailableTaskStatuses|type TaskStatuses|func.*TaskStatus|func IsValidTaskStatus|func NormalizeTaskStatus)" pkg/domain/task_status.go | head -20

# Confirm task.go no longer carries the moved symbols:
grep -E "TaskStatus|AvailableTaskStatuses|IsValidTaskStatus|NormalizeTaskStatus" pkg/domain/task.go
# Expected: zero matches

# Confirm task.go's imports are pruned:
grep -E '"context"|"fmt"|"github.com/bborbe/validation"' pkg/domain/task.go
# Expected: zero matches

# Confirm public API unchanged from outside the package:
go build ./...
# Expected: exit 0

# Confirm existing test still passes:
go test ./pkg/domain/... -run TestTaskStatus -count=1
# Expected: PASS
</verification>
