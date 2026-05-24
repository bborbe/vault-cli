---
status: approved
spec: [014-bug-work-on-silent-failure-and-hardcoded-slash-command]
created: "2026-05-24T14:33:00Z"
queued: "2026-05-24T14:24:43Z"
branch: dark-factory/bug-work-on-silent-failure-and-hardcoded-slash-command
---

<summary>
- `StartSession` in `pkg/ops/claude_session.go` now parses `num_turns`, `is_error`, `subtype`, and `result` from claude's JSON output
- Returns error when `num_turns == 0` (slash command unknown, no conversation created)
- Returns error when `is_error == true` (claude reported an error)
- Error message includes the `result` field text for debugging
- Existing happy-path behavior unchanged for sessions with >=1 turn and no error
- Full test coverage for error detection paths
</summary>

<objective>
Update `StartSession` in `pkg/ops/claude_session.go` to detect and surface failures from Claude's headless mode output. When Claude returns `num_turns: 0` or `is_error: true`, `StartSession` must return an error instead of silently returning the session_id.
</objective>

<context>
Read CLAUDE.md for project conventions.
Read these files before making changes:
- `pkg/ops/claude_session.go` — `StartSession` method (lines 67-106), current JSON parsing (lines 94-99)
- `pkg/ops/claude_session_test.go` — existing test pattern, mock command runner setup
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-error-wrapping-guide.md` — error wrapping patterns
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — test structure

Reference JSON response from spec (what Claude actually returns):
```json
{"type":"result","subtype":"success","is_error":false,"duration_ms":6,"num_turns":0,"result":"Unknown command: /work-on-task","session_id":"1b08ddc8-bc8b-46f6-9930-225064545f8d", ...}
```
</context>

<requirements>
1. In `pkg/ops/claude_session.go`, update the `result` struct used for JSON unmarshalling (lines 94-99) to include additional fields:
   ```go
   var result struct {
       SessionID string `json:"session_id"`
       NumTurns  int    `json:"num_turns"`
       IsError   bool   `json:"is_error"`
       Result    string `json:"result"`
   }
   ```

2. After unmarshalling the JSON and checking for empty `SessionID` (lines 101-103), add error detection for zero turns and is_error:
   ```go
   if result.SessionID == "" {
       return "", fmt.Errorf("claude returned empty session_id")
   }

   if result.NumTurns == 0 {
       return "", fmt.Errorf("claude returned 0 turns: %s", result.Result)
   }

   if result.IsError {
       return "", fmt.Errorf("claude reported error: %s", result.Result)
   }
   ```

3. Use `errors.Wrap` from `github.com/bborbe/errors` for error wrapping, following the pattern in `pkg/ops/workon.go`. However, the `fmt.Errorf` form is acceptable here since the error context (num_turns, is_error) is already encoded in the error message itself.

4. In `pkg/ops/claude_session_test.go`, add test cases for the new error detection:
   - Test: `num_turns: 0` with result "Unknown command: /x" returns error containing "0 turns" and "Unknown command: /x"
   - Test: `is_error: true` with result "something failed" returns error containing "error" and "something failed"
   - Test: happy path with `num_turns: 3`, `is_error: false`, `result: "ok"` returns session_id and nil error (must be a named `It` that shows up in `-v` output)
   - Test: existing happy path continues to work (existing `Context("successful session start")` test)

5. The existing test at line 44 (`output = []byte(`{"session_id":"abc-123","result":"ok"}`)`) should be updated to include `num_turns: 1` to make it a valid happy-path response:
   ```go
   output = []byte(`{"session_id":"abc-123","result":"ok","num_turns":1,"is_error":false}`)
   ```

6. Add a new explicit happy-path test case with descriptive name (for the acceptance criteria verification):
   ```go
   Context("returns session_id when num_turns >= 1 and is_error is false", func() {
       BeforeEach(func() {
           output = []byte(`{"session_id":"happy-path-sid","result":"done","num_turns":3,"is_error":false}`)
       })

       It("returns session_id and nil error", func() {
           sessionID, err := starter.StartSession(ctx, "prompt", "/vault")
           Expect(err).To(BeNil())
           Expect(sessionID).To(Equal("happy-path-sid"))
       })
   })
   ```

7. Keep all existing tests that still make sense (command fails, invalid JSON, empty session_id). Update the "empty session_id" test to include `num_turns: 1` and `is_error: false` so the test only checks the empty session_id path.

8. Ensure test coverage of both error branches:
   - `num_turns == 0` error
   - `is_error == true` error
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- The signature of `StartSession` must NOT change — it already returns `(string, error)`
- Error messages must be informative: include the `result` field text so users can understand what went wrong
- `subtype` field does not need to be checked — `is_error` is the authoritative error indicator
</constraints>

<verification>
Run `make precommit` — must pass.
Grep verification:
- `grep -nE '"num_turns"|"is_error"|"result"' pkg/ops/claude_session.go` returns >=4 lines (SessionID + 3 new fields)
- `go test ./pkg/ops/... -run StartSession -v` lists the happy-path test case by name
- `go test ./pkg/ops/...` exits 0
</verification>