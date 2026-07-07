# sync upstream updates preserve custom main

## Goal

把官方 `Wei-Shaw/sub2api` 的最新 `upstream/main` 更新安全合入下游二开主线，同时保留本地 `custom/main` 上已经做出的业务定制，避免污染上游镜像分支或丢失二开提交。

## What I already know

- 当前分支是 `custom/main`，工作区在创建本任务前是干净的。
- `origin` 指向二开仓库 `https://github.com/blessonism/sub2api.git`。
- `upstream` 指向官方仓库 `https://github.com/Wei-Shaw/sub2api.git`，push 地址禁用。
- 普通 `git fetch --all --tags --prune` 因本地 `v0.1.145` tag 与 upstream 同名 tag 冲突失败。
- 使用 `git fetch --no-tags upstream '+refs/heads/*:refs/remotes/upstream/*'` 已成功刷新 upstream 分支引用。
- `origin/main` / 本地 `main` 相对 `upstream/main` 落后 398 个提交。
- `custom/main` 相对 `upstream/main` 有 152 个下游提交，同时落后 398 个上游提交。
- `git merge-tree --write-tree custom/main upstream/main` 预检显示存在内容冲突，重叠文件约 100 个。

## Assumptions

- 不直接在 `main` 上做二开改动；`main` 继续作为接近官方主线的镜像分支。
- 不直接推送、提交或改生产环境，除非用户明确确认。
- 先在 `sync/upstream-2026-07-07` 同步分支处理冲突，验证通过后再考虑合回 `custom/main`。
- 冲突解决时优先保留下游业务定制，同时吸收上游 bugfix、安全修复、协议兼容和依赖更新。

## Requirements

- 保留 `custom/main` 上的下游提交和业务功能。
- 合入 `upstream/main` 的官方更新。
- 避免把同步冲突直接留在 `custom/main`。
- 记录 tag 冲突和后续处理建议。
- 冲突解决后运行与 backend/frontend 变更范围匹配的验证。

## Acceptance Criteria

- [ ] 同步工作在 `sync/upstream-2026-07-07` 或等价安全分支完成。
- [ ] `custom/main` 的下游提交不会被 rebase、reset 或覆盖。
- [ ] 所有 merge conflict 已解决，工作区无冲突标记。
- [ ] backend/frontend 至少完成范围匹配的构建或测试检查。
- [ ] 最终给出合回 `custom/main`、处理 tag 冲突、是否推送的明确建议。

## Out of Scope

- 不执行 `git push`。
- 不执行 `git commit`，除非用户单独确认。
- 不执行生产部署、数据库迁移或容器重建。
- 不强制覆盖本地冲突 tag。

## Technical Notes

- 下游 fork 工作流要求官方更新可使用 `sync/upstream-<version-or-date>` 分支。
- 当前 merge-tree 预检已经发现 `.gitignore`、backend Ent 生成代码、handler/service、frontend usage/admin/user 页面等多个区域存在冲突。
- 后续需要先创建/切换同步分支，再执行真实 merge 并逐项解决冲突。
