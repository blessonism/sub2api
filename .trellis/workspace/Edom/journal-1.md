# Journal - Edom (Part 1)

> AI development session journal
> Started: 2026-06-18

---


## Session 1: 公告已读情况按阅读时间排序

**Date**: 2026-07-14
**Task**: 公告已读情况按阅读时间排序
**Branch**: `feature/user-public-group-account-binding`

### Summary

管理员公告已读情况改为数据库分页前按阅读时间倒序排列，未读用户置后，并同步前端默认排序与回归测试。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `c6fea247b` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: 管理员一键调整用户并发

**Date**: 2026-07-14
**Task**: 管理员一键调整用户并发
**Branch**: `feature/admin-user-concurrency-adjustment`

### Summary

为管理员用户列表增加全量并发下限调整能力，使用原子更新保证已有更高并发不被降低，并补齐前后端测试。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `3bbbce55f` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 3: 修复余额校准账期与范围消费校准

**Date**: 2026-07-24
**Task**: 修复余额校准账期与范围消费校准
**Branch**: `fix/balance-calibration-time`

### Summary

余额校准按 Token 日期分摊；新增所选范围消费增减量和目标值校准，并同步反向调整用户余额。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `1f4fc659d` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: 账号管理当天缓存命中展示

**Date**: 2026-08-26
**Task**: 账号管理当天缓存命中展示
**Branch**: `custom/main`

### Summary

扩展账号今日统计返回输入与缓存拆分 Token，在账号管理默认显示缓存命中 Token 和命中率；增加前后端定向测试并合并到 custom/main。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `636495b9b` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: 同步最新上游 main

**Date**: 2026-08-26
**Task**: 同步最新上游 main
**Branch**: `sync/upstream-20260826`

### Summary

在隔离分支 sync/upstream-20260826 合入 upstream/main@6ca1e15b0ad2；custom/main 保持不变，完成定向门禁与自动合并复核。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `c08b38738` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 6: 同步官方 v0.2.16 并修复完整检查失败

**Date**: 2026-10-14
**Task**: 同步官方 v0.2.16 并修复完整检查失败
**Branch**: `custom/main`

### Summary

保留二开并整合固定官方 v0.2.16，修复既有前后端测试、支付履约、倍率缓存内存泄漏、集成夹具隔离及 lint 问题。前端 405 文件/3144 测试通过，后端 unit 57 包、integration 51 包通过，lint 0 issues，普通与 embed 构建通过。本地 custom/main 已快进，当前任务已归档；未推送、未部署，7 项原有未跟踪路径保留。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `8983b05ff58c71a73beb405f9034158b8f7e9039` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 7: OVH 部署二开 v0.2.16

**Date**: 2026-10-14
**Task**: OVH 部署二开 v0.2.16
**Branch**: `feature/deploy-ovh-v0-2-16`

### Summary

将本地 custom/main 的 v0.2.16 部署到 ovh，镜像 sub2api-custom:8fec104efb1f。346 个迁移校验、容器 health 和公网核心资源通过；保留数据库及 override 备份；其他容器和配置不变。仅本地构建，不推送 Git。验收：.trellis/tasks/archive/2026-10/10-14-deploy-ovh-v0-2-16/validation.md。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `8fec104efb1fac129ee0665da6bae866baa80de1` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: 智力雷达接入现行 RadarBench 并部署 OVH

**Date**: 2026-10-14
**Task**: 智力雷达接入现行 RadarBench 并部署 OVH
**Branch**: `custom/main`

### Summary

按公开 binding 动态展示所有有成绩的模型和档位，保留旧型号及 0 分，区分 UTC 判分日与读取时间。后端 18 个定向回归、前端 49 个相关测试、lint/typecheck/构建与真实源回放通过。发布镜像 sub2api-custom:eb5bff47e499，healthy；9 个公网资源和实际生产雷达 UI 验证通过，可见 10 月 14 日成绩，其余 7 个容器和配置不变，无新增迁移。本地 custom/main 已快进，不推送 Git；验收 .trellis/tasks/archive/2026-10/10-14-radar-stale-data/validation.md。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `eb5bff47e4990083a0dc785a9690c6169f03c600` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: 全平台 System Prompt 注入修复与 OVH 部署

**Date**: 2026-10-17
**Task**: 全平台 System Prompt 注入修复与 OVH 部署
**Branch**: `custom/main`

### Summary

开放 DeepSeek 等全部登记平台与组合分组，补齐 Gemini/System One/Grok/Antigravity 独立转发和计数。定向测试、数据库集成、前端和新增 lint 检查通过；已合入 custom/main 并本机构建部署 OVH，公网产物与镜像一致，保留回滚备份。

### Main Changes

- Detailed change bullets were not supplied; see the summary above.

### Git Commits

| Hash | Message |
|------|---------|
| `10f2f9a00dca` | (see git log) |

### Testing

- Validation was not recorded for this session.

### Status

[OK] **Completed**

### Next Steps

- None - task complete

---

## Session 10: 糖果问题探针与智商监控

**Date**: 2026-10-26
**Task**: 10-26-model-iq-probe
**Branch**: `feature/model-iq-probe`（基线 custom/main）

### Summary

新增独立子系统"智商监控"：以糖果问题（标准答案 21）为默认探针题，按账号×模型或仅按模型主动探测 OpenAI 账号上的 GPT 模型是否降智。用户端渠道状态页（V1）模型可用性下方新增糖果探针区块（按模型汇总 + 最近 10 次圆点时间线，零账号信息）；管理端新增与渠道监控同级的智商监控页面（配置题目/期望答案/目标/间隔 + 结果明细/过滤/改判/立即探测）。判分"出现即通过"与 challenge 同语义；结果存 `model_iq_probe_results`（迁移 249）保留 30 天；leader lock 防多实例并发。

### Main Changes

- 后端：`service/model_iq_probe_{service,runner}.go`、`repository/model_iq_probe_repo.go`、`handler/admin/model_iq_probe_handler.go`、用户端 `handler/model_iq_probe_user_handler.go`、ent schema + 迁移 249、settings key `model_iq_probe_config`、OpsCleanupService 清理挂钩、wire 全套注册。
- 前端：`api/modelIqProbe.ts`、`api/admin/modelIqProbe.ts`、`components/user/monitor/CandyProbePanel.vue`、`views/admin/ModelIqMonitorView.vue`、路由 `/admin/channels/iq-monitor`、侧边栏渠道管理分组新条目、zh/en i18n（nav/custom overlay/admin）。
- spec：新增 `.trellis/spec/backend/model-iq-probe.md`。

### Testing

- 后端：判分/配置/汇总单测 + repo 集成测试（testcontainers）通过；`go build ./...`、handler/server/routes/cmd 测试通过。service 包存量失败 `TestOpenAICodexTicketGate_CompactRequestUsesForwardOutboundModel` 在 clean custom/main 上同样失败，与本任务无关。
- 前端：新增 10 个测试；全量 412 文件 / 3235 测试通过；`pnpm typecheck`、`pnpm check:i18n` 通过。

### Status

[IN PROGRESS] **待提交**（三段式提交待操作者确认；运行环境人工验收"立即探测/定时执行"待做）

### Next Steps

- 操作者确认后执行三段式提交（代码 → spec → 任务归档/journal）。
- 部署测试环境人工验收：保存配置 → 立即探测 → 结果出现；定时轮次执行。
