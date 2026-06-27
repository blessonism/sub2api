# brainstorm: 管理员查看全局兑换记录

## Goal

管理员需要一个全局页面查看用户最新的兑换/充值情况，信息口径应接近用户自己在兑换页看到的“最近活动”，但查询范围从单个用户扩展为全站用户，方便运营快速看到谁最近兑换了什么、兑换时间、兑换类型和金额/权益变化。

## What I already know

- 用户明确希望“有个页面能够看到用户最新的兑换情况”。
- 用户强调“就跟用户看到的兑换情况一样，不过我的是全局的”。
- 用户端已有 `/redeem` 页面，其中包含当前余额、兑换表单、兑换结果和“最近活动”。
- 后台已有 `/admin/redeem` 页面，但它更偏兑换码库存管理：生成、筛选、批量更新、删除、导出兑换码。
- 后台已有单用户“充值记录”弹窗 `UserBalanceHistoryModal`，可从用户列表/用量页打开，展示单个用户的余额、并发、订阅、管理员调整和 affiliate 转入记录。
- 后端已有单用户接口 `GET /api/v1/admin/users/:id/balance-history`，当前按用户维度分页查询。

## Assumptions (temporary)

- 这里的“兑换情况”不是让管理员帮用户输入兑换码，也不是管理未使用兑换码库存，而是看“已发生的兑换/充值流水”。
- 全局页面只展示最近已使用/已生效的记录，不展示未使用、过期、禁用兑换码。
- 页面需要显示用户身份信息，因为全局列表必须回答“是谁兑换的”。
- 初版可以复用现有充值记录的数据类型和视觉语义，但需要把数据源改成全局列表。

## Open Questions

- 当前无阻塞问题。

## Requirements (evolving)

- 新增一个管理员可访问的全局兑换/充值记录页面。
- 页面主内容是按时间倒序排列的全站已使用记录流。
- 每条记录至少展示：用户、记录类型、数值/权益、发生时间、兑换码摘要或来源说明。
- 类型口径应覆盖现有单用户记录弹窗中的类型：余额、推广余额转入、管理员余额调整、并发、管理员并发调整、订阅。
- 支持分页，避免一次性拉取大量记录。
- 支持按类型筛选；搜索用户邮箱/兑换码可作为候选增强。
- 不应弱化或替代现有 `/admin/redeem` 兑换码管理页。

## Acceptance Criteria (evolving)

- [ ] 管理员能从后台导航进入“全局兑换记录/充值记录”页面。
- [ ] 页面默认展示全站最新已使用记录，按发生时间倒序。
- [ ] 管理员能看到记录对应的用户邮箱或用户 ID。
- [ ] 页面展示口径与用户端最近活动、后台单用户充值记录保持一致。
- [ ] 分页、空态、加载态和错误态可用。
- [ ] 后端接口只暴露给管理员权限。

## Definition of Done (team quality bar)

- Tests added/updated (unit/integration where appropriate)
- Lint / typecheck / CI green
- Docs/notes updated if behavior changes
- Rollout/rollback considered if risky

## Out of Scope (explicit)

- 初步不做兑换码生成、批量更新、删除、导出等库存管理功能；这些继续由 `/admin/redeem` 承担。
- 初步不展示未使用、过期、禁用的兑换码库存记录。
- 初步不做财务报表级聚合统计，除非后续明确要统计总金额、按日汇总或按用户排名。
- 初步不改用户端兑换流程。

## Technical Notes

- Semble CLI 当前会话不可用，已按项目降级规则使用 `rg` 搜索。
- 下游二开任务必须遵循 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 相关用户页：`frontend/src/views/user/RedeemView.vue`。
- 相关后台兑换码管理页：`frontend/src/views/admin/RedeemView.vue`。
- 相关后台单用户记录组件：`frontend/src/components/admin/user/UserBalanceHistoryModal.vue`。
- 相关前端 API：`frontend/src/api/admin/users.ts` 的 `getUserBalanceHistory`。
- 相关后端接口：`backend/internal/handler/admin/user_handler.go` 的 `GetBalanceHistory`。
- 相关服务：`backend/internal/service/admin_service.go` 的 `GetUserBalanceHistory`。
- 相关仓储：`backend/internal/repository/redeem_code_repo.go` 的 `ListByUserPaginated`。
