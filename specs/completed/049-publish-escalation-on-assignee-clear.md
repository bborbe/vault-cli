---
status: completed
approved: "2026-09-17T13:01:52Z"
generating: "2026-09-17T13:21:21Z"
prompted: "2026-09-17T13:21:21Z"
verifying: "2026-09-17T17:21:57Z"
completed: "2026-09-17T21:00:20Z"
branch: dark-factory/publish-escalation-on-assignee-clear
---

## Publish an escalation notification when the vault CLI clears a task's assignee

## Summary

- The agent→human half of the park handoff already works: the agent controller parks a task and the operator's chat receives a message in under a second.
- The human→agent half is silent. Parking by hand — `vault-cli task set "<task>" assignee ""` — writes the task file and publishes nothing.
- This change makes the vault CLI publish the same escalation notification on that transition, into the shared notification core.
- The producer is opt-in: with no broker configured the CLI behaves exactly as it does today, so its standalone promise holds for every existing user.
- Outcome: a park pings the operator regardless of which side performed it.

## Problem

The escalation publish is reachable only from the agent controller's read-modify-write cycle. Every park performed outside that cycle — an operator clearing an assignee by hand, or an interactive session doing it on their behalf — runs the vault CLI, which writes the task file and returns. Nothing observes the transition, so nothing is published and the other side of the handoff is never told. The controller-side path was verified live in production on 2026-09-15 with chat delivery in 945 ms; the session-side path has no equivalent, which makes the human→agent half of the loop absent rather than merely slow. This change also makes the vault CLI a pipeline participant for the first time — its README states the opposite today, and that promise has to be amended deliberately rather than quietly broken.

## Goal

Clearing a task's assignee through the vault CLI publishes exactly one escalation notification through the shared notification core, carrying the same payload fields the controller-side path emits — while a CLI with no broker configured keeps behaving exactly as it does today.

## Non-goals

- Changing the controller-side publish path, its clear sites, or its behaviour.
- Re-notifying on later edits of an already-parked task — one notification per clear.
- Backfilling notifications for tasks parked before this ships.
- Covering a raw editor write of the assignee field; the CLI's own write paths are the surface this spec reaches.
- Matrix delivery, which is tracked separately.
- Making the publish mandatory. A deployment with no broker configured must keep today's behaviour exactly — no connection attempt, no new failure mode.

## Acceptance Criteria

- [ ] Clearing a non-empty `assignee` to empty emits exactly one publish command, after the task write succeeds, **through either documented clear path** — evidence: for each of `task set "<task>" assignee ""` and `task clear "<task>" assignee`, a test asserts exactly one recorded command, with the agent-escalation notification type and a nil target.
- [ ] The emitted command's metadata carries the task identifier, the task name, and the previous assignee value, **asserted per path** — evidence: for each of `task set "<task>" assignee ""` and `task clear "<task>" assignee`, a test asserts all three fields, and asserts the previous-assignee value equals the fixture's pre-clear value rather than a constant. The two paths differ in write semantics — `set` leaves the key present and empty, `clear` deletes it — so a metadata read placed after the mutation passes on one path and silently yields an empty value on the other.
- [ ] No other write emits a command — evidence: tests assert zero recorded commands for each of: the empty→empty transition, the non-empty→non-empty transition, a different frontmatter key set through the same operation, and an injected task-write failure.
- [ ] With no broker configured, no publish is attempted — evidence: a test builds the operation with an empty broker config and asserts zero recorded commands and zero connection attempts.
- [ ] A failing or unreachable broker neither fails the command nor stalls it — evidence: a test injects a sender that blocks past the timeout and asserts the operation returns success within 5 seconds, with the cleared assignee present in the task file.
- [ ] **Post-Deploy (Rung-2):** with a broker and a topic prefix configured on the installed binary, a park produces exactly one notification for that task — evidence: exactly one new command on `master-core-notification-v1-request` for the task's identifier (read via `kafka-topic-reader`), and ≥1 new line in the notification controller's log matching the fixed string `notification(agent-escalation) routed to telegram chat` — probe it with `grep -F`, since the parentheses are ERE metacharacters and a `grep -E` probe returns zero on a healthy system; and the controller's own `assignee cleared` line is absent for that task, checked as `kubectlnukeprod -n prod logs <agent-task-controller-pod> --since=10m | grep -c 'assignee cleared'` returning 0 for that task, so the ping provably did not come from the controller. Release the version before running this check: `.dark-factory.yaml` sets `autoRelease: false` so the daemon does not tag, but `.maintainer.yaml` carries `release.autoRelease: true`, so `github-releaser-agent` cuts the tag after the merge — do **not** hand-cut it (`docs/releasing-vault-cli.md`).
  - `deploy_check:` `set -o pipefail; vault-cli --version | awk '{print $NF}'`
  - `deploy_target:` `$(git fetch --tags -q; git describe --tags --abbrev=0 origin/master)`
- [ ] The documentation matches the new behaviour — evidence: `CHANGELOG.md` carries an `## Unreleased` bullet naming the assignee-clear publish, **both** contradicting README statements are gone — `grep -c 'never produces or consumes Kafka events' README.md` returns 0 **and** `grep -c 'No Kafka' README.md` returns 0 — **and** the replacement is present and names the gate: `grep -n 'broker' README.md` returns ≥1 within the amended standalone section. Deletion alone does not pass this criterion.
- [ ] `make precommit` exits 0 — evidence: exit code.

## Verification

### Container-executable

- `make precommit` — lint, vet, license, and vulnerability checks clean
- `make test` — unit and integration suites pass, including the transition, no-config, and bound tests named in the Acceptance Criteria
- `grep -rc 'SendPublishNotificationCommand' --include='*.go' pkg/ops/` returns **≥2** — both clear paths reach the publish; a single call site means one path is unwired, which a bare presence grep would not catch
- `grep -rniE 'telegram|chat_id|chat_token' --include='*.go' pkg/ | grep -v _test` returns 0 lines — no chat-specific code entered the repository

### Operator-executable

- `vault-cli --version` — the installed CLI reports the version carrying this change
- On a task whose assignee is non-empty: `vault-cli task set "<task>" assignee ""`
- Confirm exactly one new command on `master-core-notification-v1-request` for that task's identifier, and ≥1 new controller log line matching `notification(agent-escalation) routed to telegram chat` (probe with `grep -F` — unescaped parentheses make a `grep -E` probe return zero on a healthy system)
- Confirm the controller's own `assignee cleared` log line does **not** appear for that task
- Confirm the chat message carries the previous assignee value the task file held before the clear

## Desired Behavior

1. Clearing a non-empty assignee through **either documented path** — `vault-cli task set "<task>" assignee ""` or `vault-cli task clear "<task>" assignee` — writes the file as it does today and additionally emits exactly one escalation publish command, carrying the agent-escalation notification type, a nil target, and metadata naming the task identifier, task name, and previous assignee value. Both paths carry the same transition rule. They differ in write semantics — `set` leaves the key present and empty, `clear` deletes it — so the previous value must be captured before the mutation on both.
2. No other write emits anything: the empty→empty transition, the non-empty→non-empty transition, a different frontmatter key, and a failed task write all emit zero commands.
3. The producer is opt-in. With no broker configured the CLI performs no publish and makes no connection attempt.
4. The publish is bounded. A failure or an unreachable broker is logged and never fails the command, never changes its exit code or stdout, and never delays it beyond the bound.
5. The escalation reaches the operator's chat through the existing routing table, with no chat-specific code in this repository.
6. The README's standalone promise is amended to match the new behaviour rather than left contradicting it.

## Assumptions

- The shared notification core already routes the agent-escalation type to the operator's chat, so this change adds no routing — only a producer, in a repository that has never had one.
- The notification command topic is a **prefix joined to** a schema-derived name, not a bare schema-derived name. The working peer configures brokers *and* a topic prefix (`TOPIC_PREFIX`), and the consumer derives the same prefix from the deployment branch — prod maps to `master`, dev to `develop`. The prod command topic is therefore `master-core-notification-v1-request`. A producer that omits the prefix publishes into a topic nothing consumes, silently.
- The brokers are reachable from the host running the CLI. Verified from the operator's machine on 2026-09-16: all three accepted a TCP connection on the command-topic port.

## Constraints

- **Opt-in is the load-bearing constraint.** No broker config means no publish, no connection attempt, and behaviour byte-identical to today. This is what keeps the repository usable as a standalone local tool.
- **Bounded.** A single publish attempt with a 5-second timeout. A publish failure never fails the write — the same semantics the working peer documents — and the command's exit code and stdout stay unchanged for callers that do not care about the publish.
- **No chat credentials.** The CLI publishes into the shared core; the routing table decides the channel. No chat token, chat id, or per-channel secret enters this repository.
- **Mirror the working peer's transport** rather than inventing one. The peer is `bborbe/agent-task-controller`, which publishes this same notification today: it builds a sync producer from `github.com/bborbe/kafka` and sends through `NotificationPublishCommandSender` from `github.com/bborbe/notification/command/notification`, configured with brokers, a topic prefix, and a cqrs initiator identity — the command topic is that prefix joined to a schema-derived name (`<prefix>-<group>-<kind>-<version>-request`). The prefix is required for the deployed topology even though the peer's flag parser marks it optional (`required:"false"`, empty meaning unprefixed): the consumer always derives its prefix from its branch, so a producer that omits it publishes into a topic nothing consumes. The emitted metadata keys are `taskIdentifier`, `taskName`, and `previousAssignee`, matching what the peer emits. A publish failure logs one line carrying the peer's own wording — `publish agent-escalation notification for task <name> (<id>) escalated by <assignee> failed: <err>` — through the ops layer's logger, never stdout: this repository's ops layer writes no stdout. This is a fleet-consistency constraint, not an implementation preference.
- **Two new module dependencies.** This repository currently depends on neither `github.com/bborbe/kafka` nor `github.com/bborbe/notification`; both are added. The license and vulnerability checks in `make precommit` must stay clean across the addition.
- **Documentation tension to reconcile, not leave standing.** `docs/task-writing.md` documents an empty assignee as the *unclaimed inbox*; this change treats the same transition as *escalate to the operator*. Both readings ship in this repository, so the doc update this spec requires must state which applies when.
- **Both documented clear paths are covered** — `task set … assignee ""` and `task clear … assignee` — because an operator following the README or `docs/task-writing.md` uses the second, and a spec that covered only the first would leave the documented route silent while claiming the Goal met. A raw editor write of the assignee field bypasses both. The fallback surface — a filesystem watcher over the vault directories, which this repository already exports as a library operation — is deliberately not chosen: it needs a persistent host, and its predecessor was retired after its only host was found unable to reach the notification brokers.
- Authored through this spec's prompts. This repository runs dark-factory, so the code is not edited directly.

## Failure Modes

| Trigger | Expected behavior | Recovery | Detection | Reversibility |
|---|---|---|---|---|
| Broker unreachable | Write succeeds, publish fails within the bound, command exits 0 | Operator re-parks the task; the task file is already correct | ≥1 stderr line matching the frozen failure string in Constraints | Reversible |
| The core's schema identifier moves | The CLI publishes into a topic nothing consumes | Operator sees a publish attempt and no controller line; re-point the schema identifier | Zero controller log lines for the task despite a publish attempt | Reversible |
| Published with the wrong topic prefix | The CLI publishes into a topic nothing consumes | Operator sees a publish attempt and no controller line; correct the prefix to the deployment's branch value | Zero controller log lines for the task despite a publish attempt | Reversible |
| No broker configured | No publish attempted; behaviour identical to today | None needed | Absence of any connection attempt | Reversible |
| Task write fails | No publish is emitted | Existing write-error handling; command exits non-zero as today | Command exit code | Reversible |
| Crash between the successful write and the publish | Task is cleared; no notification is sent | Operator re-sets the assignee and clears it again; confirm ≥1 notification for the task identifier | No notification for a task whose assignee is empty | Irreversible for that park |
| Two concurrent clears of the same task | Both may read a non-empty assignee and both publish | Operator sees two notifications for one park; the extra is harmless, and the task file is correct | Publish count for the task identifier exceeds 1 | Reversible |
| The same task is cleared twice | The second clear is empty→empty and emits nothing | None needed — one notification per clear | Publish count for the task identifier | Reversible |
| Publish emitted but the core cannot route it | The core's existing behaviour; the CLI is unaffected | Inspect the core's logs; the CLI did its part | `kubectlnukeprod -n prod logs core-notification-controller-0 \| grep 'not routed'` returns ≥1 — reliable only at `LOGLEVEL>=2`, since that line is verbosity-gated; confirm the deployed level first (pod confirmed Running in prod 2026-09-17) | Reversible |

## Security / Abuse

- No credential enters this repository. The CLI uses the broker connection it is configured with; no chat token, chat id, or per-channel secret is added.
- The previous assignee value is copied into the notification metadata. Assignee values are agent names, not personal data, and no task body content is included.
- The publish is gated on one specific frontmatter transition, so an unrelated `task set` cannot be used to emit arbitrary notifications.
- An operator-controlled config field decides whether the CLI ever opens a broker connection, so the standalone default is preserved rather than assumed.

## Suggested Decomposition

| # | Prompt focus | Covers DBs | Covers ACs | Depends on |
|---|---|---|---|---|
| 1 | Broker config surface plus producer and sender construction — opt-in, bounded — with the no-config and timeout tests | 3, 4 | 4, 5 | — |
| 2 | Transition rule applied to both clear paths (`set` and `clear`) plus the publish call, with the transition tests | 1, 2, 5 | 1, 2, 3, 6 | prompt 1 |
| 3 | README invariant amendment, `docs/task-writing.md` reconciliation, and CHANGELOG bullet | 6 | 7 | prompt 2 |

Rationale: prompt 1 establishes the transport so prompt 2 has something to call and cannot invent its own; prompt 3 documents behaviour that must exist first. The three split along the repository's own seams — transport, write path, docs — so no single prompt holds the whole graph.

## Do-Nothing Option

Every park performed outside the controller stays silent. An operator parks a task expecting the agent side to be told, and it is not, so the loop's human→agent half remains absent while its agent→human half works. The cost is paid on every hand-park — the exact workflow this change exists to make observable.
