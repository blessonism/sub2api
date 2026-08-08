# 实施计划

1. 更新策略领域模型、输入校验和档位选择函数，加入 `condition_mode`、实际消费额和变更审计字段；补充 token-only、actual-cost-only、both 及边界测试。
2. 新增数据库迁移并同步 repository 的 policy/tier/assignment/run-change 读写和窗口聚合 SQL；补充 SQL mock 或集成覆盖聚合字段、过滤条件和新列扫描。
3. 更新管理员 API 类型与 Token 自动策略页面：模式选择、实际消费额输入、条件校验、档位摘要、预览/历史展示及中英文文案。
4. 运行后端定向测试、前端类型检查和相关 lint；检查迁移顺序、未提交无关改动和下游分支约束。

## 验证命令

- `cd backend && go test ./internal/service ./internal/repository`
- `cd frontend && pnpm typecheck`
- `git diff --check`

## 风险与回滚点

- repository SQL 的 SELECT/SCAN 或 INSERT 参数顺序不一致会导致运行时扫描失败，修改后必须运行对应测试。
- 混合指标若继续按 Token 自动排序会改变命中优先级，因此 UI 和 normalize 必须保留档位顺序。
- 旧策略必须默认 `condition_mode=token`，否则会出现历史策略无条件命中的行为变化。
