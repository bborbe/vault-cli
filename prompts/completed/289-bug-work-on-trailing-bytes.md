---
status: completed
spec: [071-bug-work-on-trailing-bytes]
summary: Replaced the whole-buffer JSON unmarshal in validateSessionTurn with a single-value decoder that ignores trailing bytes, exported the unparseable sentinel for tests, and locked the behaviour with five new fixtures plus an extended sentinel assertion.
execution_id: vault-cli-exec-289-bug-work-on-trailing-bytes
dark-factory-version: v0.198.17
created: "2026-11-22T14:45:00Z"
queued: "2026-11-22T15:02:28Z"
started: "2026-11-22T15:27:35Z"
completed: "2026-11-22T15:32:54Z"
---

# Accept a headless turn whose captured stdout carries trailing bytes (spec 071)

<summary>
- A completed headless turn is currently thrown away whenever anything follows its JSON result in the captured output, because the whole buffer is parsed as one document.
- Observed 2026-11-22: a headless `task work-on` run completed a 182-line turn, then failed with `parse claude output: invalid character '-' after top-level value`; the compensating clear removed the session id, so the task looked untouched.
- Fix: decode exactly one JSON value from the start of the buffer and ignore any suffix; a buffer with no valid JSON at its start is still reported as unparseable, so the session id is still cleared.
- The three existing rejection checks, the read-after-exit ordering, and the compensating clear are unchanged.
- Adds five fixtures, one exported test alias, a CHANGELOG bullet, and a one-paragraph doc update.
</summary>

<objective>
A completed headless turn is accepted whenever its captured output begins with a valid turn result, whatever follows it, while every existing rejection path stays intact and the behaviour is locked by fixtures a naive trim-at-a-brace cannot pass.
</objective>

<context>
Read these files fully before changing anything:

- `specs/in-progress/071-bug-work-on-trailing-bytes.md` — § Goal, § Desired Behavior, § Acceptance Criteria, § Failure Modes.
- `pkg/ops/claude_session.go`:
  - `errClaudeOutputUnparseable` (~line 317) — the sentinel. Its identity is load-bearing.
  - `validateSessionTurn(ctx, output []byte) error` (~line 323) — the function under change. The `json.Unmarshal(output, &result)` call is the only validator.
  - `runDetachedTurn` (~line 233) — reads the temp file only inside the `case exitErr := <-done` branch, then calls `validateSessionTurn`. On a validated turn it returns nil even for a non-zero child exit; on an unparseable turn with a non-zero exit it returns the exit-status error; otherwise it returns the validation error unwrapped.
- `pkg/ops/claude_session_test.go` (external package `ops_test`) — the `Context("non-interactive branch", ...)` block (~line 256). Note:
  - `const validTurnJSON` (~line 269) is a flat one-line turn object.
  - The fake runner signature is `func(args []string, dir string, stdout *os.File) (<-chan error, error)`; it writes into the caller-owned `stdout` file and returns a channel carrying the child exit result.
  - The waiter must BLOCK on success paths (the `blockWaiter` pattern) or the `select` in `runDetachedTurn` races and the spec flips between success and a spurious turn-timeout error.
- `pkg/ops/export_test.go` — the established pattern for exporting unexported symbols to the external test package.
- `docs/work-on-session-lifecycle.md` § The fate of `--output-format json` (~line 147) — states the current strict contract.

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external test package.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-precommit.md` — funlen, nestif, golines limits.
</context>

<requirements>

## 1. Change the parse in `pkg/ops/claude_session.go`

Replace the whole-buffer unmarshal inside `validateSessionTurn` with a single-value decode:

```go
dec := json.NewDecoder(bytes.NewReader(output))
if err := dec.Decode(&result); err != nil {
    return errors.Wrapf(ctx, errClaudeOutputUnparseable, "parse claude output: %v", err)
}
```

Add `"bytes"` to the imports if it is missing. Everything after the first JSON value is ignored.

Keep exactly as they are:
- the sentinel as the **wrapped cause** — `errors.Wrapf(ctx, errClaudeOutputUnparseable, ...)`. Do NOT change it to `errors.Wrap(ctx, err, ...)`; `runDetachedTurn` branches on `errors.Is(validateErr, errClaudeOutputUnparseable)`.
- the three predicates (`empty session_id`, `num_turns == 0`, `is_error`) and `rejectTurn`.
- the read-after-exit ordering in `runDetachedTurn` and its comments.

## 2. Export the sentinel for the external test package

In `pkg/ops/export_test.go` add:

```go
var ErrClaudeOutputUnparseable = errClaudeOutputUnparseable
```

Follow the file's existing style for the surrounding declarations.

## 3. Add fixtures in `pkg/ops/claude_session_test.go`

Place them in the existing non-interactive-branch context, following the file's fake-runner and blocking-waiter pattern. Each fake writes its blob to the provided `stdout` file and reports exit code 0.

1. **Nested turn object with a trailing separator** — write `{"session_id":"s","num_turns":1,"is_error":false,"result":"done","usage":{"input_tokens":5}}` followed by `"\n---\n"`. `StartSession` must return nil. The nested `usage` object is required: a flat fixture lets a cut-at-the-first-`}` implementation pass.
2. **Brace-bearing suffix** — append `"\n---\n{}"` to a valid turn object. `StartSession` must return nil. This defeats a last-`}` trim.
3. **No JSON at the start** — extend the existing "rejects an unparseable turn result" spec (empty blob, child exit 0): add `Expect(errors.Is(err, ops.ErrClaudeOutputUnparseable)).To(BeTrue())` beside its existing `parse claude output` assertion. Do not create a near-duplicate spec.
4. **Truncated prefix** — write `{"session_id":"s","num_turns":1` (no closing brace) with child exit 0. Same sentinel assertion as fixture 3.
5. **JSON scalar** — write `true` with child exit 0. Same sentinel assertion as fixture 3: a scalar cannot decode into the result struct, so the decode error must route through the sentinel.
6. **JSON `null`** — write `null` with child exit 0. `null` decodes without error into a zero result, so the empty-`session_id` predicate must reject it: assert an error, and assert `errors.Is(err, ops.ErrClaudeOutputUnparseable)` is **false** (it parsed, so it must not be reported as unparseable).

Do not weaken or delete any existing spec in that file.

## 4. Update the docs

In `docs/work-on-session-lifecycle.md` § The fate of `--output-format json`, replace this sentence:

> The validation verdict, not the process exit status, is what decides: a turn whose result is `num_turns: 0`, `is_error: true`, or unparseable is an error, and no id is persisted.

with:

> The validation verdict, not the process exit status, is what decides: a turn whose result is `num_turns: 0` or `is_error: true` is an error, and so is one whose output carries no valid turn result at its start — no id is persisted in any of those cases. Trailing bytes after the result are ignored: the first JSON value in the captured output is the turn result, whatever follows it.

Keep the rest of the section unchanged.

## 5. CHANGELOG

Add a `## Unreleased` section above the newest `## vX.Y.Z` section if absent (the releaser renames it on master), containing:

```
- fix: accept a headless turn whose captured stdout carries trailing bytes after the JSON result
```

Do not bump `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, or create a tag.

## 6. Do not touch anything else

No change to the turn timeout, the detached-child lifecycle, the supervisor wrapper, `runDetachedTurn`'s control flow, or the compensating clear. The compensating-clear case in `pkg/ops/workon_session_writeback_test.go` must stay green — `make precommit` covers it; the focused run does not.

</requirements>

<constraints>
- `git` is masked in this container (`.git` is an empty directory). Do not run `git` commands; use file reads and `make` targets.
- `pkg/ops/claude_session_test.go` already imports stdlib `"errors"`. Use that `errors.Is` for the sentinel assertions; do NOT add `github.com/bborbe/errors` — a second `errors` import will not compile, and the stdlib `Is` matches the same chain.
- No new third-party dependencies — `encoding/json` and `bytes` are stdlib.
- Keep the sentinel's identity and the `errors.Is` contract intact; this is the single most likely regression.
- Do not "fix" a failing fixture by relaxing it — if a fixture fails, the implementation is wrong.
</constraints>

<verification>

FOCUSED RUN — the five new fixtures plus the extended sentinel case and the existing predicate cases must pass:

```
go test -mod=mod ./pkg/ops -run TestSuite -ginkgo.focus='ClaudeSessionStarter' -ginkgo.v
```

The output must contain the new fixture names and the existing rejection cases.

FORMAT:

```
gofmt -e -l pkg/ops/claude_session.go pkg/ops/claude_session_test.go pkg/ops/export_test.go   # must list NO files
```

CHANGELOG PLACEMENT:

```
awk '/^## /{sec=$0} /trailing bytes/{print sec}' CHANGELOG.md
```

must print `## Unreleased`.

FULL GATE — `make precommit` at the repo root must exit 0. If it fails, fix the failing target named in the output (`make ensure`, `make format`, `make generate`, `make test`, `make lint`, `make vet`, `make vulncheck`, `make osv-scanner`, `make trivy`, `make check-changelog`, or `make addlicense`) and re-run only that target, then run `make precommit` once more. A non-zero `make precommit` must be reported as `"status":"failed"` — never rationalised as success.

KNOWN FLAKE (observed, dated) — on 2026-11-22 two consecutive `make test` runs in this repo each failed exactly one integration spec with `[FAILED] Timed out after 1.000s. Expected process to exit.`, and the failing spec moved between runs while 359/360 passed: first `integration/goal_topic_set_session_takeover_test.go:91`, then `integration/task_set_session_takeover_test.go:63`. Both use `Eventually(session).Should(gexec.Exit(...))` with Gomega's 1s default, so a loaded host can miss the deadline. If `make test` fails only with that signature in `integration/*set_session_takeover_test.go`, report the failure verbatim, do not edit those files, and do not treat it as a pass — record it as a known flake in the completion report. Any other failure is a real failure.

</verification>

<!--
Open questions surfaced for the reviewer (resolved here, not asked):

1. The spec's Suggested Decomposition names one prompt; this is it. No second prompt is expected.
2. The container masks git, so the spec's "fix commit" evidence is satisfied by the prompt's own completion commit.
3. The trailing-byte producer is out of scope by design — the fix is on the reading side.
-->
