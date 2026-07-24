# 执行计划

## Checklist

1. **Repo 消费路径**  
   - 文件：`backend/internal/repository/admin_usage_calibration_repo.go`  
   - 去掉消费分支里 `originalTokens == 0` 的拒绝。  
   - 新增 `allocateBalanceDeltaWithoutTokens(startDate string, walletDelta float64) []allocationPlanRow`（或等价内联）。  
   - 有 Token 时仍走 `allocateBalanceDelta`。

2. **单测**  
   - 文件：`backend/internal/repository/admin_usage_calibration_repo_test.go`  
   - 新增：无 Token + consumption target/delta 成功，断言单行分摊 `start_date`、`balance_delta`、余额更新。  
   - 保留/确认：有 Token 消费路径、负余额、Token 无用量拒绝仍存在。

3. **前端错误透出**  
   - 文件：`frontend/src/views/user/UsageView.vue`  
   - `submitCalibration` catch 使用 `extractApiErrorMessage`（或项目等价工具）。  
   - 可选：校准区增加一句无用量归属提示（i18n zh/en）。

4. **前端测试**  
   - 文件：`frontend/src/views/user/__tests__/UsageView.spec.ts`  
   - 模拟 API reject 带 message，断言 toast 内容为后端 message。

5. **验证命令**  
   ```bash
   cd backend && go test ./internal/repository/ -run AdminUsageCalibration -count=1
   cd backend && go test ./internal/service/ -run ConsumptionCalibration -count=1
   cd frontend && npm test -- --run src/views/user/__tests__/UsageView.spec.ts
   ```

## Review Gates

- [ ] 无 Token 消费成功且日分摊可被 `SumBalanceSpent` 按 `start_date` 计入  
- [ ] 有 Token 路径无回归  
- [ ] 前端展示真实错误 message  
- [ ] 负余额 / 负消费仍拒绝  

## Rollback

- 还原 repo 中 `originalTokens == 0` 拒绝与前端 catch 即可；无需 DB 回滚。
