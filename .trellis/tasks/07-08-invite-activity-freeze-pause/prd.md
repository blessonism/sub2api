# brainstorm: 修复邀请活动冻结与暂停状态

## Goal

修复邀请活动在管理端冻结榜单后仍显示为“进行中”的问题，并补齐运营可中途临时关闭活动、之后恢复活动的能力，避免用户端继续展示已冻结或已暂停的活动入口。

## What I already know

* 用户反馈：冻结榜单后进度仍显示进行中。
* 用户反馈：当前无法中途暂时关闭邀请活动。
* 后端已有 `CampaignStatusCancelled` / `CampaignStatusTerminated`，但它们用于删除/归档语义，不适合“暂时关闭后恢复”。
* 前端管理页已经把 `frozen` 当作有效生命周期状态使用，例如冻结步骤、最终结算条件和生命周期展示。
* 后端 `FreezeLeaderboard` 当前只保存 `end_frozen` 榜单快照，没有更新 `campaigns.status`。
* 数据库 `campaigns_status_valid` 约束当前不包含 `frozen` 或 `paused`。
* 用户端活动中心只在 `/campaigns/active` 返回活动时展示邀请活动，后端 `GetActiveCampaign` 只返回 `active` / `warmup` 且在活动时间内的活动。

## Assumptions

* “冻结榜单”应进入 `frozen` 状态，用户端不再把它当成进行中的活动；管理端仍可继续最终结算和发放。
* “暂时关闭”应实现为 `paused` 状态：不对用户端展示，不删除业务数据；后续可恢复到基于当前时间推导的 `warmup`、`active` 或 `auditing`。
* 暂停期间不接受新的邀请活动计入；恢复后若仍处于活动时间内，再继续对用户展示和计入。

## Requirements

* 冻结榜单必须持久化活动状态为 `frozen`，并保持榜单快照保存。
* 管理端应提供暂停、恢复、取消冻结按钮，且高风险确认文案可审计。
* 用户端活动中心不得把 `frozen` 或 `paused` 活动显示成“进行中”。
* 后端应提供暂停/恢复/取消冻结 API，并限制非法状态流转。
* 数据库迁移应补齐 `frozen`、`paused` 状态约束。
* 前后端中英文状态与提示文案应一致。

## Acceptance Criteria

* [x] 管理端冻结活动后，活动状态变为 `frozen`，生命周期不再显示“进行中”。
* [x] 管理端可对 `frozen` 活动执行“取消冻结/解冻”，且仅在不存在最终结算结果和发放批次时允许执行。
* [x] 管理端可将 `warmup` / `active` 活动暂停，暂停后用户端不再显示邀请活动。
* [x] 管理端可恢复 `paused` 活动，恢复后按当前时间落到 `warmup` / `active` / `auditing`。
* [x] `frozen` 活动取消冻结后，也按当前时间落到 `warmup` / `active` / `auditing`；若将恢复为 `active`，则必须不存在其他进行中的邀请活动。
* [x] 已取消、已终止、已发放活动不能暂停或恢复。
* [x] 相关后端单元测试、前端类型检查或等价范围验证通过。

## Definition of Done

* Tests added/updated for state transitions where existing test harness supports it.
* Lint / typecheck / targeted tests green.
* Trellis context includes downstream fork workflow for implement/check.
* Rollback path: revert code and migration if the `paused` lifecycle is not accepted before deployment.

## Out of Scope

* 不重做邀请活动奖励计算规则。
* 不引入多活动同时进行。
* 不改变删除/归档现有语义。
* 不处理生产数据手工迁移或线上部署。

## Technical Notes

* Relevant frontend files:
  * `frontend/src/views/admin/CampaignRewardsView.vue`
  * `frontend/src/views/user/CampaignRewardsView.vue`
  * `frontend/src/utils/campaignLifecycle.ts`
  * `frontend/src/api/admin/campaigns.ts`
  * `frontend/src/i18n/locales/zh.ts`
  * `frontend/src/i18n/locales/en.ts`
* Relevant backend files:
  * `backend/internal/service/campaign_service.go`
  * `backend/internal/repository/campaign_repo.go`
  * `backend/internal/handler/admin/campaign_handler.go`
  * `backend/internal/server/routes/admin.go`
  * `backend/migrations/180_campaign_rewards_settlement.sql`
* Downstream fork workflow:
  * base branch: `custom/main`
  * task branch metadata: `fix/invite-activity-freeze-pause`
* Verification:
  * `pnpm --dir frontend typecheck`
  * `go test ./internal/service ./internal/handler/admin ./internal/server -count=1` from `backend/`
