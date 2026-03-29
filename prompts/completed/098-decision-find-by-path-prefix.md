---
status: completed
summary: Added path-suffix/prefix matching to FindDecisionByName by extracting a findByPathMatch helper to stay within cognitive complexity limits, with 5 new test cases covering ambiguous names, full path, partial suffix, partial prefix, and alternate-path resolution.
container: vault-cli-098-decision-find-by-path-prefix
dark-factory-version: v0.69.0
created: "2026-03-29T12:17:37Z"
queued: "2026-03-29T12:21:14Z"
started: "2026-03-29T12:21:19Z"
completed: "2026-03-29T12:29:25Z"
---

<summary>
- Users can pass a path-containing identifier (e.g. "40 Trading/Weekly/2026-W12 - Review") to disambiguate decisions with similar short names
- Partial path suffixes work: "Trading/Weekly/2026-W12" matches "40 Trading/Weekly/2026-W12 - Review"
- Short names without slashes still work exactly as before when unambiguous
- Ambiguous short names without a path still produce a clear error listing all matches
- Path-prefix matching is tried before falling back to substring partial matching
</summary>

<objective>
Fix `FindDecisionByName` in `pkg/storage/decision.go` to support path-based disambiguation. When the identifier contains `/`, add a path-suffix matching step between exact match and substring partial match. This lets users resolve ambiguous decision names like "2026-W12" by passing "40 Trading/Weekly/2026-W12 - Review" or "Trading/Weekly/2026-W12".
</objective>

<context>
Read CLAUDE.md and `docs/development-patterns.md` for project conventions.

Key files to read before making changes:
- `pkg/storage/decision.go` — `FindDecisionByName` method (lines ~105-158), the single fix location
- `pkg/storage/decision_test.go` — existing `FindDecisionByName` tests using Ginkgo v2/Gomega with temp vault dirs
- `pkg/ops/decision_ack.go` — calls `FindDecisionByName` (no changes needed, but read to understand call site)
</context>

<requirements>
### 1. Add path-suffix matching to `FindDecisionByName` in `pkg/storage/decision.go`

In the `FindDecisionByName` method, between the exact-match loop (which checks `filepath.ToSlash(dec.Name) == normalizedName`) and the partial substring match loop (which uses `strings.Contains`), insert a new matching step:

```go
// Path-suffix match: when identifier contains '/', try matching against the end of the decision path.
if strings.Contains(normalizedName, "/") {
    var pathMatches []*domain.Decision
    lowerNorm := strings.ToLower(normalizedName)
    for _, dec := range decisions {
        lowerDec := strings.ToLower(filepath.ToSlash(dec.Name))
        if strings.HasSuffix(lowerDec, lowerNorm) || strings.HasPrefix(lowerDec, lowerNorm) {
            pathMatches = append(pathMatches, dec)
        }
    }
    if len(pathMatches) == 1 {
        return pathMatches[0], nil
    }
    // If path matching found multiple or zero results, fall through to substring match below
}
```

The logic:
1. Only activate when the identifier contains `/` (indicating a path, not a bare name)
2. Case-insensitive comparison, matching against both prefix and suffix of the decision's full path
3. If exactly one match, return it immediately
4. If zero or multiple matches, fall through to existing substring matching (which will produce the appropriate "not found" or "ambiguous" error)

### 2. Add test cases to `pkg/storage/decision_test.go`

Inside the existing `Describe("FindDecisionByName", ...)` block, add a new `Context` that sets up the ambiguous-name scenario and tests path-based resolution. The test setup needs decisions in subdirectories with overlapping base names.

Add these files in `BeforeEach` (in addition to or replacing the existing setup for these specific tests):

```go
Context("with ambiguous names in different paths", func() {
    BeforeEach(func() {
        tradingDir := filepath.Join(vaultPath, "40 Trading", "Weekly")
        periodicDir := filepath.Join(vaultPath, "60 Periodic Notes", "Weekly")
        Expect(os.MkdirAll(tradingDir, 0755)).To(Succeed())
        Expect(os.MkdirAll(periodicDir, 0755)).To(Succeed())

        content1 := "---\nneeds_review: true\ntype: architecture\n---\n# Review\n"
        content2 := "---\nneeds_review: true\ntype: data\n---\n# Review\n"

        Expect(os.WriteFile(
            filepath.Join(tradingDir, "2026-W12 - Review.md"),
            []byte(content1), 0600,
        )).To(Succeed())
        Expect(os.WriteFile(
            filepath.Join(periodicDir, "2026-W12.md"),
            []byte(content2), 0600,
        )).To(Succeed())
    })

    It("returns ambiguous error for short name matching multiple decisions", func() {
        _, err := store.FindDecisionByName(ctx, vaultPath, "2026-W12")
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("ambiguous match"))
    })

    It("resolves with full path", func() {
        d, err := store.FindDecisionByName(ctx, vaultPath, "40 Trading/Weekly/2026-W12 - Review")
        Expect(err).NotTo(HaveOccurred())
        Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
    })

    It("resolves with partial path suffix", func() {
        d, err := store.FindDecisionByName(ctx, vaultPath, "Trading/Weekly/2026-W12 - Review")
        Expect(err).NotTo(HaveOccurred())
        Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
    })

    It("resolves with partial path prefix", func() {
        d, err := store.FindDecisionByName(ctx, vaultPath, "40 Trading/Weekly/2026-W12")
        Expect(err).NotTo(HaveOccurred())
        Expect(d.Name).To(Equal("40 Trading/Weekly/2026-W12 - Review"))
    })

    It("resolves the other decision with its path", func() {
        d, err := store.FindDecisionByName(ctx, vaultPath, "60 Periodic Notes/Weekly/2026-W12")
        Expect(err).NotTo(HaveOccurred())
        Expect(d.Name).To(Equal("60 Periodic Notes/Weekly/2026-W12"))
    })
})
```

Also add a test that confirms existing behavior is not regressed — short unambiguous names still work:

```go
It("still resolves short name when unambiguous", func() {
    // Uses the original BeforeEach with Alpha/Beta decisions
    d, err := store.FindDecisionByName(ctx, vaultPath, "alpha")
    Expect(err).NotTo(HaveOccurred())
    Expect(d.Name).To(Equal("Alpha Decision"))
})
```

This test already exists (the "returns decision on single partial match" test), so no action needed — just verify it still passes.
</requirements>

<constraints>
- Do NOT modify `pkg/ops/decision_ack.go` or any CLI layer — only `pkg/storage/decision.go` and `pkg/storage/decision_test.go`
- Path-suffix matching must only activate when identifier contains `/` — bare names must follow existing exact-then-substring logic
- All matching must be case-insensitive (consistent with existing substring match)
- Existing tests must still pass unchanged
- Do NOT commit — dark-factory handles git
</constraints>

<verification>
```
make precommit
```

```
# Confirm path-suffix matching code was added
grep -n 'HasSuffix\|HasPrefix' pkg/storage/decision.go
# expected: two lines inside FindDecisionByName
```

```
# Confirm new test cases exist
grep -c 'ambiguous names in different paths\|resolves with full path\|resolves with partial path' pkg/storage/decision_test.go
# expected: 4 or more matches
```

```
# Run only the decision storage tests for fast feedback
go test -v -mod=vendor ./pkg/storage/... -run Decision
```
</verification>
