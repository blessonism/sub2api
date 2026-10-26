# Model IQ Probe（智商监控 / 糖果问题探针）契约

> 状态：Active。本 spec 描述下游二开新增的"智商探针"子系统的对外契约与不变量，
> 与渠道监控（channel monitor）相互独立，仅共用判分语义与清理调度。

## 定位

- 用固定题目（默认糖果问题，标准答案 **21**）探测 OpenAI 账号上的 GPT 模型是否降智。
- 配置存 settings key `model_iq_probe_config`（JSON），默认关闭；改配置不需要重启。
- 结果表 `model_iq_probe_results`（迁移 249），保留 30 天，由 OpsCleanupService 每日物理删。

## 判分语义（不可变更）

`ValidateModelIQAnswer`："出现即通过"——
- 期望答案可解析为整数：提取响应中 `-?\d+` 独立整数 token，出现即 pass（与 challenge 判分同款）；
- 否则大小写不敏感包含匹配。
- 已知宽松边界（产品选定）：回答含"不是 21 而是 20"会被判 pass，由管理端人工改判兜底。
- 生效状态 = `override_status ?? status`；人工改判是唯一纠偏通道。

## 不变量

- **用户端零账号信息**：`GET /api/v1/channel-monitors/iq-probe` 的响应只含 model/判定/时间/回答摘要，绝不输出 account_id / account_name。
- **回答原文截断**：actual_answer ≤ 4KB、error_message ≤ 2KB、用户端摘要 ≤ 120 字符。
- **多实例互斥**：调度轮次与手动 RunOnce 都走 leader lock（`model_iq_probe:runner:leader`）；
  进程内由 service 的 atomic 防重入。
- **探测通道**：`AccountTestService.TestAccountConnectionWithAccount` + 内存 SSE（复刻
  `RunAccountTestBackground`），因此 token 用量不可得——token 三列恒 NULL，切换执行通道前不得启用。
- **模型自动发现**：配置 models 为空时并发（≤3）拉取各账号上游模型目录，取 `gpt-*`
  前缀（排除 `gpt-image-*`）并集；单账号失败只记日志。
- **探针未启用或无数据时用户端返回空列表**，前端区块不渲染。

## 扩展点

- 新增判定规则：只改 `ValidateModelIQAnswer`，配套更新
  `model_iq_probe_service_test.go` 的边界用例（121 / "不是21而是20" / 多位数不误匹配）。
- 新增探针题：管理端配置 question + expected_answer 即可，无需改代码。
- 目标模式扩展：`target_mode` 枚举加值需同步 handler oneof 校验与前端选项。

## 已知偏离

- `wire_gen_test.go` 的 `provideCleanup` 调用需与新 runner 参数保持同步（wire 再生成时人工核对）。
- `internal/service` 包存量失败 `TestOpenAICodexTicketGate_CompactRequestUsesForwardOutboundModel`
  与本子系统无关（clean custom/main 同样失败）。
