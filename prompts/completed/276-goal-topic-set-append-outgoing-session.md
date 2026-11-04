---
status: completed
summary: goal set / topic set now append the displaced uuid owner to metrics_sessions before overwriting claude_session_id, skipping first bind, same id, blank value, and display-name resolves; unit, round-trip, and integration tests added.
execution_id: vault-cli-goal-topic-set-outgoing-session-exec-276-goal-topic-set-append-outgoing-session
dark-factory-version: v0.198.17
created: "2026-11-04T19:47:14Z"
queued: "2026-11-04T19:47:14Z"
started: "2026-11-04T19:49:09Z"
completed: "2026-11-04T19:54:27Z"
---

# goal set / topic set record the displaced claude_session_id owner

<summary>
- Setting a goal's or topic's recorded owner session to a new id no longer drops the previous owner without a trace
- The previous owner is kept on the page as a session history entry (session id plus a timestamp), the same record shape the goal and topic work-on bind already writes
- The new owner stamp and the history entry land in one file write, so no reader ever sees the new owner without the old one recorded
- Rewriting a session's display name into its real id (what vault-ui does when it resolves a name) records nothing — that is the same session, not a take-over
- First-time owner assignment, re-setting the same id, and blanking the stamp record nothing
- Tasks already record a displaced owner; goals and topics now do too, and additionally skip the display-name → real-id rewrite and a blanked stamp
- Every other field set on a goal or topic behaves exactly as before
</summary>

<objective>
`vault-cli goal set <goal> claude_session_id <id>` and `vault-cli topic set <topic> claude_session_id <id>` must append the outgoing owner to the page's `metrics_sessions` before overwriting the stamp, so a displaced session stays resolvable by claude-supervisor's exit/close paths (outside this repo; they match `claude_session_id` or any `metrics_sessions` entry). The append must NOT fire when the stored value is a session display name being rewritten to its uuid — vault-ui's watcher does exactly that on every resolve, and a history row there would be a bogus take-over.
</objective>

<context>
Read CLAUDE.md for project conventions, and `docs/dod.md`.

The task-level twin of this change already exists — mirror it:
- `pkg/ops/frontmatter.go` — `frontmatterSetOperation.Execute` calls `o.appendDisplacedSession(task, key, value)` immediately before `task.SetField(...)`, then a single `WriteTask`. Read `appendDisplacedSession` and its doc comment: it appends `domain.MetricsSession{SessionID: previous, StartedAt: libtime.DateOrDateTime(o.currentDateTime.Now().Time())}` and appends only the **outgoing** id, never the incoming one.
- `pkg/ops/frontmatter_session_takeover_test.go` — the Ginkgo test pattern for it (pinned `libtime.NewCurrentDateTime()` + `SetNow(libtimetest.ParseDateTime(...))`, seeded entity, captured write argument).

Files to change:
- `pkg/ops/frontmatter_entity.go` — `goalSetOperation`, `NewGoalSetOperation`, `topicSetOperation`, `NewTopicSetOperation`. Both `Execute` methods currently run their refusal guards, (goal only) `writeGoalCloseOutFieldsIfCloseOut`, then `SetField`, then one `WriteGoal` / `WriteTopic`.
- `pkg/domain/goal_frontmatter.go` — `ClaudeSessionID()`, `SetClaudeSessionID`, `MetricsSessions()`, `AppendMetricsSession`. `SetField`'s `case "claude_session_id"` calls `SetClaudeSessionID` (a bare `f.Set`).
- `pkg/domain/topic_frontmatter.go` — `MetricsSessions()`, `AppendMetricsSession`. `TopicFrontmatter` has **no** `ClaudeSessionID()` accessor and `SetField` has no `claude_session_id` case (falls to `default: f.Set`). Read the previous value with `topic.GetField("claude_session_id")` — the same call `pkg/ops/topic_workon.go` `persistTopicSessionID` uses. Do not add domain methods unless genuinely needed.
- `pkg/domain/metrics.go` — `MetricsSession{SessionID, StartedAt}` (yaml `session_id`, `started_at`). This is the record shape; do not introduce another.
- `pkg/cli/cli.go` — the `createEntitySetCommand(..., "goal", ...)` and `(..., "topic", ...)` registrations construct `ops.NewGoalSetOperation(storage.NewGoalStorage(cfg))` / `ops.NewTopicSetOperation(storage.NewTopicStorage(cfg))`. Other commands in this file build `currentDateTime := libtime.NewCurrentDateTime()` — follow that.
- `docs/work-on-session-lifecycle.md`, `docs/goal-writing.md` — both describe the session-id write; update per requirement 8.
- `integration/task_set_session_takeover_test.go` — CLI-level twin (real binary via `gexec`, file re-read); follow it for the integration case in requirement 6.
- Test callers of the two constructors: `pkg/ops/frontmatter_entity_test.go` (`Describe("NewGoalSetOperation"` and `Describe("NewTopicSetOperation"`), `pkg/ops/wikilink_roundtrip_test.go`, `pkg/ops/blocked_by_set_test.go`. Find any other caller with `grep -rn "NewGoalSetOperation\|NewTopicSetOperation" pkg integration`.

The vault-ui consumer this must not trip (not in this repo — quoted so you know the exact shape): its watcher reads a goal's `claude_session_id`, returns early `if not session_id or is_uuid(session_id)`, otherwise resolves the display name to a uuid and calls `goal set <goal> claude_session_id <resolved-uuid>`. So the resolve write happens **only** when the stored value is not a uuid.

UUID parsing: `github.com/google/uuid` is already a dependency (`pkg/ops/lint.go` uses `uuid.Parse`).
</context>

<requirements>
1. Add a `currentDateTime libtime.CurrentDateTime` dependency (`libtime "github.com/bborbe/time"`) to `goalSetOperation` and `topicSetOperation`; change the constructors to `NewGoalSetOperation(goalStorage storage.GoalStorage, currentDateTime libtime.CurrentDateTime)` and `NewTopicSetOperation(topicStorage storage.TopicStorage, currentDateTime libtime.CurrentDateTime)`. Constructors stay pure composition. Update every caller (cli.go and all test files) to pass a clock.

2. Add one unexported helper in `pkg/ops/frontmatter_entity.go` (or a small new file in `pkg/ops`) that decides whether a `claude_session_id` write displaces a real owner. It returns true only when ALL hold:
   - `key == "claude_session_id"`
   - `previous != ""`
   - `value != ""` (blanking the stamp is dismissing a dead pointer, not a take-over — this deliberately differs from the task path, where `set ""` fabricates an entry for the id being cleared)
   - `previous` parses as a uuid (`uuid.Parse(previous)` returns no error)
   - `previous != value`
   Doc-comment the uuid clause with the reason: a non-uuid stored value is a display name, and the only writer that replaces one is vault-ui's name→uuid resolve, which rewrites the same session under its real id — recording it would be a bogus take-over. Also note the accepted cost: a take-over from a page stamped with another session's display name records nothing (that stamp names no resolvable session).

3. In `goalSetOperation.Execute`: read `previous := goal.ClaudeSessionID()` before any mutation; after the refusal guards and before `goal.SetField(ctx, key, value)`, when the helper returns true call `goal.AppendMetricsSession(domain.MetricsSession{SessionID: previous, StartedAt: libtime.DateOrDateTime(o.currentDateTime.Now().Time())})`. The existing single `WriteGoal` persists both. Append the outgoing id only — never the incoming one.

4. In `topicSetOperation.Execute`: same, reading `previous := topic.GetField("claude_session_id")` and calling `topic.AppendMetricsSession(...)` before `topic.SetField`. The existing single `WriteTopic` persists both.

5. A refused write (any of the existing refusal guards returning an error) must append nothing — keep the append after the guards.

6. Tests (Ginkgo v2 / Gomega, Counterfeiter mocks, pinned clock — follow `pkg/ops/frontmatter_session_takeover_test.go`). Put them in a new file `pkg/ops/frontmatter_entity_session_takeover_test.go`. For BOTH goal and topic, using uuid literals for A and B (e.g. `"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"`, `"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"`):
   - take-over: stored A, set B → the written entity's `claude_session_id` is B and `MetricsSessions()` has exactly one new entry `{SessionID: A, StartedAt: <pinned time>}`; exactly one write call.
   - existing entries preserved: stored A with one prior `metrics_sessions` entry, set B → prior entry first, A appended last.
   - first bind: no stored id, set A → no entry appended.
   - same id: stored A, set A → no entry appended.
   - **display-name resolve (production direction):** stored `"vault-cli goal set fix"` (a display name, not a uuid), set A (uuid) → stamp is A, `MetricsSessions()` is empty — no entry.
   - non-session key: stored A, set `status` → no entry appended.
   - empty value: stored A, set `""` → stamp is empty and `MetricsSessions()` is empty — no entry. (The separate `goal clear` / `topic clear` verbs are a different operation and stay unchanged; do not test them.)
   Also add a round-trip test through the real storage (follow `pkg/ops/wikilink_roundtrip_test.go`'s temp-vault pattern) for the goal take-over: write a goal file with `claude_session_id: <A>`, run the set op with B, re-read the file from disk and assert it contains the entry line `- session_id: <A>` (the `- ` prefix is load-bearing; a bare `session_id:` also matches `claude_session_id:`), a `started_at`, and `claude_session_id: <B>`. This traverses the YAML serialization boundary.
   Also add `integration/goal_topic_set_session_takeover_test.go` following `integration/task_set_session_takeover_test.go` (real binary via `gexec`, uuid literals for A/B): for a goal and for a topic, (a) stored A, `set B` → file re-read shows exactly one `- session_id: <A>` line and `claude_session_id: <B>`; (b) stored display name, `set A` → zero `- session_id:` lines. This exercises the clock wiring in `pkg/cli/cli.go`.

7. CHANGELOG.md: add a `## Unreleased` section below the preamble and above `## v0.187.0`, with one bullet: `- fix: \`goal set\` / \`topic set <name> claude_session_id <id>\` now record the displaced owner in the page's \`metrics_sessions\` (\`session_id\` + \`started_at\`, the shape \`goal work-on\` / \`topic work-on\` already write) before overwriting the stamp, in the same write. A first bind, a re-set of the same id, a blank value, and a display-name → uuid rewrite (vault-ui's session resolve) record nothing.` Do not bump any version string and do not add a `## vX.Y.Z` heading.

8. Docs:
   - `docs/work-on-session-lifecycle.md`: in the paragraph starting "**The generic write verbs refuse a caller-supplied value for the field.**", extend the sentence "The one internal exception is the `claude_session_id` write: `task set <task> claude_session_id <new>` …" so it covers `goal set` / `topic set` as well, and add that on goals and topics the append is skipped when the stored value is not a uuid (a display name being rewritten to its id) or the new value is empty.
   - `docs/goal-writing.md` § Session (`claude_session_id`): add one sentence — when `goal set` / `topic set` replaces a different uuid owner with a non-empty id, the outgoing id is appended to `metrics_sessions` (`session_id` + `started_at`) in the same write; a first bind, the same id, a blank value, or a display-name → uuid rewrite records nothing.

9. Before you finish, re-run the `<verification>` command and confirm it passes; then walk each requirement above against the diff.
</requirements>

<constraints>
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
- The `metrics_sessions` entry shape is exactly `domain.MetricsSession{SessionID, StartedAt}` — no new fields, no new record type.
- Do not change `pkg/ops/goal_workon.go` / `pkg/ops/topic_workon.go` (`persistGoalSessionID` / `persistTopicSessionID`) or the task path in `pkg/ops/frontmatter.go`.
- Do not put the append in `GoalFrontmatter.SetClaudeSessionID` or `SetField` — the domain setter is also the bind path's empty-stamp write and holds no clock.
- Unknown frontmatter fields must survive the read-write unchanged.
- Theme, objective and vision set operations are out of scope: claude-supervisor stamps only goal and topic pages.
- No `time.Now()` — use the injected `libtime.CurrentDateTime`.
- Errors via `github.com/bborbe/errors` with context; no `fmt.Errorf`.
- No stdout from `pkg/ops`.
- Repo-relative paths only.
</constraints>

<verification>
Run `ROOTDIR=/workspace make precommit` — must pass.
Then `test -f pkg/ops/frontmatter_entity_session_takeover_test.go` must exit 0, and `awk '/^## /{sec=$0} /record the displaced owner/{print "sits under: " sec}' CHANGELOG.md` must print `sits under: ## Unreleased`.
</verification>
