---
status: approved
created: "2026-08-10T12:36:04Z"
queued: "2026-08-10T12:36:04Z"
---

<summary>
- Four error sites stop using the standard library formatter and use the project's own error package, so failures carry the context data everything else relies on.
- Parsing a recurring interval gains a "with a default" variant, replacing a fallback one caller had written by hand.
- Behaviour is unchanged apart from richer errors.
</summary>

<objective>
Apply findings M3 (`fmt.Errorf` where `github.com/bborbe/errors` is mandated) and S3 (missing `ParseRecurringIntervalDefault`) in `/workspace`.
</objective>

<context>
This repo mandates `github.com/bborbe/errors` — `errors.Errorf(ctx, format, args...)` for new errors and `errors.Wrapf(ctx, err, format, args...)` for wrapping. Both need a `ctx`.

Read these files IN FULL before editing:
- `/workspace/pkg/domain/recurring_interval.go` — `ParseRecurringInterval(s string)` currently takes **no ctx** and uses `fmt.Errorf` at three sites (unknown interval, invalid number, unknown unit).
- `/workspace/pkg/ops/complete.go` — the only non-test caller, at roughly line 253. Lines ~253-257 hand-roll a "parse, and on error fall back to daily" block with a `slog.Warn`. That hand-rolled fallback is exactly what S3 asks to replace.
- `/workspace/pkg/domain/recurring_interval_test.go` — the existing Ginkgo suite; its calls must be updated for the new signature.
- `/workspace/pkg/cli/cli.go` — `resolveSessionMode` around line 414 uses `fmt.Errorf("invalid --mode value: …")` and currently has no ctx.
</context>

<requirements>
1. Change the signature to `func ParseRecurringInterval(ctx context.Context, s string) (RecurringInterval, error)` and replace all three `fmt.Errorf` calls:
   - `errors.Errorf(ctx, "unknown recurring interval: %q", s)`
   - `errors.Wrapf(ctx, err, "invalid recurring interval number in %q", s)` (note: `Wrapf` replaces the `%w` verb — do not keep `%w`)
   - `errors.Errorf(ctx, "unknown unit %q in recurring interval %q", matches[2], s)`
   Add the `context` and `github.com/bborbe/errors` imports; drop `fmt` if it becomes unused.
2. Add a sibling in the same file:
   ```go
   // ParseRecurringIntervalDefault parses s and returns def when s cannot be parsed.
   func ParseRecurringIntervalDefault(ctx context.Context, s string, def RecurringInterval) RecurringInterval
   ```
   It calls `ParseRecurringInterval` and returns `def` on error. Give it a GoDoc comment starting with the function name.
3. Update `/workspace/pkg/ops/complete.go`:
   - The line-253 call sits inside `calculateNextDeferDate(recurring string, now time.Time) libtime.DateOrDateTime`, which has **no ctx**. **Add `ctx context.Context` as its first parameter** — this signature change is explicitly authorized (see requirement 6). Its only call site is `handleRecurringTask` at ~line 190, which already has `ctx` in scope. Do NOT use `context.Background()` or `context.TODO()` here: no production code in `pkg/` does, and it would defeat the purpose of the change.
   - Replace the hand-rolled parse-then-fallback block (lines ~253-258) with a single `ParseRecurringIntervalDefault` call.
   - **Preserve the existing `slog.Warn("unknown recurring interval, treating as daily", "interval", recurring)`** verbatim on the fallback path, in the caller — the operator must still learn the value was invalid. `pkg/ops/complete_test.go` already exercises invalid-recurring cases (lines ~562, 575, 589); those specs must still pass unchanged.
4. Update `/workspace/pkg/domain/recurring_interval_test.go` for the new signature (pass a context). Add at least one spec for `ParseRecurringIntervalDefault`: one case where parsing succeeds and one where it falls back to the default.
5. In `/workspace/pkg/cli/cli.go`, thread `ctx context.Context` into `resolveSessionMode` and replace its `fmt.Errorf` with `errors.Errorf(ctx, …)`. Update its call sites to pass the ctx already in scope.
6. Exactly **four** signature changes are authorized, and no others: `ParseRecurringInterval` (add ctx), the new `ParseRecurringIntervalDefault`, `calculateNextDeferDate` (add ctx, per requirement 3), and `resolveSessionMode` (add ctx, per requirement 5). Findings M4 (ctx in domain setters) and S4 (interface splits) remain OUT OF SCOPE.
7. Do NOT touch any file marked `// Code generated ... DO NOT EDIT.`
8. Add bullets to the `## Unreleased` section of `/workspace/CHANGELOG.md` (create the section at the top if absent).
</requirements>

<verification>
- `cd /workspace && make precommit` exits 0.
- `grep -c 'fmt.Errorf' /workspace/pkg/domain/recurring_interval.go` returns 0.
- `grep -c 'fmt.Errorf' /workspace/pkg/cli/cli.go` returns 0.
- `grep -n 'func ParseRecurringIntervalDefault' /workspace/pkg/domain/recurring_interval.go` returns exactly one match.
- `grep -n 'ParseRecurringIntervalDefault' /workspace/pkg/ops/complete.go` returns at least one match.
- `grep -n 'ParseRecurringIntervalDefault' /workspace/pkg/domain/recurring_interval_test.go` returns at least one match.
- `cd /workspace && git diff --name-only` lists ONLY: `CHANGELOG.md`, `pkg/cli/cli.go`, `pkg/domain/recurring_interval.go`, `pkg/domain/recurring_interval_test.go`, `pkg/ops/complete.go`.
</verification>

<allowed_files>
- /workspace/pkg/domain/recurring_interval.go
- /workspace/pkg/domain/recurring_interval_test.go
- /workspace/pkg/ops/complete.go
- /workspace/pkg/cli/cli.go
- /workspace/CHANGELOG.md
</allowed_files>
