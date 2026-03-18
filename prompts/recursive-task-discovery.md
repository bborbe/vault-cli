---
status: created
---

<summary>
- Task discovery (list, find, read) now searches the configured tasks directory and all its subdirectories recursively
- Tasks in subdirectories are found by ListTasks, FindTaskByName, and ReadTask
- ReadTask resolves task IDs that may exist in any subdirectory, not just the root tasks dir
- Lint already walks subdirectories — this aligns list/find/read behavior with lint
- Existing flat-directory layouts continue to work unchanged
- Task file paths are preserved correctly for read/write round-trips
</summary>

<objective>
Make all task storage operations (ListTasks, FindTaskByName, ReadTask) discover tasks recursively in the configured tasks directory and all subdirectories. This enables organizing tasks into subfolders (e.g. by status, by assignee) without losing visibility.
</objective>

<context>
Read CLAUDE.md for project conventions.

Key files:
- `pkg/storage/task.go` — `taskStorage` with `ReadTask`, `WriteTask`, `FindTaskByName`, `ListTasks`
- `pkg/storage/base.go` — `findFileByName` (flat `os.ReadDir`), `readTaskFromPath`, `isExcluded`
- `pkg/ops/lint.go` line 88 — already uses `filepath.Walk` to recurse subdirectories (reference implementation)
- `pkg/domain/task.go` — `Task` struct; `FilePath` stores absolute path, `Name` stores filename without `.md`

Current behavior:
- `ListTasks` uses `os.ReadDir` and skips directories (line 76: `if entry.IsDir() { continue }`)
- `FindTaskByName` delegates to `findFileByName` which also uses flat `os.ReadDir`
- `ReadTask` constructs path as `filepath.Join(vaultPath, tasksDir, taskID+".md")` — only checks root dir
- `WriteTask` writes to `task.FilePath` — already works with any path, no change needed
</context>

<requirements>
1. In `pkg/storage/task.go`, rewrite `ListTasks` to use `filepath.WalkDir` instead of `os.ReadDir`:
   - Walk `tasksDir` recursively
   - Skip non-`.md` files and directories (but descend into them)
   - For each `.md` file, derive `fileName` as `strings.TrimSuffix(entry.Name(), ".md")`
   - Call `t.readTaskFromPath(ctx, path, fileName)` with the full path
   - Keep the existing error-skip-and-log behavior for unreadable tasks

2. In `pkg/storage/base.go`, rewrite `findFileByName` to search recursively:
   - First try exact match at root: `filepath.Join(dir, name+".md")` (preserve fast path)
   - If not found, use `filepath.WalkDir` to search all subdirectories
   - Match logic stays the same: exact filename match first, then case-insensitive contains
   - Return the first match found (WalkDir visits in lexical order, which is deterministic)

3. In `pkg/storage/task.go`, update `ReadTask` to find the task file recursively:
   - First try the direct path `filepath.Join(vaultPath, tasksDir, taskID+".md")` (preserve fast path)
   - If not found (`os.ErrNotExist`), fall back to `findFileByName` which now searches recursively
   - This ensures tasks in subdirectories can be read by ID

4. Update `pkg/storage/task_test.go` (create if it doesn't exist) with tests:
   - `ListTasks` finds tasks in root dir (existing behavior)
   - `ListTasks` finds tasks in subdirectory (e.g. `Tasks/completed/Done Task.md`)
   - `ListTasks` finds tasks in nested subdirectory (e.g. `Tasks/users/alice/Task.md`)
   - `FindTaskByName` finds a task in a subdirectory
   - `ReadTask` finds a task that was moved to a subdirectory
   - `ReadTask` still finds a task in root dir (fast path)

5. Run `make test` to verify all tests pass.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- `WriteTask` must NOT be changed — it already uses `task.FilePath` which is correct
- Task `Name` field remains the filename without `.md`, NOT a relative path
- Symlinks outside the vault should not be followed (use `isSymlinkOutsideVault` if needed)
- All paths are repo-relative
</constraints>

<verification>
Run `make test` — must pass with no failures.
</verification>
