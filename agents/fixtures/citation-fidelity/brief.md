---
page_type: task
status: in_progress
phase: planning
---

Tags: [[Task]]

---
<!-- FIXTURE — not a real task. This brief is the constructed input for §17 Citation
     Fidelity's exercised run. It carries ONE DELIBERATE FAILURE AND ONE VALID CLAIM
     for each of the four claim classes, so a run that flags every citation fails the
     negative control and a run that flags none fails the positive one.
     See agents/task-auditor.md § 17. Do not "fix" the citations below. -->

The release-driver page is out of date and needs correcting.

# Impact

Readers auditing a release against `agents/fixtures/citation-fidelity/artifact.md` look for a
per-prompt tag that never fires. The page is otherwise correct, so the defect is narrow.

# Citations under test

Each line below is one claim. Four are deliberately false and four are valid.

- **Path — valid.** The artifact is at `agents/fixtures/citation-fidelity/artifact.md`.
- **Path — FALSE.** The changelog is at `agents/fixtures/citation-fidelity/changelog.md`.
- **Line — valid.** `artifact.md:8` reads "Only `github-releaser-agent` is live here."
- **Line — FALSE.** `artifact.md:3` reads "vault-cli runs three release drivers."
- **Quote — valid.** The artifact contains the sentence "vault-cli runs two release drivers."
- **Quote — FALSE.** The artifact states "dark-factory owns the release for this repo."
- **Count — valid.** The artifact names two drivers.
- **Count — FALSE.** The artifact names three drivers.

# Success Criteria

- [ ] The `github-releaser-agent` row is unchanged and still names `.maintainer.yaml: release.autoRelease: true`.
- [ ] The dark-factory row no longer presents its driver as live for this repo.

# Definition of Done

- [ ] The corrected page names exactly one live driver.

# Tasks

- [ ] Correct the dark-factory row in `agents/fixtures/citation-fidelity/artifact.md`.

# Progress

- Fixture created for the §17 exercised run.
