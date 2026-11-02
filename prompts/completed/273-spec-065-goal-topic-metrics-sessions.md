---
status: completed
spec: [065-bug-goal-topic-bind-drops-mid-turn-session]
summary: Extracted shared metrics_sessions reader/appender helpers and added MetricsSessions/AppendMetricsSession to GoalFrontmatter and TopicFrontmatter with matching tests
execution_id: vault-cli-goal-topic-rebind-exec-273-spec-065-goal-topic-metrics-sessions
dark-factory-version: v0.198.7
created: "2026-11-02T21:58:52Z"
queued: "2026-11-02T22:08:40Z"
started: "2026-11-02T22:10:00Z"
completed: "2026-11-02T22:15:38Z"
---

# Give goal and topic frontmatter a metrics_sessions reader and appender

<summary>
- Goal and topic pages can now carry the same session history list a task already carries.
- Reading that history from a goal or topic yields the same entries, in the same order, as reading it from a task would.
- Malformed history entries are dropped and well-formed ones kept, exactly as on tasks.
- Appending a session keeps every earlier entry and adds the new one last.
- An append with no session id is ignored.
- The history survives a write to disk and a read back.
- Task behaviour is unchanged: its reader and appender are re-pointed at a shared helper, not rewritten.
- No command, flag, or work-on behaviour changes in this prompt — it only adds the record shape the next prompt uses.
</summary>

<objective>
Add `MetricsSessions()` / `AppendMetricsSession(entry)` to `domain.GoalFrontmatter` and `domain.TopicFrontmatter` with the exact entry shape and normalisation `domain.TaskFrontmatter` already has, so the goal and topic `work-on` persist (spec 065 prompt 2) can record a displaced binding session. This is spec 065 prompt 1 of 2: it covers Desired Behavior 5 and the unit half of AC9.
</objective>

<context>
Read `CLAUDE.md` and `docs/development-patterns.md` for project conventions, and `docs/dod.md` for the Definition of Done.

Read these files fully before changing anything:

- `specs/in-progress/065-bug-goal-topic-bind-drops-mid-turn-session.md` — Desired Behavior 5, Failure Modes row "`metrics_sessions` holds a malformed entry", Constraints.
- `pkg/domain/task_frontmatter_metrics.go` — the reference implementation. Current signatures:
  - `func (f TaskFrontmatter) MetricsSessions() []MetricsSession` — switch on `f.Get("metrics_sessions")`: `[]MetricsSession` passes through; `[]any` is walked, non-map elements skipped, each map coerced by `coerceMetricsSession`, an all-malformed list returns `nil`; any other shape returns `nil`.
  - `func (f *TaskFrontmatter) AppendMetricsSession(entry MetricsSession)` — ignores `entry.SessionID == ""`, otherwise `f.Set("metrics_sessions", append(f.MetricsSessions(), entry))`.
  - `func coerceMetricsSession(m map[string]any) (MetricsSession, bool)` — drops an entry with absent/empty `session_id`; `started_at` degrades to the zero time via `coerceMetricsTime`.
- `pkg/domain/metrics.go` — `type MetricsSession struct { SessionID string \`yaml:"session_id" json:"session_id"\`; StartedAt libtime.DateOrDateTime \`yaml:"started_at" json:"started_at"\` }`.
- `pkg/domain/frontmatter_map.go` — `FrontmatterMap` (`Get` value receiver, `Set` / `Delete` pointer receivers, `RawMap()`).
- `pkg/domain/goal_frontmatter.go` — `type GoalFrontmatter struct { FrontmatterMap }`; note `ClaudeSessionID()` / `SetClaudeSessionID(v string)` as the accessor style.
- `pkg/domain/topic_frontmatter.go` — `type TopicFrontmatter struct { FrontmatterMap }`; it has no typed `claude_session_id` accessor (topics use `GetField` / `SetField`). Do not add one here.
- `pkg/domain/goal.go` / `pkg/domain/topic.go` — `Goal` embeds `GoalFrontmatter`, `Topic` embeds `TopicFrontmatter`, so the new methods are promoted onto `*domain.Goal` / `*domain.Topic`.
- `pkg/domain/task_frontmatter_metrics_test.go` — `Describe("MetricsSessions", …)` and `Describe("YAML round-trip", …)`: the test shapes to mirror (generic on-disk `[]any` of `map[string]any`, `DescribeTable` for element coercion, `yaml.Marshal(fm.RawMap())` + `yaml.Unmarshal` round-trip using `gopkg.in/yaml.v3`).

Coding-plugin docs (in-container paths):
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-testing-guide.md` — Ginkgo v2 / Gomega, external `_test` packages.
- `/home/node/.claude/plugins/marketplaces/coding/docs/go-architecture-patterns.md` — doc comments on exported symbols.
</context>

<requirements>
1. **Extract the task logic into two unexported package-level helpers** in `pkg/domain/task_frontmatter_metrics.go`, placed next to `coerceMetricsSession`:

   ```go
   // metricsSessionsOf reads "metrics_sessions" from any frontmatter map. It is the
   // single reader shared by tasks, goals and topics so all three normalise the
   // on-disk list identically.
   func metricsSessionsOf(f FrontmatterMap) []MetricsSession

   // appendMetricsSessionTo appends one entry to "metrics_sessions", preserving every
   // prior well-formed entry. An empty SessionID is ignored.
   func appendMetricsSessionTo(f *FrontmatterMap, entry MetricsSession)
   ```

   Move the current body of `TaskFrontmatter.MetricsSessions` into `metricsSessionsOf` verbatim (same switch, same skip rules, same `nil` returns). `appendMetricsSessionTo` does exactly what `TaskFrontmatter.AppendMetricsSession` does today: return on empty `SessionID`, else `f.Set("metrics_sessions", append(metricsSessionsOf(*f), entry))`.

2. **Re-point the task methods** at the helpers, keeping their signatures and doc comments:
   - `func (f TaskFrontmatter) MetricsSessions() []MetricsSession { return metricsSessionsOf(f.FrontmatterMap) }`
   - `func (f *TaskFrontmatter) AppendMetricsSession(entry MetricsSession) { appendMetricsSessionTo(&f.FrontmatterMap, entry) }`

   `RemoveMetricsSession`, `ClearMetricsSessions` and every other task method stay as they are. This is a behaviour-preserving refactor: `pkg/domain/task_frontmatter_metrics_test.go` must pass **without modification**. The extraction exists so the goal and topic methods do not duplicate a 25-line block (the repo runs the `dupl` linter on non-test files).

3. **Add the goal methods** to `pkg/domain/goal_frontmatter.go`, directly after `ClearClaudeSessionID`:

   ```go
   // MetricsSessions reads the "metrics_sessions" field as a slice of MetricsSession,
   // with the same shape and normalisation as TaskFrontmatter.MetricsSessions.
   // Returns nil when the key is absent, the value is not a list, or every element is malformed.
   func (f GoalFrontmatter) MetricsSessions() []MetricsSession { return metricsSessionsOf(f.FrontmatterMap) }

   // AppendMetricsSession appends one entry to "metrics_sessions", preserving every
   // prior well-formed entry. An empty SessionID is ignored.
   func (f *GoalFrontmatter) AppendMetricsSession(entry MetricsSession) {
   	appendMetricsSessionTo(&f.FrontmatterMap, entry)
   }
   ```

4. **Add the identical pair to `pkg/domain/topic_frontmatter.go`** on `TopicFrontmatter` (value receiver for `MetricsSessions`, pointer receiver for `AppendMetricsSession`), placed after `SetDeferDate`, with doc comments naming `TopicFrontmatter`.

5. **Do not** add `GetField` / `SetField` arms for `metrics_sessions` on goals or topics, do not add a remove/clear method, and do not add any refusal logic — `goal set` / `topic set` are out of scope (spec Non-goals).

6. **Add tests** in two new files, package `domain_test`, BSD header `Copyright (c) 2026 Benjamin Borbe`:
   - `pkg/domain/goal_frontmatter_metrics_test.go` — `var _ = Describe("GoalFrontmatter metrics_sessions", …)` building frontmatter with `domain.NewGoalFrontmatter(...)`.
   - `pkg/domain/topic_frontmatter_metrics_test.go` — `var _ = Describe("TopicFrontmatter metrics_sessions", …)` building with `domain.NewTopicFrontmatter(...)`.

   Each file covers, mirroring `task_frontmatter_metrics_test.go`:
   - **Absent:** a fresh frontmatter (`nil` map) returns `nil` from `MetricsSessions()`.
   - **Append preserves order:** append `s1` then `s2` with distinct `libtime.DateOrDateTime(time.Date(...UTC))` values; `MetricsSessions()` returns `[s1, s2]` with `s1`'s `StartedAt` equal to what was appended.
   - **Generic on-disk shape:** `map[string]any{"metrics_sessions": []any{map[string]any{"session_id": "s1", "started_at": time.Date(...)}, map[string]any{"session_id": "s2", "started_at": "2026-08-24T18:14:35Z"}}}` reads as two typed entries; `s2.StartedAt.String()` equals `"2026-08-24T18:14:35Z"`.
   - **Malformed entries dropped, order kept (spec Failure Modes row 2):** a list `[]any{map{"session_id":"s1"}, "not-a-map", map{"session_id":""}, map{"started_at":"2026-08-24T09:00:00Z"}, map{"session_id":"s2"}}` reads as exactly `[s1, s2]`; then `AppendMetricsSession(s3)` yields `[s1, s2, s3]`.
   - **Non-list value:** `"metrics_sessions": "not-a-list"` reads as `nil`, and an append onto it yields a one-entry list holding only the appended id (the same normalisation as on tasks).
   - **Empty id ignored:** `AppendMetricsSession(domain.MetricsSession{SessionID: ""})` leaves `fm.Get("metrics_sessions")` `nil`.
   - **YAML round-trip (boundary test):** append two entries, `yaml.Marshal(fm.RawMap())`, assert the YAML text contains `"- session_id: s1"` (the list-item form — not a bare `session_id:`, which `claude_session_id:` would also satisfy), `yaml.Unmarshal` into `map[string]any`, rebuild with `NewGoalFrontmatter` / `NewTopicFrontmatter`, and assert both entries come back in order with `StartedAt.Time().Equal(...)` true.
   - **Coexists with `claude_session_id`:** a frontmatter holding `claude_session_id: "owner"` plus one appended entry `"binder"` keeps `claude_session_id` unchanged (goal: `ClaudeSessionID()`; topic: `GetField("claude_session_id")`) and `MetricsSessions()` holds only `"binder"`.

   Use `gopkg.in/yaml.v3`, `libtime "github.com/bborbe/time"`, `. "github.com/onsi/ginkgo/v2"`, `. "github.com/onsi/gomega"`. The `domain_test` suite file already exists — do not add a second `RunSpecs`.

7. **Self-check.** Before finishing, re-run `<verification>` and confirm every step passes; walk Desired Behavior 5 and the Failure Modes "malformed entry" row against the change.
</requirements>

<constraints>
- Goal and topic entries use the same shape (`session_id` + `started_at`) and normalisation as the task's (`pkg/domain/task_frontmatter_metrics.go`) — one shared reader/appender, no lookalike.
- The task path is unchanged in behaviour: `TaskFrontmatter.MetricsSessions` / `AppendMetricsSession` keep their signatures and results; `task_frontmatter_metrics_test.go` passes unmodified. `persistSessionAndMetrics`, `task set claude_session_id`, and `task append-metrics-session` are untouched.
- `goal set` / `topic set` are untouched; no refusal for a caller-supplied scalar `metrics_sessions` on goals/topics (known gap, spec Non-goals).
- No new CLI verb or flag.
- The id is taken verbatim, with no UUID validation.
- Errors (if any are introduced) wrap with `github.com/bborbe/errors`; tests use Ginkgo/Gomega.
- No work-on behaviour changes in this prompt — `pkg/ops/` is not touched.
- Do NOT edit `CHANGELOG.md` — this prompt adds an internal record shape with no user-visible change; spec 065 prompt 2 adds the single `## Unreleased` bullet. Treat the `docs/dod.md` CHANGELOG item as satisfied by prompt 2.
- Do NOT commit — dark-factory handles git.
- Existing tests must still pass.
</constraints>

<verification>
Run `ROOTDIR=/workspace make test` — must exit 0.

Focused runs (confirm the new `Describe` names appear in the verbose output — a focus matching zero specs exits 0 while running nothing):

```
go test -mod=mod ./pkg/domain/... -ginkgo.focus="metrics_sessions" -ginkgo.v
go test -mod=mod ./pkg/domain/... -ginkgo.focus="TaskFrontmatter metrics" -ginkgo.v
```

Evidence greps:

```
grep -c 'func metricsSessionsOf(f FrontmatterMap) \[\]MetricsSession' pkg/domain/task_frontmatter_metrics.go   # 1
grep -c 'func appendMetricsSessionTo(f \*FrontmatterMap, entry MetricsSession)' pkg/domain/task_frontmatter_metrics.go   # 1
grep -c 'func (f GoalFrontmatter) MetricsSessions() \[\]MetricsSession' pkg/domain/goal_frontmatter.go   # 1
grep -c 'func (f \*GoalFrontmatter) AppendMetricsSession(entry MetricsSession)' pkg/domain/goal_frontmatter.go   # 1
grep -c 'func (f TopicFrontmatter) MetricsSessions() \[\]MetricsSession' pkg/domain/topic_frontmatter.go   # 1
grep -c 'func (f \*TopicFrontmatter) AppendMetricsSession(entry MetricsSession)' pkg/domain/topic_frontmatter.go   # 1
grep -c -- '- session_id: s1' pkg/domain/goal_frontmatter_metrics_test.go   # >= 1
grep -c -- '- session_id: s1' pkg/domain/topic_frontmatter_metrics_test.go  # >= 1
```

Finally run `ROOTDIR=/workspace make precommit` — must exit 0.
</verification>
