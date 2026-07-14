# 同步官方 v0.1.151 到二开主线

## Goal

在不丢失 `custom/main` 下游业务定制的前提下，将官方稳定标签 `v0.1.151` 的增量安全合入版本化同步分支，完成范围匹配的验证，为后续合回 `custom/main` 提供可审计结果。

## Known Baseline

- 当前基线为干净的 `custom/main`，版本文件仍为 `0.1.150`。
- `custom/main` 已包含 v0.1.151 中的 setup-token 自动刷新、Codex `image_gen` 命名空间剥离、GPT-5.6 计费与用量修复。
- 尚缺 OpenAI Fast/Flex 用户级规则、Grok Responses reasoning effort 保留、Codex `originator` / `User-Agent` 身份配对修复。
- `git merge-tree` 预检显示可以自动合并；重叠范围集中在设置 DTO/视图、账号测试与用量探针。
- 官方标签对象中的 `VERSION` 仍是 `0.1.150`，发布后紧随标签的官方回写提交才更新为 `0.1.151`；下游 Docker 构建排除 `.git` 且未传 `VERSION` build arg，因此同步时必须显式带上该发布元数据。

## Requirements

- 基于 `custom/main` 创建 `sync/upstream-v0.1.151`，不直接改动 `main`。
- 合入官方签名标签 `v0.1.151`，保留已有二开能力与模块化 i18n 结构。
- 不重复手写或 cherry-pick 已包含的三个修复，以 Git 合并结果为准。
- 核对版本号、Fast/Flex 用户规则端到端字段、Codex 身份头覆盖路径、Grok reasoning effort 行为及迁移序列。
- 执行与 30 个剩余增量文件匹配的后端测试、迁移回归、前端类型和组件测试。
- 不执行 `git commit`、`git push`、生产部署或生产数据库迁移，除非用户另行明确确认。

## Acceptance Criteria

- [x] 当前分支为 `sync/upstream-v0.1.151`，base branch 记录为 `custom/main`。
- [x] 分支内容包含官方 `v0.1.151`，并纳入官方发布后的 `VERSION=0.1.151` 回写，且无未解决冲突。
- [x] 下游业务代码和 locale overlay 结构未被意外覆盖。
- [x] Fast/Flex 用户级策略的后端 DTO、可信用户上下文、策略优先级和前端编辑字段完整对齐。
- [x] Codex 身份配对覆盖普通转发、透传、WebSocket、账号测试和用量探针。
- [x] Grok Responses 兼容 `reasoning_effort`，相关测试通过。
- [x] 范围匹配的后端、迁移和前端检查通过，或清楚记录无法通过的外部原因。
- [x] 工作区只包含本次同步和 Trellis 记录，不提交、不推送、不部署。

## Out of Scope

- 同步 `upstream/main` 中超出 `v0.1.151` 的后续提交。
- 改造上游新功能或调整产品行为。
- 合回 `custom/main`、发布镜像或更新生产环境。
