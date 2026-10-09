---
status: completed
summary: Resolved task/goal `launcher:` frontmatter to the session launcher (task > goal > vault claude_script) via exported ops.ResolveLauncher/ResolveTaskLauncher, wired into `task work-on` for both start and resume, with shape validation and fail-loud handling of an unresolvable explicit launcher
execution_id: vault-cli-launcher-resolver-exec-254-launcher-field-resolution
dark-factory-version: v0.196.0
created: "2026-10-09T09:41:25Z"
queued: "2026-10-09T09:41:25Z"
started: "2026-10-09T09:42:27Z"
completed: "2026-10-09T09:59:16Z"
---

# Resolve a task's launcher from task / goal `launcher:` frontmatter

<summary>
- A task or goal can name the launcher script its Claude session starts with, via an optional `launcher:` frontmatter field
- Precedence: the task's own value, else its goals' value, else the vault's configured `claude_script` — unchanged behaviour when nobody sets the field
- A bare launcher name resolves next to the vault's own launcher; a value containing a slash is used as given; the value is not checked against a launcher list, but a value with unsafe characters or a `..` step is refused
- Two goals naming different launchers (with no task value) is refused with an error naming both, instead of picking one by frontmatter order
- A goal link that no longer resolves is skipped with a warning, so existing tasks with stale goal links keep working
- `vault-cli task work-on` starts and resumes the session with the resolved launcher, so a trading task opens on Claude while the vault default stays deepseek
- An explicitly chosen launcher that is not installed fails the work-on loudly instead of silently starting nothing
- The resolution is an exported library function so vault-ui can build its resume command with the exact same rule
</summary>

<objective>
Make `launcher:` frontmatter on tasks and goals decide which launcher script `vault-cli task work-on` starts and resumes a task's Claude session with (task > goal > vault `claude_script`), and export that rule from `pkg/ops` so vault-ui reuses it instead of re-implementing it. Different work needs different models; today every task opens on the vault default.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read `docs/task-writing.md` § Frontmatter and `docs/goal-writing.md` § Frontmatter — the `launcher:` paragraph there is the contract this prompt implements (already shipped in v0.166.3).
Read `docs/work-on-session-lifecycle.md` — the start/resume flow being changed.
Read `pkg/domain/task_frontmatter.go` (`Goals()`, `Assignee()` — the accessor style) and `pkg/domain/goal_frontmatter.go` (`Assignee()`).
Read `pkg/ops/workon.go` — `NewWorkOnOperation`, `workOnOperation`, `Execute` (note it calls `FindTaskByName`, then writes the task and the daily note BEFORE `handleClaudeSession`), `handleClaudeSession` (`w.starter.StartSession`), the interactive resume branch (`w.resumer.ResumeSession`), and how `ErrStarterUnavailable` is turned into a non-fatal warning.
Read `pkg/ops/claude_session.go` (`NewClaudeSessionStarter`, returns nil when `exec.LookPath` fails) and `pkg/ops/claude_resume.go` (`NewClaudeResumer`).
Read `pkg/cli/cli.go` — the `task work-on` command, which constructs `ops.NewClaudeSessionStarter(vault.GetClaudeScript(), locker)` / `ops.NewClaudeResumer(...)` and `ops.NewWorkOnOperation(...)`. The two other starter/resumer construction sites in that file belong to `goal work-on` and `topic work-on` — leave them unchanged.
Read `pkg/storage/storage.go` (`GoalStorage`, `FindGoalByName`) and `pkg/ops/workon_test.go` + `pkg/ops/workon_session_writeback_test.go` (Ginkgo/Gomega + counterfeiter mock style; existing `NewWorkOnOperation` callers).
Reference semantics (another repo's JavaScript implementation): task value wins; else the distinct non-empty goal values; >1 distinct → error naming them; exactly 1 → that; none → vault script. Bare name → `filepath.Join(filepath.Dir(vaultScript), name)` (so vault script `claude` + `cc-x` → `cc-x`); value containing `/` → as given.
</context>

<requirements>
1. Add `Launcher() string` accessors to `TaskFrontmatter` and `GoalFrontmatter` reading the `launcher` key, matching the existing `Assignee()` style. Add a domain test that parses real frontmatter text containing `launcher: cc-private-claude` and asserts `Launcher()`.
2. Add `pkg/ops/launcher.go` with an exported pure function `ResolveLauncher(ctx context.Context, vaultScript string, taskLauncher string, goalLaunchers []string) (string, error)` implementing exactly the precedence and path rules in `<context>` (use `filepath.Join`/`filepath.Dir`, never string concatenation). Trim whitespace; ignore empty values; de-duplicate goal values before the conflict check. Refuse (return an error naming the value) any non-empty task or goal value that does not match `^[A-Za-z0-9._/-]+$` or that contains a `..` path segment — the value comes from worker-writable frontmatter and is later executed, so it must be a plain path; this is a shape check, not a launcher-list check. Errors use the project's existing `errors` style in `pkg/ops`; the conflict error names every distinct goal launcher.
3. Add an exported `ResolveTaskLauncher(ctx context.Context, goalStorage storage.GoalStorage, vaultPath string, vaultScript string, task *domain.Task) (resolved string, warnings []string, err error)`. It returns the task's own `Launcher()` resolution when set (no goal lookups). Otherwise it looks up each name in `task.Goals()` (strip `[[`/`]]`, skip empty names such as `[[]]`) with `FindGoalByName`; a goal that cannot be found (or any other `FindGoalByName` error) is SKIPPED and reported as a warning string naming it (a stale goal link must never block work-on); collected `Launcher()` values go to `ResolveLauncher`.
4. Add a `goalStorage storage.GoalStorage` dependency to `NewWorkOnOperation`, and a launcher factory dependency (e.g. `func(script string) (ClaudeSessionStarter, ClaudeResumer)`) so the starter/resumer pair can be built for a resolved script. Keep the vault-default pair as today for the unchanged case. Update the `task work-on` wiring in `pkg/cli/cli.go` and every existing `NewWorkOnOperation` caller (including tests). Do NOT change `goal work-on` / `topic work-on`.
5. In `Execute`, resolve the launcher immediately after `FindTaskByName` and BEFORE any task / daily-note write. A resolution error (goal conflict, rejected value) returns that error and leaves the task file untouched. When the resolved launcher differs from the vault default, also build the factory pair here, before any write; a nil starter for a fresh start (or a nil resumer when interactive) is a hard error naming the launcher, with the task file and daily note untouched. Resolution warnings are appended to the result's warnings the same way existing work-on warnings are.
6. When the resolved launcher differs from the vault default, build the starter/resumer pair from it via the factory and use that pair for BOTH the fresh-start and the resume branch. If the factory returns a nil starter (or nil resumer on the resume branch) for an explicitly chosen launcher, return a hard error naming the launcher path — do NOT reuse the non-fatal `ErrStarterUnavailable` warning and do NOT fall back to the vault default. The vault-default path keeps today's semantics exactly.
7. Tests (Ginkgo, DescribeTable where natural), each asserting the RESOLVED path, not merely non-empty:
   - `ResolveLauncher`: vault `/s/cc-private` + task `cc-private-claude` → `/s/cc-private-claude`; nothing set → `/s/cc-private`; goal `cc-private-claude` + task empty → `/s/cc-private-claude`; goal `cc-private-claude` + task `cc-private` → `/s/cc-private`; two goals agreeing → that one; two goals disagreeing → error containing both names; `/other/cc-x` → `/other/cc-x`; vault `claude` + task `cc-x` → `cc-x`; task `cc-x; rm -rf /`, `cc-$(id)`, `cc x`, `../cc-x` → each an error.
   - `ResolveTaskLauncher` with a fake goal storage: goal inheritance; task override (no goal lookup made); missing goal → skipped with a warning naming it and the vault default returned; `[[]]` skipped.
   - `workOnOperation`: a task carrying `launcher:` starts through the pair the factory built for the resolved path (assert the factory argument); a task without it uses the vault-default pair and never calls the factory; a goal conflict returns an error and the task storage `WriteTask` fake was not called; an explicit launcher whose factory returns a nil starter returns an error naming it AND `WriteTask` was not called; a task with no launcher and a stale goal link starts on the vault default with a warning.
   - One boundary test: write a temp executable file, resolve a launcher to it, and assert `NewClaudeSessionStarter(<resolved path>, locker)` returns non-nil (exercises `exec.LookPath` with a resolved path).
8. In `docs/task-writing.md` and `docs/goal-writing.md`, replace the sentence `The value is not validated against a launcher list; it is passed through as given.` with `The value is not checked against a launcher list, but it must match ^[A-Za-z0-9._/-]+$ with no \`..\` segment; anything else is refused.` Then add a `## Unreleased` CHANGELOG bullet: `feat: vault-cli task work-on starts and resumes sessions with the task's launcher: (task > goal > vault claude_script); ops.ResolveLauncher / ops.ResolveTaskLauncher exported for vault-ui`.
9. Self-check: before finishing, re-run `<verification>` and confirm it passes; walk each requirement above against the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass; a task without `launcher:` (on itself or any reachable goal) must behave exactly as before — same script, same args, same warnings (exception: the new stale-goal-link warning from requirement 3)
- A stale / unresolvable goal link is a warning, never an error
- Do NOT validate the launcher value against a list of known launchers (operator decision 2026-10-09) — but DO refuse any value not matching `^[A-Za-z0-9._/-]+$` or containing a `..` segment (worker-writable input)
- Never fall back to the vault default when an explicitly set launcher cannot be found — fail with an error naming it
- Do NOT change `goal work-on` or `topic work-on`
- Do NOT hand-bump versions or tags — `autoRelease` owns them; only the `## Unreleased` bullet
- Do NOT run `go mod vendor`
- Repo-relative paths only
</constraints>

<verification>
Run `make precommit` — must pass.
Run `grep -n 'func ResolveLauncher' pkg/ops/launcher.go` — must print one line.
Run `grep -n 'func ResolveTaskLauncher' pkg/ops/launcher.go` — must print one line.
Run `grep -n 'func (f TaskFrontmatter) Launcher' pkg/domain/task_frontmatter.go` — must print one line.
</verification>
