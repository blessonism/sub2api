# 智商探测 SWE 徽章跳转

## Goal

将用户侧 GPT 智商探测面板中的 DeepSWE 徽章点击目标改为 DeepSWE 项目主页，让用户能从面板直接查看基准题目来源。

## Requirements

- 顶部 DeepSWE 徽章点击后打开 `https://github.com/datacurve-ai/deep-swe`。
- 保持新标签页打开和安全 `rel` 属性。
- 不改变智商探测数据源、图表展示、状态逻辑或现有文案。

## Acceptance Criteria

- [ ] `GptIntelligencePanel` 中 DeepSWE 徽章的 `href` 指向 deep-swe GitHub 项目。
- [ ] 现有快照渲染测试覆盖该跳转目标。
- [ ] 相关前端测试通过。

## Definition of Done

- 代码改动范围聚焦在用户侧智商探测面板。
- 测试已按改动风险补充或更新。
- 遵守下游 fork 工作流，任务上下文包含下游分支约束。

## Technical Approach

在 `frontend/src/components/user/monitor/GptIntelligencePanel.vue` 中替换 DeepSWE 徽章的静态链接，并在 `frontend/src/components/user/__tests__/GptIntelligencePanel.spec.ts` 的现有渲染用例里断言链接地址。

## Decision (ADR-lite)

**Context**: 当前徽章文字是 DeepSWE 探针，但链接指向 `codexradar.com/current.json`，用户想点击查看 DeepSWE 项目。

**Decision**: 只替换徽章跳转地址为用户指定的 GitHub 仓库地址。

**Consequences**: 用户可直接访问项目说明；原 JSON 数据源如果仍由其他逻辑使用，本任务不改变其获取方式。

## Out of Scope

- 不新增额外按钮或说明文本。
- 不调整智商探测 API、数据刷新或图表逻辑。
- 不处理当前工作区其他未提交的活动奖励 UI 改动。

## Technical Notes

- 已读取 `.trellis/spec/guides/downstream-fork-workflow.md`。
- 前端 spec 索引 `.trellis/spec/frontend/index.md` 未提供额外 Pre-Development Checklist。
- Semble MCP / CLI 未在当前会话可用，已按降级规则使用 `rg` 定位代码。
