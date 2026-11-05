# 保留本地二开并同步 main 最新版本

## 目标
在保留当前本地二开和未推送提交的基础上，接入已确认的官方 upstream/main 最新提交 8e92c892b47f。

## 已确认事实
- 用户已批准任务与规划，并明确选择同步官方 upstream/main 及开始实施。
- 起点为 custom/main ae2e47d03，工作区干净，相对 origin/custom/main 领先 5 个提交。
- origin 指向 blessonism/sub2api；upstream 指向 Wei-Shaw/sub2api，push 已禁用。
- 2026-11-05 已使用 --no-tags 分别 fetch 两个 remote。
- origin/main = 4a5665da5b2c6b83c4597844ea6e573746c821b1，提交日期为 2026-06-16，已完整包含于 custom/main。
- upstream/main = 8e92c892b47f3f934f18ce63da4c4dcb75f0b6bd，提交日期为 2026-11-05，相对 origin/main 领先 3920 个提交。
- rerere.enabled=true，rerere.autoupdate=false。

## 要求
- R1：保留本地全部二开提交与当前功能；已经移除的对话历史模块不得重新引入。
- R2：使用 sync/upstream-* 分支完成预检、合并和定向验证。
- R3：预检与合并锁定同一个目标 SHA；保留可追溯的检查记录。
- R4：提交与推送须遵守项目明确确认要求，生产部署不在本次范围。

## 验收标准
- AC1（R1）：原始 ae2e47d03 是同步结果的祖先；直接重叠的二开能力完成针对性复核。
- AC2（R2、R3）：确定的目标 SHA 完整包含于同步结果，冲突已解决且 git diff --check 通过。
- AC3（R1、R3）：同步检查矩阵所选检查通过；环境阻塞项清楚记录。
- AC4（R4）：提交、推送和主线合入的实际状态在收尾明确说明。

## 范围外
生产部署、数据库迁移执行、无关功能开发、重写共享历史。
