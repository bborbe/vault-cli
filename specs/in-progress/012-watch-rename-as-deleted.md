---
status: verifying
tags:
    - dark-factory
    - spec
approved: "2026-05-14T14:19:04Z"
generating: "2026-05-14T14:20:38Z"
prompted: "2026-05-14T14:24:52Z"
verifying: "2026-05-14T14:31:32Z"
branch: dark-factory/watch-rename-as-deleted
---

## Summary

- `vault-cli watch` currently emits a `renamed` event when a watched file is moved out of the watched directory — leaking an fsnotify implementation detail
- From the watcher's point of view, such a file is gone; the event carries no destination and consumers cannot follow it
- Real-world impact: Obsidian's default delete moves files to `.trash/` (a rename). Consumers handling only `deleted`/`created`/`modified` silently drop the event, so deleted tasks linger in downstream UIs
- Map fsnotify's `Rename` op to the existing `deleted` event type and remove `renamed` from the watch event API entirely
- Breaking change for any consumer that relies on the literal string `renamed`; consumers handling `deleted` automatically benefit

## Problem

`vault-cli watch` exposes four event types — `created`, `modified`, `deleted`, `renamed` — that map one-to-one onto fsnotify ops. The `renamed` event fires when a watched file is moved or renamed OUT of the watched directory. From the watcher's perspective the file is gone, identical to a delete; there is no destination path on the event so the consumer cannot follow the file. The `renamed` type is leaking fsnotify internals into a public contract without giving consumers anything actionable.

This causes a concrete downstream bug: when a user deletes a task note in Obsidian, Obsidian moves it to `.trash/`, which is a rename. `vault-cli watch` emits `renamed`, but the task-orchestrator UI only handles `created`/`modified`/`deleted` and drops the event, so the deleted task card lingers on screen.

## Goal

After this change, `vault-cli watch` emits exactly three event types: `created`, `modified`, `deleted`. Both fsnotify `Remove` and `Rename` ops are reported as `deleted` with the original (now-gone) path. The `renamed` event type no longer exists in the output, the help text, or the source.

## Assumptions

- fsnotify `Rename` op carries no destination path on the source event — consumers cannot follow the file regardless of how we label the event
- macOS and Linux fsnotify both emit `Rename` for the source path AND `Create` for the destination path on within-directory moves (load-bearing for the in-dir-rename failure-mode row)
- No internal vault-cli caller depends on the literal `"renamed"` string — `watch` events flow only to external stdout consumers via JSON
- The known external consumer (task-orchestrator) already handles `deleted`, so no consumer-side change is required for the lingering-card bug to resolve

## Non-goals

- Detecting renames-within-the-watched-dir as a logical "moved" event. fsnotify already fires `Rename` for the source and `Create` for the destination on macOS/Linux; the desired behavior is to report those as `deleted` (source) plus `created` (destination), not as a paired move event
- Changing event payload shape — `event`, `name`, `vault`, `path`, `type` fields remain identical
- Adding new event types or fields
- Changing the debouncer window or key
- Migrating consumers that currently depend on the literal `renamed` string (none known internally; task-orchestrator already handles `deleted`)

## Desired Behavior

1. When the watcher receives an fsnotify `Rename` op for a file, it emits a single event with `event: "deleted"` and the original path
2. When the watcher receives an fsnotify `Remove` op for a file, it emits a single event with `event: "deleted"` (unchanged from today)
3. The string `"renamed"` does not appear in any event emitted by `vault-cli watch`
4. `vault-cli watch --help` documents three event types: `created`, `modified`, `deleted`
5. Atomic-save sequences (editor temp+rename onto target path) produce a `deleted` followed by a `created` on the same path; the existing debouncer collapses these by key (`vault:relpath`) within its window, matching prior behavior for `Remove`+`Create` sequences
6. Renames within the watched directory surface as `deleted` for the source path and `created` for the destination path (OS-level fsnotify behavior, unchanged)

## Constraints

- Event JSON shape on stdout must not change beyond the removal of the `renamed` value
- The debouncer key and window are unchanged
- Existing tests for `created`, `modified`, `deleted` must continue to pass without modification
- Follows the Interface → Constructor → Struct → Method pattern and existing `pkg/ops/` rules (no `fmt.Print*` in ops layer)
- `docs/dod.md` applies: CHANGELOG `## Unreleased` entry, doc comments on any new exported symbols, Ginkgo v2 + Gomega tests, integration test registration if the help-text contract is asserted there, README.md update if it documents the watch event vocabulary

## Failure Modes

| Trigger | Expected behavior | Recovery |
|---------|-------------------|----------|
| File moved out of watched dir | Single `event: "deleted"` with original path | Consumer treats as delete |
| File renamed within watched dir | `event: "deleted"` (source path) + `event: "created"` (destination path) | Consumer reconciles by path |
| Editor atomic save (temp + rename onto target) | `deleted` + `created` on the target path; debouncer may collapse within window | None needed; matches today's `Remove`+`Create` behavior |
| External consumer depends on literal `"renamed"` | Will no longer receive that string | Consumer must add `deleted` handling (most already have it) |

## Security / Abuse Cases

Not applicable. The watcher consumes local filesystem events from a user-specified vault path; this change narrows the surface (one fewer event type) and does not alter input handling or trust boundaries.

## Acceptance Criteria

- [ ] fsnotify `Rename` ops produce an event with `event: "deleted"`
- [ ] fsnotify `Remove` ops still produce an event with `event: "deleted"`
- [ ] No code path in `vault-cli watch` can emit `event: "renamed"`
- [ ] Ginkgo integration test (real temp dir, `Label("integration")`): create file, `mv` it outside the watched dir, assert exactly one event with `event: "deleted"` and the original path
- [ ] Ginkgo integration test: create file, `rm` it, assert exactly one event with `event: "deleted"`
- [ ] `vault-cli watch --help` lists only `created`, `modified`, `deleted` as event values; `renamed` is not mentioned
- [ ] If `integration/cli_test.go` asserts help-text contracts for `watch`, the assertion is updated
- [ ] `CHANGELOG.md` has an `## Unreleased` entry calling out the breaking event-API change (`renamed` removed; `Rename` ops now reported as `deleted`)
- [ ] `README.md` does not mention `renamed` as a watch event value (update or assert absence)
- [ ] `make precommit` passes in the vault-cli root

**Scenario coverage:** No new dark-factory scenario. The behavior is fully reachable via Ginkgo integration tests using a real temp dir and real fsnotify; the existing `integration/cli_test.go` already covers the CLI surface. No load-bearing E2E gap.

## Verification

```
cd ~/Documents/workspaces/vault-cli
make precommit
```

End-to-end smoke (manual, optional):

1. Build vault-cli locally
2. Run `vault-cli watch --vault <temp-vault>` against a temp vault
3. Create a file, `mv` it out of the vault → assert a single `{"event":"deleted",...}` line
4. Create a file, `rm` it → assert a single `{"event":"deleted",...}` line
5. With the new binary in place, delete a task in Obsidian (which moves to `.trash/`); the task-orchestrator UI card should disappear within ~3 seconds

## Do-Nothing Option

Leaving `renamed` in place keeps the leaky abstraction and the downstream bug. Every consumer that wants to handle Obsidian deletes must learn that `renamed` can mean "gone" and special-case it. Fixing it in the watcher is a one-line behavior change plus tests and resolves the task-orchestrator lingering-card bug with no consumer code changes required. Not acceptable to leave as-is.

## Verification Result

**Verified:** 2026-05-14T14:39:01Z (HEAD 46c2ffb)
**Binary:** /Users/bborbe/Documents/workspaces/go/bin/vault-cli (rebuilt via `go install ./...`)
**Scenario:** Live replay against fresh binary with temp vault `/tmp/spec012-test/{Tasks,Goals}` — `mv` outside watched dir + `rm` of created file; Ginkgo `pkg/ops` suite (553 specs) including two new `Label("integration")` cases.
**Evidence:**
- Live `mv`: `{"event":"deleted","name":"MovedTask","vault":"spec012","path":"Tasks/MovedTask.md","type":"task"}`
- Live `rm`: `{"event":"deleted","name":"DeletedTask","vault":"spec012","path":"Tasks/DeletedTask.md","type":"task"}`
- Zero `renamed` occurrences in live output (`grep -c renamed /tmp/spec012-watch.log` = 0)
- `pkg/ops/watch.go:157-170` `mapFsnotifyOp` maps both `Remove` and `Rename` to `"deleted"`; grep confirms `"renamed"` string is absent from all `.go` files except `watch_test.go` NotTo-assertion
- `vault-cli watch --help` lists only `created, modified, deleted`
- `pkg/ops` Ginkgo run: `Ran 553 of 555 Specs ... SUCCESS! -- 553 Passed | 0 Failed`
- `make precommit`: "ready to commit"
- CHANGELOG.md:5 carries breaking-change note (released as v0.64.1, was Unreleased pre-release)
- README.md: no `renamed` matches
**Verdict:** PASS
