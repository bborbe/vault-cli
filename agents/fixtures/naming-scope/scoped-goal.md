---
page_type: goal
status: in_progress
created: 2026-10-23
priority: 2
---
Tags: [[Goal]]

---
<!-- FIXTURE — not a real goal. Clean control for goal-auditor's name-vs-scope check
     (docs/goal-writing.md § Naming Method, Goal Scope Fit smell #10). Same body as
     symptom-named-goal.md; only the title differs, so it names the end state the
     criteria describe. A correct audit emits NO name-vs-scope WARN here. -->

The operator's attention stays on managers rather than on every running worker, and work continues without the operator re-showing the same gates.

# Impact

Workers ask the operator directly for every decision, so every worker is a tab to watch and every gate is re-shown. The recurring gates are one visible symptom; the root is that no manager–worker channel exists, so the operator is the only path between a worker and its next step.

# Success Criteria

- [ ] Every running worker reports to a manager; the operator sees managers, not individual workers
- [ ] A gate raised by a worker is answered by its manager without the operator re-showing it
- [ ] New workers are started by their manager, not by the operator

# Non-goals

- Changing the attention board's rendering
- Retiring the operator's manual override

# Tasks

- [ ] [[Give Each Worker a Manager Channel]] — closes SC1

# Definition of Done

- [ ] All success criteria checked with evidence recorded in `# Results`
