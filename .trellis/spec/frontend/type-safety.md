# Type Safety

> Type safety patterns in this project.

---

## Overview

<!--
Document your project's type safety conventions here.

Questions to answer:
- What type system do you use?
- How are types organized?
- What validation library do you use?
- How do you handle type inference?
-->

(To be filled by the team)

---

## Type Organization

<!-- Where types are defined, shared types vs local types -->

(To be filled by the team)

---

## Validation

<!-- Runtime validation patterns (Zod, Yup, io-ts, etc.) -->

(To be filled by the team)

---

## Common Patterns

<!-- Type utilities, generics, type guards -->

### Pattern: User dashboard API-backed pages

When adding a user-facing page backed by `/usage/dashboard/*` APIs:

- Define the response interfaces in `frontend/src/api/usage.ts` next to the request function.
- Keep frontend field names aligned with backend JSON names; do not remap security-sensitive fields such as `masked_email` into ambiguous names like `email`.
- Add route metadata in `frontend/src/router/index.ts`, including `requiresAuth`, `titleKey`, and `descriptionKey`.
- Add the page to `frontend/src/components/layout/AppSidebar.vue` through `buildSelfNavItems` when both regular users and admins' "My Account" section should see it.
- Add route prefetch coverage when the page should be eagerly prefetched.
- Add zh/en i18n keys for route titles, navigation labels, loading/error/empty states, and metric labels.

Required tests:

- API test verifies the endpoint path and typed method.
- View test covers loading, success, empty/error states, and important display fields.
- Sidebar/prefetch tests cover new route visibility and prefetch registration.

---

## Forbidden Patterns

<!-- any, type assertions, etc. -->

(To be filled by the team)
