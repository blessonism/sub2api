# 同步验证记录

## 输入与结果
- 日期：2026-11-05（Asia/Singapore）。
- 二开基线：`ae2e47d03937dd5f50aa81dfd4a4c78bdada683f`，保留其中 5 个未推送提交。
- 用户确认的官方目标：`8e92c892b47f3f934f18ce63da4c4dcb75f0b6bd`；新增 110 个官方提交，VERSION 为 `0.2.19`。
- 工作分支：`sync/upstream-20261105`；采用保留双方历史的 merge commit，两个输入分别为上述二开基线与官方目标。
- 本地同步提交：`19d51cf3929a8f8bb4ba3905cb5b648c0bcd237b`；已核对第一、第二父提交依次为二开基线与官方目标，双方均为同步结果的祖先。
- 20 个冲突文件均已解决；未解决索引项为 0；`git diff --check` 与 `git diff --cached --check` 均通过。
- 已验证的代码/spec tree：`bcc80d13dbbfe0fbff6ec96895e97ef14a8fa40b`（不含随后落盘的任务记录）。

## 冲突解决与二开保留
- 认证缓存升级到 27，同时保留图片按张计费、分组提示词和按模型路由；回归覆盖合并字段 JSON 往返及旧 24/25/26 版拒绝。
- 账号受管 extra 的锁定行查询、scan 和 fixture 同时保留 Codex ticket 与 Grok 模型/能力 metadata。
- 保留用户分组账号绑定外层，核心调度使用上游图片尺寸分层与代理隔离回退。
- 保留分时计费选项、实际成本到 Langfuse 的采集链及 HTTP/WS 分组提示词。
- 采用上游对 Codex 透传缺省 instructions 的修复，不恢复其已删除的本地拦截。
- 账号创建表单保留本地 header/图片回填 UI，加入上游图片尺寸配置，去除合并造成的重复声明与重复开关。
- 用户侧 CSV 导出继续保持移除，保留错误请求页及其现有“不渲染导出按钮”的测试约束。
- 此前两次对话历史清理提交删除的 46 条路径全部仍不存在；退休路由回归通过。
- Ent 按合并后的 schema 重新生成，第二次生成无 diff；Wire 按合并后的 provider 重新生成。

## 验证
| 检查 | 结果 |
| --- | --- |
| `go test -tags=unit ./...` | 61 个有测试的包通过；补齐 4 处测试构造器参数后最终复验通过，其他包使用成功缓存 |
| `go test -tags=integration ./...` | 55 个有测试的包通过；本地 OrbStack Docker 可用，真实 PostgreSQL repository 套件执行成功（32.824s），非 skip 退出 |
| `go vet ./...` | 通过 |
| preflight 选中的 20 项检查 | 全部通过，详见 `research/check-results-summary.json`；前端 8 项由完整套件和 typecheck 覆盖，后端/integration 12 项由 CLI 执行 |
| `pnpm typecheck` | 通过 |
| `pnpm test:run` + 失败文件定向复验 | 428 个测试文件，合计 3465 例通过、4 例原有跳过；首次运行 427 文件通过，唯一旧模型默认值断言修为上游 gpt-5.6-sol 后该文件 18 例通过 |
| 80 个变更前端文件 `pnpm exec eslint ...` | 通过 |
| `pnpm exec vite build` | 通过，存在普通 chunk-size 提示 |
| Ent 第二次生成、冲突标记、diff whitespace | 通过 |

首次快速门禁有 4 项因冷构建超过 60 秒、1 项因构造器签名不匹配失败；只复跑这 5 项，全部通过。没有重复运行已通过的快速门禁。

## 本地收尾与边界
- 用户明确选择“仅完成本地提交，暂不推送”：本轮执行同步 merge、任务归档、journal 共 3 个本地提交。
- 未执行 push、未创建/合入 PR；本地 custom/main 仍在原基线。
- 共享合入须推送 origin 的同步分支，并通过以 `origin/custom/main` 为目标的 PR 与完整 CI。后续 GitHub PR/run 命令必须显式使用 `--repo blessonism/sub2api`（`repo view` 使用仓库位置参数），否则 fork 环境可能默认解析到官方仓库。
- 本机未安装 golangci-lint；本地 `go vet` 已通过，完整 golangci-lint 门禁由获准推送后的官方 CI 定义执行。
- 未部署生产、未执行生产迁移、未改写历史。

## 回滚
- 本轮提交保留原二开基线及其 5 个本地提交；`custom/main` 不随同步分支移动。
- 提交后或共享后：使用 `git revert -m 1 <merge-commit>` 或回退对应 PR；不强制重置共享主线。
