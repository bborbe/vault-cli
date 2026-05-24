---
status: draft
created: "2026-05-24T00:00:00Z"
---

<summary>
- Replace context.Background() in storage read callbacks with the caller's context
- Thread ctx through libtime.ParseTime calls in storage
- Ensure storage operations respect context cancellation
</summary>

<objective>
Fix context.Background() usage in pkg/storage/. The storage layer should propagate the caller's context so that operations can be cancelled and respect deadlines.
</objective>

<context>
Read `CLAUDE.md` for project conventions.
Read `go-context-cancellation-in-loops.md` for proper context propagation.

Files to read before making changes:
- `pkg/storage/decision.go` (~line 61) — libtime.ParseTime called with context.Background()
- `pkg/storage/base.go` (~line 99) — context.Background() in walk callback
- `pkg/domain/frontmatter_map.go` (~line 72) — libtime.ParseTime with context.Background()

The fix typically involves threading ctx through the call chain from the storage method to the parse call.
</context>

<requirements>
### 1. Fix pkg/storage/decision.go line 61

The `libtime.ParseTime(context.Background(), v)` is inside `ListDecisions`. Thread ctx through ListDecisions to the parse call.

Read the method signature of ListDecisions to understand how to pass ctx through.

Change:
```go
if t, err := libtime.ParseTime(context.Background(), v); err == nil {
```
to use ctx instead of context.Background().

### 2. Fix pkg/storage/base.go

The walk callback at line 99 uses context.Background() in the return. Change to use ctx which should be passed to the callback or the method wrapping the walk.

### 3. Fix pkg/domain/frontmatter_map.go line 72

The libtime.ParseTime call uses context.Background(). If this method is called from ops code with ctx available, thread ctx through. If this is a domain method without ctx, consider whether ctx should be added as a parameter, or whether this is an acceptable exception since domain validation is typically fast.

For frontmatter_map.go: Read the call site to determine if ctx is available. If so, thread it through. If not, document why this is an acceptable exception.

### 4. Ensure libtime import

Ensure the libtime package is imported correctly.
</requirements>

<constraints>
- Only change files in this repo
- Do NOT commit — dark-factory handles git
- Existing tests must still pass
- Context propagation is the goal — storage operations should respect cancellation
</constraints>

<verification>
```
make precommit
```
</verification>
