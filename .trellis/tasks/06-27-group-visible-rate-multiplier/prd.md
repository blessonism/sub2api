# feature: 分组倍率双口径

## Goal

新增“实际倍率”和“用户可见倍率”双口径能力：`rate_multiplier` 继续作为真实扣费倍率，新增 `visible_rate_multiplier` 作为用户侧展示倍率。用户看到可见倍率，但余额扣减、`actual_cost`、管理员统计和对账继续按真实倍率计算。

## What I already know

- 现有真实扣费倍率由 `user_group_rate_multipliers(user_id, group_id).rate_multiplier` 覆盖 `groups.rate_multiplier`，网关通过 `userGroupRateResolver.Resolve` 解析。
- 用户排行榜当前使用 `discount_rate_multiplier` 展示倍率，近期任务正在确保它等于当前真实生效倍率；本任务要正式拆出展示口径。
- 用户侧 `/groups/rates` 现在返回用户专属真实倍率，Key 页面和可用渠道页用它覆盖分组倍率展示；本任务必须改为可见倍率口径。
- 用户用量明细现在暴露 `usage_logs.rate_multiplier`，这是请求发生时的真实倍率快照；本任务需要新增可见倍率快照避免历史明细泄露真实倍率。
- 用户排行榜未命中可见倍率时按产品决策显示 `-`，不回退展示真实倍率。
- 用户已确认：用户侧金额保持真实，不按可见倍率伪造。
- 本仓库是 `Wei-Shaw/sub2api` 下游二开，变更必须遵守 `.trellis/spec/guides/downstream-fork-workflow.md`。

## Requirements

- 数据层新增：
  - `groups.visible_rate_multiplier`：分组默认可见倍率，`NULL` 表示跟随实际分组倍率。
  - `user_group_rate_multipliers.visible_rate_multiplier`：用户在分组下的专属可见倍率，`NULL` 表示继承分组可见倍率或实际倍率。
  - `usage_logs.visible_rate_multiplier`：请求发生时的可见倍率快照。
- 倍率解析规则：
  - 实际扣费倍率：用户专属真实倍率优先，否则分组真实倍率。
  - 用户可见倍率：用户专属可见倍率优先，其次分组可见倍率，最后真实扣费倍率。
- 网关计费链路必须继续使用真实倍率计算 `actual_cost`、余额扣减和订阅额度扣减。
- 写入 usage log 时必须记录 `visible_rate_multiplier` 快照；旧数据为空时用户侧回退真实倍率。
- 普通用户接口不得返回真实倍率：
  - `/groups/available` 的 `rate_multiplier` 对用户返回可见倍率。
  - `/groups/rates` 返回当前用户各分组的可见倍率覆盖结果。
  - 用户排行榜 `discount_rate_multiplier` 返回可见倍率；无用户专属/分组可见倍率时返回 `NULL`，前端展示 `-`。
  - 用户用量列表、tooltip、CSV 中的 `rate_multiplier` 返回可见倍率快照。
- 管理员接口允许读写双倍率：
  - 分组 DTO 返回 `rate_multiplier` 和 `visible_rate_multiplier`。
  - 分组专属倍率弹窗支持实际扣费倍率和用户可见倍率双列编辑。
  - 管理员用户分组配置弹窗支持用户级实际倍率和可见倍率。
  - 管理员用量日志继续展示真实 `rate_multiplier`，可额外展示 `visible_rate_multiplier`。

## Acceptance Criteria

- [ ] 旧 usage log 没有可见倍率快照时，用户用量明细回退显示真实倍率；用户排行榜无可见倍率配置时显示 `-`。
- [ ] 分组配置可见倍率后，没有用户专属可见倍率的用户看到分组可见倍率。
- [ ] 用户配置专属可见倍率后，用户侧展示优先使用专属可见倍率。
- [ ] 可见倍率与真实倍率不一致时，`actual_cost`、余额扣减和管理员统计仍按真实倍率。
- [ ] 用户侧接口和前端页面不泄露真实倍率字段。
- [ ] 管理员侧可以查看和编辑实际倍率与可见倍率。
- [ ] 后端与前端针对性测试通过。

## Technical Approach

- 复用现有分组与用户分组专属配置模型，不新增独立暗改表。
- 扩展 `Group`、`UserGroupRateEntry`、`GroupRateMultiplierInput`、`UsageLog` 等服务类型，保持 `RateMultiplier` 表示真实倍率，新增 `VisibleRateMultiplier *float64`。
- 新增可见倍率解析方法，优先复用现有用户分组倍率仓储，避免重复 SQL 逻辑。
- 用户侧 DTO 映射或服务层输出时改写展示倍率；管理员 DTO 保留真实倍率并额外返回可见倍率。
- 前端复用现有 `GroupBadge`、分组倍率弹窗和用户分组配置弹窗，增加可见倍率输入与继承语义。

## Definition of Done

- 新增迁移、后端类型/仓储/服务/DTO/接口实现。
- 更新前端 API 类型、管理弹窗、用户展示组件。
- 覆盖后端 resolver/计费/用户接口与前端关键 UI 测试。
- 不修改历史 `actual_cost`、余额流水或订阅额度记录。

## Out of Scope

- 不伪造用户侧金额。
- 不把该功能接入自动策略配置项；自动策略仍负责真实倍率。
- 不新增生产操作步骤；迁移随部署执行。

## Technical Notes

- 主要后端文件：`backend/internal/service/user_group_rate.go`、`backend/internal/service/user_group_rate_resolver.go`、`backend/internal/repository/user_group_rate_repo.go`、`backend/internal/repository/usage_log_repo.go`、`backend/internal/handler/dto/*`。
- 主要前端文件：`frontend/src/api/groups.ts`、`frontend/src/api/admin/groups.ts`、`frontend/src/components/admin/group/GroupRateMultipliersModal.vue`、`frontend/src/components/admin/user/UserAllowedGroupsModal.vue`、用户侧 Key/渠道/用量/排行榜页面。
- 实现和检查上下文包含下游 fork 工作流、后端/前端 spec、代码复用与跨层思考指南。
