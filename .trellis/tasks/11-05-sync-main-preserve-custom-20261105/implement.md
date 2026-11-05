# 执行计划

## 顺序
1. [x] 读取 fork 工作流；确认工作区干净、5 个本地提交及 rerere 配置。
2. [x] 分别无 tags fetch；解析目标 SHA 并完成只读 preflight。
3. [x] 用户审阅规划并决定是字面 origin/main 还是推荐的官方 upstream/main；未确认前不合并代码。
4. [x] 读取 trellis-before-dev 与受影响层规范；确认任务上下文并执行 task.py start。
5. [x] 从 ae2e47d03937 创建 sync/upstream-20261105；锁定获准 SHA，git merge --no-commit --no-ff <SHA>。
6. [x] 逐文件解决冲突，复核 rerere 建议；保留二开功能与已移除模块状态，必要时重新生成 Ent/Wire。
7. [x] 生成并运行检查矩阵，补充最近本地二开行为检查；读 trellis-check 并完成全范围复核。
8. [x] 更新记录、完成 git diff --check、检查冲突标记及暂存 diff，形成可审查结果。
9. [x] 已获用户明确确认，仅执行同步合并、任务归档和会话记录共 3 个本地提交；不执行 push/PR。

## 验证命令
- python3 tools/upstream_sync.py preflight --base ae2e47d03937 --upstream 8e92c892b47f3f934f18ce63da4c4dcb75f0b6bd --format json
- git diff --check
- git diff --name-only --diff-filter=U
- 按 CLI 对未提交 merge 的支持情况，锁定完整合并 tree 或提交生成 check 计划；不以 HEAD 代替尚未提交的合并结果。
- python3 tools/upstream_sync.py check --base ae2e47d03937 --head <合并结果> --dry-run --format json
- python3 tools/upstream_sync.py check --base ae2e47d03937 --head <合并结果> --jobs 3
- 验证最终结果包含 ae2e47d03937 与获准目标两个祖先。

## 风险与回滚点
重叠 128 条路径，主要涉及共享网关、路由、DTO、Ent、配置、i18n 与页面。未提交 merge 可 abort；提交后使用 revert -m 1，不重置共享主线。环境不足导致 integration 不能执行时保留记录且不宣称合入门禁已通过。

## 开始前复核
- [x] prd/design/implement 已落盘。
- [x] base branch 为 custom/main，工作分支为 sync/upstream-20261105。
- [x] implement.jsonl/check.jsonl 均包含 fork 工作流。
- [x] 用户确认目标与实施；PRD 收敛并再次通读。
