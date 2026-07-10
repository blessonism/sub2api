# 加速上游同步并固化回归门禁

## Goal

将下游仓库跟进 `upstream/main` 的本地关键路径从反复串行检查压缩为“只读预检、集中解冲突、按影响并行回归、CI 一次全量门禁”，同时避免 Dashboard 数据生产者和 Vue 动态插槽这类不会触发 Git 冲突的语义回归。

## Requirements

- 提供只读预检命令，输入下游基线与上游目标 ref，输出 merge-base、双方变更、直接重叠、重命名/删除/大幅重写等结构风险、受影响下游提交和应运行的检查。
- 预检不得执行 fetch、merge、checkout、reset、commit、push 或修改工作树。
- 提供结构化、可维护的“路径 -> 检查”矩阵，至少覆盖前端类型检查、用量页回归、Dashboard handler/view、Dashboard repository integration 和 i18n overlay。
- 提供并行检查入口；同一检查只执行一次，失败时返回非零，支持 dry-run/list 模式和按 lane 过滤。
- 为 `sync/upstream-*` 分支增加快速 CI 门禁并复用 Go/pnpm 缓存；现有 CI 继续承担一次完整检查，不在新 workflow 中重复全量套件。
- 更新下游 fork 工作流，明确预检、同步分支、rerere 复核、定向回归、完整 CI 和合回 `custom/main` 的顺序。
- 仓库级启用 `rerere.enabled=true`、`rerere.autoupdate=false`；复用历史解决方案但不自动暂存。

## Acceptance Criteria

- [ ] 本地 refs 上预检通常在 30 秒内完成，并能输出人类可读及 JSON 报告。
- [ ] 对历史同步 `2c503a333^1` 与 `2c503a333^2` dry-run 时，报告包含仓储和用户 UsageView 重叠风险。
- [ ] 历史 dry-run 自动选择 Dashboard integration 与用户 UsageView 回归检查。
- [ ] 检查器并行运行独立 lane，总耗时由最长 lane 主导，而不是所有命令串行相加。
- [ ] CLI 单元测试覆盖变更解析、风险识别、规则匹配、dry-run 和失败传播。
- [ ] CI、Makefile 入口和操作文档使用同一份检查矩阵，不复制命令清单。
- [ ] 不触碰生产环境，不自动 fetch/push，不修改 `main` 或 `custom/main`。

## Definition of Done

- 脚本测试、格式/静态检查和历史 dry-run 通过。
- sync workflow 语法与命令路径经过本地验证。
- `.trellis/spec/guides/downstream-fork-workflow.md` 记录新流程和回滚边界。
- 所有实现位于 `feature/upstream-sync-fast-path`，目标分支为 `custom/main`。

## Research References

- [`research/historical-sync-dry-run.md`](research/historical-sync-dry-run.md) — 历史拓扑、NUL-safe diff、拆分信号和能力选测下界。
- [`research/existing-sync-tooling.md`](research/existing-sync-tooling.md) — 现有 Makefile/CI/测试命令及避免重复全量检查的集成方式。

## Decision (ADR-lite)

**Context**: 仓库同时包含 Go、Vue、integration tests 和大量下游二开；全量检查可靠但慢，单纯依赖冲突列表又无法发现语义丢失。

**Decision**: 使用 Python 标准库实现单一 CLI，以 JSON 保存检查矩阵；本地执行 diff-aware 定向检查，GitHub Actions 在 sync 分支运行同一 CLI，现有 CI 保留一次全量检查。

**Consequences**: 无新增运行时依赖，规则可审查且命令单一来源；路径规则需要随新增二开能力维护，未知领域仍会回退到通用检查。

## Out of Scope

- 不自动执行真实 merge 或自动解决冲突。
- 不自动 fetch、不处理同名 tag 冲突。
- 不替代现有完整 CI、安全扫描或发布流程。
- 不承诺大型结构重构的人工冲突处理时间，但避免重复串行检查。

## Technical Notes

- `upstream` push 地址必须继续保持禁用。
- 当前主工作树存在无关 Trellis/Agent 改动，本任务使用独立 worktree。
- 用户已明确确认仓库级 rerere 配置。
