# 同步设计

## 分支与输入边界
- 保留基线：custom/main ae2e47d03937，包含 5 个尚未推送的本地提交。
- 用户字面指定 origin/main（4a5665da5b2c）已经是保留基线的祖先，因此无需再次合并。
- 用户已确认目标：官方 upstream/main 8e92c892b47f3f934f18ce63da4c4dcb75f0b6bd（2026-11-05 fetch 结果）。
- 官方目标相对共同祖先 3f1c8f36ce8c 新增 110 个提交、修改 468 条路径；二开侧 346 个提交、1880 条路径，直接重叠 128 条。重叠是风险上界，不是最终冲突数。
- 实施只在 sync/upstream-20261105 进行；custom/main 和镜像 main 保持原提交直到后续合入获准。

## 兼容与复核
使用正常 merge 保留双方历史，不使用 reset、rebase 或整文件偏向策略。逐项核对重叠文件的二开生产者、消费端和行为断言。重点保留 Langfuse 观测与正文采集、对话历史模块删除状态、分组系统提示词、按模型路由、分组调度、排行榜、倍率策略、监控和活动能力。Ent 和 Wire 生成代码遵循合并后的 schema/provider 重新生成，避免手工遗失字段。

## 验证策略
优先运行 tools/upstream_sync_checks.json 选中的 frontend/backend/integration lane，另对本地最近 5 个提交涉及的观测层、模块删除执行定向检查。必须区分真实 integration 环境不可用与断言失败，不能以 mock 替代。按仓库 CI 定义执行适当完整验证，避免重复相同检查。

## 提交、共享与回滚
- 合并时使用 --no-commit，检查完成后才请求本地 commit 的明确确认；不隐式推送。
- merge 未提交时可 git merge --abort；保留基线不受影响。
- commit 获准后保留同步分支；push 与以 custom/main 为目标的 PR 另行遵守明确确认及 CI 门禁。
- 本次不执行生产部署、数据库迁移、main 强制更新或共享历史改写。
