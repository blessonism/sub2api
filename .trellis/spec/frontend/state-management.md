# State Management

> How state is managed in this project.

---

## Overview

<!--
Document your project's state management conventions here.

Questions to answer:
- What state management solution do you use?
- How is local vs global state decided?
- How do you handle server state?
- What are the patterns for derived state?
-->

(To be filled by the team)

---

## State Categories

<!-- Local state, global state, server state, URL state -->

(To be filled by the team)

---

## When to Use Global State

<!-- Criteria for promoting state to global -->

(To be filled by the team)

---

## Server State

<!-- How server data is cached and synchronized -->

### Convention: Separate Saved Server State From Editable Drafts

**Scope / Trigger**: Use this when a view lets users edit a server-backed policy or configuration while other UI controls need to act on the last saved server value.

**Contract**:
- Keep a canonical saved ref for the last successful server response, e.g. `savedMonitoringPolicy`.
- Keep editable form state separately, e.g. `monitoringPolicyForm`.
- Header/status bars, background runner controls, and optimistic toggles must use the saved ref unless they explicitly represent unsaved draft input.
- Settings panels, derived draft previews, and save payloads must use the editable form state.

**Why**: Mixing saved state and draft state can submit unrelated unsaved form edits from inline controls, or show mismatched derived values such as a draft interval with a stale saved threshold.

**Good/Base/Bad Cases**:
- Good: Inline auto-schedule switch builds its payload from `savedMonitoringPolicy` and only overrides the target boolean field.
- Base: Settings page derived labels use `monitoringPolicyForm` so they update as the user edits inputs before saving.
- Bad: A top-level runner switch calls `updatePolicy(normalizedFormPayload())`, because it may persist unrelated unsaved draft fields.

**Tests Required**:
- Inline controls must have regression tests proving unrelated draft fields are not submitted.
- Derived values must have tests for both saved-state consumers and draft-state consumers when the same policy field feeds both surfaces.

---

## Common Mistakes

<!-- State management mistakes your team has made -->

(To be filled by the team)
