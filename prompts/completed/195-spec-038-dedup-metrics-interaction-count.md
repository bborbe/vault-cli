---
status: completed
spec: [038-bug-metrics-interaction-count-double-counts-reused-session]
summary: 'Deduped session ids in the interaction counter''s Count() so a session recorded N times in metrics_sessions contributes its user-turn total exactly once, added three regression specs (adjacent/interleaved/duplicated-missing) that fail when the dedup is removed, and added the fix: CHANGELOG bullet under ## Unreleased'
execution_id: vault-cli-exec-195-spec-038-dedup-metrics-interaction-count
dark-factory-version: dev
created: "2026-08-24T20:53:45Z"
queued: "2026-08-24T21:00:42Z"
started: "2026-08-24T21:00:44Z"
completed: "2026-08-24T21:04:18Z"
branch: dark-factory/bug-metrics-interaction-count-double-counts-reused-session
---

<summary>
- `metrics_interaction_count` stops being inflated when a task is worked on more than once against the same Claude session.
- Today two work-on runs that reuse one `claude_session_id` append two `metrics_sessions` entries with the same id; the counter reads that session's JSONL log once per entry, so the shared session's user turns are counted twice (observed 24 vs ground truth 12).
- The fix deduplicates session ids inside the counter: an id appearing N times in the recorded list contributes its user-turn total exactly once, no matter where the duplicates sit.
- Dedup is a single set-based pass over the id list — linear time and bounded memory, even for a task worked on many times in one session.
- Regression coverage is added as three new fixture specs: adjacent duplicates (plus a repeated-N variant), interleaved duplicates (order-independent), and duplicated missing/unsafe ids (still contribute 0, never panic).
- All six pre-existing InteractionCounter specs pass unchanged — no existing test is modified.
- The mutation check proves the new specs actually pin the fix: removing the dedup makes them fail, restoring it makes the suite green again.
- One `fix:` bullet is added under the existing `## Unreleased` CHANGELOG section; no version strings are hand-bumped (autoRelease releaser owns that).
</summary>

<objective>
Make the passive interaction counter honor the "distinct session_id" contract it was specified with (parent spec 036, Desired Behavior item 3): a session id recorded more than once in `metrics_sessions` is counted exactly once, so the stored `metrics_interaction_count` for a double-worked task equals the ground-truth user-turn total of the single underlying session. Add regression specs that fail if the dedup is ever removed, with all other counting semantics unchanged.
</objective>

<context>
Read `/workspace/CLAUDE.md` for project conventions, `/workspace/docs/dod.md` for the Definition of Done, and `/workspace/docs/development-patterns.md` for the ops/storage layering rules.

Read these files fully before making changes:

- `/workspace/pkg/ops/interaction_count.go` — the ONLY production file you change. The current `Count` (the bug — sums per entry with no dedup) reads exactly:

  ```go
  // Count returns the total number of user turns across the session logs for the
  // given session ids. An unsafe id is skipped before any path is built.
  func (c *interactionCounter) Count(ctx context.Context, sessionIDs []string) int {
  	total := 0
  	for _, id := range sessionIDs {
  		select {
  		case <-ctx.Done():
  			return total
  		default:
  		}
  		if !isSafeSessionID(id) {
  			continue
  		}
  		path := filepath.Join(c.projectsDir, encodeProjectDir(c.sessionDir), id+".jsonl")
  		total += countUserTurnsInSessionLog(ctx, path)
  	}
  	return total
  }
  ```

  The rest of the file (`NewInteractionCounter`, `interactionCounter` struct, `isSafeSessionID`, `encodeProjectDir`, `countUserTurnsInSessionLog`, `metricsSessionIDs`, `earliestStartedAt`) stays byte-identical. In particular do NOT touch `metricsSessionIDs` — both callers in `/workspace/pkg/ops/complete.go` (lines ~187 and ~310) route `metricsSessionIDs(sessions)` straight into `c.interactionCounter.Count(...)`, so deduping inside `Count` fixes both call sites with zero changes to `complete.go`. Do NOT change the `InteractionCounter` interface or its doc comment — keeping the interface byte-identical means `go generate` (run by `make precommit`) produces no diff in `mocks/interaction-counter.go`, which already exists.

- `/workspace/pkg/ops/interaction_count_test.go` — the file you extend. It is a `package ops_test` Ginkgo suite with exactly six `It` blocks inside `Describe("InteractionCounter", ...)`: "Sum across sessions", "Missing file contributes 0", "Malformed lines skipped", "Non-user types ignored", "Unsafe session ids rejected", "Large line". It has a `BeforeEach` that creates `projectsDir` via `os.MkdirTemp`, `sessionDir = "/home/node/vault-cli"`, `encodedDir = "-home-node-vault-cli"`, and `counter = ops.NewInteractionCounter(projectsDir, sessionDir)`; an `AfterEach` that removes `projectsDir`; a `writeFixture := func(id string, lines ...string)` helper that writes `<projectsDir>/<encodedDir>/<id>.jsonl`; and `userLine := \`{"type":"user","message":{"role":"user","content":"hi"}}\``. The six original specs must not be modified — only new `It` blocks are appended.

- `/workspace/pkg/ops/complete.go` — read-only. Confirms the two `Count` call sites (non-recurring write at ~line 187, recurring cycle archive at ~line 310). You do NOT change this file; it is listed so you can confirm dedup in `Count` covers both.

- `/workspace/specs/in-progress/036-passive-per-task-metrics.md` — the parent spec whose counting contract this bug violates. Desired Behavior item 3 (line 71): "for each **distinct** `session_id` recorded in `metrics_sessions`, count the `type: "user"` entries in that session's Claude Code JSONL log... the stored count is the sum". Acceptance Criterion 4 (line 45) defines the count as the total across the task's recorded session JSONL files with missing/unreadable contributing 0.

- `/workspace/CHANGELOG.md` — the `## Unreleased` section already exists (it has four bullets; the last is the spec-036 passive-metrics `feat:` bullet). You append one new `fix:` bullet after that last bullet, before `## v0.115.0`. Read the section so the append lands in the right place; read `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` for the entry format rules.

Note on the spec's evidence command: the spec's acceptance criteria cite `go test ./pkg/ops/ -run InteractionCounter -v` as evidence. In this repo that command matches NO Go test function — the only suite entrypoint is `func TestSuite(t *testing.T)` in `/workspace/pkg/ops/ops_suite_test.go`, so `-run InteractionCounter` would print `ok ... [no tests to run]` and exit 0 without running a single spec (a false pass). The `<verification>` section below therefore uses working equivalents that satisfy the AC intent: run the full `pkg/ops` suite and grep the verbose output for the new spec descriptions. Keep the `-run TestSuite` trap in mind: a wrong `-run` value that matches nothing is indistinguishable from a pass by exit code alone.

Read these coding-plugin docs (present in the container at these paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega conventions.
- `/home/node/.claude/plugins/marketplaces/coding/docs/changelog-guide.md` — `## Unreleased` bullet format and conventional prefixes.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-context-cancellation-in-loops.md` — the non-blocking `select` in the `Count` loop (already present; keep it).
</context>

<requirements>

## 1. Dedup session ids in `Count()` (`/workspace/pkg/ops/interaction_count.go`)

Replace the current `Count` implementation (quoted verbatim in `<context>`) with exactly this:

```go
// Count returns the total number of user turns across the session logs for the
// given session ids, counting each distinct id once. An unsafe id is skipped
// before any path is built.
func (c *interactionCounter) Count(ctx context.Context, sessionIDs []string) int {
	total := 0
	seen := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		select {
		case <-ctx.Done():
			return total
		default:
		}
		if !isSafeSessionID(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		path := filepath.Join(c.projectsDir, encodeProjectDir(c.sessionDir), id+".jsonl")
		total += countUserTurnsInSessionLog(ctx, path)
	}
	return total
}
```

Semantics to preserve exactly:

- Order of checks per id is load-bearing: the `isSafeSessionID(id)` guard runs BEFORE the seen-check and the seen-mark, so an unsafe id (`../x`, `a/b`, `a\b`, `..`, `""`) still contributes 0, still never has a path built for it, and never poisons the dedup set for a later safe id. This keeps every existing guard semantic from spec 036 unchanged (fires spec 038 AC3).
- The `seen` map is the ONLY dedup state and is local to one `Count` call — set-based, single pass, preallocated to `len(sessionIDs)`, so a task worked on many times in one session counts in linear time with bounded memory (fires spec 038 Constraint "Dedup must be bounded" and Failure Mode "Very long `metrics_sessions` list").
- The existing non-blocking `select { case <-ctx.Done(): return total; default: }` per iteration stays exactly as-is (context cancellation in the loop, per `go-context-cancellation-in-loops.md`). It returns the partial total, never an error.
- Do NOT change any other symbol in the file. In particular `metricsSessionIDs` keeps copying every recorded id verbatim (accumulation stays on the write side per spec 038 Constraint — the dedup lives only on the read side inside `Count`), and the `InteractionCounter` interface plus its doc comment stay byte-identical so `go generate` produces no diff in `mocks/interaction-counter.go`.

Failure-mode coverage: this requirement alone is what makes the stored count correct for a double-worked task (spec 038 Failure Modes row 1: "Same session id recorded N times → session counted exactly once").

## 2. New duplicate-id fixture specs (`/workspace/pkg/ops/interaction_count_test.go`)

Append three new `It` blocks INSIDE the existing `Describe("InteractionCounter", ...)`, after the "Large line" spec, at the end of the `BeforeEach`/fixture scope. Do NOT modify, reorder, or rename any of the six existing `It` blocks — they pass unchanged (fires AC5).

```go
	It("Duplicate session ids counted once", func() {
	writeFixture("s1", userLine, userLine, userLine)
	// Adjacent duplicate (AC1): the shared session's 3 user turns count once.
	Expect(counter.Count(ctx, []string{"s1", "s1"})).To(Equal(3))
	Expect(counter.Count(ctx, []string{"s1"})).To(Equal(3))
	// Repeated N times (Desired Behavior 2): never more than once.
	Expect(counter.Count(ctx, []string{"s1", "s1", "s1"})).To(Equal(3))
})

	It("Interleaved duplicate session ids counted once", func() {
	writeFixture("s1", userLine, userLine, userLine)
	writeFixture("s2", userLine, userLine)
	// AC2: each distinct session once, order-independent.
	Expect(counter.Count(ctx, []string{"s1", "s2", "s1"})).To(Equal(5))
	Expect(counter.Count(ctx, []string{"s2", "s1", "s2"})).To(Equal(5))
})

	It("Duplicated missing file contributes 0", func() {
	// Failure Mode 4: a missing file contributes 0 whether duplicated or not (AC3).
	Expect(counter.Count(ctx, []string{"missing", "missing"})).To(Equal(0))
	// Failure Mode 3: every unsafe id still contributes 0 when duplicated, and the
	// dedup never panics on a malformed/empty id (AC3, unchanged guard table).
	for _, id := range []string{"../x", "a/b", `a\b`, "..", ""} {
		Expect(counter.Count(ctx, []string{id, id})).To(Equal(0))
	}
})
```

Notes:

- `writeFixture`, `userLine`, `counter`, and `ctx` are all in scope from the existing `BeforeEach` — no new helpers. The fixture s1 has 3 `type:"user"` turns and s2 has 2, matching the AC1/AC2 numbers.
- The "Duplicated missing file contributes 0" spec needs no fixtures (the missing file and all unsafe ids contribute 0 regardless), so it passes even in a fresh temp dir.
- These three new specs fire AC1 (adjacent + equals `Count(["s1"])`), AC2 (interleaved + order-independence), and AC3 (missing duplicated → 0; unsafe duplicated → 0), plus the "repeated N times" clause of Desired Behavior 2 and Failure Modes 3 and 4.

## 3. CHANGELOG — one `fix:` bullet under the existing `## Unreleased` (`/workspace/CHANGELOG.md`)

`## Unreleased` already exists (four bullets; the last is the spec-036 passive-metrics `feat:` bullet). Append exactly one new bullet as a new line directly after that last bullet and before the `## v0.115.0` heading:

```
- fix: `metrics_interaction_count` no longer double-counts a task worked on more than once in the same Claude session — the interaction counter now deduplicates `session_id`s before summing `type:"user"` turns, so a session recorded N times in `metrics_sessions` contributes its user-turn total exactly once and the stored count for a double-worked task matches the ground truth of the single underlying session log instead of reporting double
```

- `fix:` prefix per the changelog-guide conventional-prefix table (bug fix → patch bump; the releaser reads the prefix).
- Do NOT touch the preamble, do NOT rename `## Unreleased` to a version, do NOT bump `.claude-plugin/plugin.json` or either version field in `.claude-plugin/marketplace.json`, and do NOT `git tag` — this repo is `autoRelease: true` and the github-releaser owns version bumps + tags post-merge. Add the bullet only.

## 4. Mutation check — proves the new specs pin the dedup (AC4)

Verify by hand, in this order:

1. Temporarily remove the dedup from `Count` in `/workspace/pkg/ops/interaction_count.go` — delete the `seen := make(map[string]struct{}, len(sessionIDs))` line, the `if _, ok := seen[id]; ok { continue }` block, and the `seen[id] = struct{}{}` line (the loop then counts every entry again, as the bug does).
2. Run `go test -count=1 ./pkg/ops/` and confirm it exits NON-ZERO, with failures naming the new duplicate-id specs (the adjacent/interleaved assertions fail because the shared session is counted twice).
3. Restore the dedup exactly as written in requirement 1.
4. Restore gate — before doing anything else, run `grep -c 'seen\[id\] = struct{}{}' pkg/ops/interaction_count.go`; it MUST print `1`. If it prints `0`, step 3 did not happen — restore the code and re-check. Do not proceed until it prints `1`.
5. Re-run `go test -count=1 ./pkg/ops/` and confirm it exits 0.

If step 2 passes with the dedup removed (i.e. tests actually ran and reported success — the output must name the three new specs, not `[no tests to run]`), the new specs do not cover the bug; rewrite them before continuing.

Do NOT pass `-run` to any of these `go test` invocations. The suite entrypoint is `TestSuite` in `/workspace/pkg/ops/ops_suite_test.go`; a `-run` value that matches nothing prints `ok ... [no tests to run]` and exits 0, which would make the mutation check look like a pass against an uncovered bug.

</requirements>

<constraints>
- Parent spec 036 semantics must not regress: AC3 (`metrics_completed_at` written; `metrics_interaction_count` a non-negative integer), AC5 (recurring archive to `metrics_cycles` then reset of the active accumulator; `claude_session_id` cleared per existing behavior), AC6 (no-anchor task completes with the count field absent, never zero). This prompt touches none of those paths — the fix is purely inside the counter's `Count`.
- `metrics_sessions` accumulation is unchanged: two work-on runs still append two entries (parent AC2). The dedup lives on the read side (the counter), never on the write side — the accumulator must not start suppressing entries, and neither `/workspace/pkg/ops/workon.go` nor `/workspace/pkg/ops/complete.go` is modified.
- Best-effort missing-file→0 contract unchanged: a session whose JSONL is missing or unreadable contributes 0 and does not fail completion.
- Read-only contract on `~/.claude` unchanged: the fix reads session logs only, never writes.
- Path-traversal guard unchanged: the `isSafeSessionID` rules (reject empty, `.`, `..`, path separators) stay; an unsafe id contributes 0 and no path is ever built for it.
- The fix is confined to `pkg/ops/interaction_count.go` (`Count()`), `pkg/ops/interaction_count_test.go` (three NEW specs only — the six original InteractionCounter specs are not modified), and `CHANGELOG.md`. No change to domain structs, frontmatter serialization, the work-on/complete hooks, `pkg/cli/cli.go`, or `mocks/`.
- Dedup must be bounded (set-based, one pass) — a task worked on many times in one session produces a long id list, and counting must complete in linear time.
- All existing tests in `pkg/ops` continue to pass; the six original InteractionCounter specs are not modified.
- `make precommit` passes. This is a bug fix on `kind: bug`; CHANGELOG and version alignment follow the autoRelease releaser as in parent spec 036 (add a `## Unreleased` bullet only; do NOT hand-bump the three plugin JSON version fields or CHANGELOG versions, and do NOT create a git tag).
- `pkg/ops/` is a library layer — no `fmt.Print*`, no `os.Stdout` writes (see `/workspace/CLAUDE.md` § Key Design Decisions). `fmt.Sprintf` and `slog` are fine.
- Errors are wrapped with `github.com/bborbe/errors` propagating the incoming `ctx`; never `fmt.Errorf`, never `context.Background()` in `pkg/`.
- Do NOT commit — dark-factory handles git. Do not use git in this rung (scope rule: commits, branches, and tags are the operator's and the releaser's job, not this prompt's).
- Existing tests must still pass.
</constraints>

<verification>
Run everything from `/workspace`.

**1. Unit suite green:**
```
make test
```
Must exit 0.

**2. The dedup is in place in `Count`** — each must print exactly `1`:
```
grep -c 'seen := make(map\[string\]struct{}, len(sessionIDs))' pkg/ops/interaction_count.go
grep -c 'if _, ok := seen\[id\]; ok {' pkg/ops/interaction_count.go
grep -c 'seen\[id\] = struct{}{}' pkg/ops/interaction_count.go
grep -c 'counting each distinct id once' pkg/ops/interaction_count.go
```

**3. The unchanged symbols are intact** (nothing outside `Count` moved) — each must print exactly `1`:
```
grep -c 'func metricsSessionIDs(sessions \[\]domain.MetricsSession) \[\]string' pkg/ops/interaction_count.go
grep -c 'func isSafeSessionID(id string) bool' pkg/ops/interaction_count.go
grep -c 'func NewInteractionCounter(projectsDir string, sessionDir string) InteractionCounter' pkg/ops/interaction_count.go
grep -c 'type InteractionCounter interface' pkg/ops/interaction_count.go
```
Note: these are present because they were not modified. The grep for `seen[id] = struct{}{}` in step 2 and the mutation-check restore gate (step 6 below) are the real regressions guards.

**4. Exactly nine `It` blocks in the counter spec — six original plus three new** — must print `9`:
```
grep -c '^	It("' pkg/ops/interaction_count_test.go
```

**5. The new specs exist and pass** — each must print a number `>= 1` (the descriptions come from requirement 2):
```
go test -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "Duplicate session ids counted once"
go test -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "Interleaved duplicate session ids counted once"
go test -count=1 ./pkg/ops/ -v -ginkgo.v 2>&1 | grep -c "Duplicated missing file contributes 0"
```
Do not use `-run` here — the suite entrypoint is `TestSuite`; a wrong `-run` matches nothing and exits 0 without running specs.

**6. Mutation check for AC4** — follow requirement 4 exactly: remove the dedup, confirm `go test -count=1 ./pkg/ops/` exits NON-ZERO with failures naming the new duplicate-id specs, restore the dedup, then run the restore gate and confirm it prints `1` before continuing:
```
grep -c 'seen\[id\] = struct{}{}' pkg/ops/interaction_count.go
```
Then re-run `go test -count=1 ./pkg/ops/` and confirm exit 0.

**7. Changelog bullet added once, versions untouched:**
```
grep -c 'no longer double-counts a task worked on more than once in the same Claude session' CHANGELOG.md
grep -m1 '^## v' CHANGELOG.md
grep '"version"' .claude-plugin/plugin.json
```
The first must print exactly `1`. The second must still print `## v0.115.0`. The third must still show `0.115.0` (do NOT hand-bump — autoRelease releaser owns versions).

**8. Out-of-scope files byte-identical** — `make precommit` runs `go generate`, so check AFTER `make precommit`. Each of these must print `0`:
```
grep -c 'seen' pkg/ops/complete.go
grep -c 'seen' pkg/ops/workon.go
grep -c 'Duplicate session ids counted once' pkg/ops/complete_test.go
```
(`pkg/ops/complete.go`, `pkg/ops/workon.go`, and `pkg/ops/complete_test.go` must be untouched by this prompt.)

**9. Full gate, once, at the end:**
```
make precommit
```
Must exit 0. If it fails, fix and re-run only the failing target until green, then re-run `make precommit` once.
</verification>
