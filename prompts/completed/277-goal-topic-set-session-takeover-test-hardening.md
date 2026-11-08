---
status: completed
summary: Added goal/topic non-uuid display-name take-over tests and made the integration AfterEach nil-safe
execution_id: vault-cli-goal-topic-set-outgoing-session-exec-277-goal-topic-set-session-takeover-test-hardening
dark-factory-version: v0.198.17
created: "2026-11-04T20:00:17Z"
queued: "2026-11-04T20:00:17Z"
started: "2026-11-04T20:02:25Z"
completed: "2026-11-04T20:05:54Z"
---

# harden goal/topic set session take-over tests

<summary>
- A genuine take-over where the new owner value is not a uuid still records the outgoing uuid owner, and a test now pins that for goals and topics
- Without that test, a future "both values must be uuids" simplification would silently drop real take-overs while every test stayed green
- The goal/topic set integration test no longer panics in its cleanup step when vault setup fails, so a setup failure reports its real cause
- Production code is unchanged
</summary>

<objective>
Close two test gaps left by the goal/topic `claude_session_id` take-over change, so a regression in the displaced-owner guard cannot pass the suite and a fixture failure is reported instead of masked by a nil-func panic.
</objective>

<context>
Read CLAUDE.md for project conventions.
- `pkg/ops/frontmatter_entity.go` — `displacesSessionOwner(key, previous, value string) bool`: true only when key is `claude_session_id`, `previous` and `value` are non-empty, `previous` parses as a uuid, and they differ. Note: only `previous` must be a uuid; `value` may be anything non-empty.
- `pkg/ops/frontmatter_entity_session_takeover_test.go` — the unit tests for goal and topic (seed helpers, `bind`-style setter, pinned clock, `sessionA` uuid constant). Follow its existing structure.
- `integration/goal_topic_set_session_takeover_test.go` — declares `var cleanup func()` and assigns it only inside `createVault`; its `AfterEach` calls `cleanup()` unconditionally.
- `integration/task_set_session_takeover_test.go` — sibling for style reference.
</context>

<requirements>
1. In `pkg/ops/frontmatter_entity_session_takeover_test.go`, add one `It` for goals and one for topics: stored `claude_session_id` is the uuid `sessionA`, set the value to the non-uuid display name `"incoming display name owner"` (use exactly this literal — the existing display-name tests use a different one) → the written entity's `claude_session_id` is the display name, and `MetricsSessions()` holds exactly one entry with `SessionID == sessionA` and `StartedAt` equal to the pinned time.
2. In `integration/goal_topic_set_session_takeover_test.go`, make `AfterEach` safe when `createVault` never ran or failed early: initialise `cleanup` to a no-op with `cleanup = func() {}` in a new `BeforeEach` placed directly before the existing `AfterEach` (not at declaration — a declaration-time value runs once per suite and would leave the previous spec's closure in place), so the `AfterEach` call can never dereference nil.
3. Do not change any production file under `pkg/` other than tests, and do not touch CHANGELOG.md (test-only change, already covered by the existing `## Unreleased` bullet).
4. Before you finish, re-run `<verification>` and confirm it passes; then walk requirements 1–3 against the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- Ginkgo v2 / Gomega only; reuse the existing helpers and constants in each file.
- Repo-relative paths only.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must pass.
Then `grep -c "incoming display name owner" pkg/ops/frontmatter_entity_session_takeover_test.go` must print a number ≥ 2, and `grep -n "cleanup = func() {}" integration/goal_topic_set_session_takeover_test.go` must print one line.
</verification>
