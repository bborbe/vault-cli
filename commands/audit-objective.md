---
description: Audit objective file against Objective Writing Guide for quality, clarity, and completeness
argument-hint: <objective-file-path>
---

<objective>
Invoke the objective-auditor agent to audit the objective at $ARGUMENTS for compliance with Objective Writing Guide and OKR principles.
</objective>

<process>
1. Resolve objective file path from $ARGUMENTS
   - If no directory prefix, prepend `22 Objectives/`
   - If no `.md` extension, append `.md`
   - Fail with clear error if file does not exist or is unreadable

2. Invoke objective-auditor agent with resolved file path

3. Agent preparation
   - Read and internalize Objective Writing Guide
   - Read and internalize Objective Template
   - Extract objective from file

4. Validation checks (conceptual validity)
   - Outcome-focused (not task- or deliverable-based)
   - Duration appropriate (3-12 months, not goal/theme)
   - Strategic value (meaningful change, not BAU)
   - Measurable (success can be verified)
   - Scope appropriate (multiple goals, not task list)

5. Evaluation checks (OKR quality)
   - Qualitative and non-metric (numbers belong in Key Results)
   - Clear, specific, and unambiguous
   - Single primary intent (not overloaded)
   - Inspiring and directional
   - Alignment with theme and contributing goals

6. Scoring and severity assessment
   - Overall score: 0-100 scale
   - Dimension scores: clarity, focus, ambition, alignment
   - Severity levels: Critical, Major, Minor

7. Reporting
   - Summarize overall quality
   - List critical issues first
   - Provide actionable recommendations with example rewrites
   - Highlight strengths worth preserving
</process>

<success_criteria>
- Objective file path resolved correctly
- Audit covers all criteria from Objective Writing Guide
- Report includes:
  - Overall score (0-100)
  - Dimension scores (clarity, focus, ambition, alignment)
  - Severity-labeled issues (Critical, Major, Minor)
  - Concrete recommendations with example rewrites
  - Identified strengths
</success_criteria>
