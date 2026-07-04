# Component Guidelines

> How components are built in this project.

---

## Overview

<!--
Document your project's component conventions here.

Questions to answer:
- What component patterns do you use?
- How are props defined?
- How do you handle composition?
- What accessibility standards apply?
-->

(To be filled by the team)

---

## Component Structure

<!-- Standard structure of a component file -->

(To be filled by the team)

---

## Props Conventions

<!-- How props should be defined and typed -->

### Convention: DataTable server-side sorting control

**What**: When a page uses `DataTable` with `server-side-sort`, the parent view should own the effective sort state and pass it back through `sortKey` and `sortOrder` when the sort key can also be changed by custom controls.

**Why**: Custom column menus, such as admin usage "today / 30 days" sorting, must keep the header icon, persisted state, query params, and backend results aligned. Local-only sorting of the current `data` array is forbidden for server-side user-visible lists.

**Example**:
```vue
<DataTable
  :server-side-sort="true"
  :sort-key="tableSortKey"
  :sort-order="sortState.sort_order"
  @sort="handleSort"
/>
```

**Related**: For admin user usage sorting, the backend must apply usage aggregation order before pagination.

---

## Styling Patterns

<!-- How styles are applied (CSS modules, styled-components, Tailwind, etc.) -->

(To be filled by the team)

---

## Accessibility

<!-- A11y requirements and patterns -->

(To be filled by the team)

---

## Common Mistakes

<!-- Component-related mistakes your team has made -->

### Common Mistake: Credential dialogs closing during local mode switches

**Symptom**: Browser password managers, credential pickers, or local auth-mode buttons may close a `BaseDialog` form while users select credential-related options, causing unsaved input to be lost.

**Cause**: `BaseDialog` closes on Escape by default, and credential forms often include mode-switch buttons plus username/password fields inside a form. If pointer/mouse/touch/click/input/change events from browser credential UI are allowed to propagate, real browser behavior can misclassify local credential selection as a modal-close interaction. Global foreground-activity listeners can also observe credential input events and trigger refresh/reporting chains that reset local dialog state.

**Fix**: For credential-entry dialogs, pass `:close-on-escape="false"` and `:close-on-click-outside="false"`, mark the credential form with `data-ignore-foreground-activity="true"`, stop propagation on local mode-switch controls, and isolate username/password fields from pointer/mouse/touch/click/input/change propagation. Keep explicit close controls such as cancel, close button, or successful submit.

```vue
<BaseDialog
  :show="credentialDialogOpen"
  :close-on-escape="false"
  :close-on-click-outside="false"
  @close="closeCredentialDialog"
>
  <form data-ignore-foreground-activity="true" @pointerdown.stop @touchstart.stop @click.stop>
    <button
      type="button"
      @pointerdown.stop.prevent="setCredentialMode('password')"
      @pointerup.stop
      @mousedown.stop
      @mouseup.stop
      @touchstart.stop.prevent="setCredentialMode('password')"
      @touchend.stop
      @click.stop.prevent="setCredentialMode('password')"
    >
      账号密码
    </button>

    <div @pointerdown.stop @pointerup.stop @mousedown.stop @mouseup.stop @touchstart.stop @touchend.stop @click.stop @input.stop @change.stop>
      <input autocomplete="username" />
      <input type="password" autocomplete="current-password" />
    </div>
  </form>
</BaseDialog>
```

**Prevention**: Add a view test that opens the dialog, dispatches pointer/mouse/touch/click events on the credential mode switch and credential inputs, dispatches input/change events, enters values, and asserts the dialog remains visible. Add a store test for any global activity listener to ensure events inside `data-ignore-foreground-activity="true"` do not trigger reporting.
