# Configurable Purchase Plan Link

## Goal

Allow the purchase/subscription entry link to be configured from the existing settings surface, so operators can change where users are sent to buy套餐 without code changes.

## What I already know

- User requested: “购买套餐的跳转链接能够在设置中设置”.
- A user payment flow exists in `frontend/src/views/user/PaymentView.vue` and subscription plan cards in `frontend/src/components/payment/SubscriptionPlanCard.vue`.
- Admin payment plan management exists in `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`.
- This downstream fork must keep custom work on `custom/main`-based feature branches and include `.trellis/spec/guides/downstream-fork-workflow.md` in implementation/check context.

## Assumptions (temporary)

- “设置” means the existing admin system settings/configuration UI and backend setting/config mechanism, if present.
- The configured link should affect the user-facing “购买套餐/订阅” entry without requiring redeploy.
- Empty setting should preserve the current in-app purchase behavior unless the existing product pattern suggests otherwise.

## Open Questions

- Resolved: this task applies to the sidebar “购买套餐 / Buy Plan” external entry. The built-in `/purchase` recharge/subscription flow remains unchanged.

## Requirements (evolving)

- Reuse the existing `purchase_subscription_enabled` and `purchase_subscription_url` settings as the single source of truth.
- Add the purchase plan jump URL controls to the admin Settings page.
- Use the configured URL for the sidebar “购买套餐 / Buy Plan” external link.
- Hide the external buy-plan link when the setting is disabled or the URL is empty.
- Preserve the built-in `/purchase` recharge/subscription flow and its existing payment feature flag behavior.

## Acceptance Criteria (evolving)

- [x] Admin can set and save the purchase plan jump URL in settings.
- [x] User sidebar buy-plan entry reads the setting and opens that configured URL.
- [x] Disabled or empty URL hides the external sidebar link while preserving the current `/purchase` flow.
- [x] Frontend tests cover settings save payload and sidebar link wiring.

## Definition of Done (team quality bar)

- Tests added/updated where appropriate.
- Typecheck / targeted tests pass or blockers are documented.
- Docs/task context updated if behavior changes.
- Rollout/rollback considered if risky.

## Out of Scope (explicit)

- Reworking subscription plans, order creation, or payment providers.
- Adding a new payment provider integration.
- Changing production secrets or deployment config.

## Technical Notes

- Must follow `.trellis/spec/guides/downstream-fork-workflow.md`.
- Initial code search found payment UI at `frontend/src/views/user/PaymentView.vue` and `frontend/src/components/payment/SubscriptionPlanCard.vue`.
- Backend already exposed `purchase_subscription_enabled` and `purchase_subscription_url` in admin/public settings, so this implementation only needed frontend wiring and tests.
- Validation completed with targeted Vitest coverage and frontend typecheck.
