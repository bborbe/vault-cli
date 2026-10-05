---
status: idea
kind: bug
---

# Build Failure: bborbe/vault-cli

Filed automatically by the build-fix agent for the CI episode `87de49d1fcfe04d1403418807ea743a456909b67`.

## Summary

The default-branch build for `bborbe/vault-cli` is failing; the build-fix diagnosis classified this as a code/test bug (verdict `file_spec`).

## Reproduction

Failing workflow(s): test

Episode SHA: `87de49d1fcfe04d1403418807ea743a456909b67`

Log evidence:

```text
| Workflow | Job | Failed Step | Run |
|---|---|---|---|
| CI | test | Run precommit checks | [Run](https://github.com/bborbe/vault-cli/actions/runs/29486613583) |
```

## Expected vs Actual

**Expected:** green CI on the default branch.
**Actual:** `The codecov step in the test workflow fails with 'No coverage reports found' because the Go project has no coverage.py or gcov data — this is a workflow configuration bug (wrong coverage tool for Go), not a dependency drift or vulnerability issue.`

## Why this is a bug

The default-branch build is the repository's quality gate; a red build blocks merges. Diagnosis: `The codecov step in the test workflow fails with 'No coverage reports found' because the Go project has no coverage.py or gcov data — this is a workflow configuration bug (wrong coverage tool for Go), not a dependency drift or vulnerability issue.`
