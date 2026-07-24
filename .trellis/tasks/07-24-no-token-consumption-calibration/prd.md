# 无原始Token时支持消费校准并透出真实错误

## Goal

允许管理员在所选日期范围**没有原始 Token 用量**时仍可提交消费校准（用于冲销「今日 0 Token / 有校准消费」这类历史余额校准挂账），并把后端真实失败原因展示在前端，避免一律显示「校准提交失败」。

## Background

- 消费校准当前强制 `originalTokens > 0`，否则返回 `ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE`。
- 旧余额校准（尤其无日分摊的 balance-only）会把负向 `balance_delta` 按 `created_at` 记入「今日消费」。
- 场景：今日 Token=0、今日消费=$70（来自校准），近 14 天真实用量仅 $20；在有 Token 的范围校准只能改 $20，平不掉今日 $70。
- 前端 `submitCalibration` 任意失败都 toast `adminCalibrationFailed`（「校准提交失败」），不展示后端 `message`/`reason`。

## Requirements

### R1 无原始 Token 的消费校准

- 当所选消费校准日期范围内 **原始 `usage_logs` Token 合计为 0** 时，**不再拒绝**消费校准。
- 消费差额仍由后端在事务内按「范围当前总消费」计算（原始 `actual_cost` + 已有校准消费差额），不信任前端预览。
- 消费增加 → 等额扣余额；消费减少 → 等额退余额。
- 校准后范围总消费 ≥ 0、用户余额 ≥ 0；否则整笔事务失败（现有错误码保留）。
- 差额为 0 时仍返回 `ADMIN_USAGE_CALIBRATION_NO_CHANGE`。
- Token 校准路径：范围无原始 Token 时**仍拒绝**按比例分摊（行为不变）；仅消费（及消费触发的钱包侧）可走无 Token 分支。
- 余额校准（非消费）路径不变。

### R2 无 Token 时的归属口径

- 无原始 Token 时，整笔消费对应的钱包差额写入**单日**分摊，归属日为消费校准的 **`start_date`**（管理员所选范围起始日）。
- 该日分摊行：`original_tokens = 0`，`token_delta = 0`，`balance_delta = walletDelta`（`walletDelta = -consumption_delta`）。
- 有原始 Token 时，继续按现有「原始 Token 占比 + 最大余数」按日分摊，行为不变。

### R3 前端错误透出

- 校准提交失败时，优先展示后端返回的可读 `message`（中文优先），而不是固定「校准提交失败」。
- 若无 message，再回退到 i18n `adminCalibrationFailed`。
- 可选：对 `ADMIN_USAGE_CALIBRATION_NO_ORIGINAL_USAGE` 等 reason 保留专用文案作为兜底（本任务主路径将不再因消费无 Token 触发该码）。

### R4 前端体验（轻量）

- 打开校准对话框时，若范围无原始 Token 且勾选消费，不阻断提交；可在 UI 上提示「无原始用量时，消费差额将整笔记入开始日」。
- 默认仍可保持 Token 校准勾选状态；若同时勾选 Token 且无原始用量，Token 部分仍失败——建议：仅 Token 失败时错误信息清晰；或提交时若无用量自动不附带 token（**本任务不做自动剥离**，由错误透出引导用户取消 Token 勾选）。

## Acceptance Criteria

- [x] 所选范围原始 Token=0 时，仅提交消费校准（delta 或 target）可成功，用户余额与范围消费按规则等额反向变化。
- [x] 无 Token 消费校准产生一条日分摊：`allocation_date = start_date`，`balance_delta = -consumption_delta`，`token_delta = 0`。
- [x] 校准后「按 start_date 归属」的消费统计包含该差额；无日分摊的旧逻辑不被重复计入。
- [x] 范围仍有原始 Token 时，消费/Token 分摊行为与改前一致。
- [x] 消费校准导致消费或余额为负时仍 400，前端展示后端 message。
- [x] API 失败时前端 toast 展示后端 message，不再只显示「校准提交失败」。
- [x] 仓储/服务单测覆盖：无 Token 消费 target/delta、负余额拒绝、有 Token 路径回归。
- [x] 前端单测或现有测试覆盖错误 message 透出（若项目已有 apiError 工具则复用）。

## Out of Scope

- 不支持「冲销指定历史校准单 ID」。
- 不改 Token 校准在无原始用量时的拒绝逻辑。
- 不改正向余额校准不计入消费的规则。
- 不做校准历史列表的撤销/回滚 UI。
- 不自动在提交时剥离 Token 字段。

## Notes

- 相关实现：`admin_usage_calibration_repo.go`、`admin_usage_calibration_service.go`、`UsageView.vue`、`apiError.ts`。
- 触发场景来自线上：今日 0 Token / 消费 $70 无法被近 14 天 $20 范围校准抹平。
